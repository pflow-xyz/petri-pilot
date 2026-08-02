package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/pflow-xyz/go-pflow/verify"
)

// mutexModelJSON is the two-client mutual-exclusion net: one semaphore token
// gates both critical sections.
const mutexModelJSON = `{
  "name":"mutex",
  "places":[
    {"id":"idle1","initial":1},{"id":"busy1"},
    {"id":"idle2","initial":1},{"id":"busy2"},
    {"id":"sem","initial":1}],
  "transitions":[
    {"id":"acquire1"},{"id":"release1"},
    {"id":"acquire2"},{"id":"release2"}],
  "arcs":[
    {"from":"idle1","to":"acquire1"},{"from":"sem","to":"acquire1"},{"from":"acquire1","to":"busy1"},
    {"from":"busy1","to":"release1"},{"from":"release1","to":"idle1"},{"from":"release1","to":"sem"},
    {"from":"idle2","to":"acquire2"},{"from":"sem","to":"acquire2"},{"from":"acquire2","to":"busy2"},
    {"from":"busy2","to":"release2"},{"from":"release2","to":"idle2"},{"from":"release2","to":"sem"}]
}`

// brokenMutexModelJSON drops the semaphore from client 2, so both clients can
// enter their critical section at once.
const brokenMutexModelJSON = `{
  "name":"broken-mutex",
  "places":[
    {"id":"idle1","initial":1},{"id":"busy1"},
    {"id":"idle2","initial":1},{"id":"busy2"},
    {"id":"sem","initial":1}],
  "transitions":[
    {"id":"acquire1"},{"id":"release1"},
    {"id":"acquire2"},{"id":"release2"}],
  "arcs":[
    {"from":"idle1","to":"acquire1"},{"from":"sem","to":"acquire1"},{"from":"acquire1","to":"busy1"},
    {"from":"busy1","to":"release1"},{"from":"release1","to":"idle1"},{"from":"release1","to":"sem"},
    {"from":"idle2","to":"acquire2"},{"from":"acquire2","to":"busy2"},
    {"from":"busy2","to":"release2"},{"from":"release2","to":"idle2"}]
}`

func callVerify(t *testing.T, model, properties string) *verifyOutput {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_verify"
	req.Params.Arguments = map[string]any{
		"model":      model,
		"properties": properties,
	}

	result, err := handleVerify(context.Background(), req)
	if err != nil {
		t.Fatalf("handleVerify error: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned an error: %s", resultText(result))
	}

	var out verifyOutput
	if err := json.Unmarshal([]byte(resultText(result)), &out); err != nil {
		t.Fatalf("unmarshal result: %v\nraw: %s", err, resultText(result))
	}
	return &out
}

type verifyOutput struct {
	Verdicts []struct {
		Property struct {
			Kind string `json:"kind"`
			Name string `json:"name"`
		} `json:"property"`
		Status         string `json:"status"`
		Method         string `json:"method"`
		Detail         string `json:"detail"`
		Evidence       string `json:"evidence"`
		Counterexample *struct {
			Trace       []string       `json:"trace"`
			Marking     map[string]int `json:"marking"`
			Explanation string         `json:"explanation"`
		} `json:"counterexample"`
	} `json:"verdicts"`
	Proved  int    `json:"proved"`
	Refuted int    `json:"refuted"`
	Unknown int    `json:"unknown"`
	OK      bool   `json:"ok"`
	Summary string `json:"summary"`
}

func resultText(result *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func TestVerifyProvesMutualExclusion(t *testing.T) {
	out := callVerify(t, mutexModelJSON, `["deadlock-free","bounded","mutex:busy1,busy2"]`)

	if !out.OK {
		t.Fatalf("expected all properties proved, got %s", out.Summary)
	}
	if out.Proved != 3 {
		t.Errorf("proved = %d, want 3", out.Proved)
	}

	// The mutex property should come back as a structural proof, not an
	// enumeration — that is what makes it hold for any number of clients.
	for _, v := range out.Verdicts {
		if strings.HasPrefix(v.Property.Name, "mutex:") {
			if v.Method != string(verify.MethodStructural) {
				t.Errorf("mutex method = %q, want structural", v.Method)
			}
			if !strings.Contains(v.Evidence, "sem") {
				t.Errorf("evidence %q should cite the semaphore invariant", v.Evidence)
			}
		}
	}
}

func TestVerifyRefutesBrokenMutexWithTrace(t *testing.T) {
	out := callVerify(t, brokenMutexModelJSON, `["mutex:busy1,busy2"]`)

	if out.OK {
		t.Fatal("broken mutex should not pass")
	}
	if out.Refuted != 1 {
		t.Fatalf("refuted = %d, want 1", out.Refuted)
	}

	ce := out.Verdicts[0].Counterexample
	if ce == nil {
		t.Fatal("refutation must include a counterexample")
	}
	if len(ce.Trace) == 0 {
		t.Error("counterexample must include a replayable trace")
	}
	if ce.Marking["busy1"] != 1 || ce.Marking["busy2"] != 1 {
		t.Errorf("counterexample marking = %v, want both clients busy", ce.Marking)
	}
}

func TestVerifyInvariantExpression(t *testing.T) {
	out := callVerify(t, mutexModelJSON, `["busy1 + busy2 + sem == 1"]`)

	if !out.OK {
		t.Fatalf("expected the invariant to be proved: %s", out.Verdicts[0].Detail)
	}
	if out.Verdicts[0].Method != string(verify.MethodStructural) {
		t.Errorf("method = %q, want structural", out.Verdicts[0].Method)
	}
}

func TestVerifyObjectForm(t *testing.T) {
	props := `[
      {"kind":"mutual-exclusion","name":"one at a time","places":["busy1","busy2"],"bound":1},
      {"kind":"unreachable","name":"no double entry","target":{"busy1":1,"busy2":1}}
    ]`

	out := callVerify(t, mutexModelJSON, props)

	if !out.OK {
		t.Fatalf("expected both properties proved, got %s", out.Summary)
	}
	if out.Verdicts[0].Property.Name != "one at a time" {
		t.Errorf("name = %q, want the supplied label", out.Verdicts[0].Property.Name)
	}
}

// TestVerifyUnknownIsNotOK pins the honesty rule end to end: an undecidable
// property must keep OK false rather than reading as a pass.
func TestVerifyUnknownIsNotOK(t *testing.T) {
	out := callVerify(t, mutexModelJSON, `["nosuchplace + busy1 <= 1"]`)

	if out.OK {
		t.Error("OK = true despite an undecided property")
	}
	if out.Unknown != 1 {
		t.Errorf("unknown = %d, want 1", out.Unknown)
	}
	if !strings.Contains(out.Verdicts[0].Detail, "nosuchplace") {
		t.Errorf("detail %q should name the unknown place", out.Verdicts[0].Detail)
	}
}

func TestVerifyErrors(t *testing.T) {
	tests := []struct {
		name  string
		args  map[string]any
		match string
	}{
		{"missing model", map[string]any{"properties": `["bounded"]`}, "model"},
		{"missing properties", map[string]any{"model": mutexModelJSON}, "properties"},
		{"bad model", map[string]any{"model": "not json", "properties": `["bounded"]`}, "model"},
		{"properties not an array", map[string]any{"model": mutexModelJSON, "properties": `"bounded"`}, "array"},
		{"empty properties", map[string]any{"model": mutexModelJSON, "properties": `[]`}, "no properties"},
		{"bad property", map[string]any{"model": mutexModelJSON, "properties": `["not a property"]`}, "propert"},
		{"bad max_states", map[string]any{"model": mutexModelJSON, "properties": `["bounded"]`, "max_states": "zero"}, "max_states"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{}
			req.Params.Name = "petri_verify"
			req.Params.Arguments = tt.args

			result, err := handleVerify(context.Background(), req)
			if err != nil {
				t.Fatalf("unexpected transport error: %v", err)
			}
			if !result.IsError {
				t.Fatalf("expected an error result, got: %s", resultText(result))
			}
			if !strings.Contains(strings.ToLower(resultText(result)), strings.ToLower(tt.match)) {
				t.Errorf("error %q should mention %q", resultText(result), tt.match)
			}
		})
	}
}

func TestParsePropertiesShorthandKinds(t *testing.T) {
	props, err := parseProperties(`["deadlock-free","bounded","live","terminating","conserves",
		"reachable:done=1","unreachable:bad=1","mutex:a,b","a + b == 1"]`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	want := []verify.Kind{
		verify.KindDeadlockFree, verify.KindBounded, verify.KindLive,
		verify.KindTerminating, verify.KindConserves,
		verify.KindReachable, verify.KindUnreachable,
		verify.KindMutualExclusion, verify.KindInvariant,
	}
	if len(props) != len(want) {
		t.Fatalf("got %d properties, want %d", len(props), len(want))
	}
	for i := range want {
		if props[i].Kind != want[i] {
			t.Errorf("property %d kind = %q, want %q", i, props[i].Kind, want[i])
		}
	}
}

func TestParsePropertiesObjectValidation(t *testing.T) {
	for _, src := range []string{
		`[{"kind":"invariant"}]`,               // missing expr
		`[{"kind":"reachable"}]`,               // missing target
		`[{"kind":"mutual-exclusion"}]`,        // missing places
		`[{"kind":"nonsense"}]`,                // unknown kind
		`[{"name":"no kind"}]`,                 // missing kind
		`[{"kind":"invariant","expr":"a +="}]`, // unparseable expr
	} {
		t.Run(src, func(t *testing.T) {
			if props, err := parseProperties(src); err == nil {
				t.Errorf("parseProperties(%s) = %+v, want error", src, props)
			}
		})
	}
}

// TestAnalyzeSurfacesInvariants checks petri_analyze hands back the net's
// conservation laws. These are the one part of the analysis that stays valid
// when the state space is too big to explore, so an agent needs to see them.
func TestAnalyzeSurfacesInvariants(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_analyze"
	req.Params.Arguments = map[string]any{"model": mutexModelJSON}

	result, err := handleAnalyze(context.Background(), req)
	if err != nil {
		t.Fatalf("handleAnalyze error: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool error: %s", resultText(result))
	}

	var out struct {
		PInvariants []string `json:"p_invariants"`
		TInvariants []string `json:"t_invariants"`
	}
	if err := json.Unmarshal([]byte(resultText(result)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	want := "busy1 + busy2 + sem == 1" // the mutual-exclusion law
	found := false
	for _, inv := range out.PInvariants {
		if inv == want {
			found = true
		}
	}
	if !found {
		t.Errorf("p_invariants = %v, want to include %q", out.PInvariants, want)
	}

	// The mutex net cycles, so it must have T-invariants too.
	if len(out.TInvariants) == 0 {
		t.Error("t_invariants is empty for a cyclic net")
	}
}

// TestAnalyzeInvariantsEmptyNotNull: an acyclic, non-conserving net should
// return empty arrays rather than null, so callers can iterate unconditionally.
func TestAnalyzeInvariantsEmptyNotNull(t *testing.T) {
	model := `{"name":"pump","places":[{"id":"buffer"}],"transitions":[{"id":"produce"}],
	           "arcs":[{"from":"produce","to":"buffer"}]}`

	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_analyze"
	req.Params.Arguments = map[string]any{"model": model}

	result, err := handleAnalyze(context.Background(), req)
	if err != nil || result.IsError {
		t.Fatalf("analyze failed: %v %s", err, resultText(result))
	}

	raw := resultText(result)
	if !strings.Contains(raw, `"p_invariants": []`) {
		t.Errorf("expected an empty array for p_invariants, got:\n%s", raw)
	}
}

// TestValidateSurfacesInvariants: petri_validate reports conservation laws too,
// not just structural findings. They cost nothing (no state exploration) and are
// the part of the answer that still holds on models too large to analyse.
func TestValidateSurfacesInvariants(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_validate"
	req.Params.Arguments = map[string]any{"model": mutexModelJSON}

	result, err := handleValidate(context.Background(), req)
	if err != nil {
		t.Fatalf("handleValidate error: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool error: %s", resultText(result))
	}

	var out struct {
		Valid       bool     `json:"valid"`
		PInvariants []string `json:"p_invariants"`
		TInvariants []string `json:"t_invariants"`
	}
	if err := json.Unmarshal([]byte(resultText(result)), &out); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, resultText(result))
	}

	if !out.Valid {
		t.Error("mutex model should be valid")
	}

	want := "busy1 + busy2 + sem == 1"
	found := false
	for _, inv := range out.PInvariants {
		if inv == want {
			found = true
		}
	}
	if !found {
		t.Errorf("p_invariants = %v, want to include %q", out.PInvariants, want)
	}
}

// TestValidateAndAnalyzeAgreeOnInvariants pins the reason both tools delegate to
// one implementation: they must never report different laws for the same model.
func TestValidateAndAnalyzeAgreeOnInvariants(t *testing.T) {
	extract := func(handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error), name string) []string {
		req := mcp.CallToolRequest{}
		req.Params.Name = name
		req.Params.Arguments = map[string]any{"model": mutexModelJSON}

		result, err := handler(context.Background(), req)
		if err != nil || result.IsError {
			t.Fatalf("%s failed: %v %s", name, err, resultText(result))
		}
		var out struct {
			PInvariants []string `json:"p_invariants"`
		}
		if err := json.Unmarshal([]byte(resultText(result)), &out); err != nil {
			t.Fatalf("%s unmarshal: %v", name, err)
		}
		return out.PInvariants
	}

	fromValidate := extract(handleValidate, "petri_validate")
	fromAnalyze := extract(handleAnalyze, "petri_analyze")

	if strings.Join(fromValidate, "|") != strings.Join(fromAnalyze, "|") {
		t.Errorf("petri_validate and petri_analyze disagree:\n validate: %v\n analyze:  %v",
			fromValidate, fromAnalyze)
	}
}

// TestColoredPflowModelNotTruncated is the regression for the pflow.xyz
// format converter keeping only Initial[0]/Weight[0]: a colored model was
// silently truncated to its first color, so a transition requiring a BLUE
// token from a red-only pool verified as fireable. The converter now routes
// through go-pflow's parser and colored-net unfolding.
func TestColoredPflowModelNotTruncated(t *testing.T) {
	model := `{"token":["red","blue"],
	  "places":{"pool":{"initial":[1,0]},"out":{"initial":[0,0]}},
	  "transitions":{"take":{}},
	  "arcs":[{"source":"pool","target":"take","weight":[0,1]},
	          {"source":"take","target":"out","weight":[0,1]}]}`

	out := callVerify(t, model, `["unreachable:out=1"]`)
	if !out.OK {
		t.Errorf("take needs a blue token and none exists — out=1 must be unreachable: %s",
			out.Verdicts[0].Detail)
	}

	// And the positive direction: give it a blue token and out=1 is reachable.
	model2 := `{"token":["red","blue"],
	  "places":{"pool":{"initial":[1,2]},"out":{"initial":[0,0]}},
	  "transitions":{"take":{}},
	  "arcs":[{"source":"pool","target":"take","weight":[0,1]},
	          {"source":"take","target":"out","weight":[0,1]}]}`
	out = callVerify(t, model2, `["reachable:out=1"]`)
	if !out.OK {
		t.Errorf("with blue tokens available, out=1 must be reachable: %s", out.Verdicts[0].Detail)
	}
}

// TestPflowInhibitorArcPreserved is the regression for inhibitor flags being
// dropped twice over: the pflow.xyz converter had no inhibitTransition field,
// and buildVerifyNet turned inhibitor arcs into normal consuming arcs.
func TestPflowInhibitorArcPreserved(t *testing.T) {
	// t is inhibited by gate (weight 1). gate holds a token, so t can never
	// fire and out stays empty. With the inhibitor dropped, t consumed from
	// src happily and out=1 was reachable.
	model := `{"places":{"gate":{"initial":1},"src":{"initial":1},"out":{"initial":0}},
	  "transitions":{"t":{}},
	  "arcs":[{"source":"src","target":"t","weight":1},
	          {"source":"t","target":"out","weight":1},
	          {"source":"gate","target":"t","weight":1,"inhibitTransition":true}]}`

	out := callVerify(t, model, `["unreachable:out=1"]`)
	if !out.OK {
		t.Errorf("t is inhibited by gate=1 — out=1 must be unreachable: %s", out.Verdicts[0].Detail)
	}
}

package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// These tests pin the fixes that building the pflow showcase
// (pflow-xyz/examples/showcase) surfaced. Each names the behaviour it
// prevents from coming back.

func callToolHandler(t *testing.T, handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error), name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("%s: handler error: %v", name, err)
	}
	return result
}

func decodeToolJSON(t *testing.T, result *mcp.CallToolResult, into any) {
	t.Helper()
	if result.IsError {
		t.Fatalf("tool returned an error: %s", resultText(result))
	}
	if err := json.Unmarshal([]byte(resultText(result)), into); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, resultText(result))
	}
}

// A source feeding a sink at a declared rate: with the model's rate the ODE
// produces 6 per unit time, at the old flat default only 1.
const declaredRateModel = `{"name":"declared-rate",
 "places":[{"id":"served","initial":0}],
 "transitions":[{"id":"arrive","rate":6}],
 "arcs":[{"from":"arrive","to":"served"}]}`

func TestODEUsesDeclaredRates(t *testing.T) {
	var out struct {
		Rates map[string]float64 `json:"rates"`
		Final map[string]float64 `json:"final"`
	}
	decodeToolJSON(t, callToolHandler(t, handleOde, "petri_ode", map[string]any{
		"model": declaredRateModel, "tspan": "[0, 1]", "plot": false,
	}), &out)
	if out.Rates["arrive"] != 6 {
		t.Fatalf("declared rate ignored: rates=%v", out.Rates)
	}
	if out.Final["served"] < 5.9 || out.Final["served"] > 6.1 {
		t.Fatalf("expected ~6 served at t=1 with arrive=6, got %v", out.Final["served"])
	}
	// The rates argument still overrides.
	decodeToolJSON(t, callToolHandler(t, handleOde, "petri_ode", map[string]any{
		"model": declaredRateModel, "tspan": "[0, 1]", "plot": false, "rates": `{"arrive": 2}`,
	}), &out)
	if out.Rates["arrive"] != 2 {
		t.Fatalf("rates argument no longer overrides: %v", out.Rates)
	}
}

func TestSolverRatesBlockOverridesDeclared(t *testing.T) {
	model := `{"name":"solver-rates",
 "places":[{"id":"served","initial":0}],
 "transitions":[{"id":"arrive","rate":6}],
 "arcs":[{"from":"arrive","to":"served"}],
 "simulation":{"solver":{"rates":{"arrive":0}}}}`
	var out struct {
		Rates map[string]float64 `json:"rates"`
	}
	decodeToolJSON(t, callToolHandler(t, handleOde, "petri_ode", map[string]any{
		"model": model, "tspan": "[0, 1]", "plot": false,
	}), &out)
	if out.Rates["arrive"] != 0 {
		t.Fatalf("simulation.solver.rates should win over the transition rate: %v", out.Rates)
	}
}

// A source into a place with capacity 2: the reachability graph must stop at
// 2, so "queue <= 2" is provable, not refutable.
const cappedQueueModel = `{"name":"capped",
 "places":[{"id":"queue","initial":0,"capacity":2}],
 "transitions":[{"id":"arrive"}],
 "arcs":[{"from":"arrive","to":"queue"}]}`

func TestVerifyHonoursCapacity(t *testing.T) {
	var out struct {
		Verdicts []struct {
			Property struct{ Name string } `json:"property"`
			Status   string                `json:"status"`
		} `json:"verdicts"`
	}
	decodeToolJSON(t, callToolHandler(t, handleVerify, "petri_verify", map[string]any{
		"model": cappedQueueModel, "properties": `["queue <= 2", "bounded"]`,
	}), &out)
	for _, v := range out.Verdicts {
		if v.Status != "proved" {
			t.Fatalf("%s: expected proved with capacity enforced, got %s", v.Property.Name, v.Status)
		}
	}
}

// The shipped ERC-20 shape: a map ledger guarded by its own balances.
const ledgerDSL = `(schema ledger
  (version v1.0.0)
  (states
    (state balances :type map[string]int64 :exported)
    (state total_supply :type int64 :initial 0 :exported))
  (actions
    (action mint :guard {amount > 0})
    (action transfer :guard {balances[from] >= amount && amount > 0}))
  (arcs
    (arc mint -> balances :keys (to) :value amount)
    (arc mint -> total_supply :value amount)
    (arc balances -> transfer :keys (from) :value amount)
    (arc transfer -> balances :keys (to) :value amount))
  (constraints
    (constraint conserved {sum(balances) == total_supply})))`

func TestSimulateDSLSeesMapStates(t *testing.T) {
	var out struct {
		Success bool `json:"success"`
		Failed  []struct {
			TransitionID string `json:"transition_id"`
			Reason       string `json:"reason"`
		} `json:"failed"`
		Fired []string `json:"fired"`
	}
	decodeToolJSON(t, callToolHandler(t, handleSimulateWithSteps, "petri_simulate", map[string]any{
		"model": ledgerDSL,
		"steps": `[{"transition":"mint","bindings":{"to":"ana","amount":10}},
		           {"transition":"transfer","bindings":{"from":"ana","to":"ben","amount":4}},
		           {"transition":"transfer","bindings":{"from":"ben","to":"ana","amount":9}}]`,
	}), &out)
	if len(out.Fired) != 2 || out.Fired[0] != "mint" || out.Fired[1] != "transfer" {
		t.Fatalf("expected mint and one transfer to fire, got fired=%v failed=%+v", out.Fired, out.Failed)
	}
	if len(out.Failed) != 1 || strings.Contains(out.Failed[0].Reason, "unknown identifier") {
		t.Fatalf("the overdraft should be refused by the guard, not by an unresolved name: %+v", out.Failed)
	}
}

// Two colors; the people color never touches the pantry and the beans color
// never touches the queue, so unfolding used to leave two arc-less places.
const twoColorModel = `{"@context":"https://pflow.xyz/schema","@type":"PetriNet","@version":"1.1",
 "token":["https://pflow.xyz/tokens/red","https://pflow.xyz/tokens/brown"],
 "places":{
   "queue":{"initial":[2,0],"capacity":[4,0],"x":100,"y":100},
   "pantry":{"initial":[0,6],"capacity":[0,12],"x":100,"y":300},
   "served":{"initial":[0,0],"x":500,"y":200}},
 "transitions":{"order":{"x":300,"y":200},"arrive":{"x":100,"y":200},"restock":{"x":100,"y":400}},
 "arcs":[
   {"source":"arrive","target":"queue","weight":[1,0]},
   {"source":"queue","target":"order","weight":[1,0]},
   {"source":"pantry","target":"order","weight":[0,2]},
   {"source":"order","target":"served","weight":[1,0]},
   {"source":"restock","target":"pantry","weight":[0,6]},
   {"source":"pantry","target":"restock","weight":[0,6],"inhibitTransition":true}]}`

func TestColoredModelUnfoldsToShortConnectedPlaces(t *testing.T) {
	parsed, err := parseModelV2(twoColorModel)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]int{}
	for _, p := range parsed.Model.Places {
		ids[p.ID] = p.Capacity
	}
	for _, want := range []string{"queue.red", "pantry.brown", "served.red"} {
		if _, ok := ids[want]; !ok {
			t.Fatalf("expected unfolded place %q, have %v", want, ids)
		}
	}
	for _, gone := range []string{"queue.brown", "pantry.red", "served.brown", "queue.https://pflow.xyz/tokens/red"} {
		if _, ok := ids[gone]; ok {
			t.Fatalf("arc-less color copy %q should have been pruned: %v", gone, ids)
		}
	}
	if ids["queue.red"] != 4 || ids["pantry.brown"] != 12 {
		t.Fatalf("per-color capacity not carried: %v", ids)
	}

	var val struct {
		Valid  bool                    `json:"valid"`
		Errors []struct{ Code string } `json:"errors"`
	}
	decodeToolJSON(t, callToolHandler(t, handleValidate, "petri_validate", map[string]any{"model": twoColorModel}), &val)
	if !val.Valid {
		t.Fatalf("colored model should validate after pruning, errors=%+v", val.Errors)
	}
}

func TestAnalyzeFullMarshalsColoredModel(t *testing.T) {
	result := callToolHandler(t, handleAnalyze, "petri_analyze", map[string]any{"model": twoColorModel, "full": true})
	if result.IsError {
		t.Fatalf("petri_analyze full=true failed: %s", resultText(result))
	}
	if strings.Contains(resultText(result), "Inf") {
		t.Fatalf("non-finite importance leaked into the JSON:\n%s", resultText(result))
	}
}

const accessModelV1 = `{"name":"gated",
 "places":[{"id":"new","initial":1},{"id":"done","initial":0}],
 "transitions":[{"id":"finish"}],
 "arcs":[{"from":"new","to":"finish"},{"from":"finish","to":"done"}],
 "roles":[{"id":"barista","name":"Barista"},{"id":"manager","name":"Manager","inherits":["barista"]}],
 "access":[{"transition":"finish","roles":["manager"]}]}`

func TestPreviewPermissionsSeesModelAccess(t *testing.T) {
	check := func(model string) {
		t.Helper()
		var out struct {
			Content string `json:"content"`
		}
		decodeToolJSON(t, callToolHandler(t, handlePreview, "petri_preview", map[string]any{"model": model, "file": "permissions"}), &out)
		if strings.Contains(out.Content, "No access control rules") {
			t.Fatalf("preview ignored the model's roles/access:\n%s", out.Content)
		}
		if !strings.Contains(out.Content, "manager") {
			t.Fatalf("expected the manager role in permissions.go:\n%s", out.Content)
		}
	}
	check(accessModelV1)

	// The v2 envelope petri_migrate emits carries the same declarations as extensions.
	migrated := callToolHandler(t, handleMigrate, "petri_migrate", map[string]any{"model": accessModelV1})
	if migrated.IsError {
		t.Fatalf("migrate: %s", resultText(migrated))
	}
	check(resultText(migrated))
}

// --- readings ported from sim.pflow.xyz, and the engine-selection refusal ---

func TestInvariantsToolReportsLawsCyclesSiphonsAndCaveats(t *testing.T) {
	var out struct {
		Laws        []struct{ Expression string } `json:"laws"`
		TInvariants []any                         `json:"tInvariants"`
		Caveats     []string                      `json:"caveats"`
	}
	decodeToolJSON(t, callToolHandler(t, handleInvariants, "petri_invariants", map[string]any{"model": cappedQueueModel}), &out)
	if len(out.Caveats) == 0 || !strings.Contains(out.Caveats[0], "CAPACITY_BOUND") {
		t.Fatalf("expected the capacity encoding caveat, got %v", out.Caveats)
	}
	var mutex struct {
		Laws []struct{ Expression string } `json:"laws"`
	}
	decodeToolJSON(t, callToolHandler(t, handleInvariants, "petri_invariants", map[string]any{"model": accessModelV1}), &mutex)
	if len(mutex.Laws) != 1 || !strings.Contains(mutex.Laws[0].Expression, "done + new == 1") {
		t.Fatalf("expected the one-token law, got %+v", mutex.Laws)
	}
}

func TestCanonicalIDIsRenamingInvariant(t *testing.T) {
	a := `{"name":"a","places":[{"id":"p","initial":1},{"id":"q"}],"transitions":[{"id":"t"}],"arcs":[{"from":"p","to":"t"},{"from":"t","to":"q"}]}`
	b := `{"name":"b","places":[{"id":"start","initial":1},{"id":"end"}],"transitions":[{"id":"go"}],"arcs":[{"from":"start","to":"go"},{"from":"go","to":"end"}]}`
	var oa, ob struct {
		CanonicalID string `json:"canonical_id"`
	}
	decodeToolJSON(t, callToolHandler(t, handleCanonical, "petri_canonical", map[string]any{"model": a}), &oa)
	decodeToolJSON(t, callToolHandler(t, handleCanonical, "petri_canonical", map[string]any{"model": b}), &ob)
	if oa.CanonicalID == "" || oa.CanonicalID != ob.CanonicalID {
		t.Fatalf("renaming changed the canonical id: %q vs %q", oa.CanonicalID, ob.CanonicalID)
	}
}

func TestLumpingRefusesGatedAndRunsOnPlain(t *testing.T) {
	var gated struct {
		Backward struct {
			Refusals []string `json:"refusals"`
		} `json:"backward_differential_equivalence"`
	}
	decodeToolJSON(t, callToolHandler(t, handleLumping, "petri_lumping", map[string]any{"model": cappedQueueModel}), &gated)
	if len(gated.Backward.Refusals) == 0 {
		t.Fatalf("a capacity-gated net should be refused by backward differential equivalence")
	}
	if r := callToolHandler(t, handleLumping, "petri_lumping", map[string]any{"model": declaredRateModel, "observable": `["served"]`}); r.IsError {
		t.Fatalf("plain model: %s", resultText(r))
	}
}

func TestDatasetIsDeterministicCSV(t *testing.T) {
	args := map[string]any{"model": declaredRateModel, "cases": 5, "seed": 3}
	one := resultText(callToolHandler(t, handleDataset, "petri_dataset", args))
	two := resultText(callToolHandler(t, handleDataset, "petri_dataset", args))
	if one != two {
		t.Fatalf("same seed produced different logs")
	}
	if !strings.HasPrefix(one, "case_id,activity,timestamp") || !strings.Contains(one, "arrive") {
		t.Fatalf("unexpected CSV:\n%s", one)
	}
}

func TestODERefusesScheduledAndGatedModels(t *testing.T) {
	scheduled := `{"name":"day","places":[{"id":"q","initial":0}],"transitions":[{"id":"arrive","rate":4,"schedule":[{"until":2,"value":10},{"until":8,"value":2}]}],"arcs":[{"from":"arrive","to":"q"}]}`
	for name, model := range map[string]string{"scheduled": scheduled, "capped": cappedQueueModel} {
		var out struct {
			Diverged bool     `json:"diverged"`
			Caveats  []string `json:"caveats"`
		}
		decodeToolJSON(t, callToolHandler(t, handleOde, "petri_ode", map[string]any{"model": model, "plot": false}), &out)
		if !out.Diverged || len(out.Caveats) == 0 {
			t.Fatalf("%s: petri_ode should refuse with reasons, got diverged=%v caveats=%v", name, out.Diverged, out.Caveats)
		}
	}
	// Stages alone are not a gate: the check runs on the expanded net.
	staged := `{"name":"erlang","places":[{"id":"a","initial":3},{"id":"b"}],"transitions":[{"id":"serve","rate":2,"stages":3}],"arcs":[{"from":"a","to":"serve"},{"from":"serve","to":"b"}]}`
	var ok struct {
		Diverged bool `json:"diverged"`
	}
	decodeToolJSON(t, callToolHandler(t, handleOde, "petri_ode", map[string]any{"model": staged, "plot": false}), &ok)
	if ok.Diverged {
		t.Fatalf("an Erlang service time is not a gate")
	}
}

func TestVerifyCarriesConversionCaveats(t *testing.T) {
	var out struct {
		Caveats []string `json:"caveats"`
	}
	decodeToolJSON(t, callToolHandler(t, handleVerify, "petri_verify", map[string]any{"model": cappedQueueModel, "properties": `["queue <= 2"]`}), &out)
	if len(out.Caveats) == 0 || !strings.Contains(out.Caveats[0], "CAPACITY_BOUND") {
		t.Fatalf("verify should name the capacity encoding: %v", out.Caveats)
	}
}

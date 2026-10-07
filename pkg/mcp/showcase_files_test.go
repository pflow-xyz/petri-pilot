package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The pflow showcase (pflow-xyz/examples/showcase) is the model family these
// fixes were found against. When that checkout sits beside this one, replay
// its files through the handlers; otherwise skip. The inline tests in
// showcase_fixes_test.go are the ones that always run.
func showcaseFile(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join("..", "..", "..", "pflow-xyz", "examples", "showcase", name)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Skipf("showcase not checked out: %v", err)
	}
	return string(b)
}

func TestShowcaseThemeValidatesAndAnalyzes(t *testing.T) {
	theme := showcaseFile(t, "cafe.jsonld")
	var val struct {
		Valid       bool     `json:"valid"`
		Errors      []any    `json:"errors"`
		PInvariants []string `json:"p_invariants"`
	}
	decodeToolJSON(t, callToolHandler(t, handleValidate, "petri_validate", map[string]any{"model": theme}), &val)
	if !val.Valid {
		t.Fatalf("theme should validate: %+v", val.Errors)
	}
	if !strings.Contains(strings.Join(val.PInvariants, "|"), "barista_free.red + brewing.red == 1") {
		t.Fatalf("expected the short-named barista invariant, got %v", val.PInvariants)
	}
	if r := callToolHandler(t, handleAnalyze, "petri_analyze", map[string]any{"model": theme, "full": true}); r.IsError {
		t.Fatalf("analyze full: %s", resultText(r))
	}
}

func TestShowcaseServiceVerifyRespectsQueueCapacity(t *testing.T) {
	model := showcaseFile(t, "cafe-service.json")
	var out struct {
		Verdicts []struct {
			Property struct{ Name string } `json:"property"`
			Status   string                `json:"status"`
		} `json:"verdicts"`
	}
	decodeToolJSON(t, callToolHandler(t, handleVerify, "petri_verify", map[string]any{
		"model": model, "properties": `["queue <= 8", "brewing + machine_free == 1"]`, "max_states": "5000",
	}), &out)
	for _, v := range out.Verdicts {
		if v.Status == "refuted" {
			t.Fatalf("%s must not be refuted once capacity is enforced", v.Property.Name)
		}
	}
}

func TestShowcaseKineticsRunsAtDeclaredRates(t *testing.T) {
	model := showcaseFile(t, "cafe-kinetics.json")
	var out struct {
		Rates map[string]float64 `json:"rates"`
		Final map[string]float64 `json:"final"`
	}
	decodeToolJSON(t, callToolHandler(t, handleOde, "petri_ode", map[string]any{"model": model, "tspan": "[0, 24]", "plot": false}), &out)
	if out.Rates["arrive"] != 6 || out.Rates["restock"] != 0.2 {
		t.Fatalf("declared rates not used: %v", out.Rates)
	}
	if out.Final["orders"] < 80 {
		t.Fatalf("at the declared rates orders back up past 80 by t=24, got %v", out.Final["orders"])
	}
}

func TestShowcaseLoyaltyLedgerSimulates(t *testing.T) {
	dsl := showcaseFile(t, "cafe-loyalty.pflow")
	var out struct {
		Fired  []string `json:"fired"`
		Failed []struct {
			Reason string `json:"reason"`
		} `json:"failed"`
	}
	decodeToolJSON(t, callToolHandler(t, handleSimulateWithSteps, "petri_simulate", map[string]any{
		"model": dsl,
		"steps": `[{"transition":"earn","bindings":{"member":"ana","amount":10}},
		           {"transition":"gift","bindings":{"from":"ana","to":"ben","amount":4}},
		           {"transition":"redeem","bindings":{"member":"ben","amount":3}},
		           {"transition":"redeem","bindings":{"member":"ben","amount":5}}]`,
	}), &out)
	if len(out.Fired) != 3 {
		t.Fatalf("earn, gift and the first redeem should fire: fired=%v failed=%+v", out.Fired, out.Failed)
	}
	if len(out.Failed) != 1 || strings.Contains(out.Failed[0].Reason, "unknown identifier") {
		t.Fatalf("the overdraft must be a guard refusal: %+v", out.Failed)
	}
}

func TestShowcaseOrderPreviewCarriesAccess(t *testing.T) {
	model := showcaseFile(t, "cafe-order.json")
	var out struct {
		Content string `json:"content"`
	}
	decodeToolJSON(t, callToolHandler(t, handlePreview, "petri_preview", map[string]any{"model": model, "file": "permissions"}), &out)
	if !strings.Contains(out.Content, "refund") || strings.Contains(out.Content, "No access control rules") {
		t.Fatalf("permissions preview should carry the six access rules:\n%s", out.Content)
	}
	_ = json.Valid
}

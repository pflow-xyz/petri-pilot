package mcp

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"

	"github.com/pflow-xyz/petri-pilot/pkg/runtime/sim"
)

func TestStochastic_CoffeeShop_SingleRealization(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_stochastic"
	req.Params.Arguments = map[string]any{
		"model":   mustJSON(t, coffeeShopModel()),
		"tspan":   "[0, 10]",
		"samples": 100,
		"seed":    42,
	}
	res, err := handleStochastic(context.Background(), req)
	if err != nil {
		t.Fatalf("handleStochastic: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	var resp stochasticResponse
	if err := json.Unmarshal([]byte(textBlock(t, res)), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Realizations != 1 {
		t.Errorf("realizations = %d, want 1", resp.Realizations)
	}
	if len(resp.Times) != 100 {
		t.Errorf("times count = %d, want 100", len(resp.Times))
	}
	// Mass conservation: total token count is preserved by transitions
	// (each fires 2 in / 1 out then 1 in / 1+1 out → net 0 over a cycle).
	// We have 2 orders + 1 barista = 3 tokens. With single realization,
	// final total should be near 3 (could be ±1 due to incomplete cycles).
	total := 0.0
	for _, v := range resp.FinalMean {
		total += v
	}
	if math.Abs(total-3) > 1 {
		t.Errorf("final total tokens = %v, want ~3", total)
	}
}

func TestStochastic_MultipleRealizations(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_stochastic"
	req.Params.Arguments = map[string]any{
		"model":        mustJSON(t, coffeeShopModel()),
		"tspan":        "[0, 10]",
		"samples":      80,
		"realizations": 20,
		"seed":         42,
	}
	res, err := handleStochastic(context.Background(), req)
	if err != nil {
		t.Fatalf("handleStochastic: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	var resp stochasticResponse
	if err := json.Unmarshal([]byte(textBlock(t, res)), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Realizations != 20 {
		t.Errorf("realizations = %d, want 20", resp.Realizations)
	}
	if resp.Stdev == nil || resp.Stdev["delivered"] == nil {
		t.Errorf("expected stdev for 'delivered' with multiple realizations")
	}
	// Mean of 'delivered' should approach 2 over time (mean of the
	// stochastic ensemble matches the deterministic equilibrium).
	finalDel := resp.FinalMean["delivered"]
	if finalDel < 1.5 || finalDel > 2.5 {
		t.Errorf("mean delivered at t=10 = %v, want ~2", finalDel)
	}
	if path := os.Getenv("STOCHASTIC_OUT"); path != "" {
		if img := extractImageBytes(t, res); len(img) > 0 {
			if err := os.WriteFile(path, img, 0644); err != nil {
				t.Fatalf("write: %v", err)
			}
			t.Logf("wrote %d bytes to %s", len(img), path)
		}
	}
}

func TestStochastic_Combinations(t *testing.T) {
	cases := []struct {
		m, w int
		want float64
	}{
		{0, 1, 0},
		{1, 1, 1},
		{5, 1, 5},
		{5, 2, 10}, // C(5, 2) = 10
		{5, 3, 10}, // C(5, 3) = 10
		{10, 0, 1},
	}
	for _, c := range cases {
		got := combinations(c.m, c.w)
		if got != c.want {
			t.Errorf("combinations(%d, %d) = %v, want %v", c.m, c.w, got, c.want)
		}
	}
}

func TestStochastic_Reproducible(t *testing.T) {
	// Same seed should produce identical realizations.
	args := map[string]any{
		"model":        mustJSON(t, coffeeShopModel()),
		"tspan":        "[0, 5]",
		"samples":      30,
		"realizations": 5,
		"seed":         123,
	}
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_stochastic"
	req.Params.Arguments = args
	res1, err := handleStochastic(context.Background(), req)
	if err != nil {
		t.Fatalf("handleStochastic 1: %v", err)
	}
	res2, err := handleStochastic(context.Background(), req)
	if err != nil {
		t.Fatalf("handleStochastic 2: %v", err)
	}
	if textBlock(t, res1) != textBlock(t, res2) {
		t.Errorf("same seed produced different output")
	}
}

// suppliedMachineModel is a machine that never wants for work and one supplier
// feeding it: `make` needs a job and ten units of stock, `deliver` brings fifty
// at a time. Both of the machine's inputs are non-kinetic, so its pace is its
// own declared rate and the only thing that can hold it up is the stock.
//
// Deliveries at rate 9 are 450 units an hour against a machine that would take
// 600, so the machine idles for the difference — about a quarter of the run —
// while the stock is refilled and drawn down all day and its mean never goes
// near zero. That is the state a trajectory cannot show and the café shipped
// in: limited by a resource that never looks scarce in the plot.
func suppliedMachineModel() *goflowmetamodel.Model {
	no := false
	return &goflowmetamodel.Model{
		Name: "supplied_machine",
		Places: []goflowmetamodel.Place{
			{ID: "jobs", Initial: 100000},
			{ID: "stock", Initial: 100},
			{ID: "made"},
		},
		Transitions: []goflowmetamodel.Transition{
			{ID: "make", Rate: 60},
			{ID: "deliver", Rate: 9},
		},
		Arcs: []goflowmetamodel.Arc{
			{From: "jobs", To: "make", Kinetic: &no},
			{From: "stock", To: "make", Weight: 10, Kinetic: &no},
			{From: "make", To: "made"},
			{From: "deliver", To: "stock", Weight: 50},
		},
	}
}

// TestStochastic_ReportsWhatTheRunWasWaitingFor pins the diagnostic to this
// engine too. pkg/runtime/sim has the same rule and the same fixture; two
// engines answering "what is this model limited by" differently is the failure
// both files' comments are about.
func TestStochastic_ReportsWhatTheRunWasWaitingFor(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_stochastic"
	req.Params.Arguments = map[string]any{
		"model": mustJSON(t, suppliedMachineModel()),
		// Rates are passed explicitly: this tool defaults every transition to
		// 1.0 rather than reading Transition.Rate, so a fixture whose whole
		// point is the ratio between two rates has to say them here.
		"rates":        `{"make": 60, "deliver": 9}`,
		"tspan":        "[0, 8]",
		"samples":      200,
		"realizations": 24,
		"seed":         20260807,
	}
	res, err := handleStochastic(context.Background(), req)
	if err != nil {
		t.Fatalf("handleStochastic: %v", err)
	}
	var resp stochasticResponse
	if err := json.Unmarshal([]byte(textBlock(t, res)), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	var stock *contendedPlace
	for i := range resp.Contended {
		if resp.Contended[i].Place == "stock" {
			stock = &resp.Contended[i]
		}
		if resp.Contended[i].Fraction > 1 {
			t.Errorf("%s was short for %.2f of the run, which is more run than there is",
				resp.Contended[i].Place, resp.Contended[i].Fraction)
		}
	}
	if stock == nil {
		t.Fatalf("nothing reported as contended; a supply covering 75%% of demand is the constraint. got %+v",
			resp.Contended)
	}
	// Same band as pkg/runtime/sim's TestContentionNamesAFullySubscribedResource,
	// on the same fixture, for the same reason: 450 units an hour into a machine
	// that can take 600 idles it about a quarter of the time.
	const want = 1 - 450.0/600
	if stock.Fraction < want*0.6 || stock.Fraction > want*1.6 {
		t.Errorf("stock held up the machine for %.1f%% of the run, want about %.0f%%",
			stock.Fraction*100, want*100)
	}
	if len(stock.Blocking) != 1 || stock.Blocking[0] != "make" {
		t.Errorf("stock reported as blocking %v, want [make]", stock.Blocking)
	}

	// The kind comes off the same classifier pkg/runtime/sim ranks on, and the
	// two engines are separate copies of the propensity loop — one of them
	// deciding a place is a resource while the other calls it a queue would
	// order the list differently for the same model, which is the failure both
	// files' comments are about. Compared rather than asserted literally: this
	// fixture is deliberately a bare machine that declares neither a capacity
	// nor a conservation law, so what matters is that both engines say so.
	kinds := sim.ClassifySupply(suppliedMachineModel())
	for _, c := range resp.Contended {
		if c.Kind == "" {
			t.Errorf("%s reported with no supply kind; a caller then has to tell a shortage from an "+
				"idle queue by reading the place's name", c.Place)
		}
		if want := kinds[c.Place]; c.Kind != want {
			t.Errorf("%s classified %q here and %q in pkg/runtime/sim", c.Place, c.Kind, want)
		}
	}
	queued := ""
	for _, c := range resp.Contended {
		if !c.Kind.IsCapacity() {
			queued = c.Place
			continue
		}
		if queued != "" {
			t.Errorf("%s is a %s constraint but ranks below the queue %s — a caller reading the top of "+
				"this list would be told an empty queue is what to fix", c.Place, c.Kind, queued)
		}
	}
	t.Logf("stock: mean %.1f at the horizon, blocking %v for %.0f%% of the run",
		resp.FinalMean["stock"], stock.Blocking, stock.Fraction*100)
}

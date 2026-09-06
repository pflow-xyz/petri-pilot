package mcp

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"
)

// busyCoffeeShop is coffeeShopModel scaled up so one SSA run produces
// hundreds of firings per transition — enough events for the likelihood
// to pin each rate, which two orders and one barista never could.
func busyCoffeeShop() *goflowmetamodel.Model {
	m := coffeeShopModel()
	for i := range m.Places {
		switch m.Places[i].ID {
		case "order_pending":
			m.Places[i].Initial = 300
		case "barista_idle":
			m.Places[i].Initial = 4
		}
	}
	return m
}

// recordPaths runs petri_stochastic with record_events and returns the
// paths JSON the tool emitted — the producer half of the round trip.
func recordPaths(t *testing.T, model *goflowmetamodel.Model, rates string, realizations int) string {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_stochastic"
	req.Params.Arguments = map[string]any{
		"model":         mustJSON(t, model),
		"rates":         rates,
		"tspan":         "[0, 40]",
		"realizations":  realizations,
		"record_events": true,
	}
	res, err := handleStochastic(context.Background(), req)
	if err != nil {
		t.Fatalf("handleStochastic: %v", err)
	}
	if res.IsError {
		t.Fatalf("petri_stochastic error: %s", textBlock(t, res))
	}
	var resp stochasticResponse
	if err := json.Unmarshal([]byte(textBlock(t, res)), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Paths) != realizations {
		t.Fatalf("record_events: got %d paths, want %d", len(resp.Paths), realizations)
	}
	for i, p := range resp.Paths {
		if len(p.Events) == 0 {
			t.Fatalf("path %d recorded no events", i)
		}
		if p.Horizon != 40 {
			t.Fatalf("path %d horizon %v, want 40", i, p.Horizon)
		}
	}
	b, _ := json.Marshal(resp.Paths)
	return string(b)
}

func TestStochastic_RecordEventsOffByDefault(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_stochastic"
	req.Params.Arguments = map[string]any{"model": mustJSON(t, coffeeShopModel())}
	res, err := handleStochastic(context.Background(), req)
	if err != nil || res.IsError {
		t.Fatalf("handleStochastic: err=%v res=%s", err, textBlock(t, res))
	}
	if strings.Contains(textBlock(t, res), `"paths"`) {
		t.Fatal("paths must be omitted unless record_events is set")
	}
}

func TestFitDiscrete_RoundTripRecoversRates(t *testing.T) {
	model := busyCoffeeShop()
	trueRates := map[string]float64{"start_brew": 2.0, "finish_brew": 0.7, "deliver": 1.3}
	paths := recordPaths(t, model, `{"start_brew":2.0,"finish_brew":0.7,"deliver":1.3}`, 3)

	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_fit_discrete"
	req.Params.Arguments = map[string]any{
		"model":      mustJSON(t, model),
		"paths":      paths,
		"parameters": `{"start_brew":1.0,"finish_brew":1.0,"deliver":1.0}`,
		"max_iter":   2000,
	}
	res, err := handleFitDiscrete(context.Background(), req)
	if err != nil {
		t.Fatalf("handleFitDiscrete: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	var resp fitDiscreteResponse
	if err := json.Unmarshal([]byte(textBlock(t, res)), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Paths != 3 || resp.Events == 0 {
		t.Fatalf("paths=%d events=%d", resp.Paths, resp.Events)
	}
	if resp.FinalLoss >= resp.InitialLoss {
		t.Fatalf("fit did not reduce −log L: %v -> %v", resp.InitialLoss, resp.FinalLoss)
	}
	for id, want := range trueRates {
		got := resp.FittedRates[id]
		if rel := math.Abs(got-want) / want; rel > 0.15 {
			t.Errorf("%s: fitted %.3f, true %.3f (rel err %.1f%%)", id, got, want, rel*100)
		}
		// The optimiser must land on the closed-form MLE, not near it: the
		// objective is convex in each rate and the MLE is its exact minimum.
		if mle := resp.MLE[id]; math.Abs(got-mle)/mle > 0.02 {
			t.Errorf("%s: fitted %.4f but closed-form MLE is %.4f", id, got, mle)
		}
		if resp.Counts[id] == 0 || !(resp.Exposure[id] > 0) {
			t.Errorf("%s: counts=%d exposure=%v", id, resp.Counts[id], resp.Exposure[id])
		}
	}
}

func TestFitDiscrete_MarkingOptionalAndPartialFit(t *testing.T) {
	model := busyCoffeeShop()
	raw := recordPaths(t, model, `{"start_brew":2.0,"finish_brew":0.7,"deliver":1.3}`, 1)

	// Strip every post-firing marking: the tool must derive them.
	var paths []discretePathJSON
	if err := json.Unmarshal([]byte(raw), &paths); err != nil {
		t.Fatal(err)
	}
	for i := range paths {
		for j := range paths[i].Events {
			paths[i].Events[j].Marking = nil
		}
	}
	stripped, _ := json.Marshal(paths)

	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_fit_discrete"
	req.Params.Arguments = map[string]any{
		"model":       mustJSON(t, model),
		"paths":       string(stripped),
		"parameters":  `["deliver"]`,
		"fixed_rates": `{"start_brew":2.0,"finish_brew":0.7}`,
	}
	res, err := handleFitDiscrete(context.Background(), req)
	if err != nil {
		t.Fatalf("handleFitDiscrete: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	var resp fitDiscreteResponse
	if err := json.Unmarshal([]byte(textBlock(t, res)), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.ParamOrder) != 1 || resp.ParamOrder[0] != "deliver" {
		t.Fatalf("paramOrder = %v, want [deliver]", resp.ParamOrder)
	}
	if resp.FittedRates["start_brew"] != 2.0 || resp.FittedRates["finish_brew"] != 0.7 {
		t.Fatalf("held rates moved: %v", resp.FittedRates)
	}
	if got := resp.FittedRates["deliver"]; math.Abs(got-1.3)/1.3 > 0.15 {
		t.Errorf("deliver: fitted %.3f, true 1.3", got)
	}
}

func TestFitDiscrete_RejectsInconsistentPaths(t *testing.T) {
	model := coffeeShopModel()
	cases := map[string]string{
		"unknown transition": `[{"initial":{},"horizon":5,"events":[{"time":1,"transition":"nope"}]}]`,
		"not enabled":        `[{"initial":{"order_pending":0},"horizon":5,"events":[{"time":1,"transition":"start_brew"}]}]`,
		"wrong marking":      `[{"initial":{},"horizon":5,"events":[{"time":1,"transition":"start_brew","marking":{"brewing":5}}]}]`,
		"time goes backward": `[{"initial":{},"horizon":5,"events":[{"time":2,"transition":"start_brew"},{"time":1,"transition":"finish_brew"}]}]`,
		"horizon too short":  `[{"initial":{},"horizon":0.5,"events":[{"time":1,"transition":"start_brew"}]}]`,
		"unknown place":      `[{"initial":{"espresso":1},"horizon":5,"events":[{"time":1,"transition":"start_brew"}]}]`,
		"no events":          `[{"initial":{},"horizon":5,"events":[]}]`,
	}
	for name, paths := range cases {
		req := mcp.CallToolRequest{}
		req.Params.Name = "petri_fit_discrete"
		req.Params.Arguments = map[string]any{"model": mustJSON(t, model), "paths": paths}
		res, err := handleFitDiscrete(context.Background(), req)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !res.IsError {
			t.Errorf("%s: expected a tool error, got %s", name, textBlock(t, res))
		}
	}
}

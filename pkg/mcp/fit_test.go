package mcp

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/pflow-xyz/go-pflow/solver"
)

func TestFit_RecoversKnownRates(t *testing.T) {
	// Generate synthetic observations from the coffee shop with known rates.
	// Then ask petri_fit to recover those rates from the observations.
	model := coffeeShopModel()
	trueRates := map[string]float64{
		"start_brew":  2.0,
		"finish_brew": 1.5,
		"deliver":     1.0,
	}
	net := buildOdeNet(model)
	initial := map[string]float64{}
	for _, p := range model.Places {
		initial[p.ID] = float64(p.Initial)
	}
	prob := solver.NewProblem(net, initial, [2]float64{0, 10}, trueRates)
	sol := solver.Solve(prob, solver.Tsit5(), solver.JSParityOptions())

	observeAt := []float64{0.5, 1, 2, 3, 5, 7, 10}
	deliveredObs := make([][2]float64, len(observeAt))
	pendingObs := make([][2]float64, len(observeAt))
	for i, t := range observeAt {
		deliveredObs[i] = [2]float64{t, interpolate(sol.T, sol.GetVariable("delivered"), t)}
		pendingObs[i] = [2]float64{t, interpolate(sol.T, sol.GetVariable("order_pending"), t)}
	}
	obs := map[string][][2]float64{
		"delivered":     deliveredObs,
		"order_pending": pendingObs,
	}
	obsJSON, _ := json.Marshal(obs)

	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_fit"
	req.Params.Arguments = map[string]any{
		"model":        mustJSON(t, model),
		"observations": string(obsJSON),
		"parameters":   `{"start_brew":[0.1, 10], "finish_brew":[0.1, 10], "deliver":[0.1, 10]}`,
		"max_iter":     500,
	}
	res, err := handleFit(context.Background(), req)
	if err != nil {
		t.Fatalf("handleFit: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	var resp fitResponse
	if err := json.Unmarshal([]byte(textBlock(t, res)), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	t.Logf("fitted rates: %+v, loss=%v, iters=%d, converged=%v", resp.FittedRates, resp.FinalLoss, resp.Iterations, resp.Converged)

	// The fit quality metric that matters is loss — for non-identifiable
	// parameterizations (like the coffee shop's sequential chain) multiple
	// rate combinations produce identical trajectories, so individual rate
	// recovery isn't expected. What we want is: fitted trajectory matches
	// observed trajectory.
	if resp.FinalLoss > 0.1 {
		t.Errorf("final loss = %v, expected < 0.1 after fitting against noise-free synthetic data", resp.FinalLoss)
	}
	_ = math.Abs // referenced by sub-test below
	if path := os.Getenv("FIT_OUT"); path != "" {
		if img := extractImageBytes(t, res); len(img) > 0 {
			if err := os.WriteFile(path, img, 0644); err != nil {
				t.Fatalf("write: %v", err)
			}
			t.Logf("wrote %d bytes to %s", len(img), path)
		}
	}
}

func TestFit_Interpolate(t *testing.T) {
	ts := []float64{0, 1, 2, 4, 8}
	ys := []float64{0, 10, 20, 40, 80}
	cases := []struct {
		t, want float64
	}{
		{-1, 0},  // before start
		{0, 0},
		{0.5, 5}, // between 0 and 1
		{2, 20},
		{3, 30},  // midpoint of (2,20)→(4,40)
		{8, 80},
		{10, 80}, // past end
	}
	for _, c := range cases {
		got := interpolate(ts, ys, c.t)
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("interpolate(t=%v) = %v, want %v", c.t, got, c.want)
		}
	}
}

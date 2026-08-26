package mcp

import (
	"context"
	"encoding/json"
	"math"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/pflow-xyz/go-pflow/solver"
)

// The adam path must recover the same known rates the Nelder-Mead path
// does, from the same synthetic observations.
func TestFitAdam_RecoversKnownRates(t *testing.T) {
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
	for i, tt := range observeAt {
		deliveredObs[i] = [2]float64{tt, interpolate(sol.T, sol.GetVariable("delivered"), tt)}
		pendingObs[i] = [2]float64{tt, interpolate(sol.T, sol.GetVariable("order_pending"), tt)}
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
		"method":       "adam",
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
		t.Fatalf("unmarshal: %v (%s)", err, textBlock(t, res))
	}
	if resp.Method != "adam" {
		t.Fatalf("method = %q, want adam", resp.Method)
	}
	if resp.Evals == 0 {
		t.Fatal("evals not reported")
	}
	if resp.FinalLoss > 1e-2 {
		t.Fatalf("final loss %g too high", resp.FinalLoss)
	}
	for k, want := range trueRates {
		got := resp.FittedRates[k]
		if math.Abs(got-want)/want > 0.15 {
			t.Errorf("rate %s = %.4f, want ~%.4f", k, got, want)
		}
	}
}

func TestFit_UnknownMethodRejected(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_fit"
	req.Params.Arguments = map[string]any{
		"model":        mustJSON(t, coffeeShopModel()),
		"observations": `{"delivered":[[1,0.5]]}`,
		"parameters":   `{"deliver":[0.1, 10]}`,
		"method":       "bogus",
	}
	res, err := handleFit(context.Background(), req)
	if err != nil {
		t.Fatalf("handleFit: %v", err)
	}
	if !res.IsError {
		t.Fatal("bogus method not rejected")
	}
}

// The analytic sensitivity path must broadly agree with the
// finite-difference path: same top-ranked transition, same signs on the
// dominant elasticities.
func TestOdeSensitivity_AnalyticAgreesWithFD(t *testing.T) {
	run := func(method string) odeSensitivityResponse {
		req := mcp.CallToolRequest{}
		req.Params.Name = "petri_ode_sensitivity"
		req.Params.Arguments = map[string]any{
			"model":      mustJSON(t, coffeeShopModel()),
			"observable": "delivered",
			"tspan":      "[0, 20]",
			"method":     method,
		}
		res, err := handleOdeSensitivity(context.Background(), req)
		if err != nil {
			t.Fatalf("handleOdeSensitivity(%s): %v", method, err)
		}
		if res.IsError {
			t.Fatalf("%s error: %s", method, textBlock(t, res))
		}
		var resp odeSensitivityResponse
		if err := json.Unmarshal([]byte(textBlock(t, res)), &resp); err != nil {
			t.Fatalf("unmarshal %s: %v", method, err)
		}
		return resp
	}
	fd := run("fd")
	an := run("analytic")
	if an.Method != "analytic" {
		t.Fatalf("method = %q", an.Method)
	}
	if len(an.Elasticities) != len(fd.Elasticities) {
		t.Fatalf("elasticity count differs: %d vs %d", len(an.Elasticities), len(fd.Elasticities))
	}
	// Signs must agree wherever the FD elasticity is clearly nonzero.
	for tid, e := range fd.Elasticities {
		if math.Abs(e) < 1e-3 {
			continue
		}
		if e*an.Elasticities[tid] < 0 {
			t.Errorf("sign disagreement on %s: fd=%.4f analytic=%.4f", tid, e, an.Elasticities[tid])
		}
	}
	if len(fd.Ranked) > 0 && len(an.Ranked) > 0 && fd.Ranked[0].Transition != an.Ranked[0].Transition {
		t.Errorf("top-ranked transition differs: fd=%s analytic=%s", fd.Ranked[0].Transition, an.Ranked[0].Transition)
	}
}

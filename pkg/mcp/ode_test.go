package mcp

import (
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/pflow-xyz/go-pflow/solver"
)

func TestOde_CoffeeShop_Plot(t *testing.T) {
	model := coffeeShopModel()
	net := buildOdeNet(model)
	initial := map[string]float64{}
	for _, p := range model.Places {
		initial[p.ID] = float64(p.Initial)
	}
	rates := map[string]float64{}
	for _, tr := range model.Transitions {
		rates[tr.ID] = 1.0
	}
	prob := solver.NewProblem(net, initial, [2]float64{0, 10}, rates)
	sol := solver.Solve(prob, solver.Tsit5(), solver.JSParityOptions())
	if sol == nil || len(sol.T) == 0 {
		t.Fatalf("empty solution")
	}

	vars := []string{"order_pending", "brewing", "ready", "delivered"}
	pngBytes, err := renderODEPlot(sol, vars, "Coffee shop ODE")
	if err != nil {
		t.Fatalf("renderODEPlot: %v", err)
	}
	img, err := png.Decode(strings.NewReader(string(pngBytes)))
	if err != nil {
		t.Fatalf("png decode: %v", err)
	}
	b := img.Bounds()
	if b.Dx() < 600 || b.Dy() < 300 {
		t.Fatalf("unexpected dimensions: %dx%d", b.Dx(), b.Dy())
	}
	if path := os.Getenv("ODE_PLOT_OUT"); path != "" {
		if err := os.WriteFile(path, pngBytes, 0644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		t.Logf("wrote %d bytes to %s", len(pngBytes), path)
	}
}

func TestOde_FormatTick(t *testing.T) {
	cases := map[float64]string{
		0:      "0.00",
		1:      "1",
		2.5:    "2.50",
		15.7:   "15.7",
		137.4:  "137",
		-3.14:  "-3.14",
		100.99: "101",
	}
	for in, want := range cases {
		if got := formatTick(in); got != want {
			t.Errorf("formatTick(%v) = %q want %q", in, got, want)
		}
	}
}

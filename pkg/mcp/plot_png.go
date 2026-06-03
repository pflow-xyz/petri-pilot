package mcp

import (
	"bytes"
	"fmt"
	"image/png"
	"math"

	"github.com/fogleman/gg"
	"github.com/pflow-xyz/go-pflow/solver"
)

// renderODEPlot rasterizes a time-series plot of an ODE solution. The visual
// language (colors, axes, gridlines, legend on the right) mirrors the
// browser-side SVGPlotter in pflow-xyz/public/petri-solver.js so plots
// produced by petri_ode read like the ones the pflow.xyz editor draws.

var plotColors = []string{
	"#e41a1c", "#377eb8", "#4daf4a", "#984ea3",
	"#ff7f00", "#dbb500", "#a65628", "#f781bf",
}

// odePlotSize returns the natural dimensions used by renderODEPlot. Reusable
// when callers (e.g. combined views) need to lay out the plot alongside
// another diagram.
func odePlotSize() (int, int) { return 720, 420 }

func renderODEPlot(sol *solver.Solution, variables []string, title string) ([]byte, error) {
	if sol == nil || len(sol.T) == 0 {
		return nil, fmt.Errorf("empty solution")
	}
	w, h := odePlotSize()
	dc := gg.NewContext(w, h)
	dc.SetHexColor("#ffffff")
	dc.Clear()
	drawODEPlot(dc, sol, variables, title, 0, 0, float64(w), float64(h))
	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// drawODEPlot renders the time-series plot into an existing gg.Context at
// the given offset and dimensions. Used both standalone and inside composite
// views.
func drawODEPlot(dc *gg.Context, sol *solver.Solution, variables []string, title string, originX, originY, W, H float64) {
	if sol == nil || len(sol.T) == 0 {
		return
	}
	ys := make([][]float64, 0, len(variables))
	for _, v := range variables {
		ys = append(ys, sol.GetVariable(v))
	}
	drawXYPlot(dc, sol.T, ys, variables, title, "Time", "Value", originX, originY, W, H)
}

// formatTick prints axis tick values compactly: integers without trailing
// zeros, fractional values to 2 sf.
func formatTick(v float64) string {
	if math.Abs(v) >= 100 || (math.Abs(v) >= 1 && v == math.Trunc(v)) {
		return fmt.Sprintf("%.0f", v)
	}
	if math.Abs(v) >= 10 {
		return fmt.Sprintf("%.1f", v)
	}
	return fmt.Sprintf("%.2f", v)
}

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

func renderODEPlot(sol *solver.Solution, variables []string, title string) ([]byte, error) {
	if sol == nil || len(sol.T) == 0 {
		return nil, fmt.Errorf("empty solution")
	}

	const (
		W       = 720
		H       = 420
		marginT = 40.0
		marginR = 140.0 // wide for legend
		marginB = 50.0
		marginL = 70.0
	)
	plotW := float64(W) - marginL - marginR
	plotH := float64(H) - marginT - marginB

	dc := gg.NewContext(W, H)
	dc.SetHexColor("#ffffff")
	dc.Clear()

	if title != "" {
		if f, err := pngFace(true, 16); err == nil {
			dc.SetFontFace(f)
			dc.SetHexColor("#000000")
			dc.DrawStringAnchored(title, float64(W)/2, 22, 0.5, 0.5)
		}
	}

	xmin := sol.T[0]
	xmax := sol.T[len(sol.T)-1]
	if xmax-xmin < 1e-12 {
		xmax = xmin + 1
	}
	ymin := math.Inf(1)
	ymax := math.Inf(-1)
	seriesYs := make([][]float64, 0, len(variables))
	for _, v := range variables {
		ys := sol.GetVariable(v)
		seriesYs = append(seriesYs, ys)
		for _, y := range ys {
			if y < ymin {
				ymin = y
			}
			if y > ymax {
				ymax = y
			}
		}
	}
	if math.IsInf(ymin, 1) {
		ymin, ymax = 0, 1
	}
	yrange := ymax - ymin
	if yrange < 1e-9 {
		// Constant trajectory — pad to a small visible band so the line
		// doesn't sit on the bottom axis.
		ymax = ymin + 1
		yrange = 1
	}
	ymin -= yrange * 0.1
	ymax += yrange * 0.1

	sx := func(x float64) float64 {
		return marginL + (x-xmin)/(xmax-xmin)*plotW
	}
	sy := func(y float64) float64 {
		return marginT + plotH - (y-ymin)/(ymax-ymin)*plotH
	}

	const nTicks = 5

	// Grid (drawn before axes/labels so they overlay cleanly).
	dc.SetHexColor("#dddddd")
	dc.SetLineWidth(0.5)
	for i := 0; i <= nTicks; i++ {
		x := xmin + (xmax-xmin)*float64(i)/nTicks
		px := sx(x)
		dc.DrawLine(px, marginT, px, marginT+plotH)
		dc.Stroke()
	}
	for i := 0; i <= nTicks; i++ {
		y := ymin + (ymax-ymin)*float64(i)/nTicks
		py := sy(y)
		dc.DrawLine(marginL, py, marginL+plotW, py)
		dc.Stroke()
	}

	// Axes
	dc.SetHexColor("#333333")
	dc.SetLineWidth(2)
	dc.DrawLine(marginL, marginT, marginL, marginT+plotH)
	dc.Stroke()
	dc.DrawLine(marginL, marginT+plotH, marginL+plotW, marginT+plotH)
	dc.Stroke()

	// Tick labels
	if f, err := pngFace(false, 10); err == nil {
		dc.SetFontFace(f)
		dc.SetHexColor("#000000")
		for i := 0; i <= nTicks; i++ {
			x := xmin + (xmax-xmin)*float64(i)/nTicks
			dc.DrawStringAnchored(formatTick(x), sx(x), marginT+plotH+14, 0.5, 0.5)
		}
		for i := 0; i <= nTicks; i++ {
			y := ymin + (ymax-ymin)*float64(i)/nTicks
			dc.DrawStringAnchored(formatTick(y), marginL-8, sy(y), 1.0, 0.5)
		}
	}

	// Axis titles
	if f, err := pngFace(false, 12); err == nil {
		dc.SetFontFace(f)
		dc.SetHexColor("#000000")
		dc.DrawStringAnchored("Time", marginL+plotW/2, float64(H)-15, 0.5, 0.5)
		dc.Push()
		dc.RotateAbout(-math.Pi/2, 18, marginT+plotH/2)
		dc.DrawStringAnchored("Value", 18, marginT+plotH/2, 0.5, 0.5)
		dc.Pop()
	}

	// Plot series. Use a single path per series so gg can do anti-aliased
	// strokes in one go instead of N short line segments.
	dc.SetLineWidth(2)
	for i, ys := range seriesYs {
		if len(ys) == 0 {
			continue
		}
		dc.SetHexColor(plotColors[i%len(plotColors)])
		dc.MoveTo(sx(sol.T[0]), sy(ys[0]))
		for j := 1; j < len(ys); j++ {
			dc.LineTo(sx(sol.T[j]), sy(ys[j]))
		}
		dc.Stroke()
	}

	// Legend (top-right column, outside the plot area).
	legendX := marginL + plotW + 14
	legendY := marginT + 6
	if f, err := pngFace(false, 11); err == nil {
		dc.SetFontFace(f)
		for i, v := range variables {
			dc.SetHexColor(plotColors[i%len(plotColors)])
			dc.SetLineWidth(2)
			dc.DrawLine(legendX, legendY+6, legendX+20, legendY+6)
			dc.Stroke()
			dc.SetHexColor("#000000")
			dc.DrawStringAnchored(v, legendX+24, legendY+6, 0, 0.5)
			legendY += 18
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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

package mcp

import (
	"bytes"
	"fmt"
	"image/png"
	"math"
	"sort"

	"github.com/fogleman/gg"
)

// renderOptimizePNG dispatches to a scatter plot (2 objectives) or
// parallel-coordinates chart (3+).
func renderOptimizePNG(resp optimizeResponse) ([]byte, error) {
	if len(resp.Objectives) == 2 {
		return renderOptimizeScatter(resp)
	}
	return renderOptimizeParallel(resp)
}

// renderOptimizeScatter draws a 2D scatter of samples in objective space.
// Dominated samples render as small gray dots; Pareto-optimal samples are
// larger, colored, and connected by a curve sorted along the x-axis so the
// frontier shape is visually obvious.
func renderOptimizeScatter(resp optimizeResponse) ([]byte, error) {
	const W, H = 720, 480
	dc := gg.NewContext(W, H)
	dc.SetHexColor("#ffffff")
	dc.Clear()

	if len(resp.Samples) == 0 {
		return nil, fmt.Errorf("no samples to render")
	}

	objX := resp.Objectives[0]
	objY := resp.Objectives[1]

	xmin, xmax := math.Inf(1), math.Inf(-1)
	ymin, ymax := math.Inf(1), math.Inf(-1)
	for _, s := range resp.Samples {
		x, y := s.Values[objX.Place], s.Values[objY.Place]
		if x < xmin {
			xmin = x
		}
		if x > xmax {
			xmax = x
		}
		if y < ymin {
			ymin = y
		}
		if y > ymax {
			ymax = y
		}
	}
	if xmax-xmin < 1e-9 {
		xmax = xmin + 1
	}
	if ymax-ymin < 1e-9 {
		ymax = ymin + 1
	}
	xpad := (xmax - xmin) * 0.05
	ypad := (ymax - ymin) * 0.1
	xmin -= xpad
	xmax += xpad
	ymin -= ypad
	ymax += ypad

	xLabel := fmt.Sprintf("%s (%s)", objX.Place, objX.Direction)
	yLabel := fmt.Sprintf("%s (%s)", objY.Place, objY.Direction)
	title := fmt.Sprintf("Pareto frontier — %d/%d non-dominated", resp.ParetoCount, len(resp.Samples))
	drawPlotFrame(dc, xmin, xmax, ymin, ymax, title, xLabel, yLabel, 0, 0, W, H)

	const (
		marginT = 40.0
		marginR = 140.0
		marginB = 50.0
		marginL = 70.0
	)
	plotW := float64(W) - marginL - marginR
	plotH := float64(H) - marginT - marginB
	sx := func(x float64) float64 {
		return marginL + (x-xmin)/(xmax-xmin)*plotW
	}
	sy := func(y float64) float64 {
		return marginT + plotH - (y-ymin)/(ymax-ymin)*plotH
	}

	// Dominated points first (so Pareto points overlay them).
	dc.SetHexColor("#bbbbbb")
	for _, s := range resp.Samples {
		if s.IsPareto {
			continue
		}
		dc.DrawCircle(sx(s.Values[objX.Place]), sy(s.Values[objY.Place]), 2.5)
		dc.Fill()
	}
	// Pareto frontier curve. Sort by x to draw a connected line.
	pareto := make([]optimizeSample, 0)
	for _, s := range resp.Samples {
		if s.IsPareto {
			pareto = append(pareto, s)
		}
	}
	sort.Slice(pareto, func(i, j int) bool {
		return pareto[i].Values[objX.Place] < pareto[j].Values[objX.Place]
	})
	if len(pareto) > 1 {
		dc.SetHexColor("#e64a19")
		dc.SetLineWidth(1.5)
		dc.MoveTo(sx(pareto[0].Values[objX.Place]), sy(pareto[0].Values[objY.Place]))
		for i := 1; i < len(pareto); i++ {
			dc.LineTo(sx(pareto[i].Values[objX.Place]), sy(pareto[i].Values[objY.Place]))
		}
		dc.Stroke()
	}
	// Pareto points themselves (larger, colored).
	for _, s := range pareto {
		dc.SetHexColor("#e64a19")
		dc.DrawCircle(sx(s.Values[objX.Place]), sy(s.Values[objY.Place]), 4.5)
		dc.Fill()
		dc.SetHexColor("#ffffff")
		dc.SetLineWidth(1)
		dc.DrawCircle(sx(s.Values[objX.Place]), sy(s.Values[objY.Place]), 4.5)
		dc.Stroke()
	}

	// Legend in the right column.
	legendX := marginL + plotW + 14
	legendY := marginT + 6
	if f, err := pngFace(false, 11); err == nil {
		dc.SetFontFace(f)
		dc.SetHexColor("#e64a19")
		dc.DrawCircle(legendX+8, legendY+6, 4.5)
		dc.Fill()
		dc.SetHexColor("#000000")
		dc.DrawStringAnchored("Pareto", legendX+20, legendY+6, 0, 0.5)
		legendY += 18

		dc.SetHexColor("#bbbbbb")
		dc.DrawCircle(legendX+8, legendY+6, 2.5)
		dc.Fill()
		dc.SetHexColor("#000000")
		dc.DrawStringAnchored("Dominated", legendX+20, legendY+6, 0, 0.5)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// renderOptimizeParallel draws a parallel-coordinates chart for 3+
// objectives. Each objective gets a vertical axis, normalized to [0, 1].
// Dominated samples render as faint gray polylines; Pareto-optimal as
// solid colored lines.
func renderOptimizeParallel(resp optimizeResponse) ([]byte, error) {
	const W, H = 820, 480
	dc := gg.NewContext(W, H)
	dc.SetHexColor("#ffffff")
	dc.Clear()

	n := len(resp.Objectives)
	if n < 2 {
		return nil, fmt.Errorf("parallel coordinates needs ≥2 objectives")
	}

	// Compute per-axis ranges.
	axisMin := make([]float64, n)
	axisMax := make([]float64, n)
	for i := range axisMin {
		axisMin[i] = math.Inf(1)
		axisMax[i] = math.Inf(-1)
	}
	for _, s := range resp.Samples {
		for i, obj := range resp.Objectives {
			v := s.Values[obj.Place]
			if v < axisMin[i] {
				axisMin[i] = v
			}
			if v > axisMax[i] {
				axisMax[i] = v
			}
		}
	}
	for i := range axisMin {
		if axisMax[i]-axisMin[i] < 1e-9 {
			axisMax[i] = axisMin[i] + 1
		}
	}

	const (
		marginT = 60.0
		marginB = 60.0
		marginL = 60.0
		marginR = 140.0
	)
	plotW := float64(W) - marginL - marginR
	plotH := float64(H) - marginT - marginB

	// Title
	title := fmt.Sprintf("Pareto frontier — %d/%d non-dominated", resp.ParetoCount, len(resp.Samples))
	if f, err := pngFace(true, 16); err == nil {
		dc.SetFontFace(f)
		dc.SetHexColor("#000000")
		dc.DrawStringAnchored(title, float64(W)/2, 28, 0.5, 0.5)
	}

	// Axis positions
	axisX := make([]float64, n)
	for i := 0; i < n; i++ {
		if n == 1 {
			axisX[i] = marginL + plotW/2
		} else {
			axisX[i] = marginL + plotW*float64(i)/float64(n-1)
		}
	}
	axisTop := marginT
	axisBot := marginT + plotH

	// Draw axes
	dc.SetHexColor("#333333")
	dc.SetLineWidth(1)
	for i := 0; i < n; i++ {
		dc.DrawLine(axisX[i], axisTop, axisX[i], axisBot)
		dc.Stroke()
	}

	// Axis labels at top, range labels at top and bottom
	if f, err := pngFace(true, 12); err == nil {
		dc.SetFontFace(f)
		for i, obj := range resp.Objectives {
			label := fmt.Sprintf("%s (%s)", obj.Place, obj.Direction)
			dc.SetHexColor("#000000")
			dc.DrawStringAnchored(label, axisX[i], axisTop-12, 0.5, 0.5)
		}
	}
	if f, err := pngFace(false, 9); err == nil {
		dc.SetFontFace(f)
		dc.SetHexColor("#666666")
		for i := 0; i < n; i++ {
			dc.DrawStringAnchored(formatTick(axisMax[i]), axisX[i], axisTop+8, 0.5, 1)
			dc.DrawStringAnchored(formatTick(axisMin[i]), axisX[i], axisBot+12, 0.5, 0.5)
		}
	}

	// Map a sample's objective values to axis y-coords. For "min"
	// directed objectives, invert the axis so "good" is always up.
	normalize := func(i int, v float64) float64 {
		t := (v - axisMin[i]) / (axisMax[i] - axisMin[i])
		if resp.Objectives[i].Direction == "min" {
			t = 1 - t
		}
		return axisBot - t*plotH
	}

	// Dominated polylines first (faint).
	dc.SetHexColor("#dddddd")
	dc.SetLineWidth(0.6)
	for _, s := range resp.Samples {
		if s.IsPareto {
			continue
		}
		dc.MoveTo(axisX[0], normalize(0, s.Values[resp.Objectives[0].Place]))
		for i := 1; i < n; i++ {
			dc.LineTo(axisX[i], normalize(i, s.Values[resp.Objectives[i].Place]))
		}
		dc.Stroke()
	}
	// Pareto polylines (strong, viridis-colored by index for differentiation).
	pareto := []int{}
	for idx, s := range resp.Samples {
		if s.IsPareto {
			pareto = append(pareto, idx)
		}
	}
	dc.SetLineWidth(1.5)
	for k, idx := range pareto {
		var t float64
		if len(pareto) > 1 {
			t = float64(k) / float64(len(pareto)-1)
		}
		dc.SetHexColor(viridis(t))
		s := resp.Samples[idx]
		dc.MoveTo(axisX[0], normalize(0, s.Values[resp.Objectives[0].Place]))
		for i := 1; i < n; i++ {
			dc.LineTo(axisX[i], normalize(i, s.Values[resp.Objectives[i].Place]))
		}
		dc.Stroke()
	}

	// Legend
	legendX := marginL + plotW + 14
	legendY := marginT + 6
	if f, err := pngFace(false, 11); err == nil {
		dc.SetFontFace(f)
		dc.SetHexColor("#e64a19")
		dc.SetLineWidth(2)
		dc.DrawLine(legendX, legendY+6, legendX+20, legendY+6)
		dc.Stroke()
		dc.SetHexColor("#000000")
		dc.DrawStringAnchored("Pareto", legendX+24, legendY+6, 0, 0.5)
		legendY += 18

		dc.SetHexColor("#bbbbbb")
		dc.DrawLine(legendX, legendY+6, legendX+20, legendY+6)
		dc.Stroke()
		dc.SetHexColor("#000000")
		dc.DrawStringAnchored("Dominated", legendX+24, legendY+6, 0, 0.5)
		legendY += 18

		dc.SetHexColor("#666666")
		dc.DrawStringAnchored("(min objectives", legendX, legendY+6, 0, 0.5)
		legendY += 14
		dc.DrawStringAnchored("inverted: up = good)", legendX, legendY+6, 0, 0.5)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

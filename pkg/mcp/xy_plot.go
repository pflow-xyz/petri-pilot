package mcp

import (
	"math"

	"github.com/fogleman/gg"
)

// drawXYPlot is the generic line-plot renderer that drawODEPlot and the
// rate-scan plot both delegate to. It accepts shared x values for all series
// and one y-array per series.
func drawXYPlot(dc *gg.Context, xs []float64, ys [][]float64, labels []string, title, xLabel, yLabel string, originX, originY, W, H float64) {
	if len(xs) == 0 || len(ys) == 0 {
		return
	}

	const (
		marginT = 40.0
		marginR = 140.0
		marginB = 50.0
		marginL = 70.0
	)
	plotW := W - marginL - marginR
	plotH := H - marginT - marginB

	if title != "" {
		if f, err := pngFace(true, 16); err == nil {
			dc.SetFontFace(f)
			dc.SetHexColor("#000000")
			dc.DrawStringAnchored(title, originX+W/2, originY+22, 0.5, 0.5)
		}
	}

	xmin := xs[0]
	xmax := xs[len(xs)-1]
	for _, x := range xs {
		if x < xmin {
			xmin = x
		}
		if x > xmax {
			xmax = x
		}
	}
	if xmax-xmin < 1e-12 {
		xmax = xmin + 1
	}

	ymin := math.Inf(1)
	ymax := math.Inf(-1)
	for _, series := range ys {
		for _, y := range series {
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
		ymax = ymin + 1
		yrange = 1
	}
	ymin -= yrange * 0.1
	ymax += yrange * 0.1

	left := originX + marginL
	top := originY + marginT
	right := left + plotW
	bottom := top + plotH

	sx := func(x float64) float64 {
		return left + (x-xmin)/(xmax-xmin)*plotW
	}
	sy := func(y float64) float64 {
		return bottom - (y-ymin)/(ymax-ymin)*plotH
	}

	const nTicks = 5

	dc.SetHexColor("#dddddd")
	dc.SetLineWidth(0.5)
	for i := 0; i <= nTicks; i++ {
		x := xmin + (xmax-xmin)*float64(i)/nTicks
		px := sx(x)
		dc.DrawLine(px, top, px, bottom)
		dc.Stroke()
	}
	for i := 0; i <= nTicks; i++ {
		y := ymin + (ymax-ymin)*float64(i)/nTicks
		py := sy(y)
		dc.DrawLine(left, py, right, py)
		dc.Stroke()
	}

	dc.SetHexColor("#333333")
	dc.SetLineWidth(2)
	dc.DrawLine(left, top, left, bottom)
	dc.Stroke()
	dc.DrawLine(left, bottom, right, bottom)
	dc.Stroke()

	if f, err := pngFace(false, 10); err == nil {
		dc.SetFontFace(f)
		dc.SetHexColor("#000000")
		for i := 0; i <= nTicks; i++ {
			x := xmin + (xmax-xmin)*float64(i)/nTicks
			dc.DrawStringAnchored(formatTick(x), sx(x), bottom+14, 0.5, 0.5)
		}
		for i := 0; i <= nTicks; i++ {
			y := ymin + (ymax-ymin)*float64(i)/nTicks
			dc.DrawStringAnchored(formatTick(y), left-8, sy(y), 1.0, 0.5)
		}
	}

	if f, err := pngFace(false, 12); err == nil {
		dc.SetFontFace(f)
		dc.SetHexColor("#000000")
		dc.DrawStringAnchored(xLabel, (left+right)/2, originY+H-15, 0.5, 0.5)
		dc.Push()
		yAxisX := originX + 18
		dc.RotateAbout(-math.Pi/2, yAxisX, (top+bottom)/2)
		dc.DrawStringAnchored(yLabel, yAxisX, (top+bottom)/2, 0.5, 0.5)
		dc.Pop()
	}

	dc.SetLineWidth(2)
	for i, series := range ys {
		if len(series) == 0 {
			continue
		}
		dc.SetHexColor(plotColors[i%len(plotColors)])
		dc.MoveTo(sx(xs[0]), sy(series[0]))
		for j := 1; j < len(series); j++ {
			dc.LineTo(sx(xs[j]), sy(series[j]))
		}
		dc.Stroke()
	}

	legendX := right + 14
	legendY := top + 6
	if f, err := pngFace(false, 11); err == nil {
		dc.SetFontFace(f)
		for i, label := range labels {
			dc.SetHexColor(plotColors[i%len(plotColors)])
			dc.SetLineWidth(2)
			dc.DrawLine(legendX, legendY+6, legendX+20, legendY+6)
			dc.Stroke()
			dc.SetHexColor("#000000")
			dc.DrawStringAnchored(label, legendX+24, legendY+6, 0, 0.5)
			legendY += 18
		}
	}
}

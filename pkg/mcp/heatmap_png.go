package mcp

import (
	"bytes"
	"fmt"
	"image/png"
	"math"

	"github.com/fogleman/gg"
	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"
)

// Heatmap renderer for high-place models (TTT boards, zk-ode topologies,
// general grid-structured nets). Lays out places in a 2D grid colored by
// value with a viridis-like colormap. The default is to read the marking
// from Place.Initial; callers can override with a marking map.

// HeatmapOpts customizes a heatmap render.
type HeatmapOpts struct {
	Title   string
	Marking map[string]float64
	Rows    int  // 0 = auto
	Cols    int  // 0 = auto
	Labels  bool // show place ID + value in each cell (default true)
}

func renderHeatmapPNG(model *goflowmetamodel.Model, opts *HeatmapOpts) ([]byte, error) {
	if opts == nil {
		opts = &HeatmapOpts{Labels: true}
	}
	n := len(model.Places)
	if n == 0 {
		return nil, fmt.Errorf("no places to render")
	}

	rows, cols := opts.Rows, opts.Cols
	if rows <= 0 || cols <= 0 {
		rows, cols = autoGrid(n)
	}
	if rows*cols < n {
		// User-supplied dims too small; auto.
		rows, cols = autoGrid(n)
	}

	// Resolve marking values
	values := make([]float64, n)
	ids := make([]string, n)
	minV, maxV := math.Inf(1), math.Inf(-1)
	for i, p := range model.Places {
		ids[i] = p.ID
		v := float64(p.Initial)
		if opts.Marking != nil {
			if mv, ok := opts.Marking[p.ID]; ok {
				v = mv
			}
		}
		values[i] = v
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	if math.IsInf(minV, 1) {
		minV = 0
		maxV = 1
	}
	// Avoid divide-by-zero on constant fields — make the whole grid mid-range.
	if maxV-minV < 1e-12 {
		minV = maxV - 1
	}

	const (
		cellMin = 80.0
		cellMax = 140.0
		margin  = 40.0
		titleH  = 30.0
		legendW = 90.0
	)

	// Aim for cells ~100px square, clamped.
	cell := 100.0
	if cell < cellMin {
		cell = cellMin
	}
	if cell > cellMax {
		cell = cellMax
	}
	gridW := cell * float64(cols)
	gridH := cell * float64(rows)
	W := int(margin*2 + gridW + legendW)
	H := int(titleH + margin*2 + gridH)

	dc := gg.NewContext(W, H)
	dc.SetHexColor("#ffffff")
	dc.Clear()

	if opts.Title != "" {
		drawTitle(dc, opts.Title, float64(W)/2, titleH/2+8)
	}

	originX := margin
	originY := titleH + margin

	// Cells
	for i := 0; i < n; i++ {
		r := i / cols
		c := i % cols
		x := originX + float64(c)*cell
		y := originY + float64(r)*cell
		v := values[i]
		t := (v - minV) / (maxV - minV)
		dc.SetHexColor(viridis(t))
		dc.DrawRectangle(x, y, cell, cell)
		dc.Fill()
		dc.SetHexColor("#ffffff")
		dc.SetLineWidth(1)
		dc.DrawRectangle(x, y, cell, cell)
		dc.Stroke()

		if opts.Labels {
			// Pick text color for contrast: dark on bright cells (high t),
			// light on dark cells (low t).
			textColor := "#ffffff"
			if t > 0.55 {
				textColor = "#000000"
			}
			if f, err := pngFace(true, 12); err == nil {
				dc.SetFontFace(f)
				dc.SetHexColor(textColor)
				dc.DrawStringAnchored(ids[i], x+cell/2, y+cell/2-8, 0.5, 0.5)
			}
			if f, err := pngFace(false, 11); err == nil {
				dc.SetFontFace(f)
				dc.SetHexColor(textColor)
				dc.DrawStringAnchored(formatMarkingValue(v), x+cell/2, y+cell/2+10, 0.5, 0.5)
			}
		}
	}

	// Color bar legend
	legendX := originX + gridW + 24
	legendBarW := 16.0
	legendBarH := gridH
	steps := 64
	for i := 0; i < steps; i++ {
		t := float64(i) / float64(steps-1)
		dc.SetHexColor(viridis(t))
		y := originY + legendBarH - float64(i+1)*legendBarH/float64(steps)
		dc.DrawRectangle(legendX, y, legendBarW, legendBarH/float64(steps)+0.5)
		dc.Fill()
	}
	if f, err := pngFace(false, 10); err == nil {
		dc.SetFontFace(f)
		dc.SetHexColor("#000000")
		dc.DrawStringAnchored(formatMarkingValue(maxV), legendX+legendBarW+4, originY, 0, 0.5)
		dc.DrawStringAnchored(formatMarkingValue(minV), legendX+legendBarW+4, originY+legendBarH, 0, 0.5)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// autoGrid picks rows and cols whose product is >= n, preferring near-square
// layouts. For perfect squares (1, 4, 9, 16, 25, …) returns exact rows=cols.
func autoGrid(n int) (int, int) {
	if n <= 0 {
		return 0, 0
	}
	cols := int(math.Ceil(math.Sqrt(float64(n))))
	rows := int(math.Ceil(float64(n) / float64(cols)))
	return rows, cols
}

// viridis returns a hex color along an approximate viridis colormap for
// t in [0,1]. Five anchor points sampled from matplotlib's viridis.
func viridis(t float64) string {
	t = math.Max(0, math.Min(1, t))
	anchors := []struct {
		t          float64
		r, g, b int
	}{
		{0.0, 0x44, 0x01, 0x54},
		{0.25, 0x3b, 0x52, 0x8b},
		{0.5, 0x21, 0x90, 0x8c},
		{0.75, 0x5e, 0xc9, 0x62},
		{1.0, 0xfd, 0xe7, 0x25},
	}
	for i := 0; i < len(anchors)-1; i++ {
		a, b := anchors[i], anchors[i+1]
		if t >= a.t && t <= b.t {
			k := (t - a.t) / (b.t - a.t)
			r := int(float64(a.r) + k*float64(b.r-a.r))
			g := int(float64(a.g) + k*float64(b.g-a.g))
			bl := int(float64(a.b) + k*float64(b.b-a.b))
			return fmt.Sprintf("#%02x%02x%02x", r, g, bl)
		}
	}
	last := anchors[len(anchors)-1]
	return fmt.Sprintf("#%02x%02x%02x", last.r, last.g, last.b)
}

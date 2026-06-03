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

	// Decide grid placement. If every place has a natural X/Y, bin into
	// (row, col) based on the model's bounding box. Otherwise fall back to
	// iteration order. Collisions (two places landing in the same cell)
	// drift to the next free cell — the user can pass rows/cols to force
	// a different binning if the autopick is too coarse.
	cellOf := flatGridPlacement(n, rows, cols)
	if placement := positionBinPlacement(model, rows, cols); placement != nil {
		cellOf = placement
	}

	for i := 0; i < n; i++ {
		rc := cellOf[i]
		if rc[0] < 0 {
			continue
		}
		r := rc[0]
		c := rc[1]
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

// flatGridPlacement assigns each place to (row, col) by iteration order.
func flatGridPlacement(n, rows, cols int) [][2]int {
	out := make([][2]int, n)
	for i := 0; i < n; i++ {
		if i >= rows*cols {
			out[i] = [2]int{-1, -1}
			continue
		}
		out[i] = [2]int{i / cols, i % cols}
	}
	return out
}

// positionBinPlacement returns a placement that bins places into the grid by
// their explicit X/Y coordinates. Returns nil if any place lacks a position
// (in which case the caller falls back to flat iteration order). When two
// places bin to the same cell, the second drifts to the nearest empty cell
// (BFS over neighbours).
func positionBinPlacement(model *goflowmetamodel.Model, rows, cols int) [][2]int {
	n := len(model.Places)
	for _, p := range model.Places {
		if p.X == 0 && p.Y == 0 {
			return nil
		}
	}
	minX, maxX := math.Inf(1), math.Inf(-1)
	minY, maxY := math.Inf(1), math.Inf(-1)
	for _, p := range model.Places {
		x, y := float64(p.X), float64(p.Y)
		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
	}
	// Pad the bounding box so the max value doesn't snap to cell N.
	xRange := maxX - minX
	yRange := maxY - minY
	if xRange == 0 {
		xRange = 1
	}
	if yRange == 0 {
		yRange = 1
	}

	occupied := make(map[[2]int]bool, n)
	out := make([][2]int, n)
	for i, p := range model.Places {
		col := int(float64(cols) * (float64(p.X) - minX) / xRange)
		if col >= cols {
			col = cols - 1
		}
		row := int(float64(rows) * (float64(p.Y) - minY) / yRange)
		if row >= rows {
			row = rows - 1
		}
		// Drift to nearest empty cell on collision.
		rc := [2]int{row, col}
		if occupied[rc] {
			rc = nearestFreeCell(rc, rows, cols, occupied)
		}
		occupied[rc] = true
		out[i] = rc
	}
	return out
}

// nearestFreeCell BFS-walks neighbours of `start` to find the closest unoccupied
// cell within (rows, cols). Returns start if nothing is free (shouldn't happen
// when rows*cols >= n).
func nearestFreeCell(start [2]int, rows, cols int, occupied map[[2]int]bool) [2]int {
	visited := map[[2]int]bool{start: true}
	queue := [][2]int{start}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}, {-1, -1}, {-1, 1}, {1, -1}, {1, 1}} {
			n := [2]int{cur[0] + d[0], cur[1] + d[1]}
			if n[0] < 0 || n[0] >= rows || n[1] < 0 || n[1] >= cols {
				continue
			}
			if visited[n] {
				continue
			}
			visited[n] = true
			if !occupied[n] {
				return n
			}
			queue = append(queue, n)
		}
	}
	return start
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

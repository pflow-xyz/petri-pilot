package mcp

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"
	"math"
	"sort"
	"sync"

	"github.com/fogleman/gg"
	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
)

// PNG rasterizer for petri_visualize. Mirrors the layout produced by
// generateSVG so the two reads of the same model agree visually — clients
// that render images get the PNG inline, agents that only read text still
// see the SVG.
//
// drawNet is the reusable core. It accepts a gg.Context, a model, options
// (title, marking override, per-element shading), and target bounds. The
// top-level renderPNG wraps a single-net image; combined views (net + ODE
// plot side-by-side) call drawNet directly into a wider context.

const placeR = 25.0

// RenderOpts customizes the net rendering: a title above the canvas, a
// marking override (replaces Place.Initial labels), and a per-element shading
// map keyed by ID. ShadeKind selects the color palette (currently only
// "sensitivity" — a gray → red gradient — or "marking" for blue-saturation).
//
// Highlight is a per-element explicit fill color (hex). It takes precedence
// over Shading and is the path used by the visual diff (added=green,
// removed=red).
type RenderOpts struct {
	Title     string
	Marking   map[string]float64
	Shading   map[string]float64
	ShadeKind string
	Highlight map[string]string
}

var (
	pngRegularFont *opentype.Font
	pngBoldFont    *opentype.Font
	pngFontsOnce   sync.Once
	pngFontsErr    error
)

func loadPngFonts() {
	pngRegularFont, pngFontsErr = opentype.Parse(goregular.TTF)
	if pngFontsErr != nil {
		return
	}
	pngBoldFont, pngFontsErr = opentype.Parse(gobold.TTF)
}

func pngFace(bold bool, size float64) (font.Face, error) {
	pngFontsOnce.Do(loadPngFonts)
	if pngFontsErr != nil {
		return nil, pngFontsErr
	}
	tt := pngRegularFont
	if bold {
		tt = pngBoldFont
	}
	return opentype.NewFace(tt, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
}

// netNaturalSize returns the (width, height) the net would prefer when
// rendered with no scaling — derived from explicit positions if any, else
// the auto-layout grid.
func netNaturalSize(model *goflowmetamodel.Model) (int, int) {
	hasPositions := false
	maxX, maxY := 0, 0
	for _, p := range model.Places {
		if p.X != 0 || p.Y != 0 {
			hasPositions = true
			if p.X > maxX {
				maxX = p.X
			}
			if p.Y > maxY {
				maxY = p.Y
			}
		}
	}
	for _, t := range model.Transitions {
		if t.X != 0 || t.Y != 0 {
			hasPositions = true
			if t.X > maxX {
				maxX = t.X
			}
			if t.Y > maxY {
				maxY = t.Y
			}
		}
	}
	const spacing = 120
	var w, h int
	if hasPositions {
		w = maxX + 100
		h = maxY + 100
	} else {
		n := len(model.Places)
		if len(model.Transitions) > n {
			n = len(model.Transitions)
		}
		w = n*spacing + 100
		h = 250
	}
	if w < 200 {
		w = 200
	}
	if h < 200 {
		h = 200
	}
	return w, h
}

func renderPNG(model *goflowmetamodel.Model) ([]byte, error) {
	return renderPNGWithOpts(model, nil)
}

// renderPNGWithOpts is the option-aware top-level renderer. Creates a fresh
// gg.Context at the net's natural size and delegates to drawNet.
func renderPNGWithOpts(model *goflowmetamodel.Model, opts *RenderOpts) ([]byte, error) {
	w, h := netNaturalSize(model)
	titleH := 0
	if opts != nil && opts.Title != "" {
		titleH = 28
	}
	dc := gg.NewContext(w, h+titleH)
	dc.SetHexColor("#ffffff")
	dc.Clear()
	if titleH > 0 {
		drawTitle(dc, opts.Title, float64(w)/2, 18)
	}
	drawNet(dc, model, opts, 0, float64(titleH), float64(w), float64(h))
	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// renderPNGBase64 wraps renderPNG with std base64 encoding for direct
// embedding in an MCP ImageContent block.
func renderPNGBase64(model *goflowmetamodel.Model) (string, error) {
	b, err := renderPNG(model)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// drawNet draws the model into the given gg.Context, scaled to fit
// (originX, originY, width, height). When width/height are larger than the
// model's natural size, the net is centered; when smaller, it's scaled down
// to fit. Used both standalone and inside composite layouts.
func drawNet(dc *gg.Context, model *goflowmetamodel.Model, opts *RenderOpts, originX, originY, width, height float64) {
	natW, natH := netNaturalSize(model)
	scale := math.Min(width/float64(natW), height/float64(natH))
	if scale > 1 {
		scale = 1
	}
	// Center after scaling
	offX := originX + (width-float64(natW)*scale)/2
	offY := originY + (height-float64(natH)*scale)/2

	tx := func(x int) float64 { return offX + float64(x)*scale }
	ty := func(y int) float64 { return offY + float64(y)*scale }

	const spacing = 120
	const (
		defaultPlaceY = 50
		defaultTransY = 150
	)

	placePos := make(map[string][2]int, len(model.Places))
	for i, p := range model.Places {
		x, y := p.X, p.Y
		if x == 0 && y == 0 {
			x = 50 + i*spacing
			y = defaultPlaceY
		}
		placePos[p.ID] = [2]int{x, y}
	}
	transPos := make(map[string][2]int, len(model.Transitions))
	for i, t := range model.Transitions {
		x, y := t.X, t.Y
		if x == 0 && y == 0 {
			x = 50 + i*spacing
			y = defaultTransY
		}
		transPos[t.ID] = [2]int{x, y}
	}

	pr := placeR * scale

	// Arcs (under nodes so endpoints disappear behind the shape gap correctly).
	for _, arc := range model.Arcs {
		var (
			p1, p2      [2]int
			fromIsPlace bool
			toIsPlace   bool
			ok1, ok2    bool
		)
		if pos, ok := placePos[arc.From]; ok {
			p1, ok1, fromIsPlace = pos, true, true
		} else if pos, ok := transPos[arc.From]; ok {
			p1, ok1 = pos, true
		}
		if pos, ok := placePos[arc.To]; ok {
			p2, ok2, toIsPlace = pos, true, true
		} else if pos, ok := transPos[arc.To]; ok {
			p2, ok2 = pos, true
		}
		if !ok1 || !ok2 {
			continue
		}
		fx, fy := tx(p1[0]), ty(p1[1])
		txp, typ := tx(p2[0]), ty(p2[1])
		sx, sy := edgePoint(fx, fy, txp, typ, fromIsPlace, scale)
		ex, ey := edgePoint(txp, typ, fx, fy, toIsPlace, scale)
		arcKey := arcID(arc.From, arc.To)
		drawArrow(dc, sx, sy, ex, ey, arcShadeColor(opts, arcKey), scale)
	}

	// Places.
	labelSize := math.Max(8, 12*scale)
	for _, p := range model.Places {
		pos := placePos[p.ID]
		x, y := tx(pos[0]), ty(pos[1])
		fill, stroke := placeColors(opts, p.ID)
		dc.SetHexColor(fill)
		dc.DrawCircle(x, y, pr)
		dc.Fill()
		dc.SetHexColor(stroke)
		dc.SetLineWidth(2 * scale)
		dc.DrawCircle(x, y, pr)
		dc.Stroke()
		if f, err := pngFace(false, labelSize); err == nil {
			dc.SetFontFace(f)
			dc.SetHexColor("#000000")
			dc.DrawStringAnchored(p.ID, x, y+pr+12*scale, 0.5, 0.5)
		}
		if label := placeValueLabel(opts, p); label != "" {
			if f, err := pngFace(true, labelSize); err == nil {
				dc.SetFontFace(f)
				dc.SetHexColor("#000000")
				dc.DrawStringAnchored(label, x, y, 0.5, 0.5)
			}
		}
	}

	// Transitions.
	for _, t := range model.Transitions {
		pos := transPos[t.ID]
		x, y := tx(pos[0]), ty(pos[1])
		dc.SetHexColor(transitionFillColor(opts, t.ID))
		dc.DrawRectangle(x-5*scale, y-20*scale, 10*scale, 40*scale)
		dc.Fill()
		if f, err := pngFace(false, labelSize); err == nil {
			dc.SetFontFace(f)
			dc.SetHexColor("#000000")
			dc.DrawStringAnchored(t.ID, x, y+22*scale+10*scale, 0.5, 0.5)
		}
	}
}

func arcID(from, to string) string { return from + "->" + to }

// placeColors returns the fill and stroke colors for a place, taking
// Highlight (explicit color) and shading (palette mapping) into account.
func placeColors(opts *RenderOpts, id string) (string, string) {
	defaultFill, defaultStroke := "#e3f2fd", "#1976d2"
	if opts == nil {
		return defaultFill, defaultStroke
	}
	if c, ok := opts.Highlight[id]; ok {
		return c, defaultStroke
	}
	if v, ok := shade01(opts, id); ok {
		switch opts.ShadeKind {
		case "sensitivity":
			return sensitivityFill(v), "#1976d2"
		case "marking":
			return markingFill(v), "#1976d2"
		}
	}
	return defaultFill, defaultStroke
}

// transitionFillColor returns the fill color for a transition bar; tints
// when Highlight or shading data is available.
func transitionFillColor(opts *RenderOpts, id string) string {
	if opts == nil {
		return "#333333"
	}
	if c, ok := opts.Highlight[id]; ok {
		return c
	}
	if v, ok := shade01(opts, id); ok && opts.ShadeKind == "sensitivity" {
		return sensitivityFill(v)
	}
	return "#333333"
}

// arcShadeColor returns the stroke color for an arc; tints when Highlight
// or shading is available for the arc by key "from->to".
func arcShadeColor(opts *RenderOpts, key string) string {
	if opts == nil {
		return "#333333"
	}
	if c, ok := opts.Highlight[key]; ok {
		return c
	}
	if v, ok := shade01(opts, key); ok && opts.ShadeKind == "sensitivity" {
		return sensitivityFill(v)
	}
	return "#333333"
}

// shade01 returns the [0,1] shading value for a key if present.
func shade01(opts *RenderOpts, key string) (float64, bool) {
	if opts == nil || opts.Shading == nil {
		return 0, false
	}
	v, ok := opts.Shading[key]
	return v, ok
}

// placeValueLabel returns the bold center label for a place: either the
// override from Marking (formatted as a number) or Place.Initial when >0.
func placeValueLabel(opts *RenderOpts, p goflowmetamodel.Place) string {
	if opts != nil && opts.Marking != nil {
		if v, ok := opts.Marking[p.ID]; ok {
			return formatMarkingValue(v)
		}
	}
	if p.Initial > 0 {
		return fmt.Sprintf("%d", p.Initial)
	}
	return ""
}

func formatMarkingValue(v float64) string {
	// Solver noise (3e-8, 4e-7, etc.) shouldn't read as "still active" — round
	// anything below 1e-3 to a flat "0". Real concentrations in our models are
	// O(1), so 1e-3 is comfortably below the signal floor.
	if math.Abs(v) < 1e-3 {
		return "0"
	}
	if math.Abs(v) >= 100 || v == math.Trunc(v) {
		return fmt.Sprintf("%.0f", v)
	}
	if math.Abs(v) >= 10 {
		return fmt.Sprintf("%.1f", v)
	}
	return fmt.Sprintf("%.2f", v)
}

// sensitivityFill is a gray → red gradient. v=0 is light gray, v=1 is red.
func sensitivityFill(v float64) string {
	v = math.Max(0, math.Min(1, v))
	// Interpolate from #eeeeee → #d32f2f.
	r0, g0, b0 := 0xee, 0xee, 0xee
	r1, g1, b1 := 0xd3, 0x2f, 0x2f
	r := int(float64(r0) + v*float64(r1-r0))
	g := int(float64(g0) + v*float64(g1-g0))
	b := int(float64(b0) + v*float64(b1-b0))
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

// markingFill is a white → saturated-blue gradient.
func markingFill(v float64) string {
	v = math.Max(0, math.Min(1, v))
	r0, g0, b0 := 0xff, 0xff, 0xff
	r1, g1, b1 := 0x19, 0x76, 0xd2
	r := int(float64(r0) + v*float64(r1-r0))
	g := int(float64(g0) + v*float64(g1-g0))
	b := int(float64(b0) + v*float64(b1-b0))
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

// edgePoint returns the boundary point of the node at (cx, cy) along the ray
// toward (tx, ty), padded by a small gap so the arrowhead reads as separate
// from the shape. isPlace selects circle (placeR) vs transition bar (5x20).
// scale is the active rendering scale (used to keep the gap visually
// constant in scaled-down composites).
func edgePoint(cx, cy, tx, ty float64, isPlace bool, scale float64) (float64, float64) {
	dx, dy := tx-cx, ty-cy
	if dx == 0 && dy == 0 {
		return cx, cy
	}
	gap := 3.0 * scale
	if isPlace {
		L := math.Hypot(dx, dy)
		s := (placeR*scale + gap) / L
		return cx + dx*s, cy + dy*s
	}
	hw, hh := 5.0*scale, 20.0*scale
	sx, sy := math.Inf(1), math.Inf(1)
	if dx != 0 {
		sx = hw / math.Abs(dx)
	}
	if dy != 0 {
		sy = hh / math.Abs(dy)
	}
	s := math.Min(sx, sy)
	L := math.Hypot(dx, dy)
	s += gap / L
	return cx + dx*s, cy + dy*s
}

func drawArrow(dc *gg.Context, x1, y1, x2, y2 float64, color string, scale float64) {
	dc.SetHexColor(color)
	dc.SetLineWidth(1.5 * scale)
	dc.DrawLine(x1, y1, x2, y2)
	dc.Stroke()

	dx, dy := x2-x1, y2-y1
	length := math.Sqrt(dx*dx + dy*dy)
	if length == 0 {
		return
	}
	ux, uy := dx/length, dy/length
	ah := 10.0 * scale
	const cosT, sinT = 0.866, 0.5 // 30°
	bx := x2 - ah*(ux*cosT-uy*sinT)
	by := y2 - ah*(uy*cosT+ux*sinT)
	cx := x2 - ah*(ux*cosT+uy*sinT)
	cy := y2 - ah*(uy*cosT-ux*sinT)
	dc.MoveTo(x2, y2)
	dc.LineTo(bx, by)
	dc.LineTo(cx, cy)
	dc.ClosePath()
	dc.Fill()
}

func drawTitle(dc *gg.Context, title string, x, y float64) {
	if f, err := pngFace(true, 16); err == nil {
		dc.SetFontFace(f)
		dc.SetHexColor("#000000")
		dc.DrawStringAnchored(title, x, y, 0.5, 0.5)
	}
}

// normalizeShading rescales a raw importance/marking map to [0, 1] by
// dividing by max. Keys with the largest value get 1.0; zero/negative get 0.
// Returned map is keyed identically to the input.
func normalizeShading(raw map[string]float64) map[string]float64 {
	if len(raw) == 0 {
		return nil
	}
	maxV := 0.0
	for _, v := range raw {
		if v > maxV {
			maxV = v
		}
	}
	if maxV <= 0 {
		return nil
	}
	out := make(map[string]float64, len(raw))
	for k, v := range raw {
		if v < 0 {
			v = 0
		}
		out[k] = v / maxV
	}
	return out
}

// sortedKeys is a tiny helper for deterministic test output.
func sortedKeys(m map[string]float64) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

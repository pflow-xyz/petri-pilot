package mcp

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"
	"math"
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

const placeR = 25.0

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

func renderPNG(model *goflowmetamodel.Model) ([]byte, error) {
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
	var width, height int
	if hasPositions {
		width = maxX + 100
		height = maxY + 100
	} else {
		n := len(model.Places)
		if len(model.Transitions) > n {
			n = len(model.Transitions)
		}
		width = n*spacing + 100
		height = 250
	}
	if width < 200 {
		width = 200
	}
	if height < 200 {
		height = 200
	}

	dc := gg.NewContext(width, height)
	dc.SetHexColor("#ffffff")
	dc.Clear()

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

	for _, arc := range model.Arcs {
		var (
			p1, p2           [2]int
			fromIsPlace      bool
			toIsPlace        bool
			ok1, ok2         bool
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
		fx, fy := float64(p1[0]), float64(p1[1])
		tx, ty := float64(p2[0]), float64(p2[1])
		// Pull each endpoint inward to the node boundary (plus a small gap)
		// so the line/arrowhead don't disappear under the shape.
		sx, sy := edgePoint(fx, fy, tx, ty, fromIsPlace)
		ex, ey := edgePoint(tx, ty, fx, fy, toIsPlace)
		drawArrow(dc, sx, sy, ex, ey)
	}

	for _, p := range model.Places {
		pos := placePos[p.ID]
		x, y := float64(pos[0]), float64(pos[1])
		dc.SetHexColor("#e3f2fd")
		dc.DrawCircle(x, y, placeR)
		dc.Fill()
		dc.SetHexColor("#1976d2")
		dc.SetLineWidth(2)
		dc.DrawCircle(x, y, placeR)
		dc.Stroke()
		if f, err := pngFace(false, 12); err == nil {
			dc.SetFontFace(f)
			dc.SetHexColor("#000000")
			dc.DrawStringAnchored(p.ID, x, y+45, 0.5, 0.5)
		}
		if p.Initial > 0 {
			if f, err := pngFace(true, 12); err == nil {
				dc.SetFontFace(f)
				dc.SetHexColor("#000000")
				dc.DrawStringAnchored(fmt.Sprintf("%d", p.Initial), x, y, 0.5, 0.5)
			}
		}
	}

	for _, t := range model.Transitions {
		pos := transPos[t.ID]
		x, y := float64(pos[0]), float64(pos[1])
		dc.SetHexColor("#333333")
		dc.DrawRectangle(x-5, y-20, 10, 40)
		dc.Fill()
		if f, err := pngFace(false, 12); err == nil {
			dc.SetFontFace(f)
			dc.SetHexColor("#000000")
			dc.DrawStringAnchored(t.ID, x, y+40, 0.5, 0.5)
		}
	}

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

// edgePoint returns the boundary point of the node at (cx, cy) along the ray
// toward (tx, ty), padded by a small gap so the arrowhead reads as separate
// from the shape. isPlace selects circle (placeR) vs transition bar (5x20).
func edgePoint(cx, cy, tx, ty float64, isPlace bool) (float64, float64) {
	dx, dy := tx-cx, ty-cy
	if dx == 0 && dy == 0 {
		return cx, cy
	}
	const gap = 3.0
	if isPlace {
		L := math.Hypot(dx, dy)
		s := (placeR + gap) / L
		return cx + dx*s, cy + dy*s
	}
	// Transition: rectangle of half-width 5, half-height 20. Pick the smaller
	// scale that hits an edge along the ray, then add a gap on the same ray.
	const hw, hh = 5.0, 20.0
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

func drawArrow(dc *gg.Context, x1, y1, x2, y2 float64) {
	dc.SetHexColor("#333333")
	dc.SetLineWidth(1.5)
	dc.DrawLine(x1, y1, x2, y2)
	dc.Stroke()

	dx, dy := x2-x1, y2-y1
	length := math.Sqrt(dx*dx + dy*dy)
	if length == 0 {
		return
	}
	ux, uy := dx/length, dy/length
	const ah = 10.0
	const cosT, sinT = 0.866, 0.5 // 30°
	bx := x2 - ah*(ux*cosT-uy*sinT)
	by := y2 - ah*(uy*cosT+ux*sinT)
	cx := x2 - ah*(ux*cosT+uy*sinT)
	cy := y2 - ah*(uy*cosT-ux*sinT)
	dc.SetHexColor("#333333")
	dc.MoveTo(x2, y2)
	dc.LineTo(bx, by)
	dc.LineTo(cx, cy)
	dc.ClosePath()
	dc.Fill()
}

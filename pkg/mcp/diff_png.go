package mcp

import (
	"bytes"
	"image/png"

	"github.com/fogleman/gg"
	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"
)

// Diff colors mirror typical added/removed conventions in code review.
const (
	diffAddedColor   = "#43a047" // green
	diffRemovedColor = "#d32f2f" // red
)

// renderDiffPNG draws model A and model B side-by-side. Removed elements
// (present in A, absent in B) are highlighted red on the left; added
// elements (present in B, absent in A) are highlighted green on the right.
// Unchanged elements use the default style on both panels. Title defaults
// to "petri_diff" when empty.
func renderDiffPNG(modelA, modelB *goflowmetamodel.Model, diff ModelDiff) ([]byte, error) {
	return renderDiffPNGTitled(modelA, modelB, diff, "petri_diff")
}

func renderDiffPNGTitled(modelA, modelB *goflowmetamodel.Model, diff ModelDiff, title string) ([]byte, error) {
	// Build highlight maps per side.
	leftHL := map[string]string{}
	for _, id := range diff.PlacesRemoved {
		leftHL[id] = diffRemovedColor
	}
	for _, id := range diff.TransitionsRemoved {
		leftHL[id] = diffRemovedColor
	}
	for _, key := range diff.ArcsRemoved {
		leftHL[key] = diffRemovedColor
	}

	rightHL := map[string]string{}
	for _, id := range diff.PlacesAdded {
		rightHL[id] = diffAddedColor
	}
	for _, id := range diff.TransitionsAdded {
		rightHL[id] = diffAddedColor
	}
	for _, key := range diff.ArcsAdded {
		rightHL[key] = diffAddedColor
	}

	// Each panel gets the larger of the two models' natural widths so the
	// two sides line up visually even when only one has added a column.
	natWA, natHA := netNaturalSize(modelA)
	natWB, natHB := netNaturalSize(modelB)
	panelW := natWA
	if natWB > panelW {
		panelW = natWB
	}
	panelH := natHA
	if natHB > panelH {
		panelH = natHB
	}

	const (
		titleH    = 32
		subtitleH = 22
		gap       = 16
		legendH   = 28
		bottomPad = 8
	)
	W := panelW*2 + gap
	H := titleH + subtitleH + panelH + legendH + bottomPad

	dc := gg.NewContext(W, H)
	dc.SetHexColor("#ffffff")
	dc.Clear()

	drawTitle(dc, title, float64(W)/2, float64(titleH)/2+4)

	// Subtitles for each panel.
	if f, err := pngFace(true, 13); err == nil {
		dc.SetFontFace(f)
		dc.SetHexColor("#d32f2f")
		dc.DrawStringAnchored("A (red = removed)", float64(panelW)/2, float64(titleH)+float64(subtitleH)/2, 0.5, 0.5)
		dc.SetHexColor("#43a047")
		dc.DrawStringAnchored("B (green = added)", float64(panelW)+float64(gap)+float64(panelW)/2, float64(titleH)+float64(subtitleH)/2, 0.5, 0.5)
	}

	topPad := float64(titleH + subtitleH)
	drawNet(dc, modelA, &RenderOpts{Highlight: leftHL}, 0, topPad, float64(panelW), float64(panelH))
	drawNet(dc, modelB, &RenderOpts{Highlight: rightHL}, float64(panelW+gap), topPad, float64(panelW), float64(panelH))

	// Bottom legend: counts of additions/removals.
	if f, err := pngFace(false, 11); err == nil {
		dc.SetFontFace(f)
		dc.SetHexColor("#000000")
		legendY := topPad + float64(panelH) + float64(legendH)/2
		summary := diffSummaryLine(diff)
		dc.DrawStringAnchored(summary, float64(W)/2, legendY, 0.5, 0.5)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func diffSummaryLine(d ModelDiff) string {
	if !d.HasChanges {
		return "no changes"
	}
	parts := []string{}
	if n := len(d.PlacesAdded); n > 0 {
		parts = append(parts, plural(n, "place")+" added")
	}
	if n := len(d.PlacesRemoved); n > 0 {
		parts = append(parts, plural(n, "place")+" removed")
	}
	if n := len(d.TransitionsAdded); n > 0 {
		parts = append(parts, plural(n, "transition")+" added")
	}
	if n := len(d.TransitionsRemoved); n > 0 {
		parts = append(parts, plural(n, "transition")+" removed")
	}
	if n := len(d.ArcsAdded); n > 0 {
		parts = append(parts, plural(n, "arc")+" added")
	}
	if n := len(d.ArcsRemoved); n > 0 {
		parts = append(parts, plural(n, "arc")+" removed")
	}
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "  ·  "
		}
		out += p
	}
	return out
}

func plural(n int, s string) string {
	if n == 1 {
		return "1 " + s
	}
	return formatInt(n) + " " + s + "s"
}

func formatInt(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}

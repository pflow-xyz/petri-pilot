package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"math"
	"sort"

	"github.com/fogleman/gg"
	"github.com/mark3labs/mcp-go/mcp"
)

// petri_corr_matrix renders a correlation dict (or full matrix) as a
// symmetric heatmap. Useful as a pre-flight check on SDE inputs and as a
// standalone view that doesn't reference the Petri net at all.

func corrMatrixTool() mcp.Tool {
	return mcp.NewTool("petri_corr_matrix",
		mcp.WithDescription("Render a correlation matrix as a heatmap. Useful as a pre-flight check on petri_sde correlation inputs and as a general visualization of asset/observable relationships. Accepts pairwise rho format or a full matrix array."),
		mcp.WithString("correlations",
			mcp.Description(`JSON object of pairwise correlations keyed by "A-B" with values in [-1, 1]. Place IDs/names are extracted from the keys. e.g. {"btc-eth": 0.85, "btc-sol": 0.7, "eth-sol": 0.75}. Alternative to 'matrix'`),
		),
		mcp.WithString("matrix",
			mcp.Description(`JSON nested array (full NxN matrix). Alternative to 'correlations'`),
		),
		mcp.WithString("labels",
			mcp.Description(`JSON array of axis labels. Required when using 'matrix'. Optional when using 'correlations' (auto-extracted from keys)`),
		),
		mcp.WithString("title",
			mcp.Description("Optional title shown above the heatmap"),
		),
	)
}

func handleCorrMatrix(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var labels []string
	if s := request.GetString("labels", ""); s != "" {
		if err := json.Unmarshal([]byte(s), &labels); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid labels JSON: %v", err)), nil
		}
	}

	var M [][]float64
	if s := request.GetString("matrix", ""); s != "" {
		if err := json.Unmarshal([]byte(s), &M); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid matrix JSON: %v", err)), nil
		}
		if len(labels) != len(M) {
			return mcp.NewToolResultError(fmt.Sprintf("labels length (%d) doesn't match matrix size (%d)", len(labels), len(M))), nil
		}
	} else if s := request.GetString("correlations", ""); s != "" {
		var pairs map[string]float64
		if err := json.Unmarshal([]byte(s), &pairs); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid correlations JSON: %v", err)), nil
		}
		// Extract labels from keys if not provided.
		labelSet := map[string]bool{}
		for key := range pairs {
			a, b, ok := splitCorrKey(key)
			if !ok {
				return mcp.NewToolResultError(fmt.Sprintf("correlation key %q must be \"A-B\"", key)), nil
			}
			labelSet[a] = true
			labelSet[b] = true
		}
		if len(labels) == 0 {
			for k := range labelSet {
				labels = append(labels, k)
			}
			sort.Strings(labels)
		}
		// Build matrix.
		idx := map[string]int{}
		for i, l := range labels {
			idx[l] = i
		}
		M = make([][]float64, len(labels))
		for i := range M {
			M[i] = make([]float64, len(labels))
			M[i][i] = 1
		}
		for key, rho := range pairs {
			a, b, _ := splitCorrKey(key)
			ia, ok1 := idx[a]
			ib, ok2 := idx[b]
			if !ok1 || !ok2 {
				return mcp.NewToolResultError(fmt.Sprintf("correlation key %q references label not in 'labels'", key)), nil
			}
			M[ia][ib] = rho
			M[ib][ia] = rho
		}
	} else {
		return mcp.NewToolResultError("either 'correlations' or 'matrix' is required"), nil
	}

	title := request.GetString("title", "Correlation matrix")
	pngBytes, err := renderCorrMatrixPNG(M, labels, title)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("render: %v", err)), nil
	}

	summary := map[string]any{
		"size":   len(M),
		"labels": labels,
		"matrix": M,
	}
	text, _ := json.MarshalIndent(summary, "", "  ")
	return mcp.NewToolResultImage(string(text), base64.StdEncoding.EncodeToString(pngBytes), "image/png"), nil
}

// renderCorrMatrixPNG draws a symmetric correlation heatmap with a
// diverging colormap centered at 0 (blue → white → red).
func renderCorrMatrixPNG(M [][]float64, labels []string, title string) ([]byte, error) {
	n := len(M)
	if n == 0 {
		return nil, fmt.Errorf("empty matrix")
	}

	const (
		cellMin = 50.0
		cellMax = 120.0
		margin  = 40.0
		titleH  = 30.0
		labelGutter = 60.0 // for row/col labels
		legendW = 90.0
	)
	cell := math.Max(cellMin, math.Min(cellMax, 480.0/float64(n)))
	gridW := cell * float64(n)
	gridH := gridW
	W := int(margin*2 + labelGutter + gridW + legendW)
	H := int(titleH + margin*2 + labelGutter + gridH)

	// Widen for long titles.
	if title != "" {
		if f, err := pngFace(true, 16); err == nil {
			tmp := gg.NewContext(1, 1)
			tmp.SetFontFace(f)
			tw, _ := tmp.MeasureString(title)
			minW := int(tw + margin*2 + 24)
			if minW > W {
				W = minW
			}
		}
	}

	dc := gg.NewContext(W, H)
	dc.SetHexColor("#ffffff")
	dc.Clear()

	if title != "" {
		drawTitle(dc, title, float64(W)/2, titleH/2+8)
	}

	originX := margin + labelGutter
	originY := titleH + margin + labelGutter

	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			x := originX + float64(j)*cell
			y := originY + float64(i)*cell
			dc.SetHexColor(divergingColor(M[i][j]))
			dc.DrawRectangle(x, y, cell, cell)
			dc.Fill()
			dc.SetHexColor("#ffffff")
			dc.SetLineWidth(1)
			dc.DrawRectangle(x, y, cell, cell)
			dc.Stroke()

			// Numeric value in each cell.
			textColor := "#000000"
			if math.Abs(M[i][j]) > 0.55 {
				textColor = "#ffffff"
			}
			if f, err := pngFace(true, math.Max(9, 11*cell/100)); err == nil {
				dc.SetFontFace(f)
				dc.SetHexColor(textColor)
				dc.DrawStringAnchored(fmt.Sprintf("%.2f", M[i][j]), x+cell/2, y+cell/2, 0.5, 0.5)
			}
		}
	}

	// Axis labels.
	if f, err := pngFace(false, math.Max(9, 11*cell/100)); err == nil {
		dc.SetFontFace(f)
		dc.SetHexColor("#000000")
		for i, l := range labels {
			// Left labels.
			dc.DrawStringAnchored(l, originX-8, originY+float64(i)*cell+cell/2, 1.0, 0.5)
			// Top labels — rotated 30° for readability when labels are long.
			dc.Push()
			cx := originX + float64(i)*cell + cell/2
			cy := originY - 8
			dc.RotateAbout(-math.Pi/6, cx, cy)
			dc.DrawStringAnchored(l, cx, cy, 0.5, 1.0)
			dc.Pop()
		}
	}

	// Color bar legend.
	legendX := originX + gridW + 24
	legendBarW := 16.0
	legendBarH := gridH
	steps := 64
	for i := 0; i < steps; i++ {
		// Map step i to value v in [-1, 1].
		v := -1 + 2*float64(i)/float64(steps-1)
		dc.SetHexColor(divergingColor(v))
		y := originY + legendBarH - float64(i+1)*legendBarH/float64(steps)
		dc.DrawRectangle(legendX, y, legendBarW, legendBarH/float64(steps)+0.5)
		dc.Fill()
	}
	if f, err := pngFace(false, 10); err == nil {
		dc.SetFontFace(f)
		dc.SetHexColor("#000000")
		dc.DrawStringAnchored("+1.0", legendX+legendBarW+4, originY, 0, 0.5)
		dc.DrawStringAnchored(" 0.0", legendX+legendBarW+4, originY+legendBarH/2, 0, 0.5)
		dc.DrawStringAnchored("−1.0", legendX+legendBarW+4, originY+legendBarH, 0, 0.5)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// divergingColor maps v in [-1, 1] to a blue-white-red color suitable for
// correlation matrices. Sequential viridis is wrong here because negative
// and positive correlations are qualitatively different.
func divergingColor(v float64) string {
	v = math.Max(-1, math.Min(1, v))
	if v >= 0 {
		// White → red.
		t := v
		r := int(255 + t*(0xd3-255))
		g := int(255 + t*(0x2f-255))
		b := int(255 + t*(0x2f-255))
		return fmt.Sprintf("#%02x%02x%02x", r, g, b)
	}
	// Blue → white.
	t := -v
	r := int(255 + t*(0x19-255))
	g := int(255 + t*(0x76-255))
	b := int(255 + t*(0xd2-255))
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

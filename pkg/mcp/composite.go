package mcp

import (
	"bytes"
	"image/png"

	"github.com/fogleman/gg"
	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/go-pflow/solver"
)

// Compositors that lay multiple sub-renders into a single PNG. Used by
// petri_ode layout=combined to put a net snapshot beside the ODE plot.

// renderCombinedNetAndPlot returns a PNG with the net (left, marking-shaded)
// and the ODE plot (right). The net column tries to match the plot height so
// the two read as siblings. Title is rendered as a banner above both panels.
func renderCombinedNetAndPlot(model *goflowmetamodel.Model, sol *solver.Solution, variables []string, marking map[string]float64, title string) ([]byte, error) {
	plotW, plotH := odePlotSize()
	natW, natH := netNaturalSize(model)

	// Aspect-preserving net width so the net column has the same height as
	// the plot, then ceil to whole pixels.
	netH := float64(plotH)
	netW := float64(natW) * netH / float64(natH)
	// Cap net width — for very tall models we don't want the net column to
	// dominate. Allow up to plot width.
	if netW > float64(plotW) {
		netW = float64(plotW)
	}
	// And min — for very wide-and-flat models leave at least some space for
	// labels.
	if netW < 360 {
		netW = 360
	}

	const (
		titleH = 32
		gap    = 16
	)
	W := int(netW) + gap + plotW
	H := titleH + plotH

	dc := gg.NewContext(W, H)
	dc.SetHexColor("#ffffff")
	dc.Clear()

	if title != "" {
		drawTitle(dc, title, float64(W)/2, float64(titleH)/2)
	}

	opts := &RenderOpts{Marking: marking}
	if marking != nil {
		opts.Shading = normalizeShading(marking)
		opts.ShadeKind = "marking"
	}
	drawNet(dc, model, opts, 0, float64(titleH), netW, float64(plotH))
	drawODEPlot(dc, sol, variables, "", float64(int(netW))+gap, float64(titleH), float64(plotW), float64(plotH))

	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

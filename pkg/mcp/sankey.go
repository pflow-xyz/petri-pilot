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
	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/go-pflow/solver"
)

// petri_sankey runs an ODE over the model and renders the net with arc
// widths proportional to the integrated flux through each (place →
// transition or transition → place) link. The result reads as "money
// moving" rather than "tokens firing" — the Sankey-style view of the same
// Petri net. Useful when you want to show where capital is flowing
// without explaining places and transitions to the reader.

func sankeyTool() mcp.Tool {
	return mcp.NewTool("petri_sankey",
		mcp.WithDescription("Sankey-style flow diagram: run an ODE, compute integrated flow through each arc, render the net with arc widths proportional to flow magnitude. Reads as 'where is the money going' rather than 'which transitions fire'. Use to communicate token flow to non-Petri-net audiences."),
		mcp.WithString("model",
			mcp.Required(),
			mcp.Description("Petri net model JSON or tokenmodel DSL"),
		),
		mcp.WithString("rates",
			mcp.Description("JSON object of rate constants (default 1.0 per transition)"),
		),
		mcp.WithString("tspan",
			mcp.Description("Integration span (default [0, 10])"),
		),
		mcp.WithString("title",
			mcp.Description("Optional title shown above the diagram"),
		),
	)
}

type sankeyResponse struct {
	Flows       map[string]float64 `json:"flows"`       // arc "from->to" → integrated flow
	Transitions map[string]float64 `json:"transitions"` // transition_id → integrated flux
	Tspan       [2]float64         `json:"tspan"`
	TotalFlow   float64            `json:"totalFlow"`
}

// transitionInputs returns the (place_id, weight) pairs that feed transition t.
func transitionInputs(model *goflowmetamodel.Model, transitionID string) [][2]any {
	out := [][2]any{}
	for _, arc := range model.Arcs {
		if arc.To != transitionID {
			continue
		}
		w := arc.Weight
		if w == 0 {
			w = 1
		}
		out = append(out, [2]any{arc.From, w})
	}
	return out
}

// transitionRate computes k_t · ∏ C(m(p), w(p,t)) for transition t at the
// given marking. Mirrors the mass-action propensity used by the SSA.
func transitionRate(model *goflowmetamodel.Model, transitionID string, rates map[string]float64, marking map[string]float64) float64 {
	rate := rates[transitionID]
	for _, in := range transitionInputs(model, transitionID) {
		placeID := in[0].(string)
		weight := in[1].(int)
		m := marking[placeID]
		switch weight {
		case 1:
			rate *= m
		default:
			// Binomial selection: m·(m-1)·…·(m-w+1) / w!
			prod := 1.0
			for k := 0; k < weight; k++ {
				prod *= (m - float64(k))
			}
			for k := 2; k <= weight; k++ {
				prod /= float64(k)
			}
			rate *= prod
		}
	}
	return rate
}

func handleSankey(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model: %v", err)), nil
	}
	parsed, err := parseModelV2(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model: %v", err)), nil
	}
	model := parsed.Model

	rates := map[string]float64{}
	for _, t := range model.Transitions {
		rates[t.ID] = 1.0
	}
	if s := request.GetString("rates", ""); s != "" {
		var user map[string]float64
		if err := json.Unmarshal([]byte(s), &user); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid rates: %v", err)), nil
		}
		for k, v := range user {
			rates[k] = v
		}
	}

	tspan := [2]float64{0, 10}
	if s := request.GetString("tspan", ""); s != "" {
		var ts [2]float64
		if err := json.Unmarshal([]byte(s), &ts); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid tspan: %v", err)), nil
		}
		tspan = ts
	}

	net := buildOdeNet(model)
	initial := map[string]float64{}
	for _, p := range model.Places {
		initial[p.ID] = float64(p.Initial)
	}
	prob := solver.NewProblem(net, initial, tspan, rates)
	sol := solver.Solve(prob, solver.Tsit5(), solver.JSParityOptions())
	if sol == nil || len(sol.T) < 2 {
		return mcp.NewToolResultError("solver returned insufficient samples"), nil
	}

	// Integrated flux per transition: trapezoidal rule over sol.U.
	transFlux := map[string]float64{}
	for i := 1; i < len(sol.T); i++ {
		dt := sol.T[i] - sol.T[i-1]
		for _, t := range model.Transitions {
			r1 := transitionRate(model, t.ID, rates, sol.U[i-1])
			r2 := transitionRate(model, t.ID, rates, sol.U[i])
			transFlux[t.ID] += 0.5 * (r1 + r2) * dt
		}
	}

	// Flow per arc = transition flux × arc weight.
	flows := map[string]float64{}
	for _, arc := range model.Arcs {
		w := arc.Weight
		if w == 0 {
			w = 1
		}
		var flux float64
		for _, t := range model.Transitions {
			if t.ID == arc.From || t.ID == arc.To {
				flux = transFlux[t.ID]
				break
			}
		}
		flows[arc.From+"->"+arc.To] = flux * float64(w)
	}

	total := 0.0
	for _, f := range flows {
		total += f
	}

	resp := sankeyResponse{
		Flows:       flows,
		Transitions: transFlux,
		Tspan:       tspan,
		TotalFlow:   total,
	}
	text, _ := json.MarshalIndent(resp, "", "  ")

	title := request.GetString("title", "")
	if title == "" {
		title = fmt.Sprintf("Flow over t ∈ [%g, %g] — total flux %.3g", tspan[0], tspan[1], total)
	}
	if pngBytes, perr := renderSankeyPNG(model, flows, title); perr == nil {
		return mcp.NewToolResultImage(string(text), base64.StdEncoding.EncodeToString(pngBytes), "image/png"), nil
	}
	return mcp.NewToolResultText(string(text)), nil
}

// renderSankeyPNG draws the model with arc widths scaled to flow magnitude.
// Uses the same positions as drawNet but replaces the uniform 1.5px arc
// strokes with flow-weighted bands and drops the arrowheads — the Sankey
// aesthetic emphasizes magnitude over direction (still implied by
// taper / color gradient).
func renderSankeyPNG(model *goflowmetamodel.Model, flows map[string]float64, title string) ([]byte, error) {
	w, h := netNaturalSize(model)
	titleH := 32
	dc := gg.NewContext(w, h+titleH)
	dc.SetHexColor("#ffffff")
	dc.Clear()

	if title != "" {
		drawTitle(dc, title, float64(w)/2, float64(titleH)/2+6)
	}

	placePos, transPos := resolvedPositions(model)

	// Normalize flow widths to [1.5, 14].
	maxFlow := 0.0
	for _, f := range flows {
		if f > maxFlow {
			maxFlow = f
		}
	}
	if maxFlow <= 0 {
		maxFlow = 1
	}
	widthFor := func(f float64) float64 {
		t := f / maxFlow
		return 1.5 + 12.5*math.Sqrt(t) // sqrt curve so small flows are still visible
	}

	yOff := float64(titleH)

	// Draw arcs first so nodes overlay.
	for _, arc := range model.Arcs {
		key := arc.From + "->" + arc.To
		flow := flows[key]
		var p1, p2 [2]int
		var fromIsPlace, toIsPlace bool
		var ok1, ok2 bool
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
		fx, fy := float64(p1[0]), float64(p1[1])+yOff
		tx, ty := float64(p2[0]), float64(p2[1])+yOff
		sx, sy := edgePoint(fx, fy, tx, ty, fromIsPlace, 1.0)
		ex, ey := edgePoint(tx, ty, fx, fy, toIsPlace, 1.0)

		// Color: viridis by relative flow magnitude.
		t := flow / maxFlow
		dc.SetHexColor(viridis(t))
		dc.SetLineWidth(widthFor(flow))
		dc.DrawLine(sx, sy, ex, ey)
		dc.Stroke()
	}

	// Nodes.
	for _, p := range model.Places {
		pos := placePos[p.ID]
		x, y := float64(pos[0]), float64(pos[1])+yOff
		dc.SetHexColor("#ffffff")
		dc.DrawCircle(x, y, 25)
		dc.Fill()
		dc.SetHexColor("#1976d2")
		dc.SetLineWidth(2)
		dc.DrawCircle(x, y, 25)
		dc.Stroke()
		if f, err := pngFace(false, 12); err == nil {
			dc.SetFontFace(f)
			dc.SetHexColor("#000000")
			dc.DrawStringAnchored(p.ID, x, y+37, 0.5, 0.5)
		}
	}
	for _, t := range model.Transitions {
		pos := transPos[t.ID]
		x, y := float64(pos[0]), float64(pos[1])+yOff
		dc.SetHexColor("#333333")
		dc.DrawRectangle(x-5, y-20, 10, 40)
		dc.Fill()
		if f, err := pngFace(false, 12); err == nil {
			dc.SetFontFace(f)
			dc.SetHexColor("#000000")
			dc.DrawStringAnchored(t.ID, x, y+32, 0.5, 0.5)
		}
	}

	// Top-N flow labels.
	type flowEntry struct {
		key   string
		value float64
	}
	entries := make([]flowEntry, 0, len(flows))
	for k, v := range flows {
		entries = append(entries, flowEntry{k, v})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].value > entries[j].value })

	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

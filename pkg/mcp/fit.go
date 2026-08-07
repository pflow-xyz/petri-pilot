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
	"github.com/pflow-xyz/go-pflow/solver"
)

// petri_fit solves the inverse problem: given observed (t, value) data
// points for one or more places, find the transition rates that best
// reproduce the observations under mass-action ODE dynamics. Uses
// Nelder-Mead simplex — gradient-free, robust to noisy losses, no extra
// dependencies.
//
// Loss is sum of squared residuals between simulated and observed values
// at each observation timestamp, with linear interpolation between ODE
// sample points. Parameter bounds are enforced as a hard clamp; the
// simplex still searches in the original space and the bound is applied
// at evaluation time (penalty-free outside small violations).

func fitTool() mcp.Tool {
	return mcp.NewTool("petri_fit",
		mcp.WithDescription("Fit transition rates to observed data. Given (t, value) measurements for one or more places, finds rates that minimize squared error under mass-action ODE. Uses Nelder-Mead simplex. Returns fitted rates plus a plot of observations (dots) over the fitted trajectory."),
		mcp.WithString("model",
			mcp.Required(),
			mcp.Description("Petri net model JSON or tokenmodel DSL"),
		),
		mcp.WithString("observations",
			mcp.Required(),
			mcp.Description(`JSON object: {place_id: [[t1, v1], [t2, v2], ...], ...}. Times are interpreted in model time units`),
		),
		mcp.WithString("parameters",
			mcp.Required(),
			mcp.Description(`JSON object mapping transition_id → [min, max] bounds, e.g. {"deliver": [0.01, 10]}`),
		),
		mcp.WithString("initial_guess",
			mcp.Description(`JSON object of starting rates for parameters being fit (default: midpoint of bounds)`),
		),
		mcp.WithString("fixed_rates",
			mcp.Description("JSON object of rates for transitions NOT being fit (default 1.0)"),
		),
		mcp.WithNumber("max_iter",
			mcp.Description("Max Nelder-Mead iterations (default 200, max 1000)"),
		),
		mcp.WithNumber("tol",
			mcp.Description("Convergence tolerance on simplex spread (default 1e-6)"),
		),
		mcp.WithBoolean("verbose",
			mcp.Description("Include the Nelder-Mead algorithm description in the response. Default false"),
		),
	)
}

type fitObservation struct {
	Place string
	T     float64
	V     float64
}

type fitResponse struct {
	FittedRates  map[string]float64      `json:"fittedRates"`
	FinalLoss    float64                 `json:"finalLoss"`
	Iterations   int                     `json:"iterations"`
	Converged    bool                    `json:"converged"`
	ParamOrder   []string                `json:"paramOrder"`
	Observations map[string][][2]float64 `json:"observations"`
	Explanation  string                  `json:"explanation,omitempty"`
}

func handleFit(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model parameter: %v", err)), nil
	}
	parsed, err := parseModelV2(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model JSON: %v", err)), nil
	}
	model := parsed.Model

	obsStr, err := request.RequireString("observations")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing observations: %v", err)), nil
	}
	var rawObs map[string][][2]float64
	if err := json.Unmarshal([]byte(obsStr), &rawObs); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid observations JSON: %v", err)), nil
	}
	placeSet := map[string]bool{}
	for _, p := range model.Places {
		placeSet[p.ID] = true
	}
	observations := []fitObservation{}
	for placeID, points := range rawObs {
		if !placeSet[placeID] {
			return mcp.NewToolResultError(fmt.Sprintf("observation place %q not found in model", placeID)), nil
		}
		for _, pt := range points {
			observations = append(observations, fitObservation{Place: placeID, T: pt[0], V: pt[1]})
		}
	}
	if len(observations) == 0 {
		return mcp.NewToolResultError("no observation points supplied"), nil
	}

	paramStr, err := request.RequireString("parameters")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing parameters: %v", err)), nil
	}
	var bounds map[string][2]float64
	if err := json.Unmarshal([]byte(paramStr), &bounds); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid parameters JSON: %v", err)), nil
	}
	if len(bounds) == 0 {
		return mcp.NewToolResultError("at least one parameter to fit"), nil
	}
	transitionSet := map[string]bool{}
	for _, t := range model.Transitions {
		transitionSet[t.ID] = true
	}
	for tid, rng := range bounds {
		if !transitionSet[tid] {
			return mcp.NewToolResultError(fmt.Sprintf("transition %q not found in model", tid)), nil
		}
		if rng[1] <= rng[0] {
			return mcp.NewToolResultError(fmt.Sprintf("parameter %q: max must exceed min", tid)), nil
		}
	}

	paramOrder := make([]string, 0, len(bounds))
	for k := range bounds {
		paramOrder = append(paramOrder, k)
	}
	sort.Strings(paramOrder)

	initialGuess := map[string]float64{}
	for _, k := range paramOrder {
		initialGuess[k] = (bounds[k][0] + bounds[k][1]) / 2
	}
	if s := request.GetString("initial_guess", ""); s != "" {
		var user map[string]float64
		if err := json.Unmarshal([]byte(s), &user); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid initial_guess JSON: %v", err)), nil
		}
		for k, v := range user {
			if _, ok := bounds[k]; ok {
				initialGuess[k] = v
			}
		}
	}

	fixedRates := map[string]float64{}
	for _, t := range model.Transitions {
		fixedRates[t.ID] = 1.0
	}
	if s := request.GetString("fixed_rates", ""); s != "" {
		var user map[string]float64
		if err := json.Unmarshal([]byte(s), &user); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid fixed_rates JSON: %v", err)), nil
		}
		for k, v := range user {
			fixedRates[k] = v
		}
	}

	maxIter := request.GetInt("max_iter", 200)
	if maxIter < 10 {
		maxIter = 10
	}
	if maxIter > 1000 {
		maxIter = 1000
	}
	tol := request.GetFloat("tol", 1e-6)
	if tol <= 0 {
		tol = 1e-6
	}

	// Derive tspan from observations: 0 (or min t) to max t * 1.1.
	tMin, tMax := math.Inf(1), math.Inf(-1)
	for _, o := range observations {
		if o.T < tMin {
			tMin = o.T
		}
		if o.T > tMax {
			tMax = o.T
		}
	}
	if tMin > 0 {
		tMin = 0
	}
	tspan := [2]float64{tMin, tMax * 1.1}

	net := buildOdeNet(model)
	initial := map[string]float64{}
	for _, p := range model.Places {
		initial[p.ID] = float64(p.Initial)
	}

	// Loss function: simulate with the candidate rates, sum squared error
	// over all observations.
	loss := func(x []float64) float64 {
		rates := make(map[string]float64, len(fixedRates))
		for k, v := range fixedRates {
			rates[k] = v
		}
		for i, k := range paramOrder {
			lo, hi := bounds[k][0], bounds[k][1]
			v := x[i]
			if v < lo {
				v = lo
			}
			if v > hi {
				v = hi
			}
			rates[k] = v
		}
		prob := solver.NewProblem(net, initial, tspan, rates)
		sol := solver.Solve(prob, solver.Tsit5(), solver.JSParityOptions())
		if sol == nil || len(sol.T) == 0 {
			return math.Inf(1)
		}
		var sum float64
		for _, obs := range observations {
			sim := interpolate(sol.T, sol.GetVariable(obs.Place), obs.T)
			d := sim - obs.V
			sum += d * d
		}
		return sum
	}

	x0 := make([]float64, len(paramOrder))
	for i, k := range paramOrder {
		x0[i] = initialGuess[k]
	}

	bestX, bestLoss, iters, converged := nelderMead(loss, x0, maxIter, tol)

	fittedRates := map[string]float64{}
	for i, k := range paramOrder {
		lo, hi := bounds[k][0], bounds[k][1]
		v := bestX[i]
		if v < lo {
			v = lo
		}
		if v > hi {
			v = hi
		}
		fittedRates[k] = v
	}

	resp := fitResponse{
		FittedRates:  fittedRates,
		FinalLoss:    bestLoss,
		Iterations:   iters,
		Converged:    converged,
		ParamOrder:   paramOrder,
		Observations: rawObs,
	}

	if request.GetBool("verbose", false) {
		obsCount := 0
		for _, pts := range rawObs {
			obsCount += len(pts)
		}
		resp.Explanation = verboseAnnotation("fit",
			fmt.Sprintf("%d parameters, %d observation points, iters=%d, final loss=%g, converged=%v",
				len(paramOrder), obsCount, iters, bestLoss, converged))
	}

	text, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal response: %v", err)), nil
	}

	// Final trajectory for the plot uses the same merged rates.
	plotRates := make(map[string]float64, len(fixedRates))
	for k, v := range fixedRates {
		plotRates[k] = v
	}
	for k, v := range fittedRates {
		plotRates[k] = v
	}
	prob := solver.NewProblem(net, initial, tspan, plotRates)
	sol := solver.Solve(prob, solver.Tsit5(), solver.JSParityOptions())

	if pngBytes, perr := renderFitPNG(sol, rawObs, fittedRates, bestLoss); perr == nil {
		return mcp.NewToolResultImage(string(withCaveats(text, model)), base64.StdEncoding.EncodeToString(pngBytes), "image/png"), nil
	}
	return mcp.NewToolResultText(string(withCaveats(text, model))), nil
}

// nelderMead is a vanilla simplex search. Standard reflection (alpha=1),
// expansion (gamma=2), contraction (rho=0.5), shrink (sigma=0.5) coefficients.
// Returns the best vertex, best loss, total iteration count, and whether the
// simplex shrank below `tol`.
func nelderMead(f func([]float64) float64, x0 []float64, maxIter int, tol float64) ([]float64, float64, int, bool) {
	const (
		alpha = 1.0
		gamma = 2.0
		rho   = 0.5
		sigma = 0.5
	)
	n := len(x0)
	if n == 0 {
		return x0, f(x0), 0, true
	}

	// Initial simplex: x0 plus N perturbations. 25% spread gives the
	// search enough geometry to explore before contraction starts;
	// the standard 5% suggested in textbooks gets stuck in degenerate
	// likelihood surfaces (which mass-action ODE parameter spaces often are).
	simplex := make([][]float64, n+1)
	values := make([]float64, n+1)
	simplex[0] = append([]float64(nil), x0...)
	values[0] = f(simplex[0])
	for i := 0; i < n; i++ {
		v := append([]float64(nil), x0...)
		if v[i] == 0 {
			v[i] = 0.25
		} else {
			v[i] *= 1.25
		}
		simplex[i+1] = v
		values[i+1] = f(v)
	}

	indices := func() []int {
		idx := make([]int, n+1)
		for i := range idx {
			idx[i] = i
		}
		sort.Slice(idx, func(a, b int) bool { return values[idx[a]] < values[idx[b]] })
		return idx
	}

	converged := false
	iter := 0
	for iter = 0; iter < maxIter; iter++ {
		idx := indices()
		best := idx[0]
		worst := idx[n]
		secondWorst := idx[n-1]

		// Convergence check: spread of vertex values is small.
		if values[worst]-values[best] < tol {
			converged = true
			break
		}

		// Centroid of all but worst.
		centroid := make([]float64, n)
		for _, i := range idx[:n] {
			for j := 0; j < n; j++ {
				centroid[j] += simplex[i][j]
			}
		}
		for j := 0; j < n; j++ {
			centroid[j] /= float64(n)
		}

		// Reflection
		reflected := make([]float64, n)
		for j := 0; j < n; j++ {
			reflected[j] = centroid[j] + alpha*(centroid[j]-simplex[worst][j])
		}
		fReflected := f(reflected)

		if fReflected < values[best] {
			// Expansion
			expanded := make([]float64, n)
			for j := 0; j < n; j++ {
				expanded[j] = centroid[j] + gamma*(reflected[j]-centroid[j])
			}
			fExpanded := f(expanded)
			if fExpanded < fReflected {
				simplex[worst] = expanded
				values[worst] = fExpanded
			} else {
				simplex[worst] = reflected
				values[worst] = fReflected
			}
		} else if fReflected < values[secondWorst] {
			simplex[worst] = reflected
			values[worst] = fReflected
		} else {
			// Contraction
			contracted := make([]float64, n)
			for j := 0; j < n; j++ {
				contracted[j] = centroid[j] + rho*(simplex[worst][j]-centroid[j])
			}
			fContracted := f(contracted)
			if fContracted < values[worst] {
				simplex[worst] = contracted
				values[worst] = fContracted
			} else {
				// Shrink simplex toward best.
				for _, i := range idx[1:] {
					for j := 0; j < n; j++ {
						simplex[i][j] = simplex[best][j] + sigma*(simplex[i][j]-simplex[best][j])
					}
					values[i] = f(simplex[i])
				}
			}
		}
	}

	idx := indices()
	return simplex[idx[0]], values[idx[0]], iter, converged
}

// interpolate returns y at time t via linear interpolation over (ts, ys).
// Out-of-bounds returns the nearest endpoint.
func interpolate(ts, ys []float64, t float64) float64 {
	if len(ts) == 0 {
		return 0
	}
	if t <= ts[0] {
		return ys[0]
	}
	if t >= ts[len(ts)-1] {
		return ys[len(ys)-1]
	}
	// Binary search for the bracketing interval.
	lo, hi := 0, len(ts)-1
	for hi-lo > 1 {
		mid := (lo + hi) / 2
		if ts[mid] <= t {
			lo = mid
		} else {
			hi = mid
		}
	}
	dt := ts[hi] - ts[lo]
	if dt == 0 {
		return ys[lo]
	}
	frac := (t - ts[lo]) / dt
	return ys[lo] + frac*(ys[hi]-ys[lo])
}

// renderFitPNG draws the fitted trajectory as solid lines per observable
// and the observations as filled dots at their measured (t, v).
func renderFitPNG(sol *solver.Solution, observations map[string][][2]float64, fitted map[string]float64, loss float64) ([]byte, error) {
	const W, H = 760, 460
	dc := gg.NewContext(W, H)
	dc.SetHexColor("#ffffff")
	dc.Clear()

	if sol == nil || len(sol.T) == 0 {
		return nil, fmt.Errorf("empty solution")
	}

	// Stable order for observables and legend coloring.
	places := make([]string, 0, len(observations))
	for k := range observations {
		places = append(places, k)
	}
	sort.Strings(places)

	xmin := sol.T[0]
	xmax := sol.T[len(sol.T)-1]
	ymin := math.Inf(1)
	ymax := math.Inf(-1)
	for _, p := range places {
		ys := sol.GetVariable(p)
		for _, y := range ys {
			if y < ymin {
				ymin = y
			}
			if y > ymax {
				ymax = y
			}
		}
		for _, obs := range observations[p] {
			if obs[1] < ymin {
				ymin = obs[1]
			}
			if obs[1] > ymax {
				ymax = obs[1]
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

	title := fmt.Sprintf("petri_fit — loss=%.4g, rates=%s", loss, formatRatesShort(fitted))
	drawPlotFrame(dc, xmin, xmax, ymin, ymax, title, "Time", "Value", 0, 0, W, H)

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

	for i, p := range places {
		color := plotColors[i%len(plotColors)]
		// Fitted trajectory line.
		ys := sol.GetVariable(p)
		dc.SetHexColor(color)
		dc.SetLineWidth(2)
		dc.MoveTo(sx(sol.T[0]), sy(ys[0]))
		for j := 1; j < len(sol.T); j++ {
			dc.LineTo(sx(sol.T[j]), sy(ys[j]))
		}
		dc.Stroke()
		// Observation dots.
		for _, obs := range observations[p] {
			cx, cy := sx(obs[0]), sy(obs[1])
			dc.SetHexColor(color)
			dc.DrawCircle(cx, cy, 4)
			dc.Fill()
			dc.SetHexColor("#ffffff")
			dc.SetLineWidth(1.5)
			dc.DrawCircle(cx, cy, 4)
			dc.Stroke()
		}
	}

	// Legend.
	legendX := marginL + plotW + 14
	legendY := marginT + 6
	if f, err := pngFace(false, 11); err == nil {
		dc.SetFontFace(f)
		for i, p := range places {
			dc.SetHexColor(plotColors[i%len(plotColors)])
			dc.SetLineWidth(2)
			dc.DrawLine(legendX, legendY+6, legendX+20, legendY+6)
			dc.Stroke()
			dc.DrawCircle(legendX+10, legendY+6, 3)
			dc.Fill()
			dc.SetHexColor("#000000")
			dc.DrawStringAnchored(p, legendX+24, legendY+6, 0, 0.5)
			legendY += 18
		}
		legendY += 6
		dc.SetHexColor("#666666")
		dc.DrawStringAnchored("dots = observed", legendX, legendY+6, 0, 0.5)
		legendY += 14
		dc.DrawStringAnchored("lines = fitted", legendX, legendY+6, 0, 0.5)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func formatRatesShort(r map[string]float64) string {
	keys := make([]string, 0, len(r))
	for k := range r {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := ""
	for i, k := range keys {
		if i > 0 {
			out += ", "
		}
		out += fmt.Sprintf("%s=%.3g", k, r[k])
	}
	return out
}

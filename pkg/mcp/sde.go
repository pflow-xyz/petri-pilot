package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"math"
	"math/rand"
	"sort"
	"strings"

	"github.com/fogleman/gg"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/pflow-xyz/go-pflow/solver"
)

// petri_sde adds Stochastic Differential Equation simulation to the toolbox.
// Same mass-action drift as petri_ode plus geometric Brownian motion on
// user-selected places — appropriate for DeFi price processes, interest
// rate models, and any continuous-state continuous-time problem where
// noise scales multiplicatively with the state value.
//
// dx_i = drift_i(x) * dt + sigma_i * x_i * dW_i
//
// Solved with Euler-Maruyama. Multiple independent paths, mean ± stdev
// band visualization.
//
// Distinct from petri_stochastic (Gillespie SSA), which is for discrete
// integer-count systems with random firing TIMES. petri_sde is for
// continuous-state systems with continuous noise.

func sdeTool() mcp.Tool {
	return mcp.NewTool("petri_sde",
		mcp.WithDescription("Stochastic Differential Equation simulation. Mass-action drift (as in petri_ode) plus EXOGENOUS geometric Brownian motion on user-selected places — for DeFi price processes, interest rate models, anywhere continuous noise scales with state value. This is not the net's own firing noise (that's petri_stochastic's job); it layers external uncertainty on top of the ODE. Returns mean ± stdev band over N paths."),
		mcp.WithString("model",
			mcp.Required(),
			mcp.Description("Petri net model JSON or tokenmodel DSL"),
		),
		mcp.WithString("volatility",
			mcp.Required(),
			mcp.Description(`JSON object mapping place_id → sigma (annualized vol). Places not in this map evolve deterministically. e.g. {"price_token_a": 0.6, "price_token_b": 0.4}`),
		),
		mcp.WithString("correlations",
			mcp.Description(`Optional JSON object of pairwise correlation coefficients in [-1, 1], keyed by "placeA-placeB" (sorted alphabetically). e.g. {"btc-eth": 0.7, "btc-sol": 0.6, "eth-sol": 0.5}. Missing pairs default to 0 (independent). Matrix must be positive semi-definite or the call errors.`),
		),
		mcp.WithString("rates",
			mcp.Description("JSON object of mass-action rate constants (default 1.0 per transition)"),
		),
		mcp.WithString("tspan",
			mcp.Description("Integration span (default [0, 1])"),
		),
		mcp.WithString("variables",
			mcp.Description("JSON array of place IDs to plot (default: all places)"),
		),
		mcp.WithNumber("paths",
			mcp.Description("Number of independent SDE paths (default 20, max 100). Mean and ±stdev are computed across paths"),
		),
		mcp.WithNumber("steps",
			mcp.Description("Euler-Maruyama step count (default 500). Higher = more accurate noise integration"),
		),
		mcp.WithNumber("samples",
			mcp.Description("Output sample count (default 200, downsampled from steps)"),
		),
		mcp.WithNumber("seed",
			mcp.Description("Random seed for reproducibility (default 42)"),
		),
		mcp.WithBoolean("verbose",
			mcp.Description("Include the Euler-Maruyama algorithm description in the response. Default false"),
		),
	)
}

type sdeResponse struct {
	StateLabels []string             `json:"stateLabels"`
	Tspan       [2]float64           `json:"tspan"`
	Volatility  map[string]float64   `json:"volatility"`
	Rates       map[string]float64   `json:"rates"`
	Paths       int                  `json:"paths"`
	Times       []float64            `json:"times"`
	Mean        map[string][]float64 `json:"mean"`
	Stdev       map[string][]float64 `json:"stdev"`
	FinalMean   map[string]float64   `json:"finalMean"`
	FinalStdev  map[string]float64   `json:"finalStdev"`
	Explanation string               `json:"explanation,omitempty"`
}

func handleSde(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model parameter: %v", err)), nil
	}
	parsed, err := parseModelV2(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model JSON: %v", err)), nil
	}
	model := parsed.Model

	volStr, err := request.RequireString("volatility")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing volatility parameter: %v", err)), nil
	}
	var volatility map[string]float64
	if err := json.Unmarshal([]byte(volStr), &volatility); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid volatility JSON: %v", err)), nil
	}
	placeSet := map[string]bool{}
	for _, p := range model.Places {
		placeSet[p.ID] = true
	}
	for k, v := range volatility {
		if !placeSet[k] {
			return mcp.NewToolResultError(fmt.Sprintf("volatility place %q not found in model", k)), nil
		}
		if v < 0 {
			return mcp.NewToolResultError(fmt.Sprintf("volatility for %q is negative", k)), nil
		}
	}

	// Stable ordering of volatile places — Cholesky factor uses this index.
	volatilePlaces := make([]string, 0, len(volatility))
	for k := range volatility {
		volatilePlaces = append(volatilePlaces, k)
	}
	sort.Strings(volatilePlaces)
	volIdx := map[string]int{}
	for i, k := range volatilePlaces {
		volIdx[k] = i
	}

	// Build correlation matrix (defaults to identity).
	nv := len(volatilePlaces)
	corr := make([][]float64, nv)
	for i := range corr {
		corr[i] = make([]float64, nv)
		corr[i][i] = 1
	}
	if s := request.GetString("correlations", ""); s != "" {
		var userCorr map[string]float64
		if err := json.Unmarshal([]byte(s), &userCorr); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid correlations JSON: %v", err)), nil
		}
		for key, rho := range userCorr {
			a, b, ok := splitCorrKey(key)
			if !ok {
				return mcp.NewToolResultError(fmt.Sprintf("correlation key %q must be \"placeA-placeB\"", key)), nil
			}
			ia, ok1 := volIdx[a]
			ib, ok2 := volIdx[b]
			if !ok1 || !ok2 {
				return mcp.NewToolResultError(fmt.Sprintf("correlation %q references place not in volatility map", key)), nil
			}
			if rho < -1 || rho > 1 {
				return mcp.NewToolResultError(fmt.Sprintf("correlation %q = %v out of [-1, 1]", key, rho)), nil
			}
			corr[ia][ib] = rho
			corr[ib][ia] = rho
		}
	}
	// Cholesky factor — fails if matrix isn't positive semi-definite.
	chol, err := cholesky(corr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("correlation matrix not positive semi-definite: %v", err)), nil
	}

	rates := map[string]float64{}
	for _, t := range model.Transitions {
		rates[t.ID] = 1.0
	}
	if s := request.GetString("rates", ""); s != "" {
		var user map[string]float64
		if err := json.Unmarshal([]byte(s), &user); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid rates JSON: %v", err)), nil
		}
		for k, v := range user {
			rates[k] = v
		}
	}

	tspan := [2]float64{0, 1}
	if s := request.GetString("tspan", ""); s != "" {
		var ts [2]float64
		if err := json.Unmarshal([]byte(s), &ts); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid tspan JSON: %v", err)), nil
		}
		if ts[1] <= ts[0] {
			return mcp.NewToolResultError("tspan: t1 must exceed t0"), nil
		}
		tspan = ts
	}

	var variables []string
	if s := request.GetString("variables", ""); s != "" {
		if err := json.Unmarshal([]byte(s), &variables); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid variables JSON: %v", err)), nil
		}
	}
	if len(variables) == 0 {
		for _, p := range model.Places {
			variables = append(variables, p.ID)
		}
	}

	paths := request.GetInt("paths", 20)
	if paths < 1 {
		paths = 1
	}
	if paths > 100 {
		paths = 100
	}
	steps := request.GetInt("steps", 500)
	if steps < 10 {
		steps = 10
	}
	samples := request.GetInt("samples", 200)
	if samples < 2 {
		samples = 2
	}
	if samples > steps {
		samples = steps
	}
	seed := int64(request.GetInt("seed", 42))

	// Build PetriNet + initial state for the mass-action drift function.
	net := buildOdeNet(model)
	initial := map[string]float64{}
	for _, p := range model.Places {
		initial[p.ID] = float64(p.Initial)
	}
	prob := solver.NewProblem(net, initial, tspan, rates)

	// Sample times (uniform).
	times := make([]float64, samples)
	for i := 0; i < samples; i++ {
		times[i] = tspan[0] + (tspan[1]-tspan[0])*float64(i)/float64(samples-1)
	}

	// Pre-compute sample-index → step-index mapping.
	stepDt := (tspan[1] - tspan[0]) / float64(steps)
	sampleStep := make([]int, samples)
	for i, t := range times {
		s := int((t - tspan[0]) / stepDt)
		if s > steps {
			s = steps
		}
		sampleStep[i] = s
	}

	// Stable place ordering for output.
	stateLabels := make([]string, 0, len(model.Places))
	for _, p := range model.Places {
		stateLabels = append(stateLabels, p.ID)
	}

	// Run all paths. trajectories[path][place][sampleIdx]
	trajectories := make([][]map[string][]float64, paths)
	rng := rand.New(rand.NewSource(seed))
	for p := 0; p < paths; p++ {
		subSeed := rng.Int63()
		trajectories[p] = []map[string][]float64{runSDEPath(prob, initial, volatility, volatilePlaces, chol, tspan, steps, sampleStep, subSeed)}
	}

	// Aggregate mean / stdev per place per sample.
	mean := make(map[string][]float64, len(stateLabels))
	stdev := make(map[string][]float64, len(stateLabels))
	for _, label := range stateLabels {
		m := make([]float64, samples)
		s := make([]float64, samples)
		for ti := 0; ti < samples; ti++ {
			sum := 0.0
			for p := 0; p < paths; p++ {
				sum += trajectories[p][0][label][ti]
			}
			m[ti] = sum / float64(paths)
			if paths > 1 {
				ss := 0.0
				for p := 0; p < paths; p++ {
					d := trajectories[p][0][label][ti] - m[ti]
					ss += d * d
				}
				s[ti] = math.Sqrt(ss / float64(paths-1))
			}
		}
		mean[label] = m
		if paths > 1 {
			stdev[label] = s
		}
	}

	finalMean := map[string]float64{}
	finalStdev := map[string]float64{}
	for label, vals := range mean {
		finalMean[label] = vals[len(vals)-1]
		if sd, ok := stdev[label]; ok {
			finalStdev[label] = sd[len(sd)-1]
		}
	}

	resp := sdeResponse{
		StateLabels: stateLabels,
		Tspan:       tspan,
		Volatility:  volatility,
		Rates:       rates,
		Paths:       paths,
		Times:       times,
		Mean:        mean,
		Stdev:       stdev,
		FinalMean:   finalMean,
		FinalStdev:  finalStdev,
	}

	if request.GetBool("verbose", false) {
		corrNote := ""
		if request.GetString("correlations", "") != "" {
			corrNote = ", correlations applied via Cholesky factor"
		}
		resp.Explanation = verboseAnnotation("sde",
			fmt.Sprintf("paths=%d, steps=%d, tspan=[%v, %v], %d volatile places%s",
				paths, steps, tspan[0], tspan[1], len(volatility), corrNote))
	}

	text, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal response: %v", err)), nil
	}

	if pngBytes, perr := renderSDEPNG(resp, variables); perr == nil {
		return mcp.NewToolResultImage(string(withCaveats(text, model)), base64.StdEncoding.EncodeToString(pngBytes), "image/png"), nil
	}
	return mcp.NewToolResultText(string(withCaveats(text, model))), nil
}

// runSDEPath runs one Euler-Maruyama path with optional correlated noise.
// volatilePlaces is the stable ordering whose index aligns with rows of
// chol (the lower-triangular Cholesky factor of the correlation matrix).
// On each step, we draw nv iid standard normals z, then correlated noise
// w = chol · z; w[i] is N(0,1) marginally and Cov(w[i], w[j]) = corr[i][j].
//
// Non-negativity is enforced by clamping (negative GBM values are
// unphysical for prices/balances and would explode subsequent steps).
func runSDEPath(prob *solver.Problem, initial map[string]float64, volatility map[string]float64, volatilePlaces []string, chol [][]float64, tspan [2]float64, steps int, sampleStep []int, seed int64) map[string][]float64 {
	rng := rand.New(rand.NewSource(seed))
	dt := (tspan[1] - tspan[0]) / float64(steps)
	sqrtDt := math.Sqrt(dt)
	nv := len(volatilePlaces)

	u := make(map[string]float64, len(initial))
	for k, v := range initial {
		u[k] = v
	}

	out := make(map[string][]float64, len(initial))
	for k := range initial {
		out[k] = make([]float64, len(sampleStep))
	}

	sampleIdx := 0
	for sampleIdx < len(sampleStep) && sampleStep[sampleIdx] == 0 {
		for k, v := range u {
			out[k][sampleIdx] = v
		}
		sampleIdx++
	}

	z := make([]float64, nv) // iid standard normals
	w := make([]float64, nv) // correlated normals

	t := tspan[0]
	for step := 1; step <= steps; step++ {
		du := prob.F(t, u)

		// Draw iid normals, then apply the Cholesky factor.
		for i := 0; i < nv; i++ {
			z[i] = rng.NormFloat64()
		}
		for i := 0; i < nv; i++ {
			s := 0.0
			for j := 0; j <= i; j++ {
				s += chol[i][j] * z[j]
			}
			w[i] = s
		}

		next := make(map[string]float64, len(u))
		for k, v := range u {
			next[k] = v + du[k]*dt
		}
		for i, k := range volatilePlaces {
			sigma := volatility[k]
			next[k] += sigma * u[k] * sqrtDt * w[i]
		}
		for k, v := range next {
			if v < 0 {
				next[k] = 0
			}
		}
		u = next
		t += dt

		for sampleIdx < len(sampleStep) && sampleStep[sampleIdx] == step {
			for k, v := range u {
				out[k][sampleIdx] = v
			}
			sampleIdx++
		}
	}
	for ; sampleIdx < len(sampleStep); sampleIdx++ {
		for k, v := range u {
			out[k][sampleIdx] = v
		}
	}
	return out
}

// cholesky returns the lower-triangular Cholesky factor L such that
// M = L · L^T. Errors if M is not positive semi-definite.
func cholesky(M [][]float64) ([][]float64, error) {
	n := len(M)
	L := make([][]float64, n)
	for i := range L {
		L[i] = make([]float64, n)
	}
	for i := 0; i < n; i++ {
		for j := 0; j <= i; j++ {
			sum := 0.0
			for k := 0; k < j; k++ {
				sum += L[i][k] * L[j][k]
			}
			if i == j {
				d := M[i][i] - sum
				if d < -1e-12 {
					return nil, fmt.Errorf("non-positive diagonal at row %d (value %v)", i, d)
				}
				if d < 0 {
					d = 0
				}
				L[i][j] = math.Sqrt(d)
			} else {
				if L[j][j] == 0 {
					L[i][j] = 0
				} else {
					L[i][j] = (M[i][j] - sum) / L[j][j]
				}
			}
		}
	}
	return L, nil
}

// splitCorrKey parses "placeA-placeB" → (placeA, placeB). Returns the keys
// in input order; the matrix builder symmetrizes either way.
func splitCorrKey(key string) (string, string, bool) {
	idx := strings.Index(key, "-")
	if idx < 1 || idx >= len(key)-1 {
		return "", "", false
	}
	return key[:idx], key[idx+1:], true
}

// renderSDEPNG draws mean ± stdev bands per variable across paths. Same
// visual language as petri_stochastic so the two read as siblings.
func renderSDEPNG(resp sdeResponse, variables []string) ([]byte, error) {
	const W, H = 760, 460
	dc := gg.NewContext(W, H)
	dc.SetHexColor("#ffffff")
	dc.Clear()

	xmin := resp.Times[0]
	xmax := resp.Times[len(resp.Times)-1]
	ymin := math.Inf(1)
	ymax := math.Inf(-1)
	for _, v := range variables {
		for i, m := range resp.Mean[v] {
			lo, hi := m, m
			if sd, ok := resp.Stdev[v]; ok && i < len(sd) {
				lo, hi = m-sd[i], m+sd[i]
			}
			if lo < ymin {
				ymin = lo
			}
			if hi > ymax {
				ymax = hi
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

	title := fmt.Sprintf("Petri SDE — %d paths", resp.Paths)
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

	for i, v := range variables {
		color := plotColors[i%len(plotColors)]
		bandColor := lightenColor(color, 0.4)

		if sd, ok := resp.Stdev[v]; ok && len(sd) == len(resp.Times) {
			dc.SetHexColor(bandColor)
			dc.MoveTo(sx(resp.Times[0]), sy(resp.Mean[v][0]+sd[0]))
			for j := 1; j < len(resp.Times); j++ {
				dc.LineTo(sx(resp.Times[j]), sy(resp.Mean[v][j]+sd[j]))
			}
			for j := len(resp.Times) - 1; j >= 0; j-- {
				dc.LineTo(sx(resp.Times[j]), sy(resp.Mean[v][j]-sd[j]))
			}
			dc.ClosePath()
			dc.Fill()
		}

		dc.SetHexColor(color)
		dc.SetLineWidth(2)
		dc.MoveTo(sx(resp.Times[0]), sy(resp.Mean[v][0]))
		for j := 1; j < len(resp.Times); j++ {
			dc.LineTo(sx(resp.Times[j]), sy(resp.Mean[v][j]))
		}
		dc.Stroke()
	}

	// Legend.
	legendX := marginL + plotW + 14
	legendY := marginT + 6
	if f, err := pngFace(false, 11); err == nil {
		dc.SetFontFace(f)
		for i, v := range variables {
			dc.SetHexColor(plotColors[i%len(plotColors)])
			dc.SetLineWidth(2)
			dc.DrawLine(legendX, legendY+6, legendX+20, legendY+6)
			dc.Stroke()
			dc.SetHexColor("#000000")
			label := v
			if sigma, ok := resp.Volatility[v]; ok {
				label = fmt.Sprintf("%s (σ=%.2g)", v, sigma)
			}
			dc.DrawStringAnchored(label, legendX+24, legendY+6, 0, 0.5)
			legendY += 18
		}
		if resp.Paths > 1 {
			legendY += 6
			dc.SetHexColor("#666666")
			dc.DrawStringAnchored("band: ±1 stdev", legendX, legendY+6, 0, 0.5)
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

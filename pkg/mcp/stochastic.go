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

	"github.com/fogleman/gg"
	"github.com/mark3labs/mcp-go/mcp"
	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"

	"github.com/pflow-xyz/petri-pilot/pkg/runtime/sim"
)

// petri_stochastic runs Gillespie's Stochastic Simulation Algorithm (SSA)
// on a Petri net under mass-action kinetics. Distinct from petri_ode in
// that it treats the marking as discrete integer counts and firings as
// random events — appropriate when token counts are small enough that
// noise matters (queueing, biology, scarce-resource problems).
//
// Algorithm (Gillespie 1977 / SSA):
//   1. Compute propensity a_i = k_i × C(m, w) over the *kinetic* input arcs
//      of transition i, where C is the binomial selection coefficient. An
//      input marked non-kinetic still has to be there and is still consumed,
//      but drops out of the product — a barista is a prerequisite for making
//      a drink, not a catalyst that makes it pour faster.
//   2. Total rate A = Σ a_i.
//   3. Wait time τ ~ Exp(A) (i.e. −ln U / A).
//   4. Pick transition i with probability a_i / A.
//   5. Apply firing: −w on input arcs, +w on output arcs.
//   6. Repeat until t > t_end (or A = 0 → terminal state).
//
// With n_realizations > 1, runs are independent replicates whose mean and
// ±stdev band are plotted alongside the underlying trajectories.

func stochasticTool() mcp.Tool {
	return mcp.NewTool("petri_stochastic",
		mcp.WithDescription("Gillespie Stochastic Simulation Algorithm (SSA) over the Petri net's discrete marking. Distinct from petri_ode's continuous ODE — token counts stay integer, firings are random events, results have visible noise. Use when token counts are small enough that variance matters. Multiple realizations show mean ± stdev band."),
		mcp.WithString("model",
			mcp.Required(),
			mcp.Description("Petri net model JSON or tokenmodel DSL"),
		),
		mcp.WithString("rates",
			mcp.Description("JSON object of mass-action rate constants per transition (default 1.0)"),
		),
		mcp.WithString("tspan",
			mcp.Description("Integration span [t0, tf] (default [0, 10])"),
		),
		mcp.WithString("variables",
			mcp.Description("JSON array of place IDs to plot (default: all places)"),
		),
		mcp.WithNumber("realizations",
			mcp.Description("Number of independent SSA runs (default 1, max 50). With >1, mean and ±stdev band are plotted"),
		),
		mcp.WithNumber("samples",
			mcp.Description("Number of time points to record per realization (default 200)"),
		),
		mcp.WithNumber("seed",
			mcp.Description("Random seed for reproducibility (default 42)"),
		),
		mcp.WithBoolean("verbose",
			mcp.Description("Include the Gillespie SSA algorithm description in the response. Default false"),
		),
	)
}

type stochasticResponse struct {
	StateLabels  []string             `json:"stateLabels"`
	Tspan        [2]float64           `json:"tspan"`
	Rates        map[string]float64   `json:"rates"`
	Realizations int                  `json:"realizations"`
	Times        []float64            `json:"times"`
	Mean         map[string][]float64 `json:"mean"`
	Stdev        map[string][]float64 `json:"stdev,omitempty"`
	FinalMean    map[string]float64   `json:"finalMean"`
	Contended    []contendedPlace     `json:"contended,omitempty"`
	Explanation  string               `json:"explanation,omitempty"`
}

// contendedPlace reports how much of the run a place spent being the only thing
// standing between a transition and firing.
//
// A trajectory shows what happened; this says what stopped happening, which is
// the question anyone running this tool on a capacity model is really asking. A
// mean that sits at a comfortable level all run is not evidence a place is
// plentiful: a resource consumed exactly as fast as it is delivered is refilled
// and drained all day and never looks short in the plot. The café shipped in
// that state on milk, and the only visible symptom was an arithmetic
// contradiction elsewhere — idle servers alongside heavy loss.
//
// Fraction is a share of the horizon, over every realization, in which this
// place was short while everything else Blocking's transitions needed was
// present. Places short of nothing much are left out entirely (see
// minContention); a work queue with no work in it will legitimately appear —
// and Kind is what stops that being read as a bottleneck.
type contendedPlace struct {
	Place    string  `json:"place"`
	Fraction float64 `json:"fraction"`
	// Kind says whether waiting on this place is a capacity finding
	// ("conserved" — a fixed pool whose size was set once, such as staff;
	// "bounded" — a shelf with a declared capacity, such as stock) or something
	// that claims nothing: "queue" (unbounded and fed by the net's own flow, so
	// an empty one means the work has not arrived) and "state" (a conserved
	// marker whose tokens serve nothing, such as a stoplight's colour). An empty queue is
	// the opposite of a bottleneck, so capacity kinds sort ahead of every queue
	// however large the fractions: on the café the three emptiest order queues
	// read 80-90% while the staff pool that actually decided throughput read
	// 26%, and a caller ranking on fraction alone was told the shop's idleness
	// was its constraint. See sim.SupplyKind.
	Kind     sim.SupplyKind `json:"kind"`
	Blocking []string       `json:"blocking"`
}

// minContention keeps the list an answer rather than an inventory: over a long
// enough run almost every place is briefly short of something.
const minContention = 0.01

// shortages accumulates the blocked-time bookkeeping across realizations. The
// rule it implements is the same one pkg/runtime/sim uses, deliberately: two
// engines that report different constraints for one model is the failure both
// files' comments were written about.
type shortages struct {
	waited     []float64
	holding    []map[string]bool
	candidates []int
	seen       []bool
}

func newShortages(nPlaces int) *shortages {
	return &shortages{
		waited:  make([]float64, nPlaces),
		holding: make([]map[string]bool, nPlaces),
		seen:    make([]bool, nPlaces),
	}
}

// note records that place is the sole unmet input of transition id at the
// current marking. One short place commonly holds up several transitions, so it
// is credited once per step however many it blocks.
func (s *shortages) note(place int, id string) {
	if s.holding[place] == nil {
		s.holding[place] = map[string]bool{}
	}
	s.holding[place][id] = true
	if !s.seen[place] {
		s.seen[place] = true
		s.candidates = append(s.candidates, place)
	}
}

func (s *shortages) credit(dt float64) {
	for _, p := range s.candidates {
		s.waited[p] += dt
		s.seen[p] = false
	}
	s.candidates = s.candidates[:0]
}

// report ranks the shortages. Capacity constraints come first and the longest
// wait first within each kind — same order pkg/runtime/sim produces, from the
// same classification, because two engines disagreeing about what a model is
// limited by is the failure this file's comments are about.
func (s *shortages) report(m *goflowmetamodel.Model, labels []string, totalTime float64) []contendedPlace {
	if totalTime <= 0 {
		return nil
	}
	kinds := sim.ClassifySupply(m)
	var out []contendedPlace
	for i, label := range labels {
		f := s.waited[i] / totalTime
		if f < minContention {
			continue
		}
		held := make([]string, 0, len(s.holding[i]))
		for id := range s.holding[i] {
			held = append(held, id)
		}
		sort.Strings(held)
		kind := kinds[label]
		if kind == "" {
			kind = sim.SupplyQueue
		}
		out = append(out, contendedPlace{Place: label, Fraction: f, Kind: kind, Blocking: held})
	}
	sort.Slice(out, func(i, j int) bool {
		ci, cj := out[i].Kind.IsCapacity(), out[j].Kind.IsCapacity()
		if ci != cj {
			return ci
		}
		if out[i].Fraction != out[j].Fraction {
			return out[i].Fraction > out[j].Fraction
		}
		return out[i].Place < out[j].Place
	})
	return out
}

// soleShortInput returns the index of the only input place this transition is
// short of, or -1 when it is short of none or of more than one. With two things
// missing neither is the reason the firing did not happen.
func (e *transitionEntry) soleShortInput(marking []int) int {
	short := -1
	for _, in := range e.inputs {
		if marking[in.placeIdx] >= in.weight {
			continue
		}
		if short >= 0 {
			return -1
		}
		short = in.placeIdx
	}
	return short
}

// transitionEntry holds the pre-indexed input/output arcs for one
// transition, in terms of place indices in a stable order. Used by the SSA
// inner loop so we don't keep doing map lookups per step.
type transitionEntry struct {
	id      string
	rate    float64
	inputs  []arcEntry
	outputs []arcEntry

	// Constraints that decide enablement without moving tokens. Unlike the
	// continuous solver, a discrete engine has a firing instant and so can
	// honour all of these exactly — this tool was simply not doing it, and
	// treated a read arc as an input and an inhibitor as a *source*.
	reads    []arcEntry
	inhibits []arcEntry
	caps     []capEntry
}

type arcEntry struct {
	placeIdx int
	weight   int

	// kinetic reports whether this input belongs in the rate law as well as in
	// the enablement test. A non-kinetic input is a prerequisite, not an
	// accelerant: it gates the firing and is consumed by it, but does not scale
	// how often it happens. Mass action over every input is right for chemistry
	// and wrong for a service system — a barista is not a reactant, and a full
	// pantry does not make a drink pour faster.
	kinetic bool
}

// capEntry is a post-firing capacity bound: firing raises placeIdx by delta,
// and the place may not end above limit.
type capEntry struct {
	placeIdx int
	delta    int
	limit    int
}

// gated reports whether the non-consuming constraints allow this transition to
// fire. Consuming arcs are checked by the propensity loop, which needs the
// counts anyway.
func (e *transitionEntry) gated(marking []int) bool {
	for _, r := range e.reads {
		if marking[r.placeIdx] < r.weight {
			return false
		}
	}
	for _, h := range e.inhibits {
		if marking[h.placeIdx] >= h.weight {
			return false
		}
	}
	for _, c := range e.caps {
		if marking[c.placeIdx]+c.delta > c.limit {
			return false
		}
	}
	return true
}

func handleStochastic(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model parameter: %v", err)), nil
	}
	parsed, err := parseModelV2(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model JSON: %v", err)), nil
	}
	model := parsed.Model

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

	tspan := [2]float64{0, 10}
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

	realizations := request.GetInt("realizations", 1)
	if realizations < 1 {
		realizations = 1
	}
	if realizations > 50 {
		realizations = 50
	}
	samples := request.GetInt("samples", 200)
	if samples < 2 {
		samples = 2
	}
	seed := int64(request.GetInt("seed", 42))

	// Build stable place ordering and look-up table.
	placeIdx := map[string]int{}
	stateLabels := make([]string, len(model.Places))
	initialMarking := make([]int, len(model.Places))
	for i, p := range model.Places {
		placeIdx[p.ID] = i
		stateLabels[i] = p.ID
		initialMarking[i] = p.Initial
	}

	// Pre-index transitions with arc lookups in place-index form.
	transitions := buildTransitionEntries(model, placeIdx, rates)
	if len(transitions) == 0 {
		return mcp.NewToolResultError("model has no transitions"), nil
	}

	// Time grid for sampling — all realizations record values at the same
	// grid points so we can compute mean/stdev cleanly.
	times := make([]float64, samples)
	for i := 0; i < samples; i++ {
		times[i] = tspan[0] + (tspan[1]-tspan[0])*float64(i)/float64(samples-1)
	}

	// Run all realizations. trajectories[r][p][t] = marking of place p at
	// time t in realization r.
	trajectories := make([][][]float64, realizations)
	rng := rand.New(rand.NewSource(seed))
	short := newShortages(len(stateLabels))
	for r := 0; r < realizations; r++ {
		// Each realization uses an independent stream derived from the
		// master seed so the result is reproducible across runs.
		subSeed := rng.Int63()
		trajectories[r] = runSSA(initialMarking, transitions, len(stateLabels), times, subSeed, short)
	}

	// Aggregate: mean and stdev per (place, time).
	mean := make(map[string][]float64, len(stateLabels))
	stdev := make(map[string][]float64, len(stateLabels))
	for p, label := range stateLabels {
		m := make([]float64, samples)
		s := make([]float64, samples)
		for t := 0; t < samples; t++ {
			sum := 0.0
			for r := 0; r < realizations; r++ {
				sum += trajectories[r][p][t]
			}
			m[t] = sum / float64(realizations)
			if realizations > 1 {
				ss := 0.0
				for r := 0; r < realizations; r++ {
					d := trajectories[r][p][t] - m[t]
					ss += d * d
				}
				s[t] = math.Sqrt(ss / float64(realizations-1))
			}
		}
		mean[label] = m
		if realizations > 1 {
			stdev[label] = s
		}
	}

	finalMean := map[string]float64{}
	for label, vals := range mean {
		finalMean[label] = vals[len(vals)-1]
	}

	resp := stochasticResponse{
		StateLabels:  stateLabels,
		Tspan:        tspan,
		Rates:        rates,
		Realizations: realizations,
		Times:        times,
		Mean:         mean,
		Stdev:        stdev,
		FinalMean:    finalMean,
		Contended:    short.report(model, stateLabels, (tspan[1]-tspan[0])*float64(realizations)),
	}

	if request.GetBool("verbose", false) {
		resp.Explanation = verboseAnnotation("ssa",
			fmt.Sprintf("realizations=%d, tspan=[%v, %v], %d transitions", realizations, tspan[0], tspan[1], len(model.Transitions)))
	}

	text, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal response: %v", err)), nil
	}

	if pngBytes, perr := renderStochasticPNG(resp, variables, trajectories, placeIdx); perr == nil {
		return mcp.NewToolResultImage(string(text), base64.StdEncoding.EncodeToString(pngBytes), "image/png"), nil
	}
	return mcp.NewToolResultText(string(text)), nil
}

// buildTransitionEntries precomputes per-transition arc indices so the SSA
// inner loop can compute propensities without map lookups.
// buildTransitionEntries precomputes per-transition arc indices so the SSA
// inner loop can compute propensities without map lookups.
//
// The classification comes from metamodel's firing rule rather than being
// re-derived from arc.From/arc.To here. That is the whole point: this function
// used to sort arcs itself and got read and inhibitor arcs exactly backwards,
// while three other engines in the same two repos got them right.
func buildTransitionEntries(model *goflowmetamodel.Model, placeIdx map[string]int, rates map[string]float64) []transitionEntry {
	limits := map[string]int{}
	for i := range model.Places {
		if p := &model.Places[i]; p.IsToken() && p.Capacity > 0 {
			limits[p.ID] = p.Capacity
		}
	}

	out := make([]transitionEntry, 0, len(model.Transitions))
	for _, t := range model.Transitions {
		e := transitionEntry{id: t.ID, rate: rates[t.ID]}
		delta := map[string]int{}

		for _, in := range model.Inputs(t.ID) {
			if idx, ok := placeIdx[in.Place]; ok {
				e.inputs = append(e.inputs, arcEntry{placeIdx: idx, weight: in.Weight, kinetic: in.Kinetic})
				delta[in.Place] -= in.Weight
			}
		}
		for _, o := range model.Outputs(t.ID) {
			if idx, ok := placeIdx[o.Place]; ok {
				// kinetic is carried for uniformity; it means nothing on an
				// output or a test arc, neither of which is ever in a rate law.
				e.outputs = append(e.outputs, arcEntry{placeIdx: idx, weight: o.Weight, kinetic: o.Kinetic})
				delta[o.Place] += o.Weight
			}
		}
		for _, test := range model.Tests(t.ID) {
			idx, ok := placeIdx[test.Place]
			if !ok {
				continue
			}
			a := arcEntry{placeIdx: idx, weight: test.Weight, kinetic: test.Kinetic}
			if test.Type == goflowmetamodel.InhibitorArc {
				e.inhibits = append(e.inhibits, a)
			} else {
				e.reads = append(e.reads, a)
			}
		}
		// Only a net increase can breach a bound, netted against what the same
		// firing consumes — so a full place still admits a self-loop.
		for place, limit := range limits {
			idx, ok := placeIdx[place]
			if d := delta[place]; ok && d > 0 {
				e.caps = append(e.caps, capEntry{placeIdx: idx, delta: d, limit: limit})
			}
		}
		sort.Slice(e.caps, func(i, j int) bool { return e.caps[i].placeIdx < e.caps[j].placeIdx })

		out = append(out, e)
	}
	return out
}

// runSSA executes one Gillespie realization, recording markings at every
// time in samples. Returns trajectories[placeIdx][sampleIdx].
func runSSA(initial []int, transitions []transitionEntry, nPlaces int, samples []float64, seed int64, short *shortages) [][]float64 {
	rng := rand.New(rand.NewSource(seed))
	if short != nil {
		short.credit(0) // a run cut short by maxSteps leaves scratch behind
	}
	marking := make([]int, nPlaces)
	copy(marking, initial)

	trajectories := make([][]float64, nPlaces)
	for i := range trajectories {
		trajectories[i] = make([]float64, len(samples))
	}

	t := samples[0]
	nextSample := 0
	// Pre-fill samples at t=samples[0] with the initial state.
	for nextSample < len(samples) && samples[nextSample] <= t {
		for p := 0; p < nPlaces; p++ {
			trajectories[p][nextSample] = float64(marking[p])
		}
		nextSample++
	}

	tEnd := samples[len(samples)-1]
	const maxSteps = 1_000_000

	for step := 0; step < maxSteps && t < tEnd; step++ {
		// Compute propensities.
		totalRate := 0.0
		propensities := make([]float64, len(transitions))
		for i, tr := range transitions {
			a := tr.rate
			enabled := true
			for _, in := range tr.inputs {
				m := marking[in.placeIdx]
				if m < in.weight {
					enabled = false
					break
				}
				// Combinatorial selection: C(m, w), for kinetic inputs only.
				if in.kinetic {
					a *= combinations(m, in.weight)
				}
			}
			// Read arcs, inhibitors and capacity decide enablement without
			// appearing in the propensity: a blocked transition has rate zero,
			// it does not merely fire more slowly. A non-kinetic input is the
			// third case — enabled by it, consumed from it, not sped up by it.
			if !enabled || !transitions[i].gated(marking) {
				a = 0
			}
			propensities[i] = a
			totalRate += a

			// Why this transition is not firing, when it is not. Only the
			// consuming arcs are attributed: a read arc or an inhibitor is the
			// model refusing outright, not a shortage anyone can go and fix.
			if a == 0 && short != nil {
				if p := transitions[i].soleShortInput(marking); p >= 0 && transitions[i].gated(marking) {
					short.note(p, transitions[i].id)
				}
			}
		}
		if totalRate <= 0 {
			// Dead state — no transition can fire, and no amount of time
			// changes that, so the rest of the horizon was spent waiting for
			// whatever is short.
			if short != nil {
				short.credit(tEnd - t)
			}
			break
		}

		// Time to next event ~ Exp(totalRate).
		u := rng.Float64()
		if u <= 0 {
			u = 1e-300
		}
		dt := -math.Log(u) / totalRate
		if short != nil {
			short.credit(math.Min(dt, tEnd-t))
		}
		t += dt

		// Record any sample points we crossed at the pre-firing marking.
		for nextSample < len(samples) && samples[nextSample] <= t {
			for p := 0; p < nPlaces; p++ {
				trajectories[p][nextSample] = float64(marking[p])
			}
			nextSample++
		}
		if t > tEnd {
			break
		}

		// Choose which transition fires.
		r := rng.Float64() * totalRate
		chosen := -1
		acc := 0.0
		for i, a := range propensities {
			acc += a
			if r <= acc {
				chosen = i
				break
			}
		}
		if chosen < 0 {
			chosen = len(propensities) - 1
		}
		tr := transitions[chosen]
		for _, in := range tr.inputs {
			marking[in.placeIdx] -= in.weight
		}
		for _, out := range tr.outputs {
			marking[out.placeIdx] += out.weight
		}
	}

	// Carry final marking forward through any remaining sample points.
	for ; nextSample < len(samples); nextSample++ {
		for p := 0; p < nPlaces; p++ {
			trajectories[p][nextSample] = float64(marking[p])
		}
	}
	return trajectories
}

// combinations returns C(m, w) — the number of distinct multisets of size w
// drawn from m available tokens. C(m, 0) = 1, C(m, 1) = m. For arc weights
// >1 this enforces the requirement that enough indistinguishable tokens are
// present to support the firing.
func combinations(m, w int) float64 {
	if w == 0 {
		return 1
	}
	if w == 1 {
		return float64(m)
	}
	out := 1.0
	for i := 0; i < w; i++ {
		out *= float64(m - i)
	}
	for i := 2; i <= w; i++ {
		out /= float64(i)
	}
	return out
}

// renderStochasticPNG plots the stochastic trajectories. With one
// realization, draws the trajectory as a step function (color per place).
// With >1 realizations, shows mean + shaded ±stdev band per variable.
func renderStochasticPNG(resp stochasticResponse, variables []string, trajectories [][][]float64, placeIdx map[string]int) ([]byte, error) {
	const W, H = 760, 460
	dc := gg.NewContext(W, H)
	dc.SetHexColor("#ffffff")
	dc.Clear()

	// Compute y range across selected variables and all realizations.
	xmin := resp.Times[0]
	xmax := resp.Times[len(resp.Times)-1]
	ymin := math.Inf(1)
	ymax := math.Inf(-1)
	for _, v := range variables {
		for _, y := range resp.Mean[v] {
			if y < ymin {
				ymin = y
			}
			if y > ymax {
				ymax = y
			}
		}
		if sd, ok := resp.Stdev[v]; ok && len(sd) == len(resp.Mean[v]) {
			for i, m := range resp.Mean[v] {
				s := sd[i]
				if m+s > ymax {
					ymax = m + s
				}
				if m-s < ymin {
					ymin = m - s
				}
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

	title := fmt.Sprintf("Stochastic SSA — %d realization", resp.Realizations)
	if resp.Realizations > 1 {
		title += "s"
	}
	drawPlotFrame(dc, xmin, xmax, ymin, ymax, title, "Time", "Tokens", 0, 0, W, H)

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

	dc.SetLineWidth(2)
	for i, v := range variables {
		color := plotColors[i%len(plotColors)]
		bandColor := lightenColor(color, 0.4)

		// Shaded ±stdev band when applicable.
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

		// Mean (or single realization) curve on top.
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
			dc.DrawStringAnchored(v, legendX+24, legendY+6, 0, 0.5)
			legendY += 18
		}
		if resp.Realizations > 1 {
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

// lightenColor blends a hex color with white by t in [0,1]. Used for the
// ±stdev band so it sits visibly beneath the mean curve.
func lightenColor(hex string, t float64) string {
	if len(hex) != 7 || hex[0] != '#' {
		return hex
	}
	var r, g, b int
	fmt.Sscanf(hex[1:], "%02x%02x%02x", &r, &g, &b)
	r = int(float64(r) + (255-float64(r))*t)
	g = int(float64(g) + (255-float64(g))*t)
	b = int(float64(b) + (255-float64(b))*t)
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

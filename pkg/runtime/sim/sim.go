// Package sim runs a model forward in time without changing it.
//
// Generated applications embed their model, so they can answer questions about
// what happens next — "when do I run out of beans", "how long is the queue at
// this arrival rate" — from the marking they are already holding.
//
// Everything here is a **pure projection**. A simulation takes a marking and
// returns a trajectory; it never fires a transition on an aggregate, never
// appends an event, and never touches a store. That distinction is the whole
// point of the package. The coffee-shop dashboard predates it and does the
// opposite: it drives its simulation by POSTing real transitions
// (dashboard.js executeTransition), so every simulated tick writes a real event,
// 409s are swallowed as "expected", and the app needs a reset button to
// undo having been asked a hypothetical question. A forecast should not be able
// to corrupt the thing it forecasts.
//
// Two engines, because they answer different questions:
//
//   - Forecast is the continuous one: mass-action ODE over a real-valued
//     marking, solved with Tsit5. Smooth, deterministic, fast. Right when token
//     counts are large enough that the average is the answer — 1000 coffee
//     beans drawn down 20 at a time.
//   - Simulate is the discrete one: Gillespie's SSA over integer counts, where
//     firings are random events and the result has real variance. Right when
//     counts are small enough that noise decides the outcome — three baristas,
//     a queue of four.
//
// Asking for the mean of an SSA run and asking the ODE are not the same
// question, and for a queue they can disagree sharply.
package sim

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"

	"github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/go-pflow/petri"
	"github.com/pflow-xyz/go-pflow/solver"

	"github.com/pflow-xyz/petri-pilot/pkg/dsl"
)

// DefaultRate is used for a transition whose model declares none. Mass-action
// with unit rate means "as fast as tokens allow", which is the least surprising
// reading of an unannotated net.
const DefaultRate = 1.0

// Rates returns each transition's firing rate, defaulting the unset ones.
//
// The model is the single source of truth here. A frontend that keeps its own
// rate table — as the coffee-shop dashboard does — will drift from the net it
// claims to be simulating, and nothing will report the divergence.
func Rates(m *metamodel.Model) map[string]float64 {
	out := make(map[string]float64, len(m.Transitions))
	for _, t := range m.Transitions {
		r := t.Rate
		if r == 0 {
			r = DefaultRate
		}
		out[t.ID] = r
	}
	// A model-level solver config overrides per-transition rates, matching how
	// petri_ode reads the same schema.
	if m.Simulation != nil && m.Simulation.Solver != nil {
		for id, r := range m.Simulation.Solver.Rates {
			out[id] = r
		}
	}
	return out
}

// Options configure a run. The zero value is usable: unit rates, a one-hour
// horizon and 60 samples.
type Options struct {
	// Horizon is how far forward to run, in the model's time unit.
	Horizon float64
	// Samples is how many points to report along the way.
	Samples int
	// Rates overrides individual transition rates; unset ones come from the model.
	Rates map[string]float64
	// Seed makes an SSA run reproducible. Zero picks a fixed seed rather than a
	// random one, so an unconfigured call is still repeatable — a forecast that
	// changes on refresh is indistinguishable from a bug.
	Seed int64
	// Realizations is how many independent SSA runs to average. Ignored by Forecast.
	Realizations int
}

// startFrom overlays a caller's marking onto the one the model declares.
//
// Presence decides, not value: a place the caller names at zero is zero, and a
// place they omit keeps the model's initial count. That makes a sparse map a
// scenario — "same shop, but three baristas" — rather than a marking the caller
// has to restate in full and can silently get wrong.
func startFrom(m *metamodel.Model, marking map[string]int) metamodel.Marking {
	mk := m.InitialMarking()
	for p, n := range marking {
		if _, isTokenPlace := mk[p]; isTokenPlace {
			mk[p] = n
		}
	}
	return mk
}

func (o Options) withDefaults(m *metamodel.Model) Options {
	if o.Horizon <= 0 {
		o.Horizon = 1
	}
	if o.Samples <= 1 {
		o.Samples = 60
	}
	if o.Realizations <= 0 {
		o.Realizations = 1
	}
	rates := Rates(m)
	for id, r := range o.Rates {
		rates[id] = r
	}
	o.Rates = rates
	return o
}

// Series is one place's values over time.
type Series struct {
	Place  string    `json:"place"`
	Values []float64 `json:"values"`
	// StdDev is populated only by Simulate with Realizations > 1.
	StdDev []float64 `json:"std_dev,omitempty"`
}

// Result is a trajectory: sample times plus one series per token place.
type Result struct {
	Times  []float64          `json:"times"`
	Series []Series           `json:"series"`
	Final  map[string]float64 `json:"final"`
	// Depleted names places that reach zero within the horizon, earliest first.
	// This is the question a resource model is usually being asked.
	Depleted []Depletion `json:"depleted,omitempty"`
	Method   string      `json:"method"`

	// Diverged is set when the continuous solution left the range a token count
	// can occupy — negative, or not finite. Reported rather than returned as if
	// it were an answer: a forecast of minus two trillion cups is not a smaller
	// truth than a good one, it is noise, and a dashboard will happily plot it.
	Diverged bool   `json:"diverged,omitempty"`
	Reason   string `json:"reason,omitempty"`

	// Caveats name constraints the model expresses that this run could not
	// enforce. An empty list is a claim: everything the net says was applied.
	Caveats []string `json:"caveats,omitempty"`

	// Metrics are the numbers an operator asks for, as distinct from the
	// trajectory an analyst reads. Populated by Simulate only — a continuous
	// solution has no firings to count and no percentiles to take.
	Metrics *Metrics `json:"metrics,omitempty"`
}

// Metrics summarise a stochastic run in the terms of the thing being modelled.
type Metrics struct {
	// Throughput is the mean number of firings per transition over the horizon.
	Throughput map[string]float64 `json:"throughput"`
	// Mean and P95 per place. A queue's average is reassuring and its 95th
	// percentile is what the customer standing in it experiences.
	Mean map[string]float64 `json:"mean"`
	P95  map[string]float64 `json:"p95"`
	// Utilization is the fraction of a resource pool that is busy, for every
	// pair of places named "<pool>/busy" and "<pool>/available" (or "busy" and
	// "available" within one subnet). Absent when the model has no such pair.
	Utilization map[string]float64 `json:"utilization,omitempty"`
}

// Depletion records when a place first runs out.
type Depletion struct {
	Place string  `json:"place"`
	At    float64 `json:"at"`

	// Recovered is true when the place was back above the floor by the end of
	// the horizon.
	//
	// Without this the metric conflates two different events. A pantry running
	// out of beans is a problem; a barista pool reaching zero is just everyone
	// being busy for a moment, and it refills itself by construction. Both hit
	// the floor, so both are "depleted" — reporting them identically told a
	// café owner their staff had run out.
	Recovered bool `json:"recovered,omitempty"`
}

// Forecast runs the continuous mass-action ODE forward from marking.
//
// Deterministic: the same marking and rates always give the same answer, which
// is what makes it usable as a cached projection.
func Forecast(m *metamodel.Model, marking map[string]int, opts Options) (*Result, error) {
	opts = opts.withDefaults(m)

	// A continuous solution has no firing instant, so there is nowhere to test a
	// read arc, an inhibitor, a capacity or a guard — the solver ignores all
	// four. On an ungated net that costs nothing; on a staffing model it means
	// the curve shows a shop with unlimited baristas. Refuse rather than plot
	// it: the caller has a discrete engine one call away.
	if gating := m.Gating(); len(gating) > 0 {
		return &Result{
			Method:   "ode",
			Times:    sampleTimes(opts),
			Final:    map[string]float64{},
			Diverged: true,
			Reason: "this model constrains firing in ways a continuous solution cannot express, so the ODE would " +
				"silently model an unconstrained system. Use the discrete engine (Simulate). Specifically: " +
				strings.Join(gating, "; "),
			Caveats: gating,
		}, nil
	}

	start := startFrom(m, marking)
	net, places, err := toNet(m, start)
	if err != nil {
		return nil, err
	}

	state := make(map[string]float64, len(places))
	for _, p := range places {
		state[p] = float64(start[p])
	}

	prob := solver.NewProblem(net, state, [2]float64{0, opts.Horizon}, opts.Rates)
	sol := solver.Solve(prob, solver.Tsit5(), solver.DefaultOptions())
	if sol == nil {
		return nil, fmt.Errorf("solver returned no solution")
	}

	res := &Result{Method: "ode", Final: map[string]float64{}}
	res.Times = sampleTimes(opts)
	for _, p := range places {
		// The solver chooses its own adaptive timesteps, so the trajectory is
		// resampled onto the caller's grid rather than reported at whatever
		// points Tsit5 happened to take.
		vals := resample(sol.T, sol.GetVariable(p), res.Times)
		res.Series = append(res.Series, Series{Place: p, Values: vals})
		res.Final[p] = vals[len(vals)-1]
	}
	res.Depleted = depletions(m, res)
	checkDivergence(res)
	return res, nil
}

// checkDivergence flags a continuous solution that has left the physically
// meaningful range.
//
// Mass action raises a place to the power of its arc weight, so a model with
// heavy arcs — the coffee shop draws 20 beans per espresso — produces a term in
// beans^20. With a thousand beans that is 1e60, and the ODE runs away long
// before the horizon. The net is fine and the discrete engine handles it
// without complaint; it is the continuous approximation that does not apply.
// Saying so is more useful than any number this run could return.
func checkDivergence(res *Result) {
	for _, s := range res.Series {
		for _, v := range s.Values {
			if math.IsNaN(v) || math.IsInf(v, 0) || v < -1e-6 {
				res.Diverged = true
				res.Reason = fmt.Sprintf(
					"the continuous solution left the range a token count can occupy (%s reached %.3g). "+
						"Mass action raises a place to the power of its arc weight, so heavy arcs make the ODE stiff. "+
						"Use the discrete engine (Simulate) for this model, or scale the rates down.", s.Place, v)
				return
			}
		}
	}
}

// Simulate runs Gillespie's SSA over the discrete marking.
//
// With Realizations > 1 the series carry the mean and StdDev the spread, which
// is the honest way to report a stochastic answer: a single run of a queue is
// an anecdote.
func Simulate(m *metamodel.Model, marking map[string]int, opts Options) (*Result, error) {
	opts = opts.withDefaults(m)

	trs, places, caveats, err := compile(m, opts.Rates)
	if err != nil {
		return nil, err
	}
	times := sampleTimes(opts)
	firings := make([]float64, len(trs))

	sums := make([][]float64, len(places))
	sumSquares := make([][]float64, len(places))
	// Every observation, across realizations and time, kept so percentiles are
	// taken over the actual population rather than over an average that has
	// already smoothed the tail away.
	observed := make([][]float64, len(places))
	for i := range places {
		sums[i] = make([]float64, len(times))
		sumSquares[i] = make([]float64, len(times))
		observed[i] = make([]float64, 0, len(times)*opts.Realizations)
	}

	seed := opts.Seed
	if seed == 0 {
		seed = 1
	}
	initial := startFrom(m, marking)
	for r := 0; r < opts.Realizations; r++ {
		start := make([]int, len(places))
		for i, p := range places {
			start[i] = initial[p]
		}
		counts := make([]int, len(trs))
		traj := ssa(trs, places, start, times, rand.New(rand.NewSource(seed+int64(r))), counts) //nolint:gosec // not cryptographic
		for i, c := range counts {
			firings[i] += float64(c)
		}
		for p := range places {
			for i, v := range traj[p] {
				sums[p][i] += v
				sumSquares[p][i] += v * v
			}
			observed[p] = append(observed[p], traj[p]...)
		}
	}

	n := float64(opts.Realizations)
	res := &Result{Method: "ssa", Times: times, Final: map[string]float64{}}
	for i, p := range places {
		mean := make([]float64, len(times))
		var sd []float64
		if opts.Realizations > 1 {
			sd = make([]float64, len(times))
		}
		for j := range times {
			mean[j] = sums[i][j] / n
			if sd != nil {
				variance := sumSquares[i][j]/n - mean[j]*mean[j]
				if variance < 0 {
					variance = 0 // floating-point noise around zero
				}
				sd[j] = math.Sqrt(variance)
			}
		}
		res.Series = append(res.Series, Series{Place: p, Values: mean, StdDev: sd})
		res.Final[p] = mean[len(mean)-1]
	}
	res.Depleted = depletions(m, res)
	res.Caveats = caveats
	res.Metrics = metricsOf(trs, firings, places, observed, n)
	return res, nil
}

// metricsOf turns a run into the numbers an operator asks for.
func metricsOf(trs []transition, firings []float64, places []string, observed [][]float64, n float64) *Metrics {
	mt := &Metrics{
		Throughput: make(map[string]float64, len(trs)),
		Mean:       make(map[string]float64, len(places)),
		P95:        make(map[string]float64, len(places)),
	}
	for i := range trs {
		mt.Throughput[trs[i].id] = firings[i] / n
	}
	for i, p := range places {
		mt.Mean[p] = mean(observed[i])
		mt.P95[p] = percentile(observed[i], 0.95)
	}
	mt.Utilization = utilization(places, mt.Mean)
	return mt
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var total float64
	for _, x := range xs {
		total += x
	}
	return total / float64(len(xs))
}

// percentile is the nearest-rank percentile of a copy of xs, so the caller's
// slice keeps its time ordering.
func percentile(xs []float64, q float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sorted := append([]float64(nil), xs...)
	sort.Float64s(sorted)
	i := int(math.Ceil(q*float64(len(sorted)))) - 1
	if i < 0 {
		i = 0
	}
	if i >= len(sorted) {
		i = len(sorted) - 1
	}
	return sorted[i]
}

// utilization pairs up the "available"/"busy" places a resource pool exposes
// and reports the busy fraction — the answer to "are my baristas standing
// around, or are they the bottleneck?".
//
// Matched by suffix so it works both inside a single net ("available") and
// across a composed one ("staff/available"), and only when both halves are
// present: a lone "busy" place is not evidence of a pool.
func utilization(places []string, means map[string]float64) map[string]float64 {
	pools := map[string][2]float64{}
	seen := map[string][2]bool{}
	for _, p := range places {
		pool, role := "", ""
		switch {
		case p == "available" || strings.HasSuffix(p, "/available"):
			pool, role = strings.TrimSuffix(strings.TrimSuffix(p, "available"), "/"), "available"
		case p == "busy" || strings.HasSuffix(p, "/busy"):
			pool, role = strings.TrimSuffix(strings.TrimSuffix(p, "busy"), "/"), "busy"
		case p == "in_use" || strings.HasSuffix(p, "/in_use"):
			pool, role = strings.TrimSuffix(strings.TrimSuffix(p, "in_use"), "/"), "busy"
		default:
			continue
		}
		v, s := pools[pool], seen[pool]
		if role == "available" {
			v[0], s[0] = means[p], true
		} else {
			v[1], s[1] = means[p], true
		}
		pools[pool], seen[pool] = v, s
	}

	var out map[string]float64
	for pool, v := range pools {
		if !seen[pool][0] || !seen[pool][1] {
			continue
		}
		total := v[0] + v[1]
		if total <= 0 {
			continue
		}
		if out == nil {
			out = map[string]float64{}
		}
		name := pool
		if name == "" {
			name = "pool"
		}
		out[name] = v[1] / total
	}
	return out
}

// resample linearly interpolates a solver trajectory onto the requested times.
func resample(srcT, srcV, dstT []float64) []float64 {
	out := make([]float64, len(dstT))
	if len(srcT) == 0 || len(srcV) == 0 {
		return out
	}
	j := 0
	for i, t := range dstT {
		for j+1 < len(srcT) && srcT[j+1] < t {
			j++
		}
		switch {
		case t <= srcT[0]:
			out[i] = srcV[0]
		case j+1 >= len(srcT):
			out[i] = srcV[len(srcV)-1]
		default:
			span := srcT[j+1] - srcT[j]
			if span <= 0 {
				out[i] = srcV[j]
				continue
			}
			f := (t - srcT[j]) / span
			out[i] = srcV[j] + f*(srcV[j+1]-srcV[j])
		}
	}
	return out
}

func sampleTimes(opts Options) []float64 {
	times := make([]float64, opts.Samples)
	step := opts.Horizon / float64(opts.Samples-1)
	for i := range times {
		times[i] = float64(i) * step
	}
	return times
}

// depletions reports the first sample time at which each place can no longer
// supply anything that draws on it, earliest first. A place that starts empty is
// not "depleted".
//
// Not "reaches zero": a place is out when it falls below the smallest weight
// any transition takes from it. Ten coffee beans and a weight-20 espresso arc
// is a shop that has run out of coffee, and reporting it as still stocked —
// because the number is not literally zero — answers the wrong question. The
// threshold comes from the model, so it is right for each place rather than a
// constant someone has to tune.
func depletions(m *metamodel.Model, res *Result) []Depletion {
	floor := map[string]int{}
	for i := range m.Transitions {
		for _, in := range m.Inputs(m.Transitions[i].ID) {
			if w, seen := floor[in.Place]; !seen || in.Weight < w {
				floor[in.Place] = in.Weight
			}
		}
	}

	var out []Depletion
	for _, s := range res.Series {
		if len(s.Values) == 0 {
			continue
		}
		threshold := float64(floor[s.Place]) // absent ⇒ 0: nothing draws on it
		if s.Values[0] < threshold || s.Values[0] <= 0 {
			continue // already out, or never stocked
		}
		for i, v := range s.Values {
			if v < threshold || v <= 0 {
				last := s.Values[len(s.Values)-1]
				out = append(out, Depletion{
					Place:     s.Place,
					At:        res.Times[i],
					Recovered: last >= threshold && last > 0,
				})
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].At != out[j].At {
			return out[i].At < out[j].At
		}
		return out[i].Place < out[j].Place
	})
	return out
}

// --- discrete engine -----------------------------------------------------

type arc struct {
	place  int
	weight int
}

// capBound is a post-firing capacity check, precomputed: firing this transition
// raises place by delta, and the place may not end above limit.
type capBound struct {
	place int
	delta int
	limit int
}

type transition struct {
	id      string
	rate    float64
	inputs  []arc
	outputs []arc

	// reads and inhibits gate a firing without moving tokens. Leaving them out
	// — which this engine did until the staffing work — makes every constraint
	// a model expresses structurally invisible to the simulation of it.
	reads    []arc
	inhibits []arc
	caps     []capBound

	// guard is evaluated against the marking each step, when it is decidable
	// from the marking alone. A guard needing action parameters is reported as
	// a caveat rather than guessed at.
	guard string
}

// gated reports whether the non-consuming constraints allow this transition to
// fire at marking. The consuming arcs are checked by the propensity calculation
// itself, which needs the token counts anyway.
func (t *transition) gated(marking []int) bool {
	for _, r := range t.reads {
		if marking[r.place] < r.weight {
			return false
		}
	}
	for _, h := range t.inhibits {
		if marking[h.place] >= h.weight {
			return false
		}
	}
	for _, c := range t.caps {
		if marking[c.place]+c.delta > c.limit {
			return false
		}
	}
	return true
}

// compile turns the model into index-addressed transitions, which is what makes
// the inner SSA loop cheap.
//
// The classification — what is an input, what merely tests the marking, what
// weight an unset arc carries — comes from metamodel's firing rule rather than
// being re-derived here. Only the arithmetic is local. Four engines used to own
// four answers to that question and two of them were wrong; the answer now has
// one home.
//
// Returns any caveats: constraints the model expresses that this engine cannot
// enforce. They are reported, never silently dropped.
func compile(m *metamodel.Model, rates map[string]float64) ([]transition, []string, []string, error) {
	places, index, err := tokenPlaces(m)
	if err != nil {
		return nil, nil, nil, err
	}
	if rates == nil {
		rates = Rates(m)
	}

	// Post-firing capacity bounds, keyed by the transition that could breach
	// them. Precomputed because the net delta per place is fixed by the net.
	limits := map[string]int{}
	for i := range m.Places {
		if p := &m.Places[i]; p.IsToken() && p.Capacity > 0 {
			limits[p.ID] = p.Capacity
		}
	}

	var caveats []string
	out := make([]transition, 0, len(m.Transitions))
	for i := range m.Transitions {
		t := &m.Transitions[i]
		tr := transition{id: t.ID, rate: rates[t.ID]}

		delta := map[string]int{}
		for _, in := range m.Inputs(t.ID) {
			tr.inputs = append(tr.inputs, arc{place: index[in.Place], weight: in.Weight})
			delta[in.Place] -= in.Weight
		}
		for _, o := range m.Outputs(t.ID) {
			tr.outputs = append(tr.outputs, arc{place: index[o.Place], weight: o.Weight})
			delta[o.Place] += o.Weight
		}
		for _, test := range m.Tests(t.ID) {
			a := arc{place: index[test.Place], weight: test.Weight}
			if test.Type == metamodel.InhibitorArc {
				tr.inhibits = append(tr.inhibits, a)
			} else {
				tr.reads = append(tr.reads, a)
			}
		}
		// Only a net increase can breach a bound, and the increase is netted
		// against what the same firing consumes — a full place still admits a
		// self-loop.
		for _, p := range places {
			limit, bounded := limits[p]
			if d := delta[p]; bounded && d > 0 {
				tr.caps = append(tr.caps, capBound{place: index[p], delta: d, limit: limit})
			}
		}

		if t.Guard != "" {
			if decidableFromMarking(t.Guard, m) {
				tr.guard = t.Guard
			} else {
				caveats = append(caveats, fmt.Sprintf(
					"the guard on %s needs action parameters, so it is not enforced here; "+
						"this run may fire it where the application would refuse", t.ID))
			}
		}

		out = append(out, tr)
	}
	return out, places, caveats, nil
}

// decidableFromMarking reports whether a guard can be settled by token counts
// alone.
//
// Decided by trying it rather than by pattern-matching the expression: a guard
// that evaluates cleanly against a marking with no bindings in scope references
// nothing but the marking. Anything else — an action parameter, an ambient
// request value — fails to resolve, and guessing at it would be worse than
// admitting the gap.
func decidableFromMarking(guard string, m *metamodel.Model) bool {
	probe := dsl.Marking{}
	for i := range m.Places {
		if m.Places[i].IsToken() {
			probe[m.Places[i].ID] = m.Places[i].Initial
		}
	}
	_, err := dsl.Evaluate(guard, map[string]any{}, dsl.MakeAggregates(probe))
	return err == nil
}

// allows evaluates a transition's guard against the current marking.
func (t *transition) allows(places []string, marking []int) bool {
	if t.guard == "" {
		return true
	}
	mk := make(dsl.Marking, len(places))
	for i, p := range places {
		mk[p] = marking[i]
	}
	ok, err := dsl.Evaluate(t.guard, map[string]any{}, dsl.MakeAggregates(mk))
	// A guard that fails to evaluate mid-run was classified as decidable at
	// compile time, so this is a bug rather than a modelling choice. Refuse the
	// firing: over-reporting throughput is the more damaging error for a
	// capacity question.
	return err == nil && ok
}

func tokenPlaces(m *metamodel.Model) ([]string, map[string]int, error) {
	var places []string
	index := map[string]int{}
	for i := range m.Places {
		p := &m.Places[i]
		if !p.IsToken() {
			continue // data places hold values, not counts; they have no trajectory
		}
		index[p.ID] = len(places)
		places = append(places, p.ID)
	}
	if len(places) == 0 {
		return nil, nil, fmt.Errorf("model %q has no token places to simulate", m.Name)
	}
	return places, index, nil
}

// ssa is Gillespie's direct method. Ported from the petri_simulate tooling so
// the generated app and the MCP tool answer the same question the same way.
func ssa(trs []transition, places []string, marking []int, times []float64, rng *rand.Rand, fired []int) [][]float64 {
	nPlaces := len(marking)
	traj := make([][]float64, nPlaces)
	for p := range traj {
		traj[p] = make([]float64, len(times))
	}

	var t float64
	next := 0
	record := func() {
		for next < len(times) && times[next] <= t {
			for p := 0; p < nPlaces; p++ {
				traj[p][next] = float64(marking[p])
			}
			next++
		}
	}
	record()

	tEnd := times[len(times)-1]
	const maxSteps = 1_000_000
	propensities := make([]float64, len(trs))

	for step := 0; step < maxSteps && t < tEnd; step++ {
		total := 0.0
		for i := range trs {
			a := trs[i].rate
			for _, in := range trs[i].inputs {
				m := marking[in.place]
				if m < in.weight {
					a = 0
					break
				}
				a *= combinations(m, in.weight)
			}
			// Read arcs, inhibitors, capacity and marking guards decide
			// enablement without appearing in the propensity: a blocked
			// transition has rate zero, it does not merely fire more slowly.
			if a > 0 && (!trs[i].gated(marking) || !trs[i].allows(places, marking)) {
				a = 0
			}
			propensities[i] = a
			total += a
		}
		if total <= 0 {
			break // dead marking: nothing can fire, and no amount of time changes that
		}

		u := rng.Float64()
		if u <= 0 {
			u = 1e-300
		}
		t += -math.Log(u) / total
		record()
		if t > tEnd {
			break
		}

		r := rng.Float64() * total
		chosen, acc := len(trs)-1, 0.0
		for i, a := range propensities {
			if acc += a; r <= acc {
				chosen = i
				break
			}
		}
		for _, in := range trs[chosen].inputs {
			marking[in.place] -= in.weight
		}
		for _, out := range trs[chosen].outputs {
			marking[out.place] += out.weight
		}
		if fired != nil {
			fired[chosen]++
		}
	}

	// Hold the final marking through any remaining samples.
	for ; next < len(times); next++ {
		for p := 0; p < nPlaces; p++ {
			traj[p][next] = float64(marking[p])
		}
	}
	return traj
}

// combinations is C(m, w), the number of distinct ways to select w tokens from
// m. Mass action counts selections, not tokens: a transition needing 20 beans
// from a pile of 1000 is far more likely to fire than one needing 1000.
func combinations(m, w int) float64 {
	if w <= 0 {
		return 1
	}
	if m < w {
		return 0
	}
	result := 1.0
	for i := 0; i < w; i++ {
		result *= float64(m - i)
		result /= float64(i + 1)
	}
	return result
}

// toNet builds the petri.PetriNet the ODE solver consumes.
func toNet(m *metamodel.Model, marking metamodel.Marking) (*petri.PetriNet, []string, error) {
	places, index, err := tokenPlaces(m)
	if err != nil {
		return nil, nil, err
	}

	b := petri.Build()
	for _, p := range places {
		b.Place(p, float64(marking[p]))
	}
	for _, t := range m.Transitions {
		b.Transition(t.ID)
	}
	for _, a := range m.Arcs {
		if a.Type != "" {
			continue // read/inhibitor arcs gate but do not move tokens
		}
		w := a.Weight
		if w == 0 {
			w = 1
		}
		_, fromIsPlace := index[a.From]
		_, toIsPlace := index[a.To]
		if !fromIsPlace && !toIsPlace {
			continue
		}
		b.Arc(a.From, a.To, float64(w))
	}
	return b.Done(), places, nil
}

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

	"github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/go-pflow/petri"
	"github.com/pflow-xyz/go-pflow/solver"
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
}

// Depletion records when a place first runs out.
type Depletion struct {
	Place string  `json:"place"`
	At    float64 `json:"at"`
}

// Forecast runs the continuous mass-action ODE forward from marking.
//
// Deterministic: the same marking and rates always give the same answer, which
// is what makes it usable as a cached projection.
func Forecast(m *metamodel.Model, marking map[string]int, opts Options) (*Result, error) {
	opts = opts.withDefaults(m)

	net, places, err := toNet(m, marking)
	if err != nil {
		return nil, err
	}

	state := make(map[string]float64, len(places))
	for _, p := range places {
		state[p] = float64(marking[p])
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
	res.Depleted = depletions(res)
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

	trs, places, err := compile(m, marking)
	if err != nil {
		return nil, err
	}
	times := sampleTimes(opts)

	sums := make([][]float64, len(places))
	sumSquares := make([][]float64, len(places))
	for i := range places {
		sums[i] = make([]float64, len(times))
		sumSquares[i] = make([]float64, len(times))
	}

	seed := opts.Seed
	if seed == 0 {
		seed = 1
	}
	for r := 0; r < opts.Realizations; r++ {
		start := make([]int, len(places))
		for i, p := range places {
			start[i] = marking[p]
		}
		traj := ssa(trs, start, times, rand.New(rand.NewSource(seed+int64(r)))) //nolint:gosec // not cryptographic
		for p := range places {
			for i, v := range traj[p] {
				sums[p][i] += v
				sumSquares[p][i] += v * v
			}
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
	res.Depleted = depletions(res)
	return res, nil
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

// depletions reports the first sample time at which each place hits zero,
// earliest first. A place that starts empty is not "depleted".
func depletions(res *Result) []Depletion {
	var out []Depletion
	for _, s := range res.Series {
		if len(s.Values) == 0 || s.Values[0] <= 0 {
			continue
		}
		for i, v := range s.Values {
			if v <= 0 {
				out = append(out, Depletion{Place: s.Place, At: res.Times[i]})
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At < out[j].At })
	return out
}

// --- discrete engine -----------------------------------------------------

type arc struct {
	place  int
	weight int
}

type transition struct {
	id      string
	rate    float64
	inputs  []arc
	outputs []arc
}

// compile turns the model into index-addressed transitions, which is what makes
// the inner SSA loop cheap.
func compile(m *metamodel.Model, marking map[string]int) ([]transition, []string, error) {
	places, index, err := tokenPlaces(m)
	if err != nil {
		return nil, nil, err
	}
	rates := Rates(m)

	byID := make(map[string]*transition, len(m.Transitions))
	out := make([]transition, 0, len(m.Transitions))
	for _, t := range m.Transitions {
		out = append(out, transition{id: t.ID, rate: rates[t.ID]})
	}
	for i := range out {
		byID[out[i].id] = &out[i]
	}

	for _, a := range m.Arcs {
		w := a.Weight
		if w == 0 {
			w = 1
		}
		// Read and inhibitor arcs gate a firing but move nothing. Treating them
		// as inputs here would make the simulation consume tokens the model only
		// tests — the same defect read arcs exposed in the ZK compiler.
		if a.Type != "" {
			continue
		}
		if pi, ok := index[a.From]; ok {
			if tr := byID[a.To]; tr != nil {
				tr.inputs = append(tr.inputs, arc{place: pi, weight: w})
			}
			continue
		}
		if pi, ok := index[a.To]; ok {
			if tr := byID[a.From]; tr != nil {
				tr.outputs = append(tr.outputs, arc{place: pi, weight: w})
			}
		}
	}
	return out, places, nil
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
func ssa(trs []transition, marking []int, times []float64, rng *rand.Rand) [][]float64 {
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
func toNet(m *metamodel.Model, marking map[string]int) (*petri.PetriNet, []string, error) {
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

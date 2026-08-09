package sim

import (
	"fmt"
	"sort"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

// A scenario is a question, not a state.
//
// Forecast and Simulate answer "what happens next" from a marking something is
// already holding. That is the right shape for a dashboard and the wrong shape
// for a decision: an owner asking "should I put a third barista on?" is asking
// about a shop that does not exist yet. Nothing in the app has that marking, so
// there is no aggregate to point at.
//
// A Scenario supplies the marking instead — sparsely, so the question is
// "everything as it is, but three baristas" rather than a full state the caller
// has to restate and can silently get wrong. It never touches a store.

// Segment is one interval of a piecewise-constant rate schedule: this rate
// applies until Until, in the same time unit as the horizon.
type Segment struct {
	Until float64 `json:"until"`
	Value float64 `json:"value"`
}

// Scenario is a hypothetical run.
type Scenario struct {
	// Name distinguishes this run in a comparison. Optional for a single run.
	Name string `json:"name,omitempty"`

	// Marking overrides the model's declared initial marking, place by place.
	// Absent places keep what the model declares.
	Marking map[string]int `json:"marking,omitempty"`

	// Rates overrides transition rates for the whole horizon.
	Rates map[string]float64 `json:"rates,omitempty"`

	// Schedule makes a rate vary over time — the difference between "we are
	// busy" and "we are busy between eight and ten". A constant rate cannot
	// express a morning rush, and averaging one away is exactly the smoothing
	// that hides whether the queue recovers.
	//
	// Segments are applied in Until order; the last one extends to the horizon.
	// A transition in both Rates and Schedule takes the schedule.
	Schedule map[string][]Segment `json:"schedule,omitempty"`

	Horizon      float64 `json:"hours,omitempty"`
	Samples      int     `json:"samples,omitempty"`
	Realizations int     `json:"realizations,omitempty"`
	Seed         int64   `json:"seed,omitempty"`

	// Engine is "ssa" (default) or "ode". SSA is the right default here: a
	// scenario about staffing or queueing is exactly the regime where token
	// counts are small enough that noise decides the outcome.
	Engine string `json:"engine,omitempty"`
}

// Validate checks a scenario against the model before running it, so a typo in
// a place or transition name is an error rather than a silently ignored knob.
// The failure it prevents is the worst kind: a scenario that appears to run,
// answers the unmodified question, and reports no difference.
func (s *Scenario) Validate(m *metamodel.Model) error {
	known := m.InitialMarking()
	for _, p := range sortedKeys(s.Marking) {
		if _, ok := known[p]; !ok {
			return fmt.Errorf("no token place %q in model %q", p, m.Name)
		}
	}
	rates := Rates(m)
	for _, id := range sortedKeys(s.Rates) {
		if _, ok := rates[id]; !ok {
			return fmt.Errorf("no transition %q in model %q", id, m.Name)
		}
	}
	for _, id := range sortedKeys(s.Schedule) {
		if _, ok := rates[id]; !ok {
			return fmt.Errorf("no transition %q in model %q", id, m.Name)
		}
		segs := s.Schedule[id]
		if len(segs) == 0 {
			return fmt.Errorf("schedule for %q is empty", id)
		}
		for i, seg := range segs {
			if seg.Value < 0 {
				return fmt.Errorf("schedule for %q: segment %d has a negative rate", id, i)
			}
			if i > 0 && seg.Until <= segs[i-1].Until {
				return fmt.Errorf("schedule for %q: segment %d ends at %g, not after the previous segment's %g",
					id, i, seg.Until, segs[i-1].Until)
			}
		}
	}
	switch s.Engine {
	case "", "ssa", "ode":
	default:
		return fmt.Errorf("unknown engine %q (use ssa or ode)", s.Engine)
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// options converts a scenario's knobs into engine Options.
func (s *Scenario) options() Options {
	return Options{
		Horizon:      s.Horizon,
		Samples:      s.Samples,
		Realizations: s.Realizations,
		Seed:         s.Seed,
		Rates:        s.Rates,
	}
}

// Run answers the scenario.
//
// Pure: it reads the model and returns a trajectory. Nothing is fired, nothing
// is appended, no store is touched.
func Run(m *metamodel.Model, s Scenario) (*Result, error) {
	if err := s.Validate(m); err != nil {
		return nil, err
	}
	if s.Engine == "ode" {
		return Forecast(m, s.Marking, s.options())
	}

	var (
		res *Result
		err error
	)
	if len(s.Schedule) == 0 {
		res, err = Simulate(m, s.Marking, s.options())
	} else {
		res, err = simulateScheduled(m, s)
	}
	if err != nil {
		return nil, err
	}
	return res, nil
}

// serviceTimeAssumption is what the SSA cannot avoid assuming.
//
// Gillespie draws every duration from an exponential distribution, which is the
// same as assuming a job that has already taken five minutes is no more likely
// to finish soon than one that just started. Real work is not like that: pulling
// a shot takes about as long every time. Exponential service is the most
// variable case consistent with the same average, so the queue this engine
// reports is the pessimistic one — expect roughly half the waiting and walking
// out in a shop whose steps take a predictable amount of time.
//
// It reports as an Assumption, not a Caveat, and the distinction is the whole
// reason there are two fields. Caveats are things the engine could not enforce
// about *this model* — an empty list is the claim that everything the net says
// was applied — and this is true of every model the SSA runs, so filing it
// there both mislabelled it and made the claim unfalsifiable: appended to every
// scenario result, the list could never be empty again. Editing the net cannot
// remove this one; only a different engine could.
//
// Added by the exported SSA entry points — Simulate and simulateScheduled —
// rather than by Run, so that it reaches every caller of the engine and not only
// the ones who came through a Scenario. It was in Run first, which meant a
// generated app or an MCP tool calling Simulate directly got an answer with the
// assumption silently dropped, while the console next to it showed it. The
// unexported simulate is deliberately not the place: it is also the per-segment
// engine behind a schedule, and appending there would repeat it once a segment.
const serviceTimeAssumption = "this engine assumes every step takes a random, exponentially distributed amount of " +
	"time, which is the most erratic a shop can be for a given average. Work with a predictable duration — a shot " +
	"pulls in about the same time every time — queues roughly half as much, so treat the waiting and the walkouts " +
	"here as the bad case, not the typical one."

// simulateScheduled runs the horizon in pieces, one per schedule boundary,
// carrying the marking across.
//
// Splitting the run is the honest way to do this with a Gillespie engine: SSA
// draws a waiting time from the current total propensity, so a rate that
// changes mid-draw would mean sampling from a distribution that no longer
// applies. Restarting at each boundary keeps every draw consistent with the
// rates in force when it was made.
func simulateScheduled(m *metamodel.Model, s Scenario) (*Result, error) {
	opts := s.options().withDefaults(m)
	places, _, err := tokenPlaces(m)
	if err != nil {
		return nil, err
	}

	bounds := scheduleBoundaries(s.Schedule, opts.Horizon)
	marking := startFrom(m, s.Marking)

	combined := &Result{Method: "ssa", Final: map[string]float64{}}
	series := map[string][]float64{}
	throughput := map[string]float64{}
	// One accumulator for the whole horizon, carrying both the time-weighted
	// marking summary and the blocked-time ledger. Averaging the segments' own
	// means would weight a ten-minute rush the same as a seven-hour lull, which
	// is the smoothing a schedule exists to avoid, and a segment's Contended
	// fractions are shares of that segment rather than of the run.
	stats := newRunStats(len(places))
	var caveats []string

	from := 0.0
	for _, to := range bounds {
		span := to - from
		if span <= 0 {
			continue
		}
		// Samples proportional to the segment's share of the horizon, so a
		// short rush is not reported at the same resolution as a long lull.
		samples := int(float64(opts.Samples) * span / opts.Horizon)
		if samples < 2 {
			samples = 2
		}

		segment := Options{
			Horizon:      span,
			Samples:      samples,
			Realizations: opts.Realizations,
			Seed:         opts.Seed,
			Rates:        ratesAt(m, s, from),
		}
		res, segStats, err := simulate(m, marking, segment)
		if err != nil {
			return nil, err
		}
		stats.merge(segStats)

		for _, t := range res.Times {
			combined.Times = append(combined.Times, from+t)
		}
		for _, sr := range res.Series {
			series[sr.Place] = append(series[sr.Place], sr.Values...)
		}
		if res.Metrics != nil {
			for id, n := range res.Metrics.Throughput {
				throughput[id] += n
			}
		}
		if len(caveats) == 0 {
			caveats = res.Caveats
		}

		// The next segment starts where this one ended. Rounded because a
		// marking is a token count: half a customer is not a state.
		next := map[string]int{}
		for p, v := range res.Final {
			next[p] = int(v + 0.5)
		}
		marking = startFrom(m, next)
		from = to
	}

	for _, p := range sortedKeys(series) {
		combined.Series = append(combined.Series, Series{Place: p, Values: series[p]})
		combined.Final[p] = series[p][len(series[p])-1]
	}
	combined.Depleted = depletions(m, combined)
	// Contention is the diagnostic a schedule is usually run to get: a rush is
	// the interval where capacity binds, so a scheduled run reporting nothing
	// contended is the shape of silence Contention exists to eliminate — the
	// café console's Rush box read "waiting on nothing" for a shop at 87%
	// utilization, because this was never populated at all.
	combined.Contended = contentions(m, places, stats.blocked, opts.Horizon*float64(opts.Realizations))
	combined.Caveats = caveats
	// Once for the whole run, not once per segment: splitting a horizon into
	// rate segments does not make the engine assume anything extra.
	combined.Assumptions = append(combined.Assumptions, serviceTimeAssumption)

	mt := &Metrics{Throughput: throughput, Mean: map[string]float64{}, P95: map[string]float64{}}
	for i, p := range places {
		mt.Mean[p] = stats.times.mean(i)
		mt.P95[p] = stats.times.percentile(i, 0.95)
	}
	mt.Utilization = utilization(places, mt.Mean)
	combined.Metrics = mt

	return combined, nil
}

// scheduleBoundaries collects every segment end inside the horizon, plus the
// horizon itself.
func scheduleBoundaries(schedule map[string][]Segment, horizon float64) []float64 {
	seen := map[float64]bool{}
	var out []float64
	for _, segs := range schedule {
		for _, seg := range segs {
			if seg.Until > 0 && seg.Until < horizon && !seen[seg.Until] {
				seen[seg.Until] = true
				out = append(out, seg.Until)
			}
		}
	}
	sort.Float64s(out)
	return append(out, horizon)
}

// ratesAt is the rate table in force at time t.
func ratesAt(m *metamodel.Model, s Scenario, t float64) map[string]float64 {
	rates := Rates(m)
	for id, r := range s.Rates {
		rates[id] = r
	}
	for id, segs := range s.Schedule {
		// The last segment extends past its own Until, so a schedule that stops
		// short of the horizon holds its final rate rather than falling back to
		// the model's — which would look like the rush ending twice.
		value := segs[len(segs)-1].Value
		for _, seg := range segs {
			if t < seg.Until {
				value = seg.Value
				break
			}
		}
		rates[id] = value
	}
	return rates
}

// Comparison is several scenarios answered together.
type Comparison struct {
	Scenarios []NamedResult `json:"scenarios"`
}

// NamedResult pairs a scenario's name with its answer.
type NamedResult struct {
	Name   string  `json:"name"`
	Result *Result `json:"result"`
}

// Compare runs several scenarios and returns them side by side.
//
// Every scenario is forced onto one seed, because otherwise the comparison is
// not answering the question asked. Two SSA runs of the *same* shop differ; a
// caller looking at "2 baristas" beside "3 baristas" on different seeds cannot
// tell how much of the gap is staffing and how much is dice. Sharing the seed
// is only enforceable if the comparison happens in one place, which is why this
// is a server-side operation rather than several calls.
func Compare(m *metamodel.Model, scenarios []Scenario) (*Comparison, error) {
	if len(scenarios) == 0 {
		return nil, fmt.Errorf("no scenarios to compare")
	}

	seed := scenarios[0].Seed
	if seed == 0 {
		seed = 1
	}

	out := &Comparison{}
	for i, s := range scenarios {
		s.Seed = seed
		name := s.Name
		if name == "" {
			name = fmt.Sprintf("scenario %d", i+1)
		}
		res, err := Run(m, s)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		out.Scenarios = append(out.Scenarios, NamedResult{Name: name, Result: res})
	}
	return out, nil
}

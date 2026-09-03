package sim

import (
	"fmt"
	"sort"

	"github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/go-pflow/stochastic"
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
		Schedule:     s.Schedule,
		// "" -> SSA, "ode" -> ODE; Validate already rejects anything else.
		Method: stochastic.Method(s.Engine),
		Guard:  guardFunc,
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
	return stochastic.Solve(m, s.Marking, s.options())
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

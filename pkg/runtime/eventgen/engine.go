package eventgen

import (
	"fmt"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

// The playout engine mirrors pkg/runtime/sim's SSA law exactly — enablement
// by the shared firing rule (consuming weight, read threshold, inhibitor
// threshold, post-firing capacity netting), propensity as rate times
// C(marking, weight) over KINETIC consuming inputs only. Arcs are classified
// through metamodel.Inputs/Outputs/Tests, never re-derived from From/To.
// The cross-check test in playout_test.go holds this copy's throughput to
// sim.Simulate's on the same model, because a second implementation of one
// definition is a way to be confidently wrong unless something diffs them.

type arcRef struct {
	place   int
	weight  int
	kinetic bool
}

type trans struct {
	id       string
	rate     float64
	inputs   []arcRef // consuming
	reads    []arcRef
	inhibits []arcRef
	outputs  []arcRef
	isSource bool // no consuming inputs: fires spontaneously, starts a case
}

type engine struct {
	places   []string
	capacity []int
	sink     []bool // no consuming arc leaves the place: tokens there are done
	trs      []trans
	marking  []int
	// queues holds the case id of each token, FIFO per place, aligned with
	// marking: len(queues[p]) == marking[p] always. "" is a case-less token
	// (initial marking: resource pools, gates).
	queues [][]string
}

func compile(m *metamodel.Model) (*engine, error) {
	e := &engine{}
	index := map[string]int{}
	for i := range m.Places {
		p := &m.Places[i]
		if p.Kind != "" && p.Kind != "token" {
			return nil, fmt.Errorf("eventgen: place %q has kind %q; only token places play out", p.ID, p.Kind)
		}
		index[p.ID] = len(e.places)
		e.places = append(e.places, p.ID)
		e.capacity = append(e.capacity, p.Capacity)
		e.marking = append(e.marking, p.Initial)
		q := make([]string, p.Initial)
		e.queues = append(e.queues, q)
	}

	// Rates mirror sim.Rates: unset defaults to 1, the model-level solver
	// map overrides.
	rates := map[string]float64{}
	for i := range m.Transitions {
		r := m.Transitions[i].Rate
		if r == 0 {
			r = 1
		}
		rates[m.Transitions[i].ID] = r
	}
	if m.Simulation != nil && m.Simulation.Solver != nil {
		for id, r := range m.Simulation.Solver.Rates {
			rates[id] = r
		}
	}

	for i := range m.Transitions {
		t := &m.Transitions[i]
		tr := trans{id: t.ID, rate: rates[t.ID]}
		for _, in := range m.Inputs(t.ID) {
			tr.inputs = append(tr.inputs, arcRef{place: index[in.Place], weight: in.Weight, kinetic: in.Kinetic})
		}
		for _, o := range m.Outputs(t.ID) {
			tr.outputs = append(tr.outputs, arcRef{place: index[o.Place], weight: o.Weight})
		}
		for _, test := range m.Tests(t.ID) {
			a := arcRef{place: index[test.Place], weight: test.Weight}
			if test.Type == metamodel.InhibitorArc {
				tr.inhibits = append(tr.inhibits, a)
			} else {
				tr.reads = append(tr.reads, a)
			}
		}
		tr.isSource = len(tr.inputs) == 0
		e.trs = append(e.trs, tr)
	}
	e.sink = make([]bool, len(e.places))
	consumed := make([]bool, len(e.places))
	for i := range e.trs {
		for _, in := range e.trs[i].inputs {
			consumed[in.place] = true
		}
	}
	for p := range e.sink {
		e.sink[p] = !consumed[p]
	}
	return e, nil
}

// hasSource reports whether any transition can start a case at all.
func (e *engine) hasSource() bool {
	for i := range e.trs {
		if e.trs[i].isSource {
			return true
		}
	}
	return false
}

// propensities returns per-transition rates under the current marking.
// sourcesOff zeroes source transitions — the drain phase after the last
// requested case has arrived.
func (e *engine) propensities(sourcesOff bool) ([]float64, float64) {
	props := make([]float64, len(e.trs))
	total := 0.0
	for i := range e.trs {
		tr := &e.trs[i]
		if sourcesOff && tr.isSource {
			continue
		}
		a := tr.rate
		for _, in := range tr.inputs {
			m := e.marking[in.place]
			if m < in.weight {
				a = 0
				break
			}
			if in.kinetic {
				a *= combinations(m, in.weight)
			}
		}
		if a > 0 {
			for _, rd := range tr.reads {
				if e.marking[rd.place] < rd.weight {
					a = 0
					break
				}
			}
		}
		if a > 0 {
			for _, inh := range tr.inhibits {
				if e.marking[inh.place] >= inh.weight {
					a = 0
					break
				}
			}
		}
		if a > 0 && !e.capacityAdmits(tr) {
			a = 0
		}
		props[i] = a
		total += a
	}
	return props, total
}

// capacityAdmits applies the post-firing bound, netting production against
// what the same firing consumes from the same place.
func (e *engine) capacityAdmits(tr *trans) bool {
	for _, out := range tr.outputs {
		cap := e.capacity[out.place]
		if cap == 0 {
			continue
		}
		next := e.marking[out.place] + out.weight
		for _, in := range tr.inputs {
			if in.place == out.place {
				next -= in.weight
			}
		}
		if next > cap {
			return false
		}
	}
	return true
}

// fire applies transition i: consumes FIFO tokens from its inputs, produces
// into its outputs, and returns the case id this firing belongs to — a new
// case for a source transition, else the first case-carrying token consumed,
// else "" (pure resource churn, which generates no event).
func (e *engine) fire(i int, newCase func() string) string {
	tr := &e.trs[i]
	caseID := ""
	if tr.isSource {
		caseID = newCase()
	}
	for _, in := range tr.inputs {
		q := e.queues[in.place]
		for w := 0; w < in.weight; w++ {
			tok := q[0]
			q = q[1:]
			if caseID == "" && tok != "" {
				caseID = tok
			}
		}
		e.queues[in.place] = q
		e.marking[in.place] -= in.weight
	}
	for _, out := range tr.outputs {
		for w := 0; w < out.weight; w++ {
			e.queues[out.place] = append(e.queues[out.place], caseID)
		}
		e.marking[out.place] += out.weight
	}
	return caseID
}

// inFlight counts case-carrying tokens that can still move — tokens in sink
// places (discharged, walked_out) are done, not in flight, or a completed
// day would never let the run stop.
func (e *engine) inFlight() int {
	n := 0
	for p, q := range e.queues {
		if e.sink[p] {
			continue
		}
		for _, tok := range q {
			if tok != "" {
				n++
			}
		}
	}
	return n
}

// combinations is C(m, w) — the number of distinct token combinations a
// weight-w arc can draw, the stochastic mass-action coefficient. Mirrors
// pkg/runtime/sim.
func combinations(m, w int) float64 {
	c := 1.0
	for i := 0; i < w; i++ {
		c *= float64(m-i) / float64(i+1)
	}
	return c
}

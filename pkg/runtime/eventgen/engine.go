package eventgen

import (
	"fmt"

	"github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/go-pflow/stochastic"
)

// The playout runs on go-pflow's own compiled net (stochastic.Compile):
// enablement, gating, capacity netting and the C(marking, weight) propensity
// over kinetic inputs are the sampler's, not a copy held to it by a test.
// What this file adds is the case bookkeeping a log needs — which case each
// token belongs to — and the drain phase after the last requested arrival.

type arcRef struct {
	place  int
	weight int
}

type trans struct {
	id       string
	inputs   []arcRef // consuming
	outputs  []arcRef
	isSource bool // no consuming inputs: fires spontaneously, starts a case
}

// engine is the playout state: the engine's compiled net for the rate law,
// plus the case bookkeeping the log needs and the sampler does not — which
// case each token belongs to, FIFO per place.
type engine struct {
	compiled *stochastic.Compiled
	places   []string
	sink     []bool // no consuming arc leaves the place: tokens there are done
	trs      []trans
	marking  []int
	props    []float64
	// queues holds the case id of each token, FIFO per place, aligned with
	// marking: len(queues[p]) == marking[p] always. "" is a case-less token
	// (initial marking: resource pools, gates).
	queues [][]string
}

func compile(m *metamodel.Model) (*engine, error) {
	for i := range m.Places {
		if p := &m.Places[i]; p.Kind != "" && p.Kind != "token" {
			return nil, fmt.Errorf("eventgen: place %q has kind %q; only token places play out", p.ID, p.Kind)
		}
	}
	compiled, err := stochastic.Compile(m, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("eventgen: %w", err)
	}
	e := &engine{compiled: compiled, places: compiled.Places()}
	index := map[string]int{}
	for i, p := range e.places {
		index[p] = i
	}
	e.marking = compiled.InitialMarking(m, nil)
	e.queues = make([][]string, len(e.places))
	for i, n := range e.marking {
		e.queues[i] = make([]string, n)
	}
	ids := compiled.Transitions()
	e.props = make([]float64, len(ids))
	for _, id := range ids {
		tr := trans{id: id}
		for _, in := range m.Inputs(id) {
			tr.inputs = append(tr.inputs, arcRef{place: index[in.Place], weight: in.Weight})
		}
		for _, o := range m.Outputs(id) {
			tr.outputs = append(tr.outputs, arcRef{place: index[o.Place], weight: o.Weight})
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

// propensities returns per-transition rates under the current marking — the
// engine's own rate law, through the compiled net. sourcesOff zeroes source
// transitions: the drain phase after the last requested case has arrived.
func (e *engine) propensities(sourcesOff bool) ([]float64, float64) {
	total := e.compiled.Propensities(e.marking, e.props)
	if sourcesOff {
		for i := range e.trs {
			if e.trs[i].isSource && e.props[i] > 0 {
				total -= e.props[i]
				e.props[i] = 0
			}
		}
	}
	return e.props, total
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

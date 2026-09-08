// Package eventgen plays a model forward and records what happened as an
// event log — the synthetic dataset half of the sheet-publishing story. A
// what-if model's trajectory says what the averages were; the event log says
// what each case went through, which is the form process-mining tools (and
// spreadsheet-native analysts) actually consume.
//
// The playout consumes go-pflow's eventlog types but deliberately lives
// here, not upstream: case attribution through an indistinguishable-token
// net is a heuristic (FIFO below), and a heuristic should prove itself in
// one service before it becomes library API.
//
// Determinism is a contract, same as the parity fixtures: the loop draws
// from pkg/prng, timestamps derive from a fixed epoch plus simulated time,
// and map iteration never decides anything — same model, same options, same
// bytes.
package eventgen

import (
	"fmt"
	"math"
	"time"

	"github.com/pflow-xyz/go-pflow/eventlog"
	"github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/petri-pilot/pkg/prng"
)

// Options sizes a playout.
type Options struct {
	// Cases is how many cases (arrivals) to generate. Required.
	Cases int
	// Seed drives the portable PRNG; realization r of a batched export
	// should pass Seed+r, matching the SSA engine's stream-derivation rule.
	Seed int64
	// Epoch is the wall-clock time of simulation t=0. Zero means the fixed
	// default epoch — a real value only matters if the log should interleave
	// with real data.
	Epoch time.Time
	// TimeUnit is the wall-clock duration of one model time unit (the
	// what-if models declare rates per hour). Zero means time.Hour.
	TimeUnit time.Duration
	// MaxEvents bounds the run against nets that never drain. Zero means
	// 200 events per requested case, which any sane case stays far under.
	MaxEvents int
}

// DefaultEpoch keeps synthetic timestamps recognizably synthetic (a Monday
// morning) and byte-stable across runs.
var DefaultEpoch = time.Date(2026, 1, 5, 8, 0, 0, 0, time.UTC)

// Playout runs the model as a seeded SSA and returns the firing history as
// an event log.
//
// Case semantics: every firing of a source transition (no input places)
// starts a new case; tokens carry their case id through the net, consumed
// FIFO per place; a firing's case is the first case-carrying token it
// consumes. Tokens from the initial marking (resource pools, gates) carry no
// case, so pure resource churn generates no events — the log is about the
// entities flowing through, not the furniture they flow past.
//
// The run continues until the requested number of cases has both started
// and drained (no case-carrying token left), or MaxEvents intervenes.
func Playout(m *metamodel.Model, opts Options) (*eventlog.EventLog, error) {
	if opts.Cases <= 0 {
		return nil, fmt.Errorf("eventgen: Cases must be positive")
	}
	epoch := opts.Epoch
	if epoch.IsZero() {
		epoch = DefaultEpoch
	}
	unit := opts.TimeUnit
	if unit == 0 {
		unit = time.Hour
	}
	maxEvents := opts.MaxEvents
	if maxEvents == 0 {
		maxEvents = 200 * opts.Cases
	}

	net, err := compile(m)
	if err != nil {
		return nil, err
	}
	// Refuse rather than return an empty log a caller cannot distinguish from
	// success. Cases are arrivals — firings of a source transition — so a
	// closed-loop model (predator-prey, dining philosophers) structurally
	// cannot produce one: it has trajectories, not cases.
	if !net.hasSource() {
		return nil, fmt.Errorf(
			"eventgen: model %q has no source transition (a transition with no input places), so nothing ever arrives and there are no cases to log — the event log is case-per-arrival. A closed-loop model has trajectories, not cases: ask /scenario for those",
			m.Name)
	}

	rng := prng.New(opts.Seed)
	log := eventlog.NewEventLog()
	log.Attributes["generator"] = "sim.pflow.xyz eventgen"
	log.Attributes["model"] = m.Name
	log.Attributes["seed"] = fmt.Sprintf("%d", opts.Seed)

	t := 0.0
	started := 0
	events := 0

	for events < maxEvents {
		if started >= opts.Cases && net.inFlight() == 0 {
			break
		}
		props, total := net.propensities(started >= opts.Cases)
		if total <= 0 {
			// Nothing can fire: either every case has drained (success) or
			// the net deadlocked with cases still inside — the log reports
			// what happened either way, so this is not an error.
			break
		}
		u := rng.Float64()
		if u <= 0 {
			u = 1e-300
		}
		t += -math.Log(u) / total

		r := rng.Float64() * total
		chosen := len(props) - 1
		acc := 0.0
		for i, p := range props {
			if acc += p; r <= acc {
				chosen = i
				break
			}
		}

		caseID := net.fire(chosen, func() string {
			started++
			return fmt.Sprintf("case-%04d", started)
		})
		if caseID != "" {
			log.AddEvent(eventlog.Event{
				CaseID:    caseID,
				Activity:  net.trs[chosen].id,
				Timestamp: epoch.Add(time.Duration(t * float64(unit))),
				Lifecycle: "complete",
			})
			events++
		}
	}

	// The net has sources yet none ever fired: dormant (rate 0 via the
	// solver map, like vet-clinic's emergency_arrives) or gated shut at the
	// initial marking. Same rule as above — an empty log must say why.
	if started == 0 {
		return nil, fmt.Errorf(
			"eventgen: model %q generated no cases: its source transitions never fired — dormant (rate 0) or gated shut at the initial marking. Raise a source rate for a dataset to exist",
			m.Name)
	}

	log.SortTraces()
	return log, nil
}

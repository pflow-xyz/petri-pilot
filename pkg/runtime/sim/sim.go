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
//
// Both engines live in go-pflow's stochastic package; this package is the thin
// wrapper that keeps every identifier generated code and the MCP tools already
// use, and injects the one thing go-pflow cannot carry: the pkg/dsl guard
// evaluator. Scenario, Run, Compare and the HTTP handlers stay here.
package sim

import (
	"github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/go-pflow/stochastic"

	"github.com/pflow-xyz/petri-pilot/pkg/dsl"
)

// DefaultRate is used for a transition whose model declares none. Mass-action
// with unit rate means "as fast as tokens allow", which is the least surprising
// reading of an unannotated net.
const DefaultRate = stochastic.DefaultRate

type (
	Options    = stochastic.Options
	Series     = stochastic.Series
	Result     = stochastic.Result
	Metrics    = stochastic.Metrics
	Depletion  = stochastic.Depletion
	Contention = stochastic.Contention
	SupplyKind = stochastic.SupplyKind
	// Segment is one interval of a piecewise-constant rate schedule: this rate
	// applies until Until, in the same time unit as the horizon.
	Segment = metamodel.RateSegment
)

const (
	SupplyConserved = stochastic.SupplyConserved
	SupplyBounded   = stochastic.SupplyBounded
	SupplyQueue     = stochastic.SupplyQueue
	SupplyState     = stochastic.SupplyState
)

// Rates is the rate table the model declares, DefaultRate where it declares none.
func Rates(m *metamodel.Model) map[string]float64 { return stochastic.Rates(m) }

// ClassifySupply names what each token place is, structurally, so a contention
// report can rank a capacity constraint above an idle queue.
func ClassifySupply(m *metamodel.Model) map[string]SupplyKind { return stochastic.ClassifySupply(m) }

// Forecast is the continuous engine: mass-action ODE over a real-valued marking.
func Forecast(m *metamodel.Model, marking map[string]int, opts Options) (*Result, error) {
	return stochastic.Forecast(m, marking, opts)
}

// Simulate is the discrete engine: Gillespie's SSA over integer counts, with
// marking-decidable guards enforced through pkg/dsl.
func Simulate(m *metamodel.Model, marking map[string]int, opts Options) (*Result, error) {
	opts.Guard = guardFunc
	return stochastic.Simulate(m, marking, opts)
}

// guardFunc is the pkg/dsl evaluator, injected so go-pflow never imports the
// guard language. Same call, same empty bindings as before the move.
func guardFunc(expr string, mk metamodel.Marking) (bool, error) {
	return dsl.Evaluate(expr, map[string]any{}, dsl.MakeAggregates(dsl.Marking(mk)))
}

package mcp

import (
	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"

	"github.com/pflow-xyz/petri-pilot/pkg/runtime/sim"
)

// modelRates is the rate map every simulation tool starts from: the rate each
// transition declares in the model, overridden by simulation.solver.rates,
// and 1.0 only where the model says nothing. Callers overlay the user's
// rates argument on top.
//
// Every tool used to start from a flat 1.0 and ignore the model, so a model
// that declared arrive=6 simulated at arrive=1 unless the caller repeated the
// declared rates by hand; petri_scenario read the model all along, and the
// two disagreed on the same file. sim.Rates is the one definition.
func modelRates(model *goflowmetamodel.Model) map[string]float64 {
	return sim.Rates(model)
}

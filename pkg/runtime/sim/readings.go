package sim

import (
	"fmt"

	"github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/go-pflow/metamodel/metapetri"
	"github.com/pflow-xyz/go-pflow/reachability"
)

// Ported from sim.pflow.xyz's readings.go: the algebraic invariant report
// (Farkas P-invariants, T-invariants with method tags, siphons and traps with
// deadlock witnesses) with the metapetri conversion caveats attached. The
// cross-check and VerifyModel halves of that file depend on sim's own engine
// and stay there; petri_verify carries the same caveats through its own path.

// Law is one P-invariant: a weighted sum of places the net conserves, for
// every run, from this initial marking's value. These are the trust-panel
// lines — arithmetic a user can check against any marking they are shown.
type Law struct {
	Expression   string         `json:"expression"`
	Coefficients map[string]int `json:"coefficients"`
	Value        int            `json:"value"`
}

// InvariantReport is the ONE authoritative place/transition/siphon/trap
// invariant report for a model — the single computation path sim_diagnose
// and sim_invariants both read from (Phase 6 gate: no reader gets a weaker
// copy of a result a stronger one already computed). It used to carry a
// second, plain T-invariant shape (TLaw: bare firing counts, no method tag,
// no siphons at all) alongside the richer TInvariant/SiphonTrapReport that
// DiagnosisLD alone could reach — a tool literally named "invariants" giving
// a WORSE answer than the general diagnosis tool. TInvariants and Siphons
// below are exactly diagnose.go's own fields, sourced from the same
// invariants.go/siphons.go functions, so there is no second copy to drift.
type InvariantReport struct {
	// Laws are the Farkas P-invariants: weighted place sums every run
	// preserves from this initial marking.
	Laws []Law `json:"laws"`
	// TInvariants are the minimal-support firing cycles the incidence matrix
	// admits (invariants.go): repeatable sequences that return the net to
	// wherever it started, true for any initial marking. Every entry carries
	// Method: "StructuralProof".
	TInvariants []TInvariant `json:"tInvariants,omitempty"`
	// Siphons is the algebraic deadlock-structure report (siphons.go): every
	// minimal siphon and trap found from the arc structure alone, and which
	// minimal siphons (if any) are unmarked at this model's initial marking.
	Siphons *SiphonTrapReport `json:"siphons,omitempty"`
	Caveats []string          `json:"caveats,omitempty"`
}

// Invariants derives the model's conservation laws (Farkas P-invariants),
// firing cycles (T-invariants, in the readable per-cycle shape TInvariants
// already produces) and the siphon/trap structure. Pure structure, no
// simulation; every claim holds for every trajectory from this initial
// marking.
func Invariants(m *metamodel.Model) (*InvariantReport, error) {
	res, err := metapetri.Convert(m, metapetri.Options{})
	if err != nil {
		return nil, fmt.Errorf("model does not convert for analysis: %w", err)
	}
	an := reachability.NewInvariantAnalyzer(res.Net)
	out := &InvariantReport{}
	for _, inv := range an.FindPInvariants(res.Marking) {
		out.Laws = append(out.Laws, Law{
			Expression:   inv.String(),
			Coefficients: inv.Coefficients,
			Value:        inv.Value,
		})
	}
	if ti, tErr := TInvariants(m); tErr == nil {
		out.TInvariants = ti
	}
	out.Siphons = SiphonsAndTraps(m)
	for _, n := range res.Diag.Notes {
		out.Caveats = append(out.Caveats, n.String())
	}
	return out, nil
}

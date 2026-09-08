package sim

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/go-pflow/metamodel/metapetri"
	"github.com/pflow-xyz/go-pflow/reachability"
)

// T-invariants: the algebraic half of "which firing sequences bring the net
// back to where it started", found by Farkas elimination over the transposed
// incidence matrix — the same machinery ClassifySupply already calls for
// P-invariants (see supply.go's doc comment) and readings.go's Invariants
// already calls for both. That call already exists
// (reachability.InvariantAnalyzer.FindTInvariants, wired in readings.go's
// Invariants as TLaw.Counts): what was missing was a form a reader can act on
// — which transitions, how many times each, in a sentence — and a place in
// the Diagnose/JSON-LD surface that says so with an explicit method tag, per
// the Phase 6 gate. This file adds that shape; it does not reimplement the
// algebra, and it reuses go-pflow's own metamodel->petri.PetriNet bridge
// (metapetri.Convert) rather than building a second incidence-matrix reader
// (sim.go's toNet is the OTHER existing bridge, used by ClassifySupply and
// LumpingReport for the same reason: one incidence matrix, read two ways).
//
// # What a T-invariant proves, and what it does not
//
// C*x = 0 over the incidence matrix says only that firing each transition x_t
// times nets to zero on every place — the total effect cancels. It holds for
// EVERY initial marking, since it never inspects one: that is what makes it a
// StructuralProof rather than a Measurement, the same distinction ClassifySupply
// draws for P-invariants. It does NOT say that some ORDER of those firings is
// actually executable from a given marking without going negative on some
// place along the way — two disjoint cycles sharing no place can sum to a
// T-invariant with no single fireable interleaving. Report the algebraic fact
// as what it is; TestTInvariantsFireBackToStart (invariants_test.go) confirms
// the reported counts really do return the catalog models to their start by
// actually firing them, which is a check on the CONNECTION between the algebra
// and the engine, not a re-proof of the algebra.
type TInvariant struct {
	// Transitions are the transitions with a nonzero firing count, sorted.
	Transitions []string `json:"transitions"`
	// Counts is how many times each transition in Transitions fires.
	Counts map[string]int `json:"counts"`
	// Detail names the cycle in a form a reader can act on, e.g. "firing
	// consume x2, produce returns every place to its starting count — true
	// for any initial marking, since it follows from the incidence matrix
	// alone".
	Detail string `json:"detail"`
	// Method is always StructuralProof: see the doc comment above for exactly
	// what that does and does not cover.
	Method string `json:"method"`
}

// TInvariants finds the minimal-support T-invariant basis and renders each
// member as something an operator can act on rather than a raw integer
// vector. See the package doc comment above for the exact guarantee.
func TInvariants(m *metamodel.Model) ([]TInvariant, error) {
	res, err := metapetri.Convert(m, metapetri.Options{})
	if err != nil {
		return nil, fmt.Errorf("model does not convert for analysis: %w", err)
	}
	basis := reachability.NewInvariantAnalyzer(res.Net).FindTInvariants()

	out := make([]TInvariant, 0, len(basis))
	for _, ti := range basis {
		transitions := append([]string(nil), ti.Transitions...)
		sort.Strings(transitions)
		counts := make(map[string]int, len(ti.Counts))
		for k, v := range ti.Counts {
			counts[k] = v
		}
		out = append(out, TInvariant{
			Transitions: transitions,
			Counts:      counts,
			Detail:      describeTInvariant(transitions, counts),
			Method:      StructuralProof,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i].Transitions) != len(out[j].Transitions) {
			return len(out[i].Transitions) < len(out[j].Transitions)
		}
		return strings.Join(out[i].Transitions, ",") < strings.Join(out[j].Transitions, ",")
	})
	return out, nil
}

// describeTInvariant renders a firing-count vector as a sentence: which
// transitions, how many times each, and what firing them buys.
func describeTInvariant(transitions []string, counts map[string]int) string {
	parts := make([]string, 0, len(transitions))
	for _, t := range transitions {
		c := counts[t]
		if c == 1 {
			parts = append(parts, t)
		} else {
			parts = append(parts, fmt.Sprintf("%s x%d", t, c))
		}
	}
	return fmt.Sprintf(
		"firing %s returns every place to its starting count, for any initial marking — "+
			"a structural fact from the incidence matrix alone, not a claim that this exact "+
			"interleaving is enabled from every marking along the way",
		strings.Join(parts, ", "))
}

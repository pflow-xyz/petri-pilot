// Backward differential equivalence, exactly (ROADMAP.md Phase 6: "the
// Paige-Tarjan refinement over degree-<=2 polynomial derivatives that ERODE
// implements", as opposed to LumpingReport's WL-propose-then-sample-verify
// heuristic, classify.go).
//
//	Cardelli, L., Tribastone, M., Tschaikowski, M. and Vandin, A. (2016).
//	"Symbolic Computation of Differential Equivalences." POPL 2016,
//	43rd ACM SIGPLAN Symposium on Principles of Programming Languages.
//
// # What question this answers
//
// sim's mass-action ODE (go-pflow's solver package, driven from Forecast in
// sim.go) is a system dx/dt = f(x) where each f_i is, for the models this
// file accepts, a polynomial of degree <= 2 in the place variables x — see
// "The degree bound, confirmed by reading the solver" below. A BACKWARD
// differential equivalence is a partition R of the variables such that
// summing f_i over each block of R and re-expressing the sum purely in terms
// of BLOCK-SUM variables gives a well-defined, smaller polynomial ODE system
// whose solution reproduces every member's own trajectory exactly, for EVERY
// initial condition that starts each block at a uniform value — not just the
// one initial marking a model happens to declare, and not approximately: the
// combinatorial characterization below is decided with exact rational
// arithmetic, so "equal" means the identical rational number, never two
// floats within a tolerance.
//
// This is a strictly stronger claim than LumpingReport's: that file proposes
// candidates from quantitative colour refinement (1-WL, which over-
// approximates isomorphism — CLAUDE.md, ROADMAP.md) and checks them against
// ONE run's SAMPLED trajectories, within that run's own standard deviation.
// A candidate that survives is evidence for that run's operating point, not a
// proof for every initial condition. BackwardDifferentialEquivalence answers
// the proof question instead, and is wired into Lumping.Exact
// (classify.go) so both are visible and neither replaces the other silently
// — the same discipline lumpability.go's LumpingPartition/Scope split
// already applies one level up, between the PLACE-level question both of
// these files answer and the STATE-level one ExactLumping answers.
//
// # The degree bound, confirmed by reading the solver
//
// go-pflow's buildVecODEFunction (solver/ode.go) computes each transition's
// flux as `flux := rate; for each input arc { flux *= u[place] }` — every
// DISTINCT input arc's place concentration enters the flux EXACTLY ONCE,
// regardless of that arc's Weight (weight only scales how much of the flux is
// added to or subtracted from du/dt, once flux itself is computed — see
// go-pflow's docs/engine-selection.md, vendored into this repo, on the same
// point). So a transition's monomial degree equals its REACTANT COUNT (the
// number of distinct consuming input arcs), not its stoichiometry: a
// transition with input weight 20 on a single place is still degree 1, and a
// transition consuming from two places is degree 2 regardless of their
// weights. This file implements exactly degree <= 2 (0, 1 or 2 distinct
// reactant places per transition) — the ordinary mass-action case, and the
// only one the cited combinatorial characterization needs — and REFUSES,
// naming the transition and its actual reactant count, the moment a
// transition exceeds it (extractReactions). It does not implement, and does
// not silently approximate, a higher-degree rate law.
//
// # Refusal set: reuses Forecast's own gating check
//
// Backward differential equivalence is stated for autonomous polynomial ODEs.
// Read arcs, inhibitor arcs, non-kinetic arcs, a guard, or a capacity
// something can actually reach all make the true dynamics something other
// than the polynomial this file would extract — exactly Forecast's own
// refusal condition (sim.go), which is why this calls m.Gating() directly
// rather than re-deriving a second, potentially-drifting list of the same
// four things (LumpingReport, classify.go, predates Gating and re-derives an
// overlapping-but-not-identical three of them by hand; this file does not
// repeat that). A declared rate SCHEDULE is refused too, for the same reason
// Forecast refuses it: a time-varying rate is not a fixed polynomial
// coefficient, so there is no autonomous dx/dt=f(x) to ask the question of at
// all.
//
// # The algorithm: partition refinement over exact-rational monomial coefficients
//
// Represent each place's derivative as an exact map from monomial (a
// multiset of at most 2 place indices — see monomial) to a *big.Rat
// coefficient, built once from the net's own stoichiometry and rate
// constants (extractReactions: no floating-point rounding enters here, since
// a model's declared weights are integers and SetFloat64 captures a rate's
// exact bit pattern as a rational, not an approximation of it — two rates
// compare equal here only when they are the identical float64 value, which
// is the correct exactness guarantee: a real difference between two declared
// rates, however small, is a real difference in the polynomial, not
// something a tolerance should paper over).
//
// Starting from the single all-places block (refineBackwardDE), repeatedly:
// for the CURRENT partition, collapse every place's polynomial by replacing
// each input place in every monomial with its current block id
// (collapsedPoly), and refine each block by grouping its members according
// to whether their OWN collapsed polynomial (as an exact map, compared
// termwise) matches — recording the fact that includes the PRIOR round's
// colour in the new one, exactly as classify.go's wlRefine does, so a split
// this round can never be undone by a later one. This is finite (at most n
// rounds, one place fewer live in the coarsest surviving block each time a
// split has to happen) and converges to the COARSEST partition stable under
// "same collapsed polynomial" — which is precisely the combinatorial
// characterization of backward differential equivalence the cited paper
// gives for polynomial ODEs.
//
// This is NOT lumpability.go's splitter-queue implementation (that file's
// refineOrdinaryLumping tests one candidate splitter block at a time and
// achieves the classical O(m log n) bound via the smaller-half rule). Here
// every place's FULL collapsed polynomial — every monomial, not one target
// block's rate — is recomputed each round, giving worse-case
// O(rounds x places x reactions) work rather than a linear-time refinement.
// It computes the same fixed point (the two techniques decide the identical
// relation, relational coarsest-partition refinement, over different data:
// a scalar rate per target block there, a full polynomial here), and that
// fixed point is never trusted on the refiner's own say-so:
// verifyBackwardDE independently re-derives, directly from the definition,
// whether every member of every reported block really does carry the
// identical collapsed polynomial — the same independent-verifier discipline
// lumpability.go's verifyStable applies to its own refinement.
//
// # What this deliberately does NOT implement: forward differential equivalence
//
// The ROADMAP item names both directions. Forward differential equivalence
// (same paper) is the dual notion: a partition (or more generally a linear
// change of variables) under which a chosen linear combination of each
// block — not necessarily requiring every member's OWN trajectory to
// coincide — evolves autonomously, preserving a declared OBSERVABLE rather
// than requiring pairwise member equality. The paper characterizes it via a
// SEPARATE combinatorial condition operating on the transpose role of the
// same monomial coefficients (stated in terms of which places' derivatives a
// given place's OWN value can influence, dualizing backward DE's "which
// places have the same derivative"), and a correct implementation needs its
// own partition-refinement over that dual relation — reusing this file's
// collapsedPoly/refine loop verbatim would silently answer the wrong
// question, not an approximation of the right one. Nothing in this file
// computes it, and no half version of it ships: per this task's own
// instruction, an unfinished forward-DE implementation that LOOKED complete
// would be worse than the gap staying visible in ROADMAP.md.
package sim

import (
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

// monomial is a canonical (sorted) encoding of a product of at most two
// place-or-block indices — the degree bound this file implements (see the
// package doc above). deg is 0 (the empty product, a zero-order/constant-rate
// term), 1 (a single index, at field a) or 2 (two indices, at a and b, with
// a<=b so mono2(x,y) and mono2(y,x) hash identically).
type monomial struct {
	deg  int
	a, b int
}

func mono0() monomial { return monomial{deg: 0} }
func mono1(a int) monomial {
	return monomial{deg: 1, a: a}
}
func mono2(a, b int) monomial {
	if b < a {
		a, b = b, a
	}
	return monomial{deg: 2, a: a, b: b}
}

// monoOf builds the monomial for a transition's reactant indices (already
// bounded to length <= 2 by extractReactions's degree check) — vars may name
// PLACE indices (the raw extraction) or BLOCK indices (during refinement),
// since collapsing a place to its block and then forming the monomial is the
// same operation as forming the monomial and then relabelling each factor by
// its block; this function does not care which.
func monoOf(vars []int) monomial {
	switch len(vars) {
	case 0:
		return mono0()
	case 1:
		return mono1(vars[0])
	default:
		return mono2(vars[0], vars[1])
	}
}

// deReaction is one transition's contribution to the ODE, extracted once
// with exact rational arithmetic: rate is the transition's own declared rate
// (defaulted the same way Rates(m) defaults every other reader of it,
// captured bit-exactly rather than rounded), inputs are the (<=2) reactant
// PLACE indices the flux multiplies together, and stoich is the net
// stoichiometry (production minus consumption) this reaction contributes to
// each place it touches — a place appearing as both a reactant and a product
// of the same transition (a catalyst) nets to whatever the two weights leave,
// including exactly zero.
type deReaction struct {
	rate   *big.Rat
	inputs []int
	stoich map[int]*big.Rat
}

// extractReactions builds the exact-rational reaction list this file's
// refinement operates over, indexed by tokenPlaces' place ordering (index).
// It is the only place float64 rates are converted to *big.Rat
// (SetFloat64 captures the IEEE-754 value exactly — the model's own declared
// rate, bit for bit, not a rounded approximation of it) and the only place
// the degree bound is enforced.
func extractReactions(m *metamodel.Model, index map[string]int) ([]deReaction, error) {
	rates := Rates(m)
	out := make([]deReaction, 0, len(m.Transitions))
	for i := range m.Transitions {
		t := &m.Transitions[i]
		inputs := m.Inputs(t.ID)
		if len(inputs) > 2 {
			return nil, fmt.Errorf(
				"transition %s has %d distinct reactant arc(s), and backward differential equivalence here is "+
					"implemented only for the degree-<=2 mass-action rate law go-pflow's solver actually computes "+
					"(at most 2 input places per transition — see this file's package doc, \"The degree bound\"); "+
					"refusing rather than silently approximate a higher-degree polynomial", t.ID, len(inputs))
		}
		rate := new(big.Rat)
		if rate.SetFloat64(rates[t.ID]) == nil {
			return nil, fmt.Errorf("transition %s: rate %g is not an exact rational (NaN or +/-Inf)", t.ID, rates[t.ID])
		}
		r := deReaction{rate: rate, stoich: map[int]*big.Rat{}}
		add := func(place string, weight int, sign int64) {
			pi, ok := index[place]
			if !ok {
				return // not a token place; tokenPlaces already excludes these from the whole computation
			}
			r.stoich[pi] = new(big.Rat).Add(ratOrZero(r.stoich[pi]), big.NewRat(sign*int64(weight), 1))
		}
		for _, in := range inputs {
			pi, ok := index[in.Place]
			if !ok {
				continue
			}
			r.inputs = append(r.inputs, pi)
			add(in.Place, in.Weight, -1)
		}
		for _, o := range m.Outputs(t.ID) {
			add(o.Place, o.Weight, +1)
		}
		out = append(out, r)
	}
	return out, nil
}

func ratOrZero(r *big.Rat) *big.Rat {
	if r == nil {
		return new(big.Rat)
	}
	return r
}

// collapsedPoly is dp/dt for place p, with every input place in every
// reaction's monomial replaced by its CURRENT block id (blockOf) — see the
// package doc's algorithm section. Terms landing on the same collapsed
// monomial are summed with exact rational addition; the result is the
// signature refineBackwardDE and verifyBackwardDE both compare.
func collapsedPoly(p int, reactions []deReaction, blockOf []int) map[monomial]*big.Rat {
	out := map[monomial]*big.Rat{}
	for i := range reactions {
		r := &reactions[i]
		coeff, ok := r.stoich[p]
		if !ok {
			continue
		}
		blocks := make([]int, len(r.inputs))
		for j, in := range r.inputs {
			blocks[j] = blockOf[in]
		}
		mo := monoOf(blocks)
		term := new(big.Rat).Mul(coeff, r.rate)
		if cur, ok := out[mo]; ok {
			cur.Add(cur, term)
		} else {
			out[mo] = term
		}
	}
	return out
}

// polySignature renders a collapsed polynomial as a canonical, comparable
// string, sorted by monomial. A term whose coefficient is EXACTLY the
// rational zero is dropped — it makes no difference to the polynomial
// whether that zero came from a single reaction with a zero net stoichiometry
// or from two reactions' terms exactly cancelling once collapsed, and
// dropping it is what lets two structurally different reaction sets that
// happen to sum to the identical polynomial compare equal, which is exactly
// what "identical solutions" requires.
func polySignature(poly map[monomial]*big.Rat) string {
	type kv struct {
		m monomial
		v *big.Rat
	}
	items := make([]kv, 0, len(poly))
	for m, v := range poly {
		if v.Sign() == 0 {
			continue
		}
		items = append(items, kv{m, v})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].m.deg != items[j].m.deg {
			return items[i].m.deg < items[j].m.deg
		}
		if items[i].m.a != items[j].m.a {
			return items[i].m.a < items[j].m.a
		}
		return items[i].m.b < items[j].m.b
	})
	var b strings.Builder
	for _, it := range items {
		fmt.Fprintf(&b, "%d:%d:%d=%s;", it.m.deg, it.m.a, it.m.b, it.v.RatString())
	}
	return b.String()
}

// colourToBlocks turns a slice of opaque colour strings into contiguous block
// ids, sorted by the colour string itself so the numbering is deterministic
// across calls (map iteration order is not) rather than depending on
// insertion order.
func colourToBlocks(colour []string) ([]int, int) {
	seen := map[string]bool{}
	for _, c := range colour {
		seen[c] = true
	}
	order := make([]string, 0, len(seen))
	for c := range seen {
		order = append(order, c)
	}
	sort.Strings(order)
	id := make(map[string]int, len(order))
	for i, c := range order {
		id[c] = i
	}
	blockOf := make([]int, len(colour))
	for i, c := range colour {
		blockOf[i] = id[c]
	}
	return blockOf, len(order)
}

// refineBackwardDE computes the coarsest partition of n places stable under
// "same collapsed polynomial", starting from the single all-places block —
// see the package doc's algorithm section for why this converges and to what.
func refineBackwardDE(n int, reactions []deReaction) ([]int, int) {
	if n == 0 {
		return nil, 0
	}
	colour := make([]string, n) // all "" => one block, the coarsest possible start
	_, numBlocks := colourToBlocks(colour)
	for round := 0; round < n+1; round++ {
		blockOf, _ := colourToBlocks(colour)
		next := make([]string, n)
		for p := 0; p < n; p++ {
			next[p] = colour[p] + "||" + polySignature(collapsedPoly(p, reactions, blockOf))
		}
		_, nb := colourToBlocks(next)
		colour = next
		if nb == numBlocks {
			break // stable: no block split this round
		}
		numBlocks = nb
	}
	return colourToBlocks(colour)
}

// verifyBackwardDE independently re-derives, directly from the definition,
// whether blockOf really is stable: every member of every block must carry
// the IDENTICAL collapsed polynomial under blockOf itself (not under
// whatever intermediate partition refineBackwardDE happened to compute it
// from). This is the safety net described in the package doc: a bug in the
// round-by-round bookkeeping above becomes a loud internal error, never a
// silently wrong "proved" partition.
func verifyBackwardDE(n int, reactions []deReaction, blockOf []int, numBlocks int) bool {
	sig := make([]string, numBlocks)
	seen := make([]bool, numBlocks)
	for p := 0; p < n; p++ {
		b := blockOf[p]
		s := polySignature(collapsedPoly(p, reactions, blockOf))
		if !seen[b] {
			seen[b] = true
			sig[b] = s
			continue
		}
		if sig[b] != s {
			return false
		}
	}
	return true
}

// DiffEqLumping is BackwardDifferentialEquivalence's report: a PLACE-level
// claim, PROVED with exact rational arithmetic over the net's own mass-action
// polynomial — as distinct from Lumping (classify.go), which asks the same
// PLACE-level question but only PROPOSES from colour refinement and VERIFIES
// against one run's sampled trajectory spread. Reached via Lumping.Exact,
// never replacing it, per this task's own instruction that both stay visible.
type DiffEqLumping struct {
	Applicable bool     `json:"applicable"`
	Refusals   []string `json:"refusals,omitempty"`
	// Trivial is true when the coarsest partition this net admits is the
	// discrete one (every place its own block) — a real, proved "no
	// non-trivial backward differential equivalence exists" rather than a
	// failure to find one, the same distinction LumpingPartition.Trivial
	// draws for the STATE-level question.
	Trivial bool    `json:"trivial,omitempty"`
	Classes []Class `json:"classes,omitempty"`
	// Method and Scope mirror LumpingPartition's own fields: Method says HOW
	// this was established, Scope says WHAT question it answers, so a reader
	// never has to infer either from context.
	Method   string   `json:"method,omitempty"`
	Scope    string   `json:"scope,omitempty"`
	Evidence Evidence `json:"evidence,omitempty"`
}

// backwardDEMethod is the Method tag BackwardDifferentialEquivalence reports
// when it settles the question (a non-trivial lumping found, or a proved
// refusal that none exists) — the exact counterpart to LumpingReport's
// "backward-differential-equivalence/candidate" Method on its own Classes, so
// the two are greppable as a pair.
const backwardDEMethod = "backward-differential-equivalence/exact-partition-refinement-popl2016"

// backwardDEScope is DiffEqLumping's fixed Scope text, written to be read
// alongside placeLumpingScope (Lumping's own, classify.go) since both answer
// the identically-worded PLACE-level question and only their METHOD differs.
const backwardDEScope = "PLACE-level: is there a coarser partition of the ODE's own place variables such that " +
	"summing each block's derivative and re-expressing it purely in terms of block-sum variables is well-defined, " +
	"with IDENTICAL member trajectories, for EVERY initial condition that starts each block uniform? PROVED with " +
	"exact rational arithmetic over the net's degree-<=2 mass-action polynomial (backward differential " +
	"equivalence, Cardelli, Tribastone, Tschaikowski & Vandin, POPL 2016) — distinct from Lumping's Classes " +
	"(classify.go), which propose the same PLACE-level question from colour refinement (1-WL) and verify it only " +
	"against one run's sampled trajectory spread, within that run's own standard deviation."

// BackwardDifferentialEquivalence decides backward differential equivalence
// of m's mass-action ODE exactly — see the package doc atop this file for the
// question, the degree bound this actually implements, the refusal set (m's
// own Gating(), the same one Forecast uses), and the algorithm.
//
// It REFUSES (Applicable:false, never Applicable:true with a wrong answer)
// whenever the question cannot be honestly asked of this net: a declared rate
// schedule, anything m.Gating() names (read/inhibitor/non-kinetic arcs, a
// reachable capacity, a guard), no token places, or a transition whose
// reactant count exceeds the degree-<=2 bound this file implements. A non-nil
// error is reserved for an internal contradiction (verifyBackwardDE
// disagreeing with refineBackwardDE's own output, which is a bug in this
// file, not a fact about the model) or a rate that is not representable as an
// exact rational (NaN/Inf) — never for an ordinary, expected refusal.
func BackwardDifferentialEquivalence(m *metamodel.Model) (*DiffEqLumping, error) {
	out := &DiffEqLumping{Scope: backwardDEScope}

	if m.HasSchedules() {
		out.Refusals = append(out.Refusals, "this model declares rate schedules: a time-varying rate is not a "+
			"fixed polynomial coefficient, so there is no autonomous dx/dt=f(x) to ask backward differential "+
			"equivalence of")
		return out, nil
	}
	if gating := m.Gating(); len(gating) > 0 {
		out.Refusals = append(out.Refusals, gating...)
		return out, nil
	}

	places, index, err := tokenPlaces(m)
	if err != nil {
		out.Refusals = append(out.Refusals, err.Error())
		return out, nil
	}

	reactions, err := extractReactions(m, index)
	if err != nil {
		out.Refusals = append(out.Refusals, err.Error())
		return out, nil
	}

	n := len(places)
	blockOf, numBlocks := refineBackwardDE(n, reactions)
	if !verifyBackwardDE(n, reactions, blockOf, numBlocks) {
		return nil, fmt.Errorf(
			"internal: refineBackwardDE produced a partition verifyBackwardDE rejects — this is a bug in " +
				"diffeq.go, not a fact about the model; refusing to report it as proved")
	}

	out.Applicable = true
	out.Method = backwardDEMethod
	out.Trivial = numBlocks == n
	out.Evidence = structural(
		"partition refinement over exact-rational degree-<=2 monomial coefficients (backward differential "+
			"equivalence, Cardelli, Tribastone, Tschaikowski & Vandin, POPL 2016), verified independently of the "+
			"refinement that found it",
		"this is the COARSEST partition of the net's own token places whose block sums solve a well-defined, "+
			"smaller polynomial ODE system with identical member trajectories, for EVERY initial condition that "+
			"starts each block uniform — not sampled, not tolerance-bounded",
		"forward differential equivalence (a linear change of variables not restricted to a place partition, "+
			"which this file does not compute — see its package doc); Buchholz's general linear-aggregation exact "+
			"lumpability (lumpability.go's own doc comment names the same distinction for the STATE-level "+
			"question); and anything about one specific run's own sampled trajectory, which is Lumping's claim, "+
			"not this one")

	byBlock := make([][]int, numBlocks)
	for p, b := range blockOf {
		byBlock[b] = append(byBlock[b], p)
	}
	for _, members := range byBlock {
		if len(members) < 2 {
			continue // a class of one is just a place, not a finding
		}
		ids := make([]string, len(members))
		for i, p := range members {
			ids[i] = places[p]
		}
		sort.Strings(ids)
		c := Class{
			ID: label(ids), Members: ids, Kind: KindIdenticalDynamics,
			Method: backwardDEMethod, Verified: true, Fungible: true,
			EstablishedBy: []Evidence{out.Evidence},
		}
		c.Evidence = Summarise(c.EstablishedBy)
		out.Classes = append(out.Classes, c)
	}
	return out, nil
}

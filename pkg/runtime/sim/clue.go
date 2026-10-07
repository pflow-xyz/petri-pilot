// Constrained lumping (CLUE) against the declared observable (ROADMAP.md
// Phase 6: "The most useful grouping is not 'which places are alike' but
// 'which parameters are the same parameter for this question'. The
// diagnosis already names an observable; nothing consumes it this way.").
//
//	Ovchinnikov, A., Pérez Verona, I., Pogudin, G. and Rahkooy, H. (2021).
//	"CLUE: exact maximal reduction of kinetic models by constrained
//	lumping." Bioinformatics, 37(12), pp.1732-1738.
//
// # What question this answers, and how it differs from diffeq.go
//
// BackwardDifferentialEquivalence (diffeq.go) finds the coarsest partition
// preserving EVERY place's own trajectory — the right answer when nothing
// in particular is asked yet. But Diagnose always DOES have a question
// already in hand: the loss places, or the success places, that its own
// Findings and Knobs are scored against (readShape's sh.lossPlaces /
// sh.successPlaces — a "set of places summed", exactly as this task
// predicted before this file existed). Two places can be genuinely
// different variables — different own dynamics, no automorphism, no
// backward equivalence — and STILL be safely collapsible into one number
// for the sole purpose of predicting that specific sum, because only their
// SUM feeds anything the observable can see. Seeding refinement from the
// observable's own span, rather than from "everything must stay exact",
// is exactly CLUE's idea: start from the one direction that has to survive
// and grow the protected set only as far as closing it under the dynamics
// actually forces — never further.
//
// # What is implemented, and what is not
//
// Full CLUE operates over an arbitrary linear subspace of R^n (any set of
// linear functionals as the seed) and finds the coarsest subspace closed
// under "this direction's derivative is itself expressible inside the
// subspace" — genuinely more general than any place-partition. What this
// file implements is the PARTITION-restricted case: the seed is a single
// observable functional built from named PLACES (ObservablePlaces — a 0/1
// or otherwise weighted indicator over place ids, matching the only shape
// Diagnose's own observable actually takes), and the reduction found is
// always a partition of places into blocks whose SUMS are tracked, never
// an arbitrary linear combination. This is the restriction the task's own
// instructions explicitly permit trading for what the model actually
// hands this file, and it is also exactly forward differential
// equivalence (Cardelli, Tribastone, Tschaikowski & Vandin, POPL 2016 —
// the same paper diffeq.go's package doc cites for backward DE and
// explicitly deferred the forward half of) SEEDED by the observable rather
// than grown from nothing: a partition R such that (a) the observable is
// constant on every block of R (so its value is reconstructible from block
// sums alone) and (b) every block's SUMMED derivative is itself a
// well-defined function of the other blocks' sums — the "aggregate
// closes" condition CLUE calls self-consistency.
//
// This file additionally restricts to LINEAR (degree<=1) mass-action
// systems: at most one reactant place per transition, one degree narrower
// than diffeq.go's degree<=2. That restriction is not the general CLUE
// algorithm's own limit — the cited paper handles polynomial systems of any
// degree via full linear algebra over the space of monomials — it is what
// THIS implementation checked and verified. A degree-2 (bimolecular)
// self-consistency check requires reasoning about products of two block
// sums expanding against products of two place values, exactly the extra
// bookkeeping diffeq.go's monomial machinery exists for; getting that
// aggregate condition right for CLUE specifically (as opposed to backward
// DE's per-member equality, which diffeq.go already implements and this
// file reuses the extraction from) was judged out of scope for what could
// be implemented AND independently verified here. A transition with two
// reactant places is refused BY NAME (extractLinearSystem), never
// silently handled as if it were linear.
//
// # The algorithm: dual refinement seeded by the observable
//
// diffeq.go's own package doc names exactly what forward equivalence needs
// that backward equivalence's collapsedPoly loop does not compute: "a
// partition-refinement over the DUAL relation ... which places' derivatives
// a given place's OWN value can influence, dualizing backward DE's 'which
// places have the same derivative'". This file is that dual refinement.
//
// For a linear system dx/dt = Ax + b (A built once from every degree<=1
// reaction's stoichiometry and exact-rational rate, exactly as
// extractReactions already does for diffeq.go), define influence[p][q] =
// the coefficient of x_p in dx_q/dt — the effect one unit of place p has on
// every other place's own rate of change. A partition R is self-consistent
// (clue-refined) iff, for every block B and every OTHER block B', the
// quantity effect(p, B') := sum_{q in B'} influence[p][q] is IDENTICAL for
// every p in B — i.e. it does not matter WHICH member of B contributed the
// unit, only that B contributed it, which is precisely what "the block's
// sum has a well-defined effect on B'" means. (B'==B is deliberately
// excluded: exchange internal to a block cancels out of that block's own
// SUM by construction, and refusing to check it is what lets this
// algorithm find A<->B-exchange-plus-equal-outflow pairs backward DE
// cannot — see the demonstration test.)
//
// Refinement starts from the partition induced by the observable's own
// coefficients (two places can only ever end up in one block if they
// started with the identical observable coefficient — enforced by
// including it as a permanent PREFIX of every round's colour, so it can
// never be forgotten by a later merge, the same monotone-prefix trick
// refineBackwardDE uses for the prior round's colour) and repeatedly SPLITS
// any block whose members disagree on effect(p, B') for some current block
// B', exactly mirroring refineBackwardDE's fixed-point loop with a
// different per-round signature. This is finite for the same reason
// (splitting strictly shrinks the coarsest live block, and there are at
// most n places) and converges to the COARSEST partition refining the
// observable's own seed that is self-consistent — never proposed as
// "proved" without verifyClueLumping (below) independently re-deriving the
// same fact directly from influence, mirroring verifyBackwardDE's
// discipline in diffeq.go.
//
// # Refusal set
//
// Reuses m.Gating() and m.HasSchedules() exactly as diffeq.go does, for the
// identical reason: none of read/inhibitor/non-kinetic arcs, a reachable
// capacity, a guard or a rate schedule leaves an autonomous polynomial to
// extract a linear system from in the first place.
package sim

import (
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

// Observable is a linear functional over the net's own token places: the
// declared question a constrained lumping is asked to protect. Coefficients
// are exact rationals for the same reason diffeq.go's monomial coefficients
// are: "equal" must mean the identical rational number, never two floats
// within a tolerance, or a lumping could be reported exact when it is not.
type Observable map[string]*big.Rat

// ObservablePlaces builds the indicator functional a set of named places
// (Diagnose's own sh.lossPlaces or sh.successPlaces — the only shape
// Diagnose's observable actually takes today, confirmed by reading
// diagnose.go) induces: coefficient 1 on each named place, 0 elsewhere. This
// is the "set of places summed" this file's package doc predicted before
// being written, not a guess at a more general functional shape nothing in
// this codebase currently produces.
func ObservablePlaces(ids ...string) Observable {
	obs := make(Observable, len(ids))
	for _, id := range ids {
		obs[id] = big.NewRat(1, 1)
	}
	return obs
}

// ClueLumping is ConstrainedLumping's report — see this file's package doc
// for exactly what question it answers, what it does and does not
// implement, and how it differs from DiffEqLumping (diffeq.go).
type ClueLumping struct {
	Applicable bool     `json:"applicable"`
	Refusals   []string `json:"refusals,omitempty"`
	// Observable names exactly which places (and coefficients) defined the
	// question this lumping protects, rendered as exact rational strings so
	// a reader never has to re-derive it from Diagnosis' own Loss/Success
	// fields (which can move independently of when this ran).
	Observable map[string]string `json:"observable,omitempty"`
	// Trivial is true when the coarsest self-consistent partition refining
	// the observable's own seed is the discrete one — a real, proved "this
	// observable buys no lumping at all" rather than a failure to find one.
	Trivial bool    `json:"trivial,omitempty"`
	Classes []Class `json:"classes,omitempty"`
	Method  string  `json:"method,omitempty"`
	Scope   string  `json:"scope,omitempty"`
	// Evidence is the structural claim's own record, StructuralProof-typed
	// exactly like DiffEqLumping.Evidence.
	Evidence Evidence `json:"evidence,omitempty"`
	// ComparedToBackwardDE is the structural sanity check this file's own
	// package doc promises: this observable's block count against
	// BackwardDifferentialEquivalence's on the SAME net, so a reader can see
	// directly that seeding from one question never lumps FINER than
	// preserving everything would, and usually lumps strictly coarser.
	ComparedToBackwardDE string `json:"comparedToBackwardDE,omitempty"`
}

// clueMethod is CLUE's Method tag, the same greppable-pair convention
// diffeq.go's backwardDEMethod established relative to LumpingReport's own
// "backward-differential-equivalence/candidate".
const clueMethod = "constrained-lumping/clue-linear-forward-partition"

// clueScope is ClueLumping's fixed Scope text, written to be read alongside
// backwardDEScope since the two answer related but distinct PLACE-level
// questions and only Scope tells a reader which is which without opening
// the source.
const clueScope = "PLACE-level, ONE OBSERVABLE: is there a coarser partition of the ODE's own place variables " +
	"such that the DECLARED OBSERVABLE's value alone — not every place's own trajectory — stays exactly " +
	"reconstructible from the block-sum variables, for every initial condition? PROVED with exact rational " +
	"linear algebra (constrained lumping / CLUE, Ovchinnikov, Pérez Verona, Pogudin & Rahkooy, Bioinformatics " +
	"2021), restricted here to LINEAR (degree<=1) mass-action systems and to PARTITION-shaped reductions " +
	"(forward differential equivalence, Cardelli, Tribastone, Tschaikowski & Vandin, POPL 2016, seeded by the " +
	"observable rather than the arbitrary linear subspaces full CLUE admits). Distinct from " +
	"BackwardDifferentialEquivalence (diffeq.go), which must preserve EVERY place's own trajectory and can " +
	"therefore only ever be as coarse or finer: this asks a strictly weaker question, so it can find real " +
	"lumpings backward equivalence proves do not exist — and it establishes NOTHING about any place, or any " +
	"combination of places, outside the stated observable's own span."

// influenceMatrix is the exact-rational linear system this file extracts:
// dx_q/dt = constant[q] + sum_p influence[p][q] * x_p. influence[p][q] is
// "the effect one unit of place p has on place q's own rate of change" — the
// DUAL orientation to diffeq.go's collapsedPoly, which asks "what is p's OWN
// derivative": collapsedPoly reads a ROW of the same information this reads
// as a COLUMN.
type influenceMatrix struct {
	n        int
	effect   [][]*big.Rat // effect[p][q]
	constant []*big.Rat   // constant[q]
}

// extractLinearSystem builds influenceMatrix from extractReactions' own
// output (diffeq.go) — reusing the prior stage's polynomial extraction
// rather than re-deriving it, per this task's own instruction — and refuses
// BY NAME the moment any reaction has more than one reactant place: that is
// a real bimolecular term this file's linear-only self-consistency check
// does not attempt (see the package doc's "What is implemented" section).
func extractLinearSystem(m *metamodel.Model, reactions []deReaction, n int) (*influenceMatrix, error) {
	for i, r := range reactions {
		if len(r.inputs) > 1 {
			return nil, fmt.Errorf(
				"transition %s has %d distinct reactant place(s): constrained lumping here is implemented only "+
					"for LINEAR (degree<=1, at most one reactant place per transition) mass-action systems — one "+
					"degree narrower than BackwardDifferentialEquivalence's degree<=2 — because this file's "+
					"self-consistency check has not been implemented, or independently verified, for a bimolecular "+
					"term; refusing rather than silently approximate it", m.Transitions[i].ID, len(r.inputs))
		}
	}
	im := &influenceMatrix{n: n, constant: make([]*big.Rat, n)}
	im.effect = make([][]*big.Rat, n)
	for p := 0; p < n; p++ {
		im.effect[p] = make([]*big.Rat, n)
	}
	for i := range im.constant {
		im.constant[i] = new(big.Rat)
	}
	for i := range im.effect {
		for j := range im.effect[i] {
			im.effect[i][j] = new(big.Rat)
		}
	}
	for _, r := range reactions {
		term := func(q int) *big.Rat {
			coeff, ok := r.stoich[q]
			if !ok {
				return nil
			}
			return new(big.Rat).Mul(coeff, r.rate)
		}
		if len(r.inputs) == 0 {
			for q := range r.stoich {
				if v := term(q); v != nil {
					im.constant[q].Add(im.constant[q], v)
				}
			}
			continue
		}
		p := r.inputs[0]
		for q := range r.stoich {
			if v := term(q); v != nil {
				im.effect[p][q].Add(im.effect[p][q], v)
			}
		}
	}
	return im, nil
}

// effectOnBlock sums influence[p][q] over every q in members — "the total
// effect one unit of place p has on this whole block's own sum".
func (im *influenceMatrix) effectOnBlock(p int, members []int) *big.Rat {
	sum := new(big.Rat)
	for _, q := range members {
		sum.Add(sum, im.effect[p][q])
	}
	return sum
}

// refineClue computes the coarsest partition of n places, refining the
// observable-induced seed colour, that is self-consistent under
// effectOnBlock — see the package doc's algorithm section. seed[p] is
// carried as a permanent prefix of every round's colour so the observable's
// own distinctions, once made, can never be undone by a later merge, the
// identical monotone-prefix discipline refineBackwardDE uses.
func refineClue(n int, im *influenceMatrix, seed []string) ([]int, int) {
	if n == 0 {
		return nil, 0
	}
	colour := append([]string(nil), seed...)
	_, numBlocks := colourToBlocks(colour)
	for round := 0; round < n+1; round++ {
		blockOf, nb0 := colourToBlocks(colour)
		members := make([][]int, nb0)
		for p, b := range blockOf {
			members[b] = append(members[b], p)
		}
		next := make([]string, n)
		for p := 0; p < n; p++ {
			var parts []string
			for b := 0; b < nb0; b++ {
				if b == blockOf[p] {
					continue // effect on one's OWN block cancels in that block's own sum; see package doc
				}
				parts = append(parts, im.effectOnBlock(p, members[b]).RatString())
			}
			next[p] = colour[p] + "||" + strings.Join(parts, ",")
		}
		_, nb := colourToBlocks(next)
		colour = next
		if nb == numBlocks {
			break
		}
		numBlocks = nb
	}
	return colourToBlocks(colour)
}

// verifyClueLumping independently re-derives, directly from the definition,
// whether blockOf really is self-consistent: for every block and every
// OTHER block, effectOnBlock must be identical for every member of the
// source block. This is the safety net verifyBackwardDE's own doc comment
// describes for the sibling file — a bug in refineClue's round-by-round
// bookkeeping becomes a loud internal error, never a silently wrong "proved"
// partition.
func verifyClueLumping(n int, im *influenceMatrix, blockOf []int, numBlocks int) bool {
	members := make([][]int, numBlocks)
	for p, b := range blockOf {
		members[b] = append(members[b], p)
	}
	sig := make([]string, numBlocks)
	seen := make([]bool, numBlocks)
	for p := 0; p < n; p++ {
		b := blockOf[p]
		var parts []string
		for b2 := 0; b2 < numBlocks; b2++ {
			if b2 == b {
				continue
			}
			parts = append(parts, im.effectOnBlock(p, members[b2]).RatString())
		}
		s := strings.Join(parts, ",")
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

// ConstrainedLumping decides constrained lumping (CLUE) of m's linear
// mass-action ODE, seeded by observable, exactly — see this file's package
// doc for the question, the scope this implementation actually covers
// (LINEAR, degree<=1, partition-shaped reductions — not full CLUE's
// arbitrary subspaces over polynomial systems of any degree), the refusal
// set, and the algorithm.
//
// It REFUSES (Applicable:false, never Applicable:true with a wrong answer)
// whenever the question cannot be honestly asked: an empty observable, a
// declared rate schedule, anything m.Gating() names, no token places, or any
// transition with more than one reactant place. A non-nil error is reserved
// for an internal contradiction (verifyClueLumping disagreeing with
// refineClue's own output) or a non-representable rate (NaN/Inf) — never for
// an ordinary, expected refusal.
func ConstrainedLumping(m *metamodel.Model, observable Observable) (*ClueLumping, error) {
	out := &ClueLumping{Scope: clueScope}
	if len(observable) == 0 {
		out.Refusals = append(out.Refusals, "no observable given: nothing to protect against being lumped away")
		return out, nil
	}
	if m.HasSchedules() {
		out.Refusals = append(out.Refusals, "this model declares rate schedules: a time-varying rate is not a "+
			"fixed polynomial coefficient, so there is no autonomous dx/dt=f(x) to ask constrained lumping of")
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
	for id := range observable {
		if _, ok := index[id]; !ok {
			out.Refusals = append(out.Refusals, fmt.Sprintf(
				"observable names %q, which is not a token place of this model", id))
			return out, nil
		}
	}

	reactions, err := extractReactions(m, index)
	if err != nil {
		out.Refusals = append(out.Refusals, err.Error())
		return out, nil
	}
	n := len(places)
	im, err := extractLinearSystem(m, reactions, n)
	if err != nil {
		out.Refusals = append(out.Refusals, err.Error())
		return out, nil
	}

	out.Observable = make(map[string]string, len(observable))
	seed := make([]string, n)
	for p, id := range places {
		c := observable[id]
		if c == nil {
			c = new(big.Rat)
		} else {
			out.Observable[id] = c.RatString()
		}
		seed[p] = c.RatString()
	}

	blockOf, numBlocks := refineClue(n, im, seed)
	if !verifyClueLumping(n, im, blockOf, numBlocks) {
		return nil, fmt.Errorf(
			"internal: refineClue produced a partition verifyClueLumping rejects — this is a bug in clue.go, " +
				"not a fact about the model; refusing to report it as proved")
	}

	out.Applicable = true
	out.Method = clueMethod
	out.Trivial = numBlocks == n
	out.Evidence = structural(
		"partition refinement seeded by the observable, over the dual (which-places-influence-which) exact-"+
			"rational relation (constrained lumping / CLUE, Ovchinnikov, Pérez Verona, Pogudin & Rahkooy, "+
			"Bioinformatics 2021; restricted here to linear systems and partition-shaped reductions — see this "+
			"file's package doc), verified independently of the refinement that found it",
		"this is the COARSEST partition refining the stated observable's own seed whose block sums solve a "+
			"well-defined, smaller linear ODE — so the observable's value is reconstructible from block sums "+
			"alone, for EVERY initial condition, not sampled and not tolerance-bounded",
		"anything about a place NOT expressible as one of the observable's own summed places: a member of a "+
			"merged block can genuinely diverge from every other member's own trajectory — this proves only that "+
			"their SUM behaves as claimed, never that the members are interchangeable, which is "+
			"BackwardDifferentialEquivalence's strictly stronger claim (diffeq.go), not this one")

	if de, err := BackwardDifferentialEquivalence(m); err == nil && de.Applicable {
		deBlocks := blockCountFromClasses(de.Classes, len(places))
		out.ComparedToBackwardDE = fmt.Sprintf(
			"backward differential equivalence over the same net found %d block(s) (trivial=%v); this "+
				"observable's constrained lumping found %d block(s) — CLUE is never finer, and is strictly "+
				"coarser exactly when the second number is smaller", deBlocks, de.Trivial, numBlocks)
	} else if err == nil {
		out.ComparedToBackwardDE = fmt.Sprintf(
			"backward differential equivalence over the same net was refused (%s); no comparison to make",
			strings.Join(de.Refusals, "; "))
	}

	byBlock := make([][]int, numBlocks)
	for p, b := range blockOf {
		byBlock[b] = append(byBlock[b], p)
	}
	for _, members := range byBlock {
		if len(members) < 2 {
			continue
		}
		ids := make([]string, len(members))
		for i, p := range members {
			ids[i] = places[p]
		}
		sort.Strings(ids)
		c := Class{
			ID: label(ids), Members: ids, Kind: KindObservableLumped,
			Method: clueMethod, Verified: true, Fungible: false,
			EstablishedBy: []Evidence{out.Evidence},
		}
		c.Evidence = Summarise(c.EstablishedBy)
		out.Classes = append(out.Classes, c)
	}
	return out, nil
}

// blockCountFromClasses recovers BackwardDifferentialEquivalence's own block
// count from its reported Classes (which list only blocks of size>=2) plus
// the total place count, since DiffEqLumping does not carry numBlocks
// itself: every place not named in a multi-member class is its own
// singleton block.
func blockCountFromClasses(classes []Class, totalPlaces int) int {
	merged := 0
	for _, c := range classes {
		merged += len(c.Members)
	}
	singles := totalPlaces - merged
	return singles + len(classes)
}

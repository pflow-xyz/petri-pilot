package sim

import (
	"fmt"
	"sort"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

// Siphons and traps, found algebraically against the static arc structure —
// never by exploring the reachable marking graph, which is the thing that
// blows up (ROADMAP.md Phase 6: "deadlock structure is currently found by
// enumeration when it could be found by algebra").
//
// # The definitions (Murata, "Petri Nets: Properties, Analysis and
// Applications", Proc. IEEE 77(4), 1989, §III.C)
//
// For a set of places S, •S ("preset of S") is every transition that can put
// a token into some place of S, and S• ("postset of S") is every transition
// that needs a token already in some place of S to fire — either by consuming
// one (an ordinary input arc) or merely by testing for one (a read arc; an
// inhibitor arc is the opposite condition and never belongs here, since it
// blocks a transition when S is HIGH, not when S is low).
//
//   - S is a SIPHON when •S ⊆ S•: every transition that could refill S also
//     needs S nonempty to fire. So once S holds zero tokens, every transition
//     that might have refilled it is blocked, and S stays at zero forever.
//     This is a structural fact about the arcs alone — it holds for any
//     marking that ever makes S empty, and in particular for the initial
//     marking if the model starts S empty.
//   - S is a TRAP when S• ⊆ •S, the dual: every transition draining S also
//     replenishes it, so once S holds a token it can never return to zero.
//
// # What this buys, and what it does not
//
// An unmarked minimal siphon at a model's initial marking is a structural
// deadlock WITNESS: every transition in that siphon's postset is permanently
// disabled, proved with no simulation and true for every reachable marking
// from here (see deadlockWitnessesAt). It is not, by itself, a proof that the
// WHOLE net deadlocks — other transitions elsewhere in the net may go on
// firing forever. The classical sufficient condition for full deadlock
// avoidance is stronger still (every siphon contains a marked trap) and is
// not implemented here; what is implemented is the one-directional witness
// the roadmap asks for, stated exactly as strong as it is proved.
//
// # Why enumeration is bounded, not banned
//
// Minimal siphon/trap enumeration is itself combinatorial over place subsets
// in the general case (deciding whether a MINIMAL siphon of small support
// exists is NP-hard) — that combinatorics is inherent to the question, not
// something this file is asked to avoid. What it avoids is needing the
// reachability GRAPH, which for these models is far larger than 2^|places|.
// See siphonTrapPlaceCap and siphonTrapNodeBudget for the two ways this still
// refuses cleanly rather than hang.

// siphonTrapPlaceCap bounds the search to nets whose token places fit in one
// uint64 bitmask, which is what makes every siphon/trap test below an O(1)
// mask compare per transition instead of an O(|S|) set operation. Above the
// cap this refuses (mirroring Forecast's refusal pattern) rather than switch
// to a slower representation silently or run long enough to look hung. Every
// catalog model as of 2026-09 fits comfortably inside it: vet-clinic (the
// largest single net) has 38 token places, galton-board (the largest
// composed one) has 45.
const siphonTrapPlaceCap = 63

// siphonTrapNodeBudget bounds the number of branch-and-bound decisions the
// search below will make, PER CONNECTED COMPONENT, before refusing. Minimal
// siphons/traps decompose exactly per weakly-connected component of the
// place graph (see connectedComponents), which turns a net's worst case from
// 2^|places| into a sum over its pieces — but a component that is not itself
// small and sparse still costs real search, and this is the one existing
// place where that shows up: vet-clinic (38 places, one dense component)
// completes in a little over a second, and galton-board (45 places, also one
// component) in under ten (TestSiphonSearchOnCatalog exercises every catalog
// model and TestDiagnoseEveryCatalogModel bounds it further as part of a full
// Diagnose run). A budget an order of magnitude smaller made both of those
// refuse outright — raised here once actual catalog timing was measured,
// not guessed. Above the budget this refuses (mirroring Forecast's refusal
// pattern) rather than run long enough to look hung; a fixed, documented
// number is what makes that refusal deterministic and reported, rather than
// a wall-clock guess.
const siphonTrapNodeBudget = 20_000_000

// structuralIO is the arc structure a siphon/trap test is decided against,
// read once per model through metamodel's own Inputs/Outputs/Tests — the
// firing rule's one home (CLAUDE.md) — rather than a second reading of
// m.Arcs.
type structuralIO struct {
	places []string
	// produces[t] is the bitmask of places some output arc of t writes to
	// (•{p} ∋ t for every bit set).
	produces map[string]uint64
	// needs[t] is the bitmask of places t requires nonempty to fire: an
	// ordinary consuming input, OR a read-arc test. An inhibitor test is
	// deliberately excluded — it disables t when the place is HIGH, so it is
	// never a reason t needs that place to hold a token.
	needs map[string]uint64
}

func buildStructuralIO(m *metamodel.Model) (*structuralIO, error) {
	var places []string
	for i := range m.Places {
		if m.Places[i].IsToken() {
			places = append(places, m.Places[i].ID)
		}
	}
	sort.Strings(places)
	if len(places) > siphonTrapPlaceCap {
		return nil, fmt.Errorf(
			"%d token places exceeds the %d-place cap this search uses (a uint64 bitmask per candidate set)",
			len(places), siphonTrapPlaceCap)
	}
	index := make(map[string]int, len(places))
	for i, p := range places {
		index[p] = i
	}

	produces := make(map[string]uint64, len(m.Transitions))
	needs := make(map[string]uint64, len(m.Transitions))
	for i := range m.Transitions {
		t := m.Transitions[i].ID
		var prod, need uint64
		for _, o := range m.Outputs(t) {
			if bit, ok := index[o.Place]; ok {
				prod |= 1 << uint(bit)
			}
		}
		for _, in := range m.Inputs(t) {
			if bit, ok := index[in.Place]; ok {
				need |= 1 << uint(bit)
			}
		}
		for _, tst := range m.Tests(t) {
			if tst.Type == metamodel.InhibitorArc {
				continue
			}
			if bit, ok := index[tst.Place]; ok {
				need |= 1 << uint(bit)
			}
		}
		produces[t] = prod
		needs[t] = need
	}
	return &structuralIO{places: places, produces: produces, needs: needs}, nil
}

// isSiphon reports whether s (a bitmask of places) satisfies •S ⊆ S•.
func (io *structuralIO) isSiphon(s uint64) bool {
	if s == 0 {
		return false
	}
	for t, prod := range io.produces {
		if prod&s != 0 && io.needs[t]&s == 0 {
			return false
		}
	}
	return true
}

// isTrap reports whether s satisfies the dual, S• ⊆ •S.
func (io *structuralIO) isTrap(s uint64) bool {
	if s == 0 {
		return false
	}
	for t, need := range io.needs {
		if need&s != 0 && io.produces[t]&s == 0 {
			return false
		}
	}
	return true
}

func bitsToMask(bits []int) uint64 {
	var m uint64
	for _, b := range bits {
		m |= uint64(1) << uint(b)
	}
	return m
}

func (io *structuralIO) names(s uint64) []string {
	var out []string
	for i, p := range io.places {
		if s&(1<<uint(i)) != 0 {
			out = append(out, p)
		}
	}
	return out
}

// minimalStructuralSets is the branch-and-bound search shared by minimal
// siphon and minimal trap enumeration. It decides, one place at a time in a
// fixed order, whether that place is IN or OUT of the candidate set, and
// prunes a branch the moment no completion of it can possibly satisfy the
// closure property — never falling back to brute-forcing every subset.
//
// `drives`/`rescue` name the two roles the closure property plays for
// whichever question is being asked: for a siphon, drives = produces (a
// transition already forced to write into S) and rescue = needs (some place
// in S must also be required by that same transition); for a trap the roles
// swap. `test` is the final closure check (isSiphon or isTrap).
//
// Two pruning rules do the actual work, and both are sound because in only
// ever grows and (in|undecided) only ever shrinks along any single path from
// the root:
//
//   - Dead-end: if some transition t already has drives[t]&in != 0 (forced),
//     but rescue[t] shares no bit with in OR any place still undecided, no
//     future decision can satisfy t — the whole subtree is hopeless.
//   - Minimality: if in already is (or contains) an already-found minimal
//     set, every completion from here is a non-minimal superset of it —
//     nothing further down this branch is worth keeping.
//
// `bits` is the global bit positions this search decides over — see
// connectedComponents below for why that is normally one weakly-connected
// component's places rather than every place in the net: a transition can
// only ever touch places within its own component, so the closure property
// on any set S decomposes exactly per component (S is a siphon/trap iff its
// restriction to each component is), and a minimal siphon or trap can never
// span two components — if it did, its nonempty part in one component would
// already be a smaller valid siphon/trap on its own, contradicting
// minimality. Searching per component turns an exponent over the WHOLE net
// into a sum of exponents over each piece, which is what makes vet-clinic and
// galton-board tractable in practice (see the doc comment on
// siphonTrapNodeBudget).
func minimalStructuralSets(bits []int, drives, rescue map[string]uint64, test func(uint64) bool) (sets []uint64, complete bool) {
	n := len(bits)
	if n == 0 {
		return nil, true
	}
	var minimal []uint64
	nodes := 0
	budgetExceeded := false

	isSupersetOfFound := func(s uint64) bool {
		for _, found := range minimal {
			if s&found == found {
				return true
			}
		}
		return false
	}

	var recurse func(i int, in, undecided uint64)
	recurse = func(i int, in, undecided uint64) {
		if budgetExceeded {
			return
		}
		nodes++
		if nodes > siphonTrapNodeBudget {
			budgetExceeded = true
			return
		}
		if isSupersetOfFound(in) {
			return
		}
		future := in | undecided
		for t, d := range drives {
			if d&in != 0 && rescue[t]&future == 0 {
				return // dead end: this transition can never be rescued
			}
		}
		if i == n {
			if in != 0 && test(in) {
				minimal = append(minimal, in)
			}
			return
		}
		bit := uint64(1) << uint(bits[i])
		recurse(i+1, in, undecided&^bit)     // exclude place bits[i]
		recurse(i+1, in|bit, undecided&^bit) // include place bits[i]
	}
	var full uint64
	for _, b := range bits {
		full |= uint64(1) << uint(b)
	}
	recurse(0, 0, full)
	return minimal, !budgetExceeded
}

// connectedComponents groups places into weakly-connected components of the
// place graph, where two places are connected if some transition's produces
// or needs bitmask touches both. Returned as global bit positions, one slice
// per component; singleton places with no incident transition at all still
// get their own component (a place nothing touches can never be part of any
// siphon or trap, but it costs nothing to let the search find that out).
func connectedComponents(io *structuralIO) [][]int {
	n := len(io.places)
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}
	unionMask := func(mask uint64) {
		first := -1
		for i := 0; i < n; i++ {
			if mask&(1<<uint(i)) == 0 {
				continue
			}
			if first == -1 {
				first = i
			} else {
				union(first, i)
			}
		}
	}
	// Union ALL places one transition touches together, produces and needs
	// combined — a transition that only produces into A and only needs B
	// still connects A and B, since it is the mechanism by which a set
	// containing one can be forced to answer for the other. Unioning the two
	// bitmasks separately (rather than their OR) was the first version of
	// this and it missed exactly that cross-role link, silently splitting
	// every two-place cycle A->t1->B->t2->A into two "components" of one
	// place each — TestTwoCycleMinimalSiphonAndTrapRequireBothPlaces is what
	// caught it.
	for t := range io.produces {
		unionMask(io.produces[t] | io.needs[t])
	}
	byRoot := map[int][]int{}
	for i := 0; i < n; i++ {
		r := find(i)
		byRoot[r] = append(byRoot[r], i)
	}
	roots := make([]int, 0, len(byRoot))
	for r := range byRoot {
		roots = append(roots, r)
	}
	sort.Ints(roots)
	out := make([][]int, 0, len(roots))
	for _, r := range roots {
		out = append(out, byRoot[r])
	}
	return out
}

// PlaceSet names a set of places, minimal-siphon or minimal-trap membership.
type PlaceSet struct {
	Places []string `json:"places"`
}

// DeadlockWitness records one unmarked minimal siphon and exactly what it
// proves — see the package doc comment above for the scope of the claim.
type DeadlockWitness struct {
	Siphon                []string `json:"siphon"`
	UnfireableTransitions []string `json:"unfireableTransitions"`
	Detail                string   `json:"detail"`
}

// SiphonTrapReport is the algebraic deadlock-structure surface, shaped like
// LumpingReport's Applicable/Refusals so a caller can tell "found none" from
// "could not check" without inspecting internals.
type SiphonTrapReport struct {
	Applicable        bool              `json:"applicable"`
	Refusals          []string          `json:"refusals,omitempty"`
	MinimalSiphons    []PlaceSet        `json:"minimalSiphons,omitempty"`
	MinimalTraps      []PlaceSet        `json:"minimalTraps,omitempty"`
	DeadlockWitnesses []DeadlockWitness `json:"deadlockWitnesses,omitempty"`
	Method            string            `json:"method,omitempty"`
}

// SiphonsAndTraps finds every minimal siphon and trap of m by algebra over
// the static arc structure, and reports which minimal siphons (if any) are
// unmarked at m's own initial marking — a structural deadlock witness for
// exactly the transitions named, proved without simulating anything.
func SiphonsAndTraps(m *metamodel.Model) *SiphonTrapReport {
	out := &SiphonTrapReport{Applicable: true, Method: StructuralProof}
	io, err := buildStructuralIO(m)
	if err != nil {
		out.Applicable = false
		out.Refusals = append(out.Refusals, err.Error())
		return out
	}
	if len(io.places) == 0 {
		return out // no token places; there is nothing to find, and that is not a refusal
	}

	var siphonBits, trapBits []uint64
	for _, component := range connectedComponents(io) {
		sBits, sComplete := minimalStructuralSets(component, io.produces, io.needs, io.isSiphon)
		tBits, tComplete := minimalStructuralSets(component, io.needs, io.produces, io.isTrap)
		siphonBits = append(siphonBits, sBits...)
		trapBits = append(trapBits, tBits...)
		if !sComplete {
			out.Refusals = append(out.Refusals, fmt.Sprintf(
				"minimal-siphon search over {%s} exceeded its %d-node budget before completing",
				joinPlaces(io.names(bitsToMask(component))), siphonTrapNodeBudget))
		}
		if !tComplete {
			out.Refusals = append(out.Refusals, fmt.Sprintf(
				"minimal-trap search over {%s} exceeded its %d-node budget before completing",
				joinPlaces(io.names(bitsToMask(component))), siphonTrapNodeBudget))
		}
	}
	if len(out.Refusals) > 0 {
		// A partial answer reported as complete is worse than no answer: an
		// operator reading "no unmarked siphon found" has to know whether
		// that means "proved none exists" or "the search gave up first".
		out.Applicable = false
		return out
	}

	sortPlaceSets := func(bits []uint64) []PlaceSet {
		sets := make([]PlaceSet, 0, len(bits))
		for _, s := range bits {
			sets = append(sets, PlaceSet{Places: io.names(s)})
		}
		sort.Slice(sets, func(i, j int) bool {
			if len(sets[i].Places) != len(sets[j].Places) {
				return len(sets[i].Places) < len(sets[j].Places)
			}
			return fmt.Sprint(sets[i].Places) < fmt.Sprint(sets[j].Places)
		})
		return sets
	}
	out.MinimalSiphons = sortPlaceSets(siphonBits)
	out.MinimalTraps = sortPlaceSets(trapBits)

	initial := m.InitialMarking()
	for _, s := range siphonBits {
		names := io.names(s)
		empty := true
		for _, p := range names {
			if initial[p] != 0 {
				empty = false
				break
			}
		}
		if !empty {
			continue
		}
		var dead []string
		for i := range m.Transitions {
			t := m.Transitions[i].ID
			if io.needs[t]&s != 0 {
				dead = append(dead, t)
			}
		}
		sort.Strings(dead)
		out.DeadlockWitnesses = append(out.DeadlockWitnesses, DeadlockWitness{
			Siphon:                names,
			UnfireableTransitions: dead,
			Detail: fmt.Sprintf(
				"{%s} is a siphon (•S ⊆ S•) and holds no tokens at the initial marking, so it can never gain "+
					"one again — proved from the arc structure alone, for every reachable marking, with no "+
					"simulation. Every transition needing a token from it (%v) is therefore permanently "+
					"disabled. This does not by itself prove the whole net deadlocks: other transitions "+
					"outside this siphon may keep firing indefinitely.",
				joinPlaces(names), dead),
		})
	}
	sort.Slice(out.DeadlockWitnesses, func(i, j int) bool {
		return fmt.Sprint(out.DeadlockWitnesses[i].Siphon) < fmt.Sprint(out.DeadlockWitnesses[j].Siphon)
	})
	return out
}

func joinPlaces(places []string) string {
	out := ""
	for i, p := range places {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}

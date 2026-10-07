package sim

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

// Exact automorphism orbits and a canonical form, by individualization-
// refinement — the technique nauty, bliss and saucy are built on (McKay &
// Piperno, "Practical graph isomorphism, II", Journal of Symbolic
// Computation 60 (2014), 94-112).
//
// # Why this exists (ROADMAP.md Phase 6)
//
// classify.go's refine() computes 1-WL colour refinement: the coarsest
// STABLE partition, in near-linear time. It is an over-approximation of the
// automorphism partition — two nodes can share a stable colour with no
// automorphism of the net actually exchanging them — which is exactly why
// VerifyFungible exists as a separate, empirical permutation experiment
// rather than trusting a shared colour outright. This file computes the
// EXACT partition instead: the true orbits of the net's automorphism group,
// proved rather than sampled, plus a canonical string that is identical for
// any two labellings of the same underlying (coloured) net.
//
// # The algorithm, precisely
//
// 1. Start from refine()'s own colouring (buildIncidence + wlRefine,
//    classify.go) — the coarsest stable partition. Reusing it rather than
//    recolouring from scratch is both a correctness requirement (a second,
//    hand-rolled arc-labelling scheme could quietly disagree with
//    arcLabel/refineSeed) and a performance one (1-WL already does most of
//    the discriminating work for free; individualization only has to work on
//    what 1-WL could not separate).
// 2. If some cell has more than one node (not fully discrete), pick the
//    SMALLEST non-singleton cell — the standard, simplest-correct
//    target-cell selector (a fancier one, e.g. nauty's "first cell touched
//    by the last split", is not implemented; see the size note below) — and
//    branch once per member: give that member a unique colour ("individualize"
//    it), then re-run wlRefine from there. This is one level of the search
//    tree; recurse until every branch is fully discrete (every cell size 1),
//    which is a total order over every place and transition.
// 3. Each discrete leaf's total order induces a canonical string: each
//    node's own seed colour (kind/initial/capacity/rate/tags — the same
//    per-node attributes refine() seeds with) plus its labelled incident
//    edges, both written in terms of ORDER POSITION rather than node id, so
//    two leaves compare equal iff there is a bijection between their orders
//    that preserves every attribute and every labelled edge — i.e. iff that
//    bijection is a genuine automorphism of the coloured net.
// 4. Two leaves with identical canonical strings hand back the automorphism
//    between them directly (position i of one order maps to position i of
//    the other). Every such automorphism is recorded; orbits are its
//    connected components under "some recorded automorphism maps u to v",
//    via union-find, which is exactly the union-find bound this task sets
//    (group ORDER and a minimal GENERATOR SET are harder questions this
//    orbit computation does not need or attempt).
// 5. The canonical form is the leaf with the lexicographically smallest
//    canonical string across the WHOLE search tree. Two isomorphic nets,
//    however differently labelled, necessarily explore isomorphic search
//    trees (individualizing "the same" cell members up to the isomorphism at
//    every level) and therefore produce the same smallest string — this is
//    what makes CanonicalModelID isomorphism-invariant.
//
// # Pruning: what is implemented and why it is enough here
//
// Two prunings are implemented, both standard individualization-refinement
// technique and both PROVABLY safe (they only ever skip a branch already
// known — via a genuine, previously recorded automorphism — to be
// isomorphic to one already explored, so no orbit and no canonical string is
// ever missed):
//
//   - Smallest-non-singleton-cell target selection (step 2 above), which
//     keeps the branching factor as small as the search can make it at every
//     level without needing nauty's more elaborate cell-selection heuristic.
//   - Automorphism pruning at the base of a branch: when a target cell's
//     members are enumerated, a candidate v is skipped if some already-
//     recorded automorphism fixes every node individualized so far on this
//     path (so it fixes the exact search context) AND maps that automorphism
//     to an already-tried sibling u in the same cell. Any leaf reachable
//     under v is then already reachable under u via that automorphism, so
//     the branch cannot contribute a smaller canonical string or a new
//     orbit fact.
//
// What is deliberately NOT implemented is nauty's full machinery: automorphism
// group representation via a base and strong generating set, orbit pruning
// derived from a properly maintained stabilizer chain, and non-trivial
// target-cell-selection heuristics (e.g. preferring the cell most recently
// split). Those exist to keep the search sub-exponential on graphs with
// thousands of vertices and heavy symmetry (molecule graphs, large regular
// graphs). This repo's models are dozens of places and transitions, not
// thousands (the largest catalog single net, vet-clinic, has 38 places;
// composed nets go somewhat higher) — TestExactOrbitsOnCatalog runs this
// implementation against every catalog model and each completes in well
// under a second. orbitSearchLeafBudget below is the safety net for a model
// this implementation was not sized for: it refuses (mirroring Forecast's
// and the siphon/trap search's refusal pattern in siphons.go) rather than
// run long enough to look hung.

// orbitSearchLeafBudget bounds the number of discrete leaves (fully
// individualized branches) the search below will visit before refusing.
// Every catalog model as of 2026-09 needs a handful of leaves at most — the
// two prunings above mean a genuinely symmetric net (many members in one
// orbit) individualizes cheaply, since most of the branch space collapses
// under automorphism pruning; what this budget guards against is a net with
// LARGE non-singleton cells that 1-WL and the automorphisms found so far
// cannot shrink, where the remaining search really is combinatorial in the
// cell sizes. A fixed, documented number is what makes the refusal
// deterministic and reported, rather than a wall-clock guess.
const orbitSearchLeafBudget = 200_000

// OrbitReport is ExactOrbits' result: the exact automorphism orbits of a
// net's places and transitions, and the canonical form that makes model
// identity isomorphism-invariant.
type OrbitReport struct {
	// Method names the algorithm, for the same reason every other structural
	// claim in this package does (classify.go, invariants.go, siphons.go):
	// so a consumer can tell a proof from a sample without reading source.
	Method string `json:"method"`

	// Orbits lists every NON-TRIVIAL orbit (size >= 2) among places and
	// transitions together, each sorted, the list itself sorted by first
	// member. A node touched by no orbit here is an orbit of size one — the
	// automorphism group fixes it — which is the discrete case and not worth
	// naming explicitly, the same convention groupBy (classify.go) already
	// uses for WL colour classes.
	Orbits [][]string `json:"orbits,omitempty"`

	// CanonicalForm is the lexicographically smallest canonical string found
	// across the whole search tree. It is exposed mainly so a caller can see
	// what CanonicalModelID hashes; treat it as opaque, not as a stable
	// format across versions of this algorithm.
	CanonicalForm string `json:"canonicalForm"`

	// Leaves is how many fully-discrete branches the search actually
	// visited (after pruning) — the size of the search, for anyone auditing
	// whether the budget headroom is comfortable.
	Leaves int `json:"leaves"`

	// Generators is how many distinct non-identity automorphisms were
	// recorded. Zero means the net is rigid: no automorphism at all, so
	// every WL colour class larger than one place is, by this proof, an
	// over-approximation (a Role, never a genuine FungibleSet).
	Generators int `json:"generators"`
}

// orbitOf reports the orbit (as a sorted slice) that id belongs to, or nil
// with ok=false if id is a fixed point (an orbit of size one, not recorded
// since Orbits only lists non-trivial ones).
func (r *OrbitReport) orbitOf(id string) ([]string, bool) {
	for _, o := range r.Orbits {
		for _, m := range o {
			if m == id {
				return o, true
			}
		}
	}
	return nil, false
}

// SameOrbit reports whether every member of members lies in exactly one
// automorphism orbit — the exact question VerifyFungible only samples.
// When it does not, groups is the partition members actually falls into
// under the proved orbits (fixed points reported as singleton groups), so a
// caller can say exactly how the over-approximation gap showed up rather
// than just that it did.
func (r *OrbitReport) SameOrbit(members []string) (same bool, groups [][]string) {
	if len(members) < 2 {
		return false, nil
	}
	byOrbit := map[string][]string{}
	for _, id := range members {
		key := id // fixed points are their own singleton key
		if o, ok := r.orbitOf(id); ok {
			key = o[0]
		}
		byOrbit[key] = append(byOrbit[key], id)
	}
	if len(byOrbit) == 1 {
		return true, nil
	}
	for _, g := range byOrbit {
		sort.Strings(g)
		groups = append(groups, g)
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i][0] < groups[j][0] })
	return false, groups
}

// permutation is one recorded automorphism, node id -> node id, with any id
// not present mapping to itself. Only non-identity entries are stored.
type permutation map[string]string

func (p permutation) apply(id string) string {
	if v, ok := p[id]; ok {
		return v
	}
	return id
}

// fixesPath reports whether p maps every id in path to itself — the
// condition that makes p a valid pruning witness at the current point in the
// search (an automorphism that disturbs an ancestor's individualization
// choice says nothing about whether the CURRENT branch is redundant).
func (p permutation) fixesPath(path []string) bool {
	for _, id := range path {
		if p.apply(id) != id {
			return false
		}
	}
	return true
}

// unionFind is the textbook disjoint-set structure — sufficient, per the
// task's own scope, for computing orbits (connected components of "some
// recorded generator maps u to v"); it deliberately does not attempt group
// order or a minimal generating set, which are separate, harder questions.
type unionFind struct{ parent map[string]string }

func newUnionFind(nodes []string) *unionFind {
	p := make(map[string]string, len(nodes))
	for _, n := range nodes {
		p[n] = n
	}
	return &unionFind{parent: p}
}

func (u *unionFind) find(x string) string {
	for u.parent[x] != x {
		u.parent[x] = u.parent[u.parent[x]]
		x = u.parent[x]
	}
	return x
}

func (u *unionFind) union(a, b string) {
	ra, rb := u.find(a), u.find(b)
	if ra != rb {
		u.parent[ra] = rb
	}
}

// orbitSearch carries the state one ExactOrbits call thread through the
// search tree: the graph (nodes/neighbours/seed, shared and read-only once
// built), and the accumulating results (recorded automorphisms, best
// canonical string, union-find over orbits, leaf/budget counters).
type orbitSearch struct {
	nodes      []string
	neighbours map[string][]nbr
	seed       map[string]string // original per-node seed colour, keyed by id — used in the canonical string so attributes (not just topology) are part of what "automorphism" preserves.

	leaves         int
	budgetExceeded bool

	bestCanon string
	bestOrder []string // set once, the order that produced bestCanon

	// firstOrderByCanon remembers one discrete order per canonical string
	// seen so far, so every later leaf with the same string can be turned
	// into a recorded automorphism against that representative (position i
	// of the representative's order <-> position i of this leaf's order)
	// without keeping every leaf.
	firstOrderByCanon map[string][]string

	generators []permutation
	uf         *unionFind
}

// ExactOrbits computes the automorphism group's orbits of a net's places and
// transitions, and a canonical form for the whole (coloured) net, by
// individualization-refinement (McKay & Piperno 2014; see this file's
// top-of-file comment for the algorithm and its pruning). It is EXACT: two
// nodes are reported in the same orbit only when a genuine automorphism of
// the coloured net (preserving every place's kind/initial marking/capacity,
// every transition's rate, every arc's direction/type/weight/kineticity, and
// every refine.* tag) has actually been exhibited between them.
//
// The colouring is seeded QUANTITATIVELY (refine(m, true)'s own seed, via
// buildIncidence) rather than structurally, because the use this exists for
// — settling "interchangeable" exactly — is itself a quantitative claim:
// two places that differ in rate or initial marking are not candidates for
// interchangeability regardless of wiring, and should not collapse into one
// orbit. A structural (wiring-only) variant would answer a different
// question (exact Roles rather than exact FungibleSets) and is not what
// classify.go's Role grouping needs, since Role is already the coarser,
// intentionally WL-only reading.
//
// It refuses (returning an error, never a wrong answer) once the search
// exceeds orbitSearchLeafBudget discrete leaves — see that constant's
// comment for what that guards against and what this repo's actual catalog
// costs. Every model in this repo's live catalog (vet-clinic, cafe,
// predator-prey, and the rest — see TestExactOrbitsOnCatalog) completes in
// well under that budget.
func ExactOrbits(m *metamodel.Model) (*OrbitReport, error) {
	nodes, seed, neighbours := buildIncidence(m, true)
	if len(nodes) == 0 {
		return &OrbitReport{Method: "automorphism-orbit/exact (individualization-refinement)"}, nil
	}

	s := &orbitSearch{
		nodes: nodes, neighbours: neighbours, seed: seed,
		firstOrderByCanon: map[string][]string{},
		uf:                newUnionFind(nodes),
	}
	start := wlRefine(nodes, neighbours, seed)
	s.search(start, nil)
	if s.budgetExceeded {
		return nil, fmt.Errorf(
			"orbits: individualization-refinement search over %d nodes exceeded its %d-leaf budget before completing; "+
				"this net is too large or too symmetric for this implementation's pruning (see orbitSearchLeafBudget)",
			len(nodes), orbitSearchLeafBudget)
	}

	out := &OrbitReport{
		Method:        "automorphism-orbit/exact (individualization-refinement, McKay & Piperno 2014)",
		CanonicalForm: s.bestCanon,
		Leaves:        s.leaves,
		Generators:    len(s.generators),
	}
	byRoot := map[string][]string{}
	for _, id := range nodes {
		r := s.uf.find(id)
		byRoot[r] = append(byRoot[r], id)
	}
	for _, members := range byRoot {
		if len(members) < 2 {
			continue
		}
		sort.Strings(members)
		out.Orbits = append(out.Orbits, members)
	}
	sort.Slice(out.Orbits, func(i, j int) bool { return out.Orbits[i][0] < out.Orbits[j][0] })
	return out, nil
}

// search runs one node of the individualization-refinement tree: refine
// `colour` to a fixpoint, and either record a discrete leaf or branch on the
// smallest non-singleton cell. path is every node individualized by an
// ancestor call, nearest-last, and is what fixesPath prunes against.
func (s *orbitSearch) search(colour map[string]string, path []string) {
	if s.budgetExceeded {
		return
	}
	colour = wlRefine(s.nodes, s.neighbours, colour)

	cells := map[string][]string{}
	for _, id := range s.nodes {
		cells[colour[id]] = append(cells[colour[id]], id)
	}

	// Target-cell selection must be a deterministic function of the
	// partition alone — never of Go's (randomized) map iteration order —
	// because two isomorphic branches (individualizing "the same" member of
	// two automorphic cells) have to make the SAME relative choice for their
	// resulting canonical strings to come out comparable. Colour strings are
	// themselves graph-invariant (wlRefine's hash never incorporates a node
	// id, only attributes and neighbour colours — see buildIncidence/
	// wlRefine), so breaking a size tie by the smaller colour string is a
	// structural, not an incidental, tie-break.
	var target []string
	var targetColour string
	for c, members := range cells {
		if len(members) < 2 {
			continue
		}
		if target == nil || len(members) < len(target) || (len(members) == len(target) && c < targetColour) {
			target, targetColour = members, c
		}
	}

	if target == nil {
		// Discrete: every cell is a singleton, so colour gives a total
		// order. Sort nodes by colour to get it (colours are unique here).
		order := append([]string(nil), s.nodes...)
		sort.Slice(order, func(i, j int) bool { return colour[order[i]] < colour[order[j]] })
		s.recordLeaf(order)
		return
	}
	sort.Strings(target)

	var tried []string
	for _, v := range target {
		if s.budgetExceeded {
			return
		}
		if s.prunedBy(path, tried, v) {
			continue
		}
		next := make(map[string]string, len(colour))
		for k, c := range colour {
			next[k] = c
		}
		// Individualize: give v a colour nothing else in its cell shares, so
		// the next wlRefine call is guaranteed to keep it a singleton and
		// propagate that distinction outward exactly as any other
		// structural difference would. The tag is a CONSTANT marker, never
		// v's own id: embedding the concrete id here would leak identity
		// into every downstream hash, which would make two branches that
		// individualize graph-automorphic members (e.g. two members of a
		// genuine orbit) hash differently for no structural reason — and
		// with it, silently break both automorphism detection (step 4) and
		// the canonical form (step 5), since the whole point of both is
		// that they must depend on structure and attributes alone.
		next[v] = next[v] + "#individualized"
		newPath := make([]string, len(path)+1)
		copy(newPath, path)
		newPath[len(path)] = v
		s.search(next, newPath)
		tried = append(tried, v)
	}
}

// prunedBy is the automorphism pruning described in this file's top comment:
// v is redundant if some already-recorded automorphism fixes the whole
// current path (so it applies unchanged at this exact point in the search)
// and maps v to (or from) some sibling already fully explored in this same
// loop.
func (s *orbitSearch) prunedBy(path, tried []string, v string) bool {
	for _, g := range s.generators {
		if !g.fixesPath(path) {
			continue
		}
		for _, u := range tried {
			if g.apply(u) == v || g.apply(v) == u {
				return true
			}
		}
	}
	return false
}

// recordLeaf handles one fully-discrete branch: build its canonical string,
// track the smallest one seen, and — if another leaf already produced this
// same string — derive and record the automorphism between them.
func (s *orbitSearch) recordLeaf(order []string) {
	s.leaves++
	if s.leaves > orbitSearchLeafBudget {
		s.budgetExceeded = true
		return
	}
	canon := s.canonicalString(order)
	if s.bestCanon == "" || canon < s.bestCanon {
		s.bestCanon, s.bestOrder = canon, order
	}
	if prior, ok := s.firstOrderByCanon[canon]; ok {
		s.recordAutomorphism(prior, order)
	} else {
		s.firstOrderByCanon[canon] = order
	}
}

// canonicalString writes order's own attributes and edge structure entirely
// in terms of POSITION in order (never by node id), so that two orders
// compare equal iff the position-for-position relabelling between them
// preserves every attribute and every labelled edge — exactly the definition
// of a colour-and-label-preserving graph automorphism.
func (s *orbitSearch) canonicalString(order []string) string {
	pos := make(map[string]int, len(order))
	for i, id := range order {
		pos[id] = i
	}
	var b strings.Builder
	for i, id := range order {
		if i > 0 {
			b.WriteByte(';')
		}
		b.WriteString(s.seed[id])
		edges := make([]string, 0, len(s.neighbours[id]))
		for _, n := range s.neighbours[id] {
			edges = append(edges, fmt.Sprintf("%s>%d", n.label, pos[n.node]))
		}
		sort.Strings(edges) // a multiset of incident edges, not a sequence
		b.WriteByte('|')
		b.WriteString(strings.Join(edges, ","))
	}
	return b.String()
}

// recordAutomorphism turns two discrete orders sharing a canonical string
// into the automorphism between them (position i of `from` maps to position
// i of `to`), records it for future pruning, and unions every moved pair
// into the orbit union-find. Both directions are recorded: an automorphism
// group is closed under inverse, and prunedBy checks both g.apply(u)==v and
// g.apply(v)==u, but recording the inverse explicitly keeps that check
// simple rather than implicit.
func (s *orbitSearch) recordAutomorphism(from, to []string) {
	g := permutation{}
	inv := permutation{}
	nontrivial := false
	for i := range from {
		a, b := from[i], to[i]
		if a != b {
			g[a] = b
			inv[b] = a
			nontrivial = true
			s.uf.union(a, b)
		}
	}
	if !nontrivial {
		return
	}
	s.generators = append(s.generators, g, inv)
}

// CanonicalModelID is an isomorphism-invariant identifier: the sha256 of the
// exact automorphism canonical form (ExactOrbits' CanonicalForm), hex
// encoded. It is DELIBERATELY SEPARATE from pkg/modelstore's own content
// address (sha256 of canonical JSON, unchanged by this file) rather than a
// replacement for it:
//
//   - modelstore's id answers "is this the same JSON I stored" — renaming a
//     place mints a new id, which is exactly right for versioning and for
//     Sheets links that point back at one specific stored document.
//   - CanonicalModelID answers "is this the same NET, up to relabelling" —
//     two models differing only by renaming places/transitions collapse to
//     one id here, which is what "does the catalog already have this net"
//     (ROADMAP.md Phase 6) actually needs to ask.
//
// Refuses (returns an error) under exactly the condition ExactOrbits does —
// see its doc comment and orbitSearchLeafBudget.
func CanonicalModelID(m *metamodel.Model) (string, error) {
	r, err := ExactOrbits(m)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(r.CanonicalForm))
	return hex.EncodeToString(sum[:]), nil
}

// The rest of this file wires ExactOrbits into classify.go's report, per the
// Phase 6 Gate: "every claim in a report carries its method, and a reader
// can tell proved from measured from assumed without reading the source."
// A shared colour that ExactOrbits can PROVE is one orbit is a strictly
// stronger claim than anything VerifyFungible's sampled experiment can make,
// so ClassesVerified prefers it outright; a colour ExactOrbits SPLITS is the
// over-approximation gap named in ROADMAP.md Phase 6, made visible in the
// evidence trail rather than fixed silently.

// sameOrbitOK is ClassesVerified's guard: true only when every member is
// provably one orbit. Kept as a named function (rather than inlining
// orbits.SameOrbit's two-value return at the call site) so that call site
// reads as a single boolean condition in the switch it lives in.
func sameOrbitOK(orbits *OrbitReport, members []string) bool {
	ok, _ := orbits.SameOrbit(members)
	return ok
}

// exactOrbitProof is the StructuralProof recorded when ExactOrbits confirms
// a WL-fungible candidate is a genuine automorphism orbit: interchangeable,
// proved, not sampled.
func exactOrbitProof(members []string) Evidence {
	return Evidence{
		Type:      StructuralProof,
		Algorithm: "automorphism-orbit/exact (individualization-refinement, McKay & Piperno 2014)",
		Establishes: "these places lie in ONE automorphism orbit of the coloured net: a permutation exists mapping " +
			"any one to any other while preserving every arc, arc type and weight, every place's kind/initial " +
			"marking/capacity, and every transition's rate",
		DoesNotEstablish: "what these places mean in the modelled world, or whether an operator could substitute one for another in practice",
		Summary: "proved by exact automorphism search (individualization-refinement), not sampled: " +
			strings.Join(members, ", ") + " are interchangeable (proved: automorphism)",
	}
}

// exactOrbitSplit is the StructuralProof recorded when ExactOrbits proves a
// WL-fungible candidate is NOT one orbit — the over-approximation
// ROADMAP.md Phase 6 names, surfaced explicitly rather than left implicit in
// a class that just happens to fail its experiment.
func exactOrbitSplit(members []string, groups [][]string) Evidence {
	return Evidence{
		Type:      StructuralProof,
		Algorithm: "automorphism-orbit/exact (individualization-refinement, McKay & Piperno 2014)",
		Establishes: fmt.Sprintf(
			"colour refinement (1-WL) placed %s in one class, but the exact automorphism group splits it into %s",
			strings.Join(members, ", "), formatGroups(groups)),
		DoesNotEstablish: "that any two members drawn from DIFFERENT groups above are interchangeable",
		Summary: fmt.Sprintf(
			"WL colour agreed but the exact orbit computation splits %s into %s — this is exactly the over-approximation 1-WL is known to make, not a contradiction",
			strings.Join(members, ", "), formatGroups(groups)),
	}
}

func formatGroups(groups [][]string) string {
	parts := make([]string, len(groups))
	for i, g := range groups {
		parts[i] = "{" + strings.Join(g, ",") + "}"
	}
	return strings.Join(parts, " vs ")
}

// orbitsUnavailable records that the exact search was not run to a
// conclusion (ExactOrbits refused: see orbitSearchLeafBudget) — a different
// report from either proof outcome above, and one the evidence trail must
// still carry rather than silently falling back to the older behaviour with
// no trace of why.
func orbitsUnavailable() Evidence {
	return Evidence{
		Type:      StructuralProof,
		Algorithm: "automorphism-orbit/exact (individualization-refinement, McKay & Piperno 2014)",
		Summary:   "the exact automorphism search was not run to completion for this net (see orbitSearchLeafBudget); falling back to colour-refinement plus the permutation experiment",
	}
}

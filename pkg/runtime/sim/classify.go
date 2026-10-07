package sim

// Ported from sim.pflow.xyz classify.go: the structural half (incidence,
// colour refinement, labelling). VerifyFungible, LumpingReport and
// sameTrajectory need sim's own engine and stay there.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

// Colour refinement (1-Weisfeiler-Leman) over the net, which is the principled
// form of the question "which places is this net unable to tell apart".
//
// # Why this replaced a hand-rolled signature
//
// The first version compared a fixed tuple — kind, initial marking, consumer
// count, producer count — and then required a shared name stem to call two
// places related. That is one round of this algorithm with an arbitrary
// feature vector, and it left naming as the deciding criterion, which is the
// part most likely to be wrong on a net somebody else wrote.
//
// Colour refinement instead recolours every node by its own colour plus the
// multiset of its neighbours' colours, repeatedly, until the partition stops
// changing — the coarsest stable partition, in near-linear time. It finds
// fork_0..fork_4 from the wiring alone. Names are demoted to what they should
// always have been: a label for a class that structure already found.
//
// # Two passes, because there are two questions
//
// Quantitative refinement seeds each node with everything that affects its
// dynamics — supply kind, initial marking, capacity, transition rate, arc
// weights and arc kinds. Nodes that survive with one colour are candidates for
// being *interchangeable*.
//
// Structural refinement drops the numbers and keeps the wiring. Nodes sharing
// a colour occupy the same position in the net but may carry different levels
// and rates: a role rather than a copy. This is the coarser grouping the
// fungible test cannot see — provider_avail and nurse_avail are both staff
// pools feeding services, which is a true and useful statement even though
// swapping them changes the answer.
//
// # What colour refinement does NOT establish
//
// 1-WL over-approximates isomorphism: two nodes can share a stable colour
// without any automorphism exchanging them. So a shared colour makes a class
// a *candidate*, and calling it fungible without checking would assert
// interchangeability the algorithm never proved. VerifyFungible settles it by
// experiment.

// arcLabel encodes an incident arc so refinement cannot confuse a read arc
// with a consuming one, or weight 1 with weight 20.
func arcLabel(a metamodel.Arc, out bool, quantitative bool) string {
	dir := "in"
	if out {
		dir = "out"
	}
	kind := string(a.Type)
	if kind == "" {
		kind = "consume"
	}
	if !quantitative {
		return dir + "/" + kind
	}
	w := a.Weight
	if w == 0 {
		w = 1
	}
	kinetic := "k"
	if !a.IsKinetic() {
		kinetic = "nk"
	}
	return fmt.Sprintf("%s/%s/%d/%s", dir, kind, w, kinetic)
}

// refineSeed is the human's contribution to the initial colouring: the tags
// whose keys are prefixed "refine.", sorted so the seed is order-independent.
//
// This is the whole splitting mechanism. Colour refinement computes the
// coarsest stable partition *refining* whatever it is seeded with, so a
// distinction a human draws here can never be undone by the algorithm — which
// is why adding a tag is monotone and needs no verification, while merging
// two classes the net separated is a claim and lives elsewhere.
//
// Monotone is not the same as local. Refinement propagates, so tagging one
// fork in a ring of five distinguishes all five: each becomes identifiable by
// its distance from the marked one. Nothing merges, but more than the tagged
// element may split, and a human refining a symmetric set should expect that.
//
// Only the prefixed keys take part. If every tag refined, labelling a place
// with an owner or a display colour would shatter every class it belongs to,
// and a classification that moves when someone writes down who maintains a
// resource is not one anybody can rely on.
func refineSeed(tags map[string]string) string {
	if len(tags) == 0 {
		return ""
	}
	var parts []string
	for k, v := range tags {
		if strings.HasPrefix(k, "refine.") {
			parts = append(parts, k+"="+v)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	sort.Strings(parts)
	return "|" + strings.Join(parts, ";")
}

// nbr is one labelled incident edge, as read off the arc structure by
// buildIncidence. It is a package-level type (rather than local to one
// function, as it was before orbits.go existed) because ExactOrbits'
// individualization-refinement search re-runs the same propagation rule
// (wlRefine) after individualizing a cell, and needs the same edge shape to
// do it — duplicating this as a second incidence reading is exactly the
// mistake CLAUDE.md's "the firing rule has one home" warns about, applied to
// colour refinement's own incidence structure instead of the firing rule.
type nbr struct{ label, node string }

// buildIncidence computes refine()'s STARTING colouring — every place and
// transition's own seed (kind/initial/capacity/rate plus any refine.* tag),
// before a single round of propagation — and the labelled neighbour lists
// that propagation reads. It is split out from refine() so a second
// algorithm (ExactOrbits, orbits.go) can reuse this exact colouring as ITS
// starting partition, per the roadmap's instruction to build the coloured
// graph "exactly the way refine() already colours it" rather than
// recolouring from scratch with a second, potentially-drifting
// implementation of arcLabel/refineSeed's rules.
func buildIncidence(m *metamodel.Model, quantitative bool) (nodes []string, seed map[string]string, neighbours map[string][]nbr) {
	rates := Rates(m)
	kinds := ClassifySupply(m)

	seed = map[string]string{}
	for _, p := range m.Places {
		if quantitative {
			// Capacity and initial level change what a place *is* for a
			// dynamics question, so they seed the colour. Without the initial
			// level a pool of two and a pool of nine refine together and the
			// class claims they are interchangeable.
			seed[p.ID] = fmt.Sprintf("P|%s|%d|%d", kinds[p.ID], int(p.Initial), p.Capacity)
		} else {
			seed[p.ID] = "P|" + string(kinds[p.ID])
		}
		// A human distinction is not a quantity, so it seeds both passes: it
		// separates roles as much as it separates copies.
		seed[p.ID] += refineSeed(p.Tags)
	}
	for _, t := range m.Transitions {
		if quantitative {
			// Two structurally identical services with different durations are
			// not the same parameter, so the rate seeds the colour.
			seed[t.ID] = fmt.Sprintf("T|%g", rates[t.ID])
		} else {
			seed[t.ID] = "T"
		}
		seed[t.ID] += refineSeed(t.Tags)
	}

	// Incidence, kept as labelled neighbours on both sides.
	neighbours = map[string][]nbr{}
	isPlace := map[string]bool{}
	for _, p := range m.Places {
		isPlace[p.ID] = true
	}
	for _, a := range m.Arcs {
		if isPlace[a.From] {
			neighbours[a.From] = append(neighbours[a.From], nbr{arcLabel(a, true, quantitative), a.To})
			neighbours[a.To] = append(neighbours[a.To], nbr{arcLabel(a, false, quantitative), a.From})
		} else {
			neighbours[a.From] = append(neighbours[a.From], nbr{arcLabel(a, true, quantitative), a.To})
			neighbours[a.To] = append(neighbours[a.To], nbr{arcLabel(a, false, quantitative), a.From})
		}
	}

	nodes = make([]string, 0, len(seed))
	for id := range seed {
		nodes = append(nodes, id)
	}
	sort.Strings(nodes)
	return nodes, seed, neighbours
}

// wlRefine iterates 1-Weisfeiler-Leman colour refinement to a fixpoint from
// a GIVEN starting colouring, over a given labelled-neighbour structure. It
// is refine()'s propagation loop, factored out so ExactOrbits (orbits.go)
// can re-run the identical rule after individualizing one cell, rather than
// carrying a second, hand-rolled copy of "recolour by self plus the sorted
// multiset of labelled neighbour colours, hash, repeat to a fixpoint" that
// could quietly drift from this one.
//
// It does not mutate the caller's colour map: each round builds a fresh
// map and only the local variable is reassigned.
func wlRefine(nodes []string, neighbours map[string][]nbr, colour map[string]string) map[string]string {
	// Iterate to a fixpoint. The partition can refine at most len(nodes)
	// times, and in practice stabilises in a handful of rounds.
	prev := countClasses(colour)
	for round := 0; round < len(nodes)+1; round++ {
		next := make(map[string]string, len(colour))
		for _, id := range nodes {
			parts := make([]string, 0, len(neighbours[id]))
			for _, n := range neighbours[id] {
				parts = append(parts, n.label+":"+colour[n.node])
			}
			sort.Strings(parts) // a multiset, not a sequence
			sum := sha256.Sum256([]byte(colour[id] + "||" + strings.Join(parts, ",")))
			next[id] = hex.EncodeToString(sum[:8])
		}
		colour = next
		if n := countClasses(colour); n == prev {
			break // stable: no class split this round
		} else {
			prev = n
		}
	}
	return colour
}

// refine runs colour refinement to a stable partition and returns the final
// colour of every place and transition.
func refine(m *metamodel.Model, quantitative bool) map[string]string {
	nodes, seed, neighbours := buildIncidence(m, quantitative)
	return wlRefine(nodes, neighbours, seed)
}

func countClasses(colour map[string]string) int {
	seen := map[string]bool{}
	for _, c := range colour {
		seen[c] = true
	}
	return len(seen)
}

// groupBy inverts a colouring into classes over the given ids, dropping
// singletons — a class of one is just an object.
func groupBy(ids []string, colour map[string]string) [][]string {
	byColour := map[string][]string{}
	for _, id := range ids {
		byColour[colour[id]] = append(byColour[colour[id]], id)
	}
	var out [][]string
	for _, members := range byColour {
		if len(members) < 2 {
			continue
		}
		sort.Strings(members)
		out = append(out, members)
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i]) != len(out[j]) {
			return len(out[i]) > len(out[j])
		}
		return out[i][0] < out[j][0]
	})
	return out
}

// label names a class from what its members share. Naming is cosmetic here:
// structure already decided the class, and a name that does not factor cleanly
// costs nothing but a duller label.
func label(members []string) string {
	parts := nameParts(members[0])
	for _, m := range members[1:] {
		p := nameParts(m)
		if len(p) != len(parts) {
			return strings.Join(members, "+")
		}
		for i := range parts {
			if i < len(p) && parts[i] != p[i] {
				parts[i] = "*"
			}
		}
	}
	joined := strings.Join(parts, "_")
	if strings.Trim(joined, "_*") == "" {
		return strings.Join(members, "+")
	}
	return joined
}

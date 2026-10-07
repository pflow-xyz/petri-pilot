package sim_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/pflow-xyz/go-pflow/metamodel"

	"github.com/pflow-xyz/petri-pilot/generated/cafe"
	"github.com/pflow-xyz/petri-pilot/pkg/runtime/sim"
)

func arc(from, to string, typ metamodel.ArcType) metamodel.Arc {
	return metamodel.Arc{From: from, To: to, Type: typ}
}

func tokenPlace(id string, initial int) metamodel.Place {
	return metamodel.Place{ID: id, Initial: initial}
}

func trans(id string) metamodel.Transition {
	return metamodel.Transition{ID: id}
}

// pipelineModel is stock -> wip -> done, a straight-line flow with no
// replenishment of stock and no drain on done.
//
// By inspection:
//   - {stock} is the unique minimal siphon: nothing produces into stock
//     (•{stock} = {}), so •{stock} ⊆ {stock}'s postset ({start}) holds no
//     matter what the postset is (the empty set is a subset of everything).
//     {wip} alone fails (•{wip}={start}, postset={finish}, start∉{finish})
//     and {done} alone fails the SIPHON test the other way (•{done}={finish},
//     postset={} — {finish}⊄{}).
//   - {done} is the unique minimal trap, by the dual argument: nothing drains
//     done (done's postset = {}), so postset ⊆ •{done} holds trivially.
func pipelineModel(stockInitial int) *metamodel.Model {
	return &metamodel.Model{
		Name:        "pipeline",
		Places:      []metamodel.Place{tokenPlace("stock", stockInitial), tokenPlace("wip", 0), tokenPlace("done", 0)},
		Transitions: []metamodel.Transition{trans("start"), trans("finish")},
		Arcs: []metamodel.Arc{
			arc("stock", "start", ""),
			arc("start", "wip", ""),
			arc("wip", "finish", ""),
			arc("finish", "done", ""),
		},
	}
}

// twoCycleModel is A -> t1 -> B -> t2 -> A, the minimal case where NEITHER
// place alone is a siphon or a trap but the pair is both.
//
// By inspection: •{A}={t2} (t2 produces A), postset of {A}={t1} (t1 needs A);
// t2 ∉ {t1}, so {A} fails on its own, and {B} fails symmetrically. But
// •{A,B}={t1,t2} (t1 produces B, t2 produces A) and postset{A,B}={t1,t2} (t1
// needs A, t2 needs B) — equal sets, so {A,B} is both a siphon and a trap,
// and it is minimal because no smaller nonempty subset works.
func twoCycleModel(aInitial, bInitial int) *metamodel.Model {
	return &metamodel.Model{
		Name:        "two-cycle",
		Places:      []metamodel.Place{tokenPlace("A", aInitial), tokenPlace("B", bInitial)},
		Transitions: []metamodel.Transition{trans("t1"), trans("t2")},
		Arcs: []metamodel.Arc{
			arc("A", "t1", ""),
			arc("t1", "B", ""),
			arc("B", "t2", ""),
			arc("t2", "A", ""),
		},
	}
}

func joinSorted(p []string) string {
	cp := append([]string(nil), p...)
	sort.Strings(cp)
	return "[" + strings.Join(cp, " ") + "]"
}

func placeSetStrings(sets []sim.PlaceSet) []string {
	out := make([]string, 0, len(sets))
	for _, s := range sets {
		p := append([]string(nil), s.Places...)
		sort.Strings(p)
		out = append(out, joinSorted(p)) // stable, comparable rendering
	}
	sort.Strings(out)
	return out
}

func wantSets(members ...[]string) []string {
	out := make([]string, 0, len(members))
	for _, m := range members {
		p := append([]string(nil), m...)
		sort.Strings(p)
		out = append(out, joinSorted(p))
	}
	sort.Strings(out)
	return out
}

func assertSets(t *testing.T, label string, got []sim.PlaceSet, want ...[]string) {
	t.Helper()
	g, w := placeSetStrings(got), wantSets(want...)
	if len(g) != len(w) {
		t.Fatalf("%s: got %v, want %v", label, g, w)
	}
	for i := range g {
		if g[i] != w[i] {
			t.Fatalf("%s: got %v, want %v", label, g, w)
		}
	}
}

func TestPipelineMinimalSiphonAndTrap(t *testing.T) {
	rep := sim.SiphonsAndTraps(pipelineModel(2))
	if !rep.Applicable {
		t.Fatalf("refused: %v", rep.Refusals)
	}
	assertSets(t, "siphons", rep.MinimalSiphons, []string{"stock"})
	assertSets(t, "traps", rep.MinimalTraps, []string{"done"})
	if len(rep.DeadlockWitnesses) != 0 {
		t.Fatalf("stock starts marked (2 tokens): want no deadlock witness, got %+v", rep.DeadlockWitnesses)
	}
}

// TestPipelineUnmarkedSiphonIsADeadlockWitness is the structural half of the
// roadmap ask: stock starting at zero is provable-without-simulation as
// "start can never fire again", since {stock} is a siphon and it is empty at
// the initial marking.
func TestPipelineUnmarkedSiphonIsADeadlockWitness(t *testing.T) {
	rep := sim.SiphonsAndTraps(pipelineModel(0))
	if !rep.Applicable {
		t.Fatalf("refused: %v", rep.Refusals)
	}
	if len(rep.DeadlockWitnesses) != 1 {
		t.Fatalf("want exactly one deadlock witness, got %d: %+v", len(rep.DeadlockWitnesses), rep.DeadlockWitnesses)
	}
	w := rep.DeadlockWitnesses[0]
	if got := joinSorted(w.Siphon); got != "[stock]" {
		t.Errorf("siphon = %v, want [stock]", w.Siphon)
	}
	if got := joinSorted(w.UnfireableTransitions); got != "[start]" {
		t.Errorf("unfireable = %v, want [start]", w.UnfireableTransitions)
	}
}

func TestTwoCycleMinimalSiphonAndTrapRequireBothPlaces(t *testing.T) {
	rep := sim.SiphonsAndTraps(twoCycleModel(1, 0))
	if !rep.Applicable {
		t.Fatalf("refused: %v", rep.Refusals)
	}
	assertSets(t, "siphons", rep.MinimalSiphons, []string{"A", "B"})
	assertSets(t, "traps", rep.MinimalTraps, []string{"A", "B"})
	if len(rep.DeadlockWitnesses) != 0 {
		t.Fatalf("A starts marked: want no deadlock witness, got %+v", rep.DeadlockWitnesses)
	}
}

func TestTwoCycleBothEmptyIsADeadlockWitness(t *testing.T) {
	rep := sim.SiphonsAndTraps(twoCycleModel(0, 0))
	if !rep.Applicable {
		t.Fatalf("refused: %v", rep.Refusals)
	}
	if len(rep.DeadlockWitnesses) != 1 {
		t.Fatalf("want exactly one deadlock witness, got %d", len(rep.DeadlockWitnesses))
	}
	w := rep.DeadlockWitnesses[0]
	if got := joinSorted(w.UnfireableTransitions); got != "[t1 t2]" {
		t.Errorf("unfireable = %v, want [t1 t2]", w.UnfireableTransitions)
	}
}

// ---------------------------------------------------------------------------
// Brute-force cross-check: a second, deliberately naive implementation that
// enumerates every nonempty subset of places directly against the •S ⊆ S•
// (or dual) definition, using the model's own Inputs/Outputs/Tests rather
// than sim's bitmask machinery. Used only here, never in production code.
// ---------------------------------------------------------------------------

func bruteForcePreset(m *metamodel.Model, s map[string]bool) map[string]bool {
	out := map[string]bool{}
	for i := range m.Transitions {
		t := m.Transitions[i].ID
		for _, o := range m.Outputs(t) {
			if s[o.Place] {
				out[t] = true
			}
		}
	}
	return out
}

func bruteForcePostset(m *metamodel.Model, s map[string]bool) map[string]bool {
	out := map[string]bool{}
	for i := range m.Transitions {
		t := m.Transitions[i].ID
		for _, in := range m.Inputs(t) {
			if s[in.Place] {
				out[t] = true
			}
		}
		for _, tst := range m.Tests(t) {
			if tst.Type == metamodel.InhibitorArc {
				continue
			}
			if s[tst.Place] {
				out[t] = true
			}
		}
	}
	return out
}

func subsetOf(a, b map[string]bool) bool {
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func bruteForceIsSiphon(m *metamodel.Model, s map[string]bool) bool {
	if len(s) == 0 {
		return false
	}
	return subsetOf(bruteForcePreset(m, s), bruteForcePostset(m, s))
}

func bruteForceIsTrap(m *metamodel.Model, s map[string]bool) bool {
	if len(s) == 0 {
		return false
	}
	return subsetOf(bruteForcePostset(m, s), bruteForcePreset(m, s))
}

// bruteForceMinimal enumerates ALL 2^n subsets of the given places (n must be
// small — this is exponential on purpose, as the naive reference) and
// returns those satisfying test that have no proper nonempty subset also
// satisfying it.
func bruteForceMinimal(places []string, test func(map[string]bool) bool) [][]string {
	n := len(places)
	var all [][]string
	satisfies := map[string]bool{}
	subsetPlaces := make(map[string][]string)
	for mask := 1; mask < (1 << n); mask++ {
		s := map[string]bool{}
		var members []string
		for i := 0; i < n; i++ {
			if mask&(1<<i) != 0 {
				s[places[i]] = true
				members = append(members, places[i])
			}
		}
		key := joinSorted(members)
		if test(s) {
			satisfies[key] = true
			subsetPlaces[key] = members
			all = append(all, members)
		}
	}
	var minimal [][]string
	for _, members := range all {
		isMinimal := true
		for _, other := range all {
			if len(other) < len(members) && isSubsetOfMembers(other, members) {
				isMinimal = false
				break
			}
		}
		if isMinimal {
			minimal = append(minimal, members)
		}
	}
	return minimal
}

func isSubsetOfMembers(sub, super []string) bool {
	set := map[string]bool{}
	for _, p := range super {
		set[p] = true
	}
	for _, p := range sub {
		if !set[p] {
			return false
		}
	}
	return true
}

func toSortedStrings(sets [][]string) []string {
	out := make([]string, 0, len(sets))
	for _, s := range sets {
		p := append([]string(nil), s...)
		sort.Strings(p)
		out = append(out, joinSorted(p))
	}
	sort.Strings(out)
	return out
}

// combinedModel unions the pipeline and two-cycle nets as one net sharing no
// places, so a correct search must find BOTH minimal siphons ({stock} and
// {A,B}) and both minimal traps ({done} and {A,B}) without conflating the two
// disconnected components.
func combinedModel() *metamodel.Model {
	p, c := pipelineModel(2), twoCycleModel(1, 0)
	return &metamodel.Model{
		Name:        "combined",
		Places:      append(append([]metamodel.Place{}, p.Places...), c.Places...),
		Transitions: append(append([]metamodel.Transition{}, p.Transitions...), c.Transitions...),
		Arcs:        append(append([]metamodel.Arc{}, p.Arcs...), c.Arcs...),
	}
}

func TestPrunedSearchMatchesBruteForceOnHandBuiltNets(t *testing.T) {
	for _, m := range []*metamodel.Model{pipelineModel(2), twoCycleModel(1, 0), combinedModel()} {
		var places []string
		for i := range m.Places {
			places = append(places, m.Places[i].ID)
		}
		wantSiphons := toSortedStrings(bruteForceMinimal(places, func(s map[string]bool) bool { return bruteForceIsSiphon(m, s) }))
		wantTraps := toSortedStrings(bruteForceMinimal(places, func(s map[string]bool) bool { return bruteForceIsTrap(m, s) }))

		rep := sim.SiphonsAndTraps(m)
		if !rep.Applicable {
			t.Fatalf("%s: refused: %v", m.Name, rep.Refusals)
		}
		gotSiphons := placeSetStrings(rep.MinimalSiphons)
		gotTraps := placeSetStrings(rep.MinimalTraps)

		if !equalStrings(gotSiphons, wantSiphons) {
			t.Errorf("%s: minimal siphons = %v, brute force says %v", m.Name, gotSiphons, wantSiphons)
		}
		if !equalStrings(gotTraps, wantTraps) {
			t.Errorf("%s: minimal traps = %v, brute force says %v", m.Name, gotTraps, wantTraps)
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestSiphonSearchOnCatalog runs the search against every real catalog model
// this fork ships and asserts only that it completes cleanly (Applicable, or
// a named refusal) — the point being no hang and no panic on the actual
// vet-clinic/café/predator-prey-sized nets this fork serves, not a claim
// about what those nets' siphons mean.
func TestSiphonSearchOnCatalog(t *testing.T) {
	for name, m := range loadCatalogModels(t) {
		rep := sim.SiphonsAndTraps(m)
		if !rep.Applicable && len(rep.Refusals) == 0 {
			t.Errorf("%s: not applicable but no refusal given", name)
		}
	}
}

// TestSiphonSearchCompletesOnNamedBenchmarks pins the node budget against a
// real regression: an earlier, ten-times-smaller budget refused vet-clinic
// and café outright (both are single dense 16-38 place components, so the
// per-component decomposition alone does not shrink them). These are the
// models this task named explicitly, so unlike the general catalog sweep
// above (which tolerates a refusal), this asserts Applicable outright — a
// budget regression here should fail loudly, not degrade into a silent
// "could not complete" on exactly the nets this exists for.
func TestSiphonSearchCompletesOnNamedBenchmarks(t *testing.T) {
	for _, name := range []string{"vet-clinic.json", "predator-prey.json"} {
		m := loadModel(t, name)
		rep := sim.SiphonsAndTraps(m)
		if !rep.Applicable {
			t.Errorf("%s: search refused: %v", name, rep.Refusals)
		}
	}
	if rep := sim.SiphonsAndTraps(cafe.FlatModel()); !rep.Applicable {
		t.Errorf("cafe: search refused: %v", rep.Refusals)
	}
}

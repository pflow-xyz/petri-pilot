package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"
)

// gatedModel: a licence that must be present to serve (read arc), a closed sign
// that stops service (inhibitor), and a hopper that cannot be overfilled.
func gatedModel() *goflowmetamodel.Model {
	return &goflowmetamodel.Model{
		Name: "gated",
		Places: []goflowmetamodel.Place{
			{ID: "orders", Initial: 10},
			{ID: "done"},
			{ID: "licence", Initial: 1},
			{ID: "closed_sign"},
			{ID: "hopper", Capacity: 3},
		},
		Transitions: []goflowmetamodel.Transition{
			{ID: "serve", Rate: 5},
			{ID: "refill", Rate: 5},
		},
		Arcs: []goflowmetamodel.Arc{
			{From: "orders", To: "serve"},
			{From: "serve", To: "done"},
			{From: "licence", To: "serve", Type: goflowmetamodel.ReadArc},
			{From: "closed_sign", To: "serve", Type: goflowmetamodel.InhibitorArc},
			{From: "refill", To: "hopper"},
		},
	}
}

// TestBuildOdeNetDoesNotConsumeWhatItOnlyTests is the regression test for the
// builder twelve analytic tools share.
//
// Every arc used to become a consuming arc, so a read arc drained the place it
// was only checking and an inhibitor — which blocks a firing — became a source
// feeding it. Nothing compared the tools against the firing rule, so both bugs
// were invisible in every chart they produced.
func TestBuildOdeNetDoesNotConsumeWhatItOnlyTests(t *testing.T) {
	net := buildOdeNet(gatedModel())

	var sawInhibitor bool
	for _, a := range net.Arcs {
		if a.Source == "licence" || a.Target == "licence" {
			t.Errorf("the read arc on licence reached the ODE net as %+v; it moves no tokens and must be dropped", a)
		}
		if a.Source == "closed_sign" || a.Target == "closed_sign" {
			if !a.InhibitTransition {
				t.Errorf("the inhibitor on closed_sign became an ordinary arc %+v — it would feed the transition it blocks", a)
			}
			sawInhibitor = true
		}
	}
	if !sawInhibitor {
		t.Error("the inhibitor arc vanished entirely; it should be present and marked")
	}

	// Capacity travels with the place rather than being silently dropped.
	for _, p := range net.Places {
		if p.Label == "hopper" && (len(p.Capacity) == 0 || p.Capacity[0] != 3) {
			t.Errorf("hopper reached the net with capacity %v, want [3]", p.Capacity)
		}
	}
}

// TestCaveatsAreAddedOnlyForGatedModels: the note has to appear where it
// matters and be invisible everywhere else, or every existing chart changes.
func TestCaveatsAreAddedOnlyForGatedModels(t *testing.T) {
	plain := &goflowmetamodel.Model{
		Name:        "plain",
		Places:      []goflowmetamodel.Place{{ID: "a", Initial: 5}, {ID: "b"}},
		Transitions: []goflowmetamodel.Transition{{ID: "flow"}},
		Arcs:        []goflowmetamodel.Arc{{From: "a", To: "flow"}, {From: "flow", To: "b"}},
	}
	text := []byte(`{"final":{"a":0,"b":5}}`)
	if got := string(withCaveats(text, plain)); got != string(text) {
		t.Errorf("an unconstrained model's output changed:\n got %s\nwant %s", got, text)
	}

	out := withCaveats(text, gatedModel())
	var summary map[string]any
	if err := json.Unmarshal(out, &summary); err != nil {
		t.Fatalf("caveats broke the JSON: %v\n%s", err, out)
	}
	caveats, ok := summary["caveats"].([]any)
	if !ok || len(caveats) == 0 {
		t.Fatalf("no caveats on a gated model: %s", out)
	}
	joined := strings.ToLower(strings.Join(func() []string {
		var s []string
		for _, c := range caveats {
			s = append(s, c.(string))
		}
		return s
	}(), " "))
	for _, want := range []string{"read arc", "inhibitor", "capacity"} {
		if !strings.Contains(joined, want) {
			t.Errorf("caveats do not mention %q: %v", want, caveats)
		}
	}
	// The original payload survives alongside the note.
	if summary["final"] == nil {
		t.Errorf("withCaveats dropped the tool's own summary: %s", out)
	}
}

// TestStochasticHonoursGating: the discrete engine has a firing instant, so
// unlike the ODE it can enforce all of this exactly. It simply was not.
func TestStochasticHonoursGating(t *testing.T) {
	m := gatedModel()
	places := []string{}
	idx := map[string]int{}
	for _, p := range m.Places {
		idx[p.ID] = len(places)
		places = append(places, p.ID)
	}
	rates := map[string]float64{"serve": 5, "refill": 5}

	t.Run("read arc gates and is not consumed", func(t *testing.T) {
		entries := buildTransitionEntries(m, idx, rates)
		serve := entryByID(t, entries, "serve")
		for _, in := range serve.inputs {
			if places[in.placeIdx] == "licence" {
				t.Fatal("licence is an input to serve; a read arc must not be consumed")
			}
		}
		if len(serve.reads) != 1 || places[serve.reads[0].placeIdx] != "licence" {
			t.Fatalf("serve does not read licence: %+v", serve.reads)
		}

		marking := make([]int, len(places))
		marking[idx["licence"]] = 0
		if serve.gated(marking) {
			t.Error("serve is enabled with no licence")
		}
		marking[idx["licence"]] = 1
		if !serve.gated(marking) {
			t.Error("serve is blocked with a licence present")
		}
	})

	t.Run("inhibitor blocks rather than feeds", func(t *testing.T) {
		entries := buildTransitionEntries(m, idx, rates)
		serve := entryByID(t, entries, "serve")
		for _, out := range serve.outputs {
			if places[out.placeIdx] == "closed_sign" {
				t.Fatal("closed_sign is an output of serve; an inhibitor is not a production")
			}
		}
		marking := make([]int, len(places))
		marking[idx["licence"]] = 1
		marking[idx["closed_sign"]] = 1
		if serve.gated(marking) {
			t.Error("serve fired while the shop is closed")
		}
	})

	t.Run("capacity is a post-firing bound", func(t *testing.T) {
		entries := buildTransitionEntries(m, idx, rates)
		refill := entryByID(t, entries, "refill")
		marking := make([]int, len(places))
		marking[idx["hopper"]] = 3
		if refill.gated(marking) {
			t.Error("refill would push the hopper over its capacity")
		}
		marking[idx["hopper"]] = 2
		if !refill.gated(marking) {
			t.Error("refill blocked with room to spare")
		}
	})
}

func entryByID(t *testing.T, entries []transitionEntry, id string) *transitionEntry {
	t.Helper()
	for i := range entries {
		if entries[i].id == id {
			return &entries[i]
		}
	}
	t.Fatalf("no transition %q", id)
	return nil
}

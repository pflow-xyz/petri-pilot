package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
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

// TestStochasticHonoursGating is a black-box regression test through the
// petri_stochastic tool itself, not through the engine's internals: the
// engine is go-pflow's stochastic package via pkg/runtime/sim (see
// stochastic.go), and its gating is already pinned at the algorithm level by
// go-pflow's own gating_test.go/kinetic_test.go and by
// pkg/runtime/sim.TestSSAAgreesWithTheSharedRule. What this test protects is
// the wiring at this call site — that handleStochastic actually reaches that
// engine and does not silently fall back to some other rule.
func TestStochasticHonoursGating(t *testing.T) {
	variant := func(licence, closedSign, hopperInitial int) *goflowmetamodel.Model {
		m := gatedModel()
		for i := range m.Places {
			switch m.Places[i].ID {
			case "licence":
				m.Places[i].Initial = licence
			case "closed_sign":
				m.Places[i].Initial = closedSign
			case "hopper":
				m.Places[i].Initial = hopperInitial
			}
		}
		return m
	}

	run := func(t *testing.T, m *goflowmetamodel.Model) stochasticResponse {
		t.Helper()
		req := mcp.CallToolRequest{}
		req.Params.Name = "petri_stochastic"
		req.Params.Arguments = map[string]any{
			"model":   mustJSON(t, m),
			"rates":   `{"serve": 5, "refill": 5}`,
			"tspan":   "[0, 5]",
			"samples": 60,
			"seed":    42,
		}
		res, err := handleStochastic(context.Background(), req)
		if err != nil {
			t.Fatalf("handleStochastic: %v", err)
		}
		if res.IsError {
			t.Fatalf("error: %s", textBlock(t, res))
		}
		var resp stochasticResponse
		if err := json.Unmarshal([]byte(textBlock(t, res)), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return resp
	}

	t.Run("read arc gates and is not consumed", func(t *testing.T) {
		without := run(t, variant(0, 0, 0))
		for _, v := range without.Mean["done"] {
			if v != 0 {
				t.Fatalf("serve fired with no licence: done = %v", without.Mean["done"])
			}
		}
		for _, v := range without.Mean["licence"] {
			if v != 0 {
				t.Fatalf("a read arc consumed licence: %v", without.Mean["licence"])
			}
		}

		with := run(t, variant(1, 0, 0))
		last := with.Mean["done"][len(with.Mean["done"])-1]
		if last == 0 {
			t.Fatal("serve never fired with a licence present")
		}
		for _, v := range with.Mean["licence"] {
			if v != 1 {
				t.Fatalf("a read arc consumed licence: %v", with.Mean["licence"])
			}
		}
	})

	t.Run("inhibitor blocks rather than feeds", func(t *testing.T) {
		blocked := run(t, variant(1, 1, 0))
		for _, v := range blocked.Mean["done"] {
			if v != 0 {
				t.Fatalf("serve fired while the shop is closed: done = %v", blocked.Mean["done"])
			}
		}
		for _, v := range blocked.Mean["closed_sign"] {
			if v != 1 {
				t.Fatalf("the inhibitor fed the transition it blocks: closed_sign = %v", blocked.Mean["closed_sign"])
			}
		}
	})

	t.Run("capacity is a post-firing bound", func(t *testing.T) {
		resp := run(t, variant(0, 0, 0))
		for _, v := range resp.Mean["hopper"] {
			if v > 3 {
				t.Fatalf("hopper exceeded its capacity of 3: %v", resp.Mean["hopper"])
			}
		}
		last := resp.Mean["hopper"][len(resp.Mean["hopper"])-1]
		if last < 3 {
			t.Fatalf("refill stopped well short of the capacity it could still fill: hopper = %v", last)
		}
	})
}

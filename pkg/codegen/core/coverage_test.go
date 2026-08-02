package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	metamodel "github.com/pflow-xyz/go-pflow/metamodel"
)

// batchLine is a second fixture exercising everything the coffee machine
// does not: arc weights > 1, a place->transition inhibitor, a bounded place
// with capacity > 1, multi-token markings (the ":count" render), and a
// deadlock that is not a single-token marking.
//
//	Hopper(3) --1--> Load --1--> Batch(cap 2)
//	Batch --2--> Flush --1--> Done
//	Done --inhibitor--> Load        (stop loading once one batch is done)
func batchLine() *metamodel.Model {
	return &metamodel.Model{
		Name: "BatchLine",
		Places: []metamodel.Place{
			{ID: "Hopper", Initial: 3},
			{ID: "Batch", Initial: 0, Capacity: 2},
			{ID: "Done", Initial: 0},
		},
		Transitions: []metamodel.Transition{{ID: "Load"}, {ID: "Flush"}},
		Arcs: []metamodel.Arc{
			{From: "Hopper", To: "Load"},
			{From: "Load", To: "Batch"},
			{From: "Batch", To: "Flush", Weight: 2},
			{From: "Flush", To: "Done"},
			{From: "Done", To: "Load", Type: metamodel.InhibitorArc},
		},
	}
}

// batchGolden is the batchLine trace under the greedy sorted driver
// (Flush before Load), worked out by hand:
//
//	round 1: Flush blocked (Batch < 2), Load fires -> Batch=1 Hopper=2
//	round 2: Flush blocked,             Load fires -> Batch=2 Hopper=1
//	round 3: Flush fires -> Done=1 Hopper=1
//	round 4: Flush blocked, Load inhibited by Done -> deadlock
const batchGolden = `Step #1: Load => Batch,Hopper:2
Step #2: Load => Batch:2,Hopper
Step #3: Flush => Done,Hopper
`

// TestExecutionParityWeighted runs the weighted/inhibited fixture through
// every form x language cell. Same short-mode guard as TestExecutionParity.
func TestExecutionParityWeighted(t *testing.T) {
	if testing.Short() {
		t.Skip("execution parity needs system toolchains; skipped in short mode")
	}
	dir := t.TempDir()

	runners := map[string]func(t *testing.T, src string) *exec.Cmd{
		"go":         func(_ *testing.T, src string) *exec.Cmd { return exec.Command("go", "run", src) },
		"python":     func(_ *testing.T, src string) *exec.Cmd { return exec.Command("python3", src) },
		"javascript": func(_ *testing.T, src string) *exec.Cmd { return exec.Command("node", src) },
		"lean":       func(_ *testing.T, src string) *exec.Cmd { return exec.Command("lean", "--run", src) },
		"rust": func(t *testing.T, src string) *exec.Cmd {
			bin := strings.TrimSuffix(src, ".rs")
			if out, err := exec.Command("rustc", "-O", "-o", bin, src).CombinedOutput(); err != nil {
				t.Fatalf("rustc: %v\n%s", err, out)
			}
			return exec.Command(bin)
		},
	}
	tools := map[string]string{"go": "go", "python": "python3", "javascript": "node", "rust": "rustc", "lean": "lean"}

	for lang, runner := range runners {
		for _, form := range Languages[lang].Forms {
			t.Run(lang+"/"+form, func(t *testing.T) {
				if _, err := exec.LookPath(tools[lang]); err != nil {
					t.Skipf("no %s on PATH", tools[lang])
				}

				files, err := Generate(batchLine(), Options{Language: lang, Form: form, PackageName: "main"})
				if err != nil {
					t.Fatal(err)
				}
				src := filepath.Join(dir, files[0].Name)
				if err := os.WriteFile(src, files[0].Content, 0o644); err != nil {
					t.Fatal(err)
				}

				out, err := runner(t, src).Output()
				if err != nil {
					if ee, ok := err.(*exec.ExitError); ok {
						t.Fatalf("running %s/%s: %v\nstderr:\n%s", lang, form, err, ee.Stderr)
					}
					t.Fatalf("running %s/%s: %v", lang, form, err)
				}
				if string(out) != batchGolden {
					t.Errorf("%s/%s trace mismatch:\n--- want\n%s--- got\n%s", lang, form, batchGolden, out)
				}
			})
		}
	}
}

func TestModelCheckBatchFindings(t *testing.T) {
	model, err := Normalize(batchLine(), "")
	if err != nil {
		t.Fatal(err)
	}
	f, err := ModelCheck(model)
	if err != nil {
		t.Fatal(err)
	}
	// Places sorted: Batch, Done, Hopper. Deadlock worked out by hand:
	// the run above ends at Batch=0 Done=1 Hopper=1, and the only other
	// maximal behavior differences still funnel to the same stop.
	if len(f.Deadlocks) == 0 {
		t.Fatal("expected at least one deadlock")
	}
	if f.Bounds[0] != 2 {
		t.Errorf("Batch bound = %d, want 2", f.Bounds[0])
	}
	if f.Bounds[2] != 3 {
		t.Errorf("Hopper bound = %d, want 3 (initial)", f.Bounds[2])
	}
	if f.Tactic != "decide" {
		t.Errorf("tactic = %q, want decide for a small net", f.Tactic)
	}
}

// A net with 2^9 = 512 reachable markings (nine independent toggles) crosses
// the 128-state line: the emitted tactic must be native_decide.
func TestModelCheckLargeNetUsesNativeDecide(t *testing.T) {
	m := &metamodel.Model{Name: "toggles"}
	for _, suffix := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"} {
		m.Places = append(m.Places,
			metamodel.Place{ID: "off_" + suffix, Initial: 1, Capacity: 1},
			metamodel.Place{ID: "on_" + suffix, Initial: 0, Capacity: 1},
		)
		m.Transitions = append(m.Transitions, metamodel.Transition{ID: "flip_" + suffix})
		m.Arcs = append(m.Arcs,
			metamodel.Arc{From: "off_" + suffix, To: "flip_" + suffix},
			metamodel.Arc{From: "flip_" + suffix, To: "on_" + suffix},
		)
	}
	model, err := Normalize(m, "")
	if err != nil {
		t.Fatal(err)
	}
	f, err := ModelCheck(model)
	if err != nil {
		t.Fatal(err)
	}
	if f.StateCount != 512 {
		t.Errorf("state count = %d, want 512", f.StateCount)
	}
	if f.Tactic != "native_decide" {
		t.Errorf("tactic = %q, want native_decide past 128 states", f.Tactic)
	}
}

// Thirteen independent toggles is 8192 markings — past maxStates. ModelCheck
// must refuse rather than emit a search the kernel cannot finish.
func TestModelCheckRejectsHugeStateSpace(t *testing.T) {
	m := &metamodel.Model{Name: "huge"}
	for _, s := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m"} {
		m.Places = append(m.Places,
			metamodel.Place{ID: "off_" + s, Initial: 1, Capacity: 1},
			metamodel.Place{ID: "on_" + s, Initial: 0, Capacity: 1},
		)
		m.Transitions = append(m.Transitions, metamodel.Transition{ID: "flip_" + s})
		m.Arcs = append(m.Arcs,
			metamodel.Arc{From: "off_" + s, To: "flip_" + s},
			metamodel.Arc{From: "flip_" + s, To: "on_" + s},
		)
	}
	model, err := Normalize(m, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ModelCheck(model); err == nil {
		t.Fatal("expected state-space-too-large error")
	} else if !strings.Contains(err.Error(), "state space") {
		t.Errorf("error should mention state space: %v", err)
	}
}

func TestNormalizeErrors(t *testing.T) {
	base := func() *metamodel.Model {
		return &metamodel.Model{
			Name:        "t",
			Places:      []metamodel.Place{{ID: "p", Initial: 1}},
			Transitions: []metamodel.Transition{{ID: "go"}},
			Arcs:        []metamodel.Arc{{From: "p", To: "go"}},
		}
	}

	m := base()
	m.Arcs = append(m.Arcs, metamodel.Arc{From: "p", To: "nosuch"})
	if _, err := Normalize(m, ""); err == nil || !strings.Contains(err.Error(), "unknown transition") {
		t.Errorf("place->unknown arc: got %v", err)
	}

	m = base()
	m.Arcs = append(m.Arcs, metamodel.Arc{From: "ghost", To: "p"})
	if _, err := Normalize(m, ""); err == nil || !strings.Contains(err.Error(), "unknown transition") {
		t.Errorf("unknown->place arc: got %v", err)
	}

	m = base()
	m.Arcs = append(m.Arcs, metamodel.Arc{From: "go", To: "go"})
	if _, err := Normalize(m, ""); err == nil || !strings.Contains(err.Error(), "exactly one endpoint") {
		t.Errorf("transition->transition arc: got %v", err)
	}

	m = base()
	m.Transitions[0].Bindings = []metamodel.Binding{{Name: "amount", Type: "int64"}}
	if _, err := Normalize(m, ""); err == nil || !strings.Contains(err.Error(), "bindings") {
		t.Errorf("bindings: got %v", err)
	}
}

func TestNormalizeDefaults(t *testing.T) {
	// Empty model name falls back to petri_core; zero arc weight becomes 1.
	m := &metamodel.Model{
		Places:      []metamodel.Place{{ID: "p", Initial: 1}},
		Transitions: []metamodel.Transition{{ID: "go"}},
		Arcs:        []metamodel.Arc{{From: "p", To: "go", Weight: 0}},
	}
	model, err := Normalize(m, "")
	if err != nil {
		t.Fatal(err)
	}
	if model.PackageName != "petri_core" {
		t.Errorf("package = %q, want petri_core", model.PackageName)
	}
	if model.Transitions[0].Inputs[0].Weight != 1 {
		t.Errorf("zero weight should default to 1")
	}

	files, err := Generate(m, Options{Language: "js"}) // alias for javascript
	if err != nil {
		t.Fatal(err)
	}
	if files[0].Name != "petri_core.js" {
		t.Errorf("file = %q, want petri_core.js", files[0].Name)
	}
}

func TestNameHelpers(t *testing.T) {
	cases := []struct{ fn, in, want string }{
		{"snake", "BoilWater", "boil_water"},
		{"snake", "boil water", "boil_water"},
		{"snake", "boil-water", "boil_water"},
		{"snake", "", ""},
		{"camel", "boil_water", "BoilWater"},
		{"camel", "boil-water", "BoilWater"},
		{"camel", "", ""},
		{"lowerFirst", "BoilWater", "boilWater"},
		{"lowerFirst", "", ""},
		{"upperFirst", "boil", "Boil"},
		{"upperFirst", "", ""},
	}
	for _, c := range cases {
		var got string
		switch c.fn {
		case "snake":
			got = Snake(c.in)
		case "camel":
			got = Camel(c.in)
		case "lowerFirst":
			got = LowerFirst(c.in)
		case "upperFirst":
			got = UpperFirst(c.in)
		}
		if got != c.want {
			t.Errorf("%s(%q) = %q, want %q", c.fn, c.in, got, c.want)
		}
	}

	model := Model{Places: []Place{{Name: "A"}, {Name: "B"}}}
	if got := model.PlaceIndex("B"); got != 1 {
		t.Errorf("PlaceIndex(B) = %d, want 1", got)
	}
	if got := model.PlaceIndex("nope"); got != -1 {
		t.Errorf("PlaceIndex(nope) = %d, want -1", got)
	}
}

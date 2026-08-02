package core

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	metamodel "github.com/pflow-xyz/go-pflow/metamodel"
)

// coffeeMachine is pflow-polyglot's coffee-machine net in go-pflow model
// form: 1-safe via capacity, one read arc (PourCoffee requires Payment
// without consuming it), spelled as a transition->place inhibitor.
func coffeeMachine() *metamodel.Model {
	place := func(id string, initial int) metamodel.Place {
		return metamodel.Place{ID: id, Initial: initial, Capacity: 1}
	}
	arc := func(from, to string) metamodel.Arc {
		return metamodel.Arc{From: from, To: to}
	}
	return &metamodel.Model{
		Name: "CoffeeMachine",
		Places: []metamodel.Place{
			place("Water", 1), place("BoiledWater", 0), place("CoffeeBeans", 1),
			place("GroundCoffee", 0), place("Filter", 1), place("CoffeeInPot", 0),
			place("Pending", 1), place("Sent", 0), place("Payment", 0), place("Cup", 1),
		},
		Transitions: []metamodel.Transition{
			{ID: "BoilWater"}, {ID: "GrindBeans"}, {ID: "BrewCoffee"},
			{ID: "PourCoffee"}, {ID: "Send"}, {ID: "Credit"},
		},
		Arcs: []metamodel.Arc{
			arc("Water", "BoilWater"), arc("BoilWater", "BoiledWater"),
			arc("CoffeeBeans", "GrindBeans"), arc("GrindBeans", "GroundCoffee"),
			arc("BoiledWater", "BrewCoffee"), arc("GroundCoffee", "BrewCoffee"),
			arc("Filter", "BrewCoffee"), arc("BrewCoffee", "CoffeeInPot"),
			arc("CoffeeInPot", "PourCoffee"), arc("Cup", "PourCoffee"),
			arc("Pending", "Send"), arc("Send", "Sent"),
			arc("Sent", "Credit"), arc("Credit", "Payment"),
			{From: "PourCoffee", To: "Payment", Type: metamodel.InhibitorArc},
		},
	}
}

// goldenTrace is pflow-polyglot's parity/trace.golden: the trace every
// generated core must produce under the sorted-order greedy scheduler.
const goldenTrace = `Step #1: BoilWater => BoiledWater,CoffeeBeans,Cup,Filter,Pending
Step #2: GrindBeans => BoiledWater,Cup,Filter,GroundCoffee,Pending
Step #3: BrewCoffee => CoffeeInPot,Cup,Pending
Step #4: Send => CoffeeInPot,Cup,Sent
Step #5: Credit => CoffeeInPot,Cup,Payment
Step #6: PourCoffee => Payment
`

func TestGenerateDeterministic(t *testing.T) {
	for lang, spec := range Languages {
		for _, form := range spec.Forms {
			first, err := Generate(coffeeMachine(), Options{Language: lang, Form: form, PackageName: "main"})
			if err != nil {
				t.Fatalf("%s/%s: %v", lang, form, err)
			}
			second, err := Generate(coffeeMachine(), Options{Language: lang, Form: form, PackageName: "main"})
			if err != nil {
				t.Fatalf("%s/%s: %v", lang, form, err)
			}
			if !bytes.Equal(first[0].Content, second[0].Content) {
				t.Errorf("%s/%s: regeneration is not byte-identical", lang, form)
			}
		}
	}
}

func TestUnsupportedForm(t *testing.T) {
	if _, err := Generate(coffeeMachine(), Options{Language: "rust", Form: "proof"}); err == nil {
		t.Fatal("rust does not support the proof form; expected error")
	}
	if _, err := Generate(coffeeMachine(), Options{Language: "lean", Form: "lambda"}); err == nil {
		t.Fatal("lean supports only the proof form; expected error")
	}
}

// The proof form bakes the analyzer's findings into theorems; check the
// coffee machine's known numbers appear.
func TestProofFormFindings(t *testing.T) {
	files, err := Generate(coffeeMachine(), Options{Language: "lean"})
	if err != nil {
		t.Fatal(err)
	}
	src := string(files[0].Content)
	for _, want := range []string{
		"theorem reachable_closed",
		"theorem state_count : reachable.length = 16",
		"theorem bounded",
		"theorem deadlock_count",
		"by decide", // 16 states: kernel reduction, not native_decide
	} {
		if !strings.Contains(src, want) {
			t.Errorf("generated Lean missing %q", want)
		}
	}
}

// ModelCheck must refuse an unbounded net rather than emit unprovable
// theorems.
func TestModelCheckRejectsUnbounded(t *testing.T) {
	m := &metamodel.Model{
		Name:        "unbounded",
		Places:      []metamodel.Place{{ID: "p", Initial: 0}},
		Transitions: []metamodel.Transition{{ID: "mint"}},
		Arcs:        []metamodel.Arc{{From: "mint", To: "p"}},
	}
	_, err := Generate(m, Options{Language: "lean"})
	if err == nil {
		t.Fatal("expected unbounded-net error")
	}
	if !strings.Contains(err.Error(), "unbounded") && !strings.Contains(err.Error(), "state space") {
		t.Errorf("error should mention unboundedness or state-space size: %v", err)
	}
}

func TestNormalizeRejectsUnsupported(t *testing.T) {
	m := coffeeMachine()
	m.Places[0].Kind = metamodel.DataKind
	m.Places[0].Type = "map[string]int64"
	m.Transitions[0].Guard = "balances[from] >= amount"

	_, err := Generate(m, Options{Language: "go"})
	if err == nil {
		t.Fatal("expected error for data place + guard")
	}
	for _, want := range []string{"Water", "BoilWater", "guard"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q:\n%v", want, err)
		}
	}
}

func TestUnknownLanguage(t *testing.T) {
	if _, err := Generate(coffeeMachine(), Options{Language: "cobol"}); err == nil {
		t.Fatal("expected unsupported-language error")
	}
}

// TestExecutionParity generates each language, runs it with the system
// toolchain, and diffs the output against the golden trace. Languages whose
// toolchain is not installed are skipped, loudly.
//
// Skipped under -test.short: it shells out to system toolchains (go run
// needs a writable GOCACHE), which Bazel's sandbox doesn't provide — the
// same pattern as TestServiceManagerIntegration in pkg/mcp.
func TestExecutionParity(t *testing.T) {
	if testing.Short() {
		t.Skip("execution parity needs system toolchains; skipped in short mode")
	}
	dir := t.TempDir()

	runners := map[string]func(src string) *exec.Cmd{
		"go": func(src string) *exec.Cmd {
			return exec.Command("go", "run", src)
		},
		"python": func(src string) *exec.Cmd {
			return exec.Command("python3", src)
		},
		"javascript": func(src string) *exec.Cmd {
			return exec.Command("node", src)
		},
		"rust": func(src string) *exec.Cmd {
			bin := strings.TrimSuffix(src, ".rs")
			build := exec.Command("rustc", "-O", "-o", bin, src)
			if out, err := build.CombinedOutput(); err != nil {
				t.Fatalf("rustc: %v\n%s", err, out)
			}
			return exec.Command(bin)
		},
	}
	tools := map[string]string{"go": "go", "python": "python3", "javascript": "node", "rust": "rustc"}

	runners["lean"] = func(src string) *exec.Cmd {
		return exec.Command("lean", "--run", src)
	}
	tools["lean"] = "lean"

	for lang, runner := range runners {
		for _, form := range Languages[lang].Forms {
			t.Run(lang+"/"+form, func(t *testing.T) {
				if _, err := exec.LookPath(tools[lang]); err != nil {
					t.Skipf("no %s on PATH", tools[lang])
				}

				files, err := Generate(coffeeMachine(), Options{Language: lang, Form: form, PackageName: "main"})
				if err != nil {
					t.Fatal(err)
				}
				src := filepath.Join(dir, files[0].Name)
				if err := os.WriteFile(src, files[0].Content, 0o644); err != nil {
					t.Fatal(err)
				}

				out, err := runner(src).Output()
				if err != nil {
					if ee, ok := err.(*exec.ExitError); ok {
						t.Fatalf("running generated %s/%s: %v\nstderr:\n%s", lang, form, err, ee.Stderr)
					}
					t.Fatalf("running generated %s/%s: %v", lang, form, err)
				}
				if string(out) != goldenTrace {
					t.Errorf("%s/%s trace mismatch:\n--- want\n%s--- got\n%s", lang, form, goldenTrace, out)
				}
			})
		}
	}
}

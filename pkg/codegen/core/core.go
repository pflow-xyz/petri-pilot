// Package core generates a dependency-free, single-file state machine from a
// Petri net model — the "core" the model specifies, with nothing app-shaped
// around it. Where pkg/codegen/golang emits a full event-sourced application,
// this package emits one file you can drop into an existing codebase in the
// target language: Go, Rust, Python, or JavaScript.
//
// The lineage is pflow-polyglot's "generated" form
// (github.com/stackdump/pflow-polyglot): a scheduler loop over transitions
// whose arcs have been unrolled into straight-line conditionals, generalized
// here from 1-safe set semantics to counted markings with arc weights, place
// capacities, inhibitor arcs, and read arcs.
//
// Supported subset: every place must be a token place, and transitions must
// carry no expression guards and no bindings. Models outside the subset get a
// descriptive error naming every offending element — the application
// generator is the right tool for those.
//
// Output is deterministic: places, transitions, and per-transition arc lists
// are all sorted by ID, so regenerating an unchanged model produces
// byte-identical source.
package core

import (
	"bytes"
	"embed"
	"fmt"
	"sort"
	"strings"
	"text/template"

	metamodel "github.com/pflow-xyz/go-pflow/metamodel"
)

//go:embed templates/*/*.tmpl
var templates embed.FS

// Languages maps the accepted -language values to their file extension and
// the forms they support. Callers can range over this to advertise what core
// mode supports. Templates live at templates/<language>/<form>.tmpl.
var Languages = map[string]struct {
	Ext   string
	Forms []string
}{
	"go":         {"go", []string{FormGenerated, FormInterpreter, FormLambda, FormContract}},
	"rust":       {"rs", []string{FormGenerated, FormInterpreter, FormLambda, FormContract}},
	"python":     {"py", []string{FormGenerated, FormInterpreter, FormLambda, FormContract}},
	"javascript": {"js", []string{FormGenerated, FormInterpreter, FormLambda, FormContract}},
	"lean":       {"lean", []string{FormProof}},
}

// The implementation forms (see pflow-polyglot's FORMS.md). Every form prints
// the same trace under the same greedy driver; what differs is how the net is
// encoded in the host language.
const (
	// FormGenerated unrolls every arc into straight-line conditionals with a
	// generated scheduler. The default.
	FormGenerated = "generated"
	// FormInterpreter keeps the net as runtime data walked by a generic
	// enablement/firing engine.
	FormInterpreter = "interpreter"
	// FormLambda encodes each transition as a pure marking -> marking?
	// function composed in a fixed schedule.
	FormLambda = "lambda"
	// FormContract exposes each transition as a public entry point that
	// either fires or refuses with a reason; the caller owns sequencing.
	FormContract = "contract"
	// FormProof (Lean only) re-states the generator's own model-check
	// findings as theorems the Lean kernel re-derives at compile time.
	FormProof = "proof"
)

// GeneratedFile is one emitted file.
type GeneratedFile struct {
	Name    string
	Content []byte
}

// Options configures generation.
type Options struct {
	// Language is one of the Languages keys. "js" is accepted as an alias
	// for "javascript".
	Language string

	// PackageName names the Go package / Python module comment / etc.
	// Defaults to a snake_case of the model name.
	PackageName string

	// Form selects the implementation form. Defaults to FormGenerated
	// (FormProof for lean, which supports nothing else).
	Form string
}

// --- normalized form handed to the templates -------------------------------

// WeightedArc is one arc endpoint as a transition sees it.
type WeightedArc struct {
	Place  string
	Weight int
	// Capacity is the target place's capacity, for output arcs into a
	// bounded place. 0 means unbounded.
	Capacity int
}

// Transition is one transition with its arcs resolved and classified. The
// four lists are the four clauses of the enablement rule:
//
//	Inputs:   tokens(p) >= w, consumed
//	Reads:    tokens(p) >= w, not consumed   (transition->place inhibitor)
//	Inhibits: tokens(p) <  w                 (place->transition inhibitor)
//	Outputs:  produced; if the place is bounded, tokens(p)+w <= capacity
type Transition struct {
	Name     string
	Inputs   []WeightedArc
	Outputs  []WeightedArc
	Reads    []WeightedArc
	Inhibits []WeightedArc
}

// Place is one place with its initial marking.
type Place struct {
	Name     string
	Initial  int
	Capacity int
}

// Model is the normalized net handed to the templates.
type Model struct {
	Name        string
	PackageName string
	Places      []Place
	Transitions []Transition

	// Proof carries the generation-time model-check findings; set only for
	// FormProof.
	Proof *Findings
}

// Normalize classifies every arc and checks the model against the supported
// subset. It returns an error naming every unsupported element at once, so a
// caller fixes one round of feedback rather than five.
func Normalize(m *metamodel.Model, pkgName string) (Model, error) {
	out := Model{Name: m.Name, PackageName: pkgName}
	if out.PackageName == "" {
		out.PackageName = Snake(m.Name)
	}
	if out.PackageName == "" {
		out.PackageName = "petri_core"
	}

	var unsupported []string

	placeByID := map[string]*metamodel.Place{}
	for i := range m.Places {
		p := &m.Places[i]
		placeByID[p.ID] = p
		if !p.IsToken() {
			unsupported = append(unsupported,
				fmt.Sprintf("place %q has kind %q: core mode supports token places only", p.ID, p.Kind))
			continue
		}
		out.Places = append(out.Places, Place{Name: p.ID, Initial: p.Initial, Capacity: p.Capacity})
	}
	sort.Slice(out.Places, func(i, j int) bool { return out.Places[i].Name < out.Places[j].Name })

	transByID := map[string]*Transition{}
	for _, t := range m.Transitions {
		if t.Guard != "" {
			unsupported = append(unsupported,
				fmt.Sprintf("transition %q has an expression guard: core mode supports arc-level structure only (use language 'go' for the full application generator)", t.ID))
		}
		if len(t.Bindings) > 0 {
			unsupported = append(unsupported,
				fmt.Sprintf("transition %q has bindings: core mode has no data states to bind", t.ID))
		}
		transByID[t.ID] = &Transition{Name: t.ID}
	}

	weight := func(w int) int {
		if w <= 0 {
			return 1
		}
		return w
	}

	for _, a := range m.Arcs {
		srcPlace, srcIsPlace := placeByID[a.From]
		dstPlace, dstIsPlace := placeByID[a.To]
		switch {
		case srcIsPlace && !dstIsPlace:
			t, ok := transByID[a.To]
			if !ok {
				return out, fmt.Errorf("arc %s -> %s: unknown transition %q", a.From, a.To, a.To)
			}
			w := WeightedArc{Place: a.From, Weight: weight(a.Weight)}
			if a.IsInhibitor() {
				// Blocks while tokens(p) >= weight — go-pflow's
				// tokens(P) < weight rule (compat/bridge.go).
				t.Inhibits = append(t.Inhibits, w)
			} else {
				t.Inputs = append(t.Inputs, w)
			}
			_ = srcPlace
		case !srcIsPlace && dstIsPlace:
			t, ok := transByID[a.From]
			if !ok {
				return out, fmt.Errorf("arc %s -> %s: unknown transition %q", a.From, a.To, a.From)
			}
			w := WeightedArc{Place: a.To, Weight: weight(a.Weight), Capacity: dstPlace.Capacity}
			if a.IsInhibitor() {
				// A transition->place inhibitor is a read arc: the place
				// must hold >= weight tokens, and is not consumed. This is
				// pflow-xyz's spelling for guards.
				t.Reads = append(t.Reads, w)
			} else {
				t.Outputs = append(t.Outputs, w)
			}
		default:
			return out, fmt.Errorf("arc %s -> %s: exactly one endpoint must be a place", a.From, a.To)
		}
	}

	if len(unsupported) > 0 {
		return out, fmt.Errorf("model outside the core-mode subset:\n  - %s", strings.Join(unsupported, "\n  - "))
	}

	byPlace := func(arcs []WeightedArc) func(i, j int) bool {
		return func(i, j int) bool { return arcs[i].Place < arcs[j].Place }
	}
	for _, t := range transByID {
		sort.Slice(t.Inputs, byPlace(t.Inputs))
		sort.Slice(t.Outputs, byPlace(t.Outputs))
		sort.Slice(t.Reads, byPlace(t.Reads))
		sort.Slice(t.Inhibits, byPlace(t.Inhibits))
		out.Transitions = append(out.Transitions, *t)
	}
	sort.Slice(out.Transitions, func(i, j int) bool { return out.Transitions[i].Name < out.Transitions[j].Name })

	return out, nil
}

// --- template helpers ------------------------------------------------------

// LowerFirst turns BoilWater into boilWater.
func LowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// UpperFirst turns boil_water's camel form into an exported Go identifier.
func UpperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// Snake turns BoilWater into boil_water.
func Snake(s string) string {
	var b strings.Builder
	prevLower := false
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			if prevLower {
				b.WriteByte('_')
			}
			b.WriteRune(r + ('a' - 'A'))
			prevLower = false
		case r == ' ' || r == '-':
			b.WriteByte('_')
			prevLower = false
		default:
			b.WriteRune(r)
			prevLower = r >= 'a' && r <= 'z' || r >= '0' && r <= '9'
		}
	}
	return b.String()
}

// Ident strips characters that are not identifier-safe after Snake/camel.
func Ident(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Camel turns boil_water / boil-water / BoilWater into BoilWater.
func Camel(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '_' || r == '-' || r == ' ' })
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(UpperFirst(p))
	}
	return Ident(b.String())
}

var funcs = template.FuncMap{
	"lowerFirst": func(s string) string { return Ident(LowerFirst(s)) },
	"camel":      Camel,
	"snake":      func(s string) string { return Ident(Snake(s)) },
	"ints": func(xs []int) string {
		parts := make([]string, len(xs))
		for i, x := range xs {
			parts[i] = fmt.Sprint(x)
		}
		return strings.Join(parts, ", ")
	},
}

// PlaceIndex returns the index of a place in the sorted Places list; the
// Lean template addresses places by index rather than by (possibly
// non-identifier) name.
func (m Model) PlaceIndex(name string) int {
	for i, p := range m.Places {
		if p.Name == name {
			return i
		}
	}
	return -1
}

// Generate renders the model into a single source file for the requested
// language and form.
func Generate(m *metamodel.Model, opts Options) ([]GeneratedFile, error) {
	lang := opts.Language
	if lang == "js" {
		lang = "javascript"
	}
	spec, ok := Languages[lang]
	if !ok {
		names := make([]string, 0, len(Languages))
		for n := range Languages {
			names = append(names, n)
		}
		sort.Strings(names)
		return nil, fmt.Errorf("unsupported core language %q (have: %s)", opts.Language, strings.Join(names, ", "))
	}

	form := opts.Form
	if form == "" {
		form = spec.Forms[0]
	}
	formOK := false
	for _, f := range spec.Forms {
		if f == form {
			formOK = true
			break
		}
	}
	if !formOK {
		return nil, fmt.Errorf("language %q does not support form %q (have: %s)", lang, form, strings.Join(spec.Forms, ", "))
	}

	model, err := Normalize(m, opts.PackageName)
	if err != nil {
		return nil, err
	}

	// The proof form re-states the generator's own model-check findings as
	// theorems, so the model-check runs here, at generation time. A net
	// whose state space is too large (or unbounded) is rejected: kernel
	// reduction is the proof engine, and it only reaches so far.
	if form == FormProof {
		findings, err := ModelCheck(model)
		if err != nil {
			return nil, fmt.Errorf("proof form: %w", err)
		}
		model.Proof = findings
	}

	tmplPath := "templates/" + lang + "/" + form + ".tmpl"
	tmpl, err := template.New(form + ".tmpl").Funcs(funcs).ParseFS(templates, tmplPath)
	if err != nil {
		return nil, fmt.Errorf("parsing template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "main", model); err != nil {
		return nil, fmt.Errorf("rendering: %w", err)
	}

	name := Snake(model.Name)
	if name == "" {
		name = "petri_core"
	}
	if form != FormGenerated && form != FormProof {
		name += "_" + form
	}
	return []GeneratedFile{{Name: name + "." + spec.Ext, Content: buf.Bytes()}}, nil
}

package esmodules

import (
	"strings"
	"testing"
)

// TestFrontendEncodesReadArcAsReversedInhibitor.
//
// The generated frontend hands the browser viewer/simulator a pflow-xyz
// JSON-LD net. That format has no read arc, but an inhibitor pointing
// transition -> place already means "the place must hold the weight, and
// nothing is consumed" — so a read arc is encoded by reversing it, exactly as
// go-pflow's metapetri bridge and pkg/validator do.
//
// Emitted as a plain arc, the browser drew and simulated a net that consumes
// from the place the model only tests, while the Go backend generated from the
// same model did not. The two halves of one app disagreed about the net.
func TestFrontendEncodesReadArcAsReversedInhibitor(t *testing.T) {
	src := mainTemplateSource(t)

	// The arc builder must branch on the read type rather than passing
	// from/to through unchanged.
	for _, want := range []string{
		"a.type === 'read'",
		"isRead ? a.to : a.from",
		"isRead ? a.from : a.to",
		"isRead || a.type === 'inhibitor'",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("main.tmpl arc builder is missing %q: a read arc reaches the browser as a consuming arc", want)
		}
	}

	// And the old unconditional form must be gone.
	if strings.Contains(src, "inhibit: a.type === 'inhibitor'") {
		t.Error("main.tmpl still classifies arcs by the inhibitor type alone; a read arc is not inhibit and not normal")
	}
}

func mainTemplateSource(t *testing.T) string {
	t.Helper()
	body, err := templateFS.ReadFile("templates/main.tmpl")
	if err != nil {
		t.Fatalf("reading main.tmpl: %v", err)
	}
	return string(body)
}

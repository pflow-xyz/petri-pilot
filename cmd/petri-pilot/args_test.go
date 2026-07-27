package main

import (
	"flag"
	"testing"
)

func newTestFlagSet() (*flag.FlagSet, *bool, *string, *int) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	verbose := fs.Bool("verbose", false, "")
	output := fs.String("output", "", "")
	count := fs.Int("count", 0, "")
	return fs, verbose, output, count
}

// TestReorderArgsFlagsAfterPositional is the regression for the original bug:
// every documented example put the model file first, and Go's flag package
// stopped parsing there, so the flags were silently ignored.
func TestReorderArgsFlagsAfterPositional(t *testing.T) {
	fs, verbose, output, count := newTestFlagSet()

	args := []string{"model.json", "--verbose", "--output", "out.svg", "--count", "42"}
	if err := fs.Parse(reorderArgs(fs, args)); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if !*verbose {
		t.Error("--verbose was not applied")
	}
	if *output != "out.svg" {
		t.Errorf("--output = %q, want out.svg", *output)
	}
	if *count != 42 {
		t.Errorf("--count = %d, want 42", *count)
	}
	if fs.NArg() != 1 || fs.Arg(0) != "model.json" {
		t.Errorf("positionals = %v, want [model.json]", fs.Args())
	}
}

func TestReorderArgsFlagsBeforePositional(t *testing.T) {
	fs, verbose, output, _ := newTestFlagSet()

	args := []string{"--verbose", "--output", "out.svg", "model.json"}
	if err := fs.Parse(reorderArgs(fs, args)); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if !*verbose || *output != "out.svg" {
		t.Errorf("flags not applied: verbose=%v output=%q", *verbose, *output)
	}
	if fs.NArg() != 1 || fs.Arg(0) != "model.json" {
		t.Errorf("positionals = %v, want [model.json]", fs.Args())
	}
}

func TestReorderArgsInlineValues(t *testing.T) {
	fs, verbose, output, count := newTestFlagSet()

	args := []string{"model.json", "--output=out.svg", "--count=7", "--verbose=true"}
	if err := fs.Parse(reorderArgs(fs, args)); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if *output != "out.svg" || *count != 7 || !*verbose {
		t.Errorf("inline values not applied: output=%q count=%d verbose=%v", *output, *count, *verbose)
	}
	if fs.NArg() != 1 {
		t.Errorf("positionals = %v, want 1", fs.Args())
	}
}

// TestReorderArgsFlagValueNotTreatedAsPositional guards the subtle case: the
// value of a non-boolean flag must stay attached to its flag, not be hoisted
// into the positional list.
func TestReorderArgsFlagValueNotTreatedAsPositional(t *testing.T) {
	fs, _, output, _ := newTestFlagSet()

	args := []string{"--output", "out.svg", "a.json", "b.json"}
	if err := fs.Parse(reorderArgs(fs, args)); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if *output != "out.svg" {
		t.Errorf("--output = %q, want out.svg", *output)
	}
	if fs.NArg() != 2 || fs.Arg(0) != "a.json" || fs.Arg(1) != "b.json" {
		t.Errorf("positionals = %v, want [a.json b.json]", fs.Args())
	}
}

// TestReorderArgsBoolFlagDoesNotEatNext checks that a boolean flag leaves the
// following argument alone.
func TestReorderArgsBoolFlagDoesNotEatNext(t *testing.T) {
	fs, verbose, _, _ := newTestFlagSet()

	args := []string{"--verbose", "model.json"}
	if err := fs.Parse(reorderArgs(fs, args)); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if !*verbose {
		t.Error("--verbose not applied")
	}
	if fs.NArg() != 1 || fs.Arg(0) != "model.json" {
		t.Errorf("positionals = %v, want [model.json]", fs.Args())
	}
}

func TestReorderArgsMultiplePositionals(t *testing.T) {
	fs, _, _, count := newTestFlagSet()

	args := []string{"a.json", "--count", "3", "b.json"}
	if err := fs.Parse(reorderArgs(fs, args)); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if *count != 3 {
		t.Errorf("--count = %d, want 3", *count)
	}
	if fs.NArg() != 2 || fs.Arg(0) != "a.json" || fs.Arg(1) != "b.json" {
		t.Errorf("positionals = %v, want [a.json b.json]", fs.Args())
	}
}

// TestReorderArgsDoubleDash checks the standard terminator: everything after
// "--" is positional even if it looks like a flag.
func TestReorderArgsDoubleDash(t *testing.T) {
	fs, verbose, _, _ := newTestFlagSet()

	args := []string{"--verbose", "--", "--not-a-flag.json"}
	if err := fs.Parse(reorderArgs(fs, args)); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if !*verbose {
		t.Error("--verbose not applied")
	}
	if fs.NArg() != 1 || fs.Arg(0) != "--not-a-flag.json" {
		t.Errorf("positionals = %v, want [--not-a-flag.json]", fs.Args())
	}
}

func TestReorderArgsEmpty(t *testing.T) {
	fs, _, _, _ := newTestFlagSet()
	if got := reorderArgs(fs, nil); len(got) != 0 {
		t.Errorf("reorderArgs(nil) = %v, want empty", got)
	}
}

// TestReorderArgsUnknownFlagStillReported ensures reordering does not swallow
// an unknown flag — Parse must still fail on it.
func TestReorderArgsUnknownFlagStillReported(t *testing.T) {
	fs, _, _, _ := newTestFlagSet()
	fs.SetOutput(discard{})

	args := []string{"model.json", "--nope"}
	if err := fs.Parse(reorderArgs(fs, args)); err == nil {
		t.Error("expected an error for an unknown flag")
	}
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

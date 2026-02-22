package codegen

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

// cascadeModel is a minimal 3-place, 2-transition cascade: A → B → C.
func cascadeModel() *metamodel.Model {
	return &metamodel.Model{
		Name: "cascade",
		Places: []metamodel.Place{
			{ID: "A", Initial: 1},
			{ID: "B", Initial: 0},
			{ID: "C", Initial: 0},
		},
		Transitions: []metamodel.Transition{
			{ID: "t0", Rate: 1.0},
			{ID: "t1", Rate: 1.0},
		},
		Arcs: []metamodel.Arc{
			{From: "A", To: "t0"},
			{From: "t0", To: "B"},
			{From: "B", To: "t1"},
			{From: "t1", To: "C"},
		},
	}
}

func TestNewContext_Cascade(t *testing.T) {
	model := cascadeModel()
	ctx, err := NewContext(model, "cascade", nil)
	if err != nil {
		t.Fatalf("NewContext: %v", err)
	}

	if ctx.NumPlaces != 3 {
		t.Errorf("NumPlaces = %d, want 3", ctx.NumPlaces)
	}
	if ctx.NumTransitions != 2 {
		t.Errorf("NumTransitions = %d, want 2", ctx.NumTransitions)
	}

	// Verify stoichiometry matches hand-coded cascade:
	// S = [[-1,0],[+1,-1],[0,+1]]
	expected := [][]int{
		{-1, 0},
		{+1, -1},
		{0, +1},
	}
	for p := 0; p < 3; p++ {
		for tr := 0; tr < 2; tr++ {
			if ctx.Stoichiometry[p][tr] != expected[p][tr] {
				t.Errorf("S[%d][%d] = %d, want %d", p, tr, ctx.Stoichiometry[p][tr], expected[p][tr])
			}
		}
	}

	// Verify transition inputs
	if len(ctx.Transitions[0].Inputs) != 1 || ctx.Transitions[0].Inputs[0] != 0 {
		t.Errorf("t0 inputs = %v, want [0]", ctx.Transitions[0].Inputs)
	}
	if len(ctx.Transitions[1].Inputs) != 1 || ctx.Transitions[1].Inputs[0] != 1 {
		t.Errorf("t1 inputs = %v, want [1]", ctx.Transitions[1].Inputs)
	}

	if ctx.HasScoring {
		t.Error("HasScoring should be false for cascade without scoring config")
	}
}

func TestGenerate_Cascade(t *testing.T) {
	model := cascadeModel()
	gen, err := New(Options{
		PackageName: "cascade",
		OutputDir:   t.TempDir(),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	files, err := gen.Generate(model)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Should generate 4 core files (no scoring)
	if len(files) != 4 {
		t.Errorf("generated %d files, want 4", len(files))
	}

	expectedFiles := map[string]bool{
		TemplateTopology: false,
		TemplateCircuit:  false,
		TemplateWitness:  false,
		TemplateState:    false,
	}
	for _, f := range files {
		if _, ok := expectedFiles[f.Path]; !ok {
			t.Errorf("unexpected file: %s", f.Path)
		}
		expectedFiles[f.Path] = true

		// Verify content is valid Go (starts with package declaration)
		content := string(f.Content)
		if !strings.HasPrefix(content, "package cascade") {
			t.Errorf("%s doesn't start with 'package cascade', starts with: %s",
				f.Path, content[:min(80, len(content))])
		}
	}

	for name, found := range expectedFiles {
		if !found {
			t.Errorf("missing expected file: %s", name)
		}
	}
}

func TestGenerate_CascadeTopology(t *testing.T) {
	model := cascadeModel()
	gen, err := New(Options{PackageName: "cascade"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	files, err := gen.Generate(model)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Find topology file and verify key content
	var topoContent string
	for _, f := range files {
		if f.Path == TemplateTopology {
			topoContent = string(f.Content)
			break
		}
	}

	// Verify topology constants
	checks := []string{
		"NumPlaces = 3",
		"NumTransitions = 2",
		`"A"`,
		`"B"`,
		`"C"`,
		`"t0"`,
		`"t1"`,
		"FixFromFloat(1)", // Initial marking for A
	}
	for _, check := range checks {
		if !strings.Contains(topoContent, check) {
			t.Errorf("topology.go missing expected content: %q", check)
		}
	}
}

func TestGenerate_CascadeWriteToDir(t *testing.T) {
	model := cascadeModel()
	outDir := t.TempDir()

	gen, err := New(Options{
		PackageName: "cascade",
		OutputDir:   outDir,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	paths, err := gen.GenerateToDir(model)
	if err != nil {
		t.Fatalf("GenerateToDir: %v", err)
	}

	if len(paths) != 4 {
		t.Errorf("wrote %d files, want 4", len(paths))
	}

	// Verify files exist on disk
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			t.Errorf("file not found: %s", p)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("file is empty: %s", p)
		}
	}
}

func TestNewContext_TTT(t *testing.T) {
	data, err := os.ReadFile("../../services/tic-tac-toe.json")
	if err != nil {
		t.Skipf("tic-tac-toe.json not found: %v", err)
	}

	var model metamodel.Model
	if err := json.Unmarshal(data, &model); err != nil {
		t.Fatalf("parsing model: %v", err)
	}

	ctx, err := NewContext(&model, "ttt", nil)
	if err != nil {
		t.Fatalf("NewContext: %v", err)
	}

	// TTT model has 33 places and 35 transitions
	if ctx.NumPlaces != 33 {
		t.Errorf("NumPlaces = %d, want 33", ctx.NumPlaces)
	}
	if ctx.NumTransitions != 35 {
		t.Errorf("NumTransitions = %d, want 35", ctx.NumTransitions)
	}

	// Verify some key stoichiometry entries against hand-coded values
	// x_play_00 (transition 0): consumes p00 (place 0) and x_turn (place 27)
	if ctx.Stoichiometry[0][0] != -1 {
		t.Errorf("S[p00][x_play_00] = %d, want -1", ctx.Stoichiometry[0][0])
	}
	if ctx.Stoichiometry[27][0] != -1 {
		t.Errorf("S[x_turn][x_play_00] = %d, want -1", ctx.Stoichiometry[27][0])
	}
	// x_play_00 produces x00 (place 9) and o_turn (place 28)
	if ctx.Stoichiometry[9][0] != 1 {
		t.Errorf("S[x00][x_play_00] = %d, want +1", ctx.Stoichiometry[9][0])
	}
	if ctx.Stoichiometry[28][0] != 1 {
		t.Errorf("S[o_turn][x_play_00] = %d, want +1", ctx.Stoichiometry[28][0])
	}

	// Verify max inputs: win transitions have 5 inputs
	if ctx.MaxInputsPerTransition != 5 {
		t.Errorf("MaxInputsPerTransition = %d, want 5", ctx.MaxInputsPerTransition)
	}
}

func TestGenerate_TTT(t *testing.T) {
	data, err := os.ReadFile("../../services/tic-tac-toe.json")
	if err != nil {
		t.Skipf("tic-tac-toe.json not found: %v", err)
	}

	var model metamodel.Model
	if err := json.Unmarshal(data, &model); err != nil {
		t.Fatalf("parsing model: %v", err)
	}

	gen, err := New(Options{PackageName: "ttt"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	files, err := gen.Generate(&model)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if len(files) != 4 {
		t.Errorf("generated %d files, want 4", len(files))
	}

	// Verify each file starts with correct package
	for _, f := range files {
		if !strings.HasPrefix(string(f.Content), "package ttt") {
			t.Errorf("%s doesn't start with 'package ttt'", f.Path)
		}
	}
}

func TestGenerate_TTTWithScoring(t *testing.T) {
	data, err := os.ReadFile("../../services/tic-tac-toe.json")
	if err != nil {
		t.Skipf("tic-tac-toe.json not found: %v", err)
	}

	var model metamodel.Model
	if err := json.Unmarshal(data, &model); err != nil {
		t.Fatalf("parsing model: %v", err)
	}

	scoring := &ScoringConfig{
		Candidates:     []string{"x_play_*"},
		Targets:        []string{"x_win_*"},
		Bonus:          10.0,
		Penalty:        1.5,
		UseRateWeights: true,
	}

	gen, err := New(Options{
		PackageName: "ttt",
		Scoring:     scoring,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	files, err := gen.Generate(&model)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Should generate 6 files (4 core + 2 scoring)
	if len(files) != 6 {
		t.Errorf("generated %d files, want 6", len(files))
	}

	// Verify scoring files are present
	hasScoring := false
	hasScoringWitness := false
	for _, f := range files {
		if f.Path == TemplateScoringCircuit {
			hasScoring = true
			content := string(f.Content)
			if !strings.Contains(content, "ScoringCircuit") {
				t.Error("scoring_circuit.go missing ScoringCircuit type")
			}
			if !strings.Contains(content, "ScoringBonus") {
				t.Error("scoring_circuit.go missing ScoringBonus")
			}
		}
		if f.Path == TemplateScoringWitness {
			hasScoringWitness = true
		}
	}

	if !hasScoring {
		t.Error("missing scoring_circuit.go")
	}
	if !hasScoringWitness {
		t.Error("missing scoring_witness.go")
	}
}

func TestMatchGlobs(t *testing.T) {
	ids := []string{
		"x_play_00", "x_play_01", "x_play_02",
		"o_play_00", "o_play_01",
		"x_win_row0", "x_win_col0", "x_win_diag",
		"draw",
	}

	tests := []struct {
		globs    []string
		expected int
	}{
		{[]string{"x_play_*"}, 3},
		{[]string{"o_play_*"}, 2},
		{[]string{"x_win_*"}, 3},
		{[]string{"x_play_*", "o_play_*"}, 5},
		{[]string{"draw"}, 1},
		{[]string{"nonexistent_*"}, 0},
	}

	for _, tt := range tests {
		matched := MatchGlobs(ids, tt.globs)
		if len(matched) != tt.expected {
			t.Errorf("MatchGlobs(%v, %v) = %d matches, want %d", ids, tt.globs, len(matched), tt.expected)
		}
	}
}

func TestToConstName(t *testing.T) {
	tests := []struct {
		prefix, id, expected string
	}{
		{"Place", "p00", "PlaceP00"},
		{"Trans", "x_play_00", "TransXPlay00"},
		{"Place", "win_x", "PlaceWinX"},
		{"Trans", "draw", "TransDraw"},
		{"Place", "game_active", "PlaceGameActive"},
	}

	for _, tt := range tests {
		got := toConstName(tt.prefix, tt.id)
		if got != tt.expected {
			t.Errorf("toConstName(%q, %q) = %q, want %q", tt.prefix, tt.id, got, tt.expected)
		}
	}
}

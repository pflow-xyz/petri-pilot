package zkgo

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

func TestGenerator_TicTacToe(t *testing.T) {
	// Load the tic-tac-toe model
	data, err := os.ReadFile("../../../examples/tic-tac-toe.json")
	if err != nil {
		t.Fatalf("failed to read model: %v", err)
	}

	var model metamodel.Model
	if err := json.Unmarshal(data, &model); err != nil {
		t.Fatalf("failed to parse model: %v", err)
	}

	// Create generator
	gen, err := New(Options{
		PackageName:  "tictactoe",
		IncludeTests: true,
	})
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	// Generate files
	files, err := gen.GenerateFiles(&model)
	if err != nil {
		t.Fatalf("failed to generate files: %v", err)
	}

	// Verify expected files
	expectedFiles := map[string]bool{
		"petri_state.go":         false,
		"petri_circuits.go":      false,
		"petri_game.go":          false,
		"petri_circuits_test.go": false,
	}

	for _, file := range files {
		if _, ok := expectedFiles[file.Name]; ok {
			expectedFiles[file.Name] = true
			t.Logf("Generated %s (%d bytes)", file.Name, len(file.Content))
		}
	}

	for name, found := range expectedFiles {
		if !found {
			t.Errorf("missing expected file: %s", name)
		}
	}

	// Verify state.go contains expected constants
	var stateContent string
	for _, file := range files {
		if file.Name == "petri_state.go" {
			stateContent = string(file.Content)
			break
		}
	}

	if !strings.Contains(stateContent, "NumPlaces = 33") {
		t.Error("state.go should have 33 places")
	}
	if !strings.Contains(stateContent, "NumTransitions = 35") {
		t.Error("state.go should have 35 transitions")
	}
	if !strings.Contains(stateContent, "PlaceXTurn") {
		t.Error("state.go should have PlaceXTurn constant")
	}
	if !strings.Contains(stateContent, "TXPlay00") {
		t.Error("state.go should have TXPlay00 constant")
	}
}

func TestContext_BuildsCorrectly(t *testing.T) {
	// Load the tic-tac-toe model
	data, err := os.ReadFile("../../../examples/tic-tac-toe.json")
	if err != nil {
		t.Fatalf("failed to read model: %v", err)
	}

	var model metamodel.Model
	if err := json.Unmarshal(data, &model); err != nil {
		t.Fatalf("failed to parse model: %v", err)
	}

	ctx, err := NewContext(&model, "tictactoe")
	if err != nil {
		t.Fatalf("failed to create context: %v", err)
	}

	// Verify counts
	if ctx.NumPlaces != 33 {
		t.Errorf("expected 33 places, got %d", ctx.NumPlaces)
	}
	if ctx.NumTransitions != 35 {
		t.Errorf("expected 35 transitions, got %d", ctx.NumTransitions)
	}

	// Verify some transitions have arcs
	transWithArcs := 0
	for _, tr := range ctx.Transitions {
		if len(tr.Inputs) > 0 || len(tr.Outputs) > 0 {
			transWithArcs++
		}
	}
	if transWithArcs == 0 {
		t.Error("no transitions have arcs")
	}

	t.Logf("Context: %d places, %d transitions, %d with arcs",
		ctx.NumPlaces, ctx.NumTransitions, transWithArcs)
}

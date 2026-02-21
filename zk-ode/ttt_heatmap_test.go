package zkode

import (
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

func TestTTTHeatmapBlocking(t *testing.T) {
	// Board: .,.,. / O,X,. / .,X,. — O's turn
	// X threatens column 1 at (0,1). O must block.
	board := Board{
		{"", "", ""},
		{"O", "X", ""},
		{"", "X", ""},
	}

	state := BoardToTTTODEState(board, "O")
	h := FixFromFloat(0.01)
	w := ComputeTTTHeatmapStep(state, h)

	// Log all scores
	for i := 0; i < 9; i++ {
		score := FixToFloat(w.HeatmapScores[i])
		r, c := i/3, i%3
		t.Logf("Cell (%d,%d): score=%.4f", r, c, score)
	}

	// Cell (0,1) should have the highest score among available moves
	// because it blocks X's column 1 threat
	blockScore := FixToFloat(w.HeatmapScores[1]) // (0,1) = index 1

	// Corners (0,0) and (0,2) should have lower scores due to threat penalty
	for _, idx := range []int{0, 2} {
		other := FixToFloat(w.HeatmapScores[idx])
		if other >= blockScore {
			r, c := idx/3, idx%3
			t.Errorf("Cell (%d,%d) score %.4f >= blocking cell (0,1) score %.4f", r, c, other, blockScore)
		}
	}

	// Find optimal (highest score)
	bestIdx := -1
	bestScore := -1.0
	for i := 0; i < 9; i++ {
		s := FixToFloat(w.HeatmapScores[i])
		if s > bestScore {
			bestScore = s
			bestIdx = i
		}
	}
	t.Logf("Optimal: cell %d (%d,%d) score=%.4f", bestIdx, bestIdx/3, bestIdx%3, bestScore)

	// The optimal should be (0,1) which blocks X's column
	if bestIdx != 1 {
		t.Errorf("Expected optimal at cell 1 (0,1) for blocking, got cell %d", bestIdx)
	}
}

func TestTTTHeatmapWinning(t *testing.T) {
	// Board where X can win by playing (2,2) to complete diagonal
	// X at (0,0), (1,1); O at (0,1), (1,0) — X's turn
	board := Board{
		{"X", "O", ""},
		{"O", "X", ""},
		{"", "", ""},
	}

	state := BoardToTTTODEState(board, "X")
	h := FixFromFloat(0.01)
	w := ComputeTTTHeatmapStep(state, h)

	for i := 0; i < 9; i++ {
		score := FixToFloat(w.HeatmapScores[i])
		r, c := i/3, i%3
		t.Logf("Cell (%d,%d): score=%.4f", r, c, score)
	}

	// Cell (2,2) = index 8 should have win bonus and be highest
	winScore := FixToFloat(w.HeatmapScores[8])
	if winScore < 10.0 {
		t.Errorf("Winning cell (2,2) score %.4f should include bonus >= 10.0", winScore)
	}

	// It should be the best move
	for i := 0; i < 9; i++ {
		if i == 8 {
			continue
		}
		other := FixToFloat(w.HeatmapScores[i])
		if other > winScore {
			t.Errorf("Cell %d score %.4f > winning cell score %.4f", i, other, winScore)
		}
	}
}

func TestTTTHeatmapOccupied(t *testing.T) {
	board := Board{
		{"X", "O", "X"},
		{"O", "X", "O"},
		{"", "", ""},
	}

	state := BoardToTTTODEState(board, "X")
	h := FixFromFloat(0.01)
	w := ComputeTTTHeatmapStep(state, h)

	zero := FixFromFloat(0.0)

	// Occupied cells (0-5) should have score 0
	for i := 0; i < 6; i++ {
		if w.HeatmapScores[i].Cmp(zero) != 0 {
			r, c := i/3, i%3
			t.Errorf("Occupied cell (%d,%d) has non-zero score: %.4f", r, c, FixToFloat(w.HeatmapScores[i]))
		}
	}

	// Empty cells (6-8) should have positive scores
	for i := 6; i < 9; i++ {
		score := FixToFloat(w.HeatmapScores[i])
		if score <= 0 {
			r, c := i/3, i%3
			t.Errorf("Empty cell (%d,%d) has non-positive score: %.4f", r, c, score)
		}
	}
}

func TestTTTHeatmapEmptyBoard(t *testing.T) {
	// Empty board, X's turn — should match base rates (no tactical adjustments)
	state := NewTTTODEState(TTTDefaultInitialMarking())
	h := FixFromFloat(0.01)
	w := ComputeTTTHeatmapStep(state, h)

	// Expected base rates: corner=3, edge=2, center=4
	expectedRates := [9]float64{3, 2, 3, 2, 4, 2, 3, 2, 3}

	for i := 0; i < 9; i++ {
		score := FixToFloat(w.HeatmapScores[i])
		// On empty board there are no win or block flags, so score = base rate
		// However, block detection may find "threats" from the empty board configuration.
		// With no pieces placed, opponent has 0 pieces in any line, so no 2-of-3 threats.
		if score < expectedRates[i]-0.01 || score > expectedRates[i]+0.01 {
			r, c := i/3, i%3
			t.Errorf("Cell (%d,%d): expected score ~%.1f, got %.4f", r, c, expectedRates[i], score)
		}
	}
}

func TestTTTHeatmapCircuitCompiles(t *testing.T) {
	circuit := &TTTHeatmapCircuit{}
	cs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("Heatmap circuit compilation failed: %v", err)
	}

	t.Logf("Heatmap circuit constraints: %d", cs.GetNbConstraints())
	t.Logf("Heatmap public variables: %d", cs.GetNbPublicVariables())
	t.Logf("Heatmap secret variables: %d", cs.GetNbSecretVariables())

	// Public: PreStateRoot + PostStateRoot + StepSize + 9 HeatmapScores = 12
	// gnark adds 1 for constant wire, so 13
	pubVars := cs.GetNbPublicVariables()
	if pubVars != 13 {
		t.Errorf("expected 13 public variables (12 + constant), got %d", pubVars)
	}
}

func TestTTTHeatmapCircuitProof(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping heatmap proof test in short mode")
	}

	// Compile
	circuit := &TTTHeatmapCircuit{}
	cs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compilation failed: %v", err)
	}
	t.Logf("Compiled: %d constraints", cs.GetNbConstraints())

	// Setup
	pk, vk, err := groth16.Setup(cs)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Empty board witness
	state := NewTTTODEState(TTTDefaultInitialMarking())
	h := FixFromFloat(0.01)
	w := ComputeTTTHeatmapStep(state, h)
	assignment := w.ToHeatmapCircuitAssignment()

	witness, err := frontend.NewWitness(assignment, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("witness creation failed: %v", err)
	}

	proof, err := groth16.Prove(cs, pk, witness)
	if err != nil {
		t.Fatalf("prove failed: %v", err)
	}

	publicWitness, err := witness.Public()
	if err != nil {
		t.Fatalf("public witness extraction failed: %v", err)
	}

	err = groth16.Verify(proof, vk, publicWitness)
	if err != nil {
		t.Fatalf("verification failed: %v", err)
	}

	t.Log("Heatmap circuit proof verified successfully")
}

func TestTTTHeatmapCircuitBlockingProof(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping heatmap blocking proof test in short mode")
	}

	// Blocking scenario board
	board := Board{
		{"", "", ""},
		{"O", "X", ""},
		{"", "X", ""},
	}

	circuit := &TTTHeatmapCircuit{}
	cs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		t.Fatalf("compilation failed: %v", err)
	}

	pk, vk, err := groth16.Setup(cs)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	state := BoardToTTTODEState(board, "O")
	h := FixFromFloat(0.01)
	w := ComputeTTTHeatmapStep(state, h)
	assignment := w.ToHeatmapCircuitAssignment()

	witness, err := frontend.NewWitness(assignment, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("witness creation failed: %v", err)
	}

	proof, err := groth16.Prove(cs, pk, witness)
	if err != nil {
		t.Fatalf("prove failed: %v", err)
	}

	publicWitness, err := witness.Public()
	if err != nil {
		t.Fatalf("public witness extraction failed: %v", err)
	}

	err = groth16.Verify(proof, vk, publicWitness)
	if err != nil {
		t.Fatalf("verification failed: %v", err)
	}

	t.Log("Heatmap blocking scenario proof verified successfully")
}

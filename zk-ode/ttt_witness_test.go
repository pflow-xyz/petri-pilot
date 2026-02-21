package zkode

import (
	"math/big"
	"testing"
)

func TestBoardToTTTODEState(t *testing.T) {
	// Empty board, X's turn
	var board Board
	state := BoardToTTTODEState(board, "X")

	one := FixFromFloat(1.0)
	zero := FixFromFloat(0.0)

	// All cells should be 1.0
	for i := P00; i <= P22; i++ {
		if state.Marking[i].Cmp(one) != 0 {
			t.Errorf("empty board: %s should be 1.0", TTTPlaceNames[i])
		}
	}

	// x_turn should be 1, o_turn should be 0
	if state.Marking[XTurn].Cmp(one) != 0 {
		t.Error("x_turn should be 1 for X's turn")
	}
	if state.Marking[OTurn].Cmp(zero) != 0 {
		t.Error("o_turn should be 0 for X's turn")
	}

	// Root should be non-nil
	if state.Root == nil || state.Root.Sign() == 0 {
		t.Error("state root should be non-zero")
	}
}

func TestBoardToTTTODEStateWithPieces(t *testing.T) {
	board := Board{
		{"X", "", "O"},
		{"", "X", ""},
		{"", "", "O"},
	}
	state := BoardToTTTODEState(board, "X")

	one := FixFromFloat(1.0)
	zero := FixFromFloat(0.0)

	// X at (0,0): p00=0, x00=1
	if state.Marking[P00].Cmp(zero) != 0 {
		t.Error("p00 should be 0 (X placed)")
	}
	if state.Marking[X00].Cmp(one) != 0 {
		t.Error("x00 should be 1")
	}

	// O at (0,2): p02=0, o02=1
	if state.Marking[P02].Cmp(zero) != 0 {
		t.Error("p02 should be 0 (O placed)")
	}
	if state.Marking[O02].Cmp(one) != 0 {
		t.Error("o02 should be 1")
	}

	// Empty at (0,1): p01=1
	if state.Marking[P01].Cmp(one) != 0 {
		t.Error("p01 should be 1 (empty)")
	}
}

func TestNativeTTTStepEmptyBoard(t *testing.T) {
	marking := TTTDefaultInitialMarking()
	h := FixFromFloat(0.01)

	post := NativeTTTStep(marking, h)

	for p := 0; p < TTTNumPlaces; p++ {
		if post[p] == nil {
			t.Fatalf("post marking[%d] is nil", p)
		}
	}

	// Verify that markings changed (non-trivial ODE step)
	changed := false
	for p := 0; p < TTTNumPlaces; p++ {
		if marking[p].Cmp(post[p]) != 0 {
			changed = true
			break
		}
	}
	if !changed {
		t.Error("ODE step produced no change - expected non-trivial dynamics")
	}

	t.Logf("After 1 TTT step (h=0.01):")
	for p := 0; p < TTTNumPlaces; p++ {
		pre := FixToFloat(marking[p])
		postVal := FixToFloat(post[p])
		if pre != postVal {
			t.Logf("  %s: %.6f -> %.6f (delta=%.6f)", TTTPlaceNames[p], pre, postVal, postVal-pre)
		}
	}
}

func TestTTTHeatmapRootChaining(t *testing.T) {
	state := NewTTTODEState(TTTDefaultInitialMarking())
	h := FixFromFloat(0.01)

	w1 := ComputeTTTHeatmapStep(state, h)
	w2 := ComputeTTTHeatmapStep(w1.PostState, h)

	// Post root of step 1 should equal pre root of step 2
	if w1.PostState.Root.Cmp(w2.PreState.Root) != 0 {
		t.Error("root chain broken: post root of step 1 != pre root of step 2")
	}

	// All three roots should be different
	if state.Root.Cmp(w1.PostState.Root) == 0 {
		t.Error("root should change after step 1")
	}
	if w1.PostState.Root.Cmp(w2.PostState.Root) == 0 {
		t.Error("root should change after step 2")
	}
}

func TestTTTDefaultGenesisRoot(t *testing.T) {
	state := NewTTTODEState(TTTDefaultInitialMarking())
	t.Logf("TTT genesis root (empty board, X turn): 0x%s", state.Root.Text(16))

	// Root should be deterministic
	state2 := NewTTTODEState(TTTDefaultInitialMarking())
	if state.Root.Cmp(state2.Root) != 0 {
		t.Error("genesis root is not deterministic")
	}
}

func TestToHeatmapCircuitAssignment(t *testing.T) {
	state := NewTTTODEState(TTTDefaultInitialMarking())
	h := FixFromFloat(0.01)
	w := ComputeTTTHeatmapStep(state, h)

	assignment := w.ToHeatmapCircuitAssignment()

	if assignment.PreStateRoot == nil {
		t.Error("PreStateRoot is nil")
	}
	if assignment.PostStateRoot == nil {
		t.Error("PostStateRoot is nil")
	}

	for p := 0; p < TTTNumPlaces; p++ {
		if assignment.PreMarking[p] == nil {
			t.Errorf("PreMarking[%d] is nil", p)
		}
		if assignment.PostMarking[p] == nil {
			t.Errorf("PostMarking[%d] is nil", p)
		}
	}

	for i := 0; i < 9; i++ {
		if assignment.HeatmapScores[i] == nil {
			t.Errorf("HeatmapScores[%d] is nil", i)
		}
	}
}

func TestBoardWithMoveToTTTState(t *testing.T) {
	// Board after X plays center
	board := Board{
		{"", "", ""},
		{"", "X", ""},
		{"", "", ""},
	}
	// It's O's turn after X plays
	state := BoardToTTTODEState(board, "O")

	one := FixFromFloat(1.0)
	zero := FixFromFloat(0.0)

	// Center cell occupied by X
	if state.Marking[P11].Cmp(zero) != 0 {
		t.Error("p11 should be 0 (X placed at center)")
	}
	if state.Marking[X11].Cmp(one) != 0 {
		t.Error("x11 should be 1")
	}

	// O's turn
	if state.Marking[OTurn].Cmp(one) != 0 {
		t.Error("o_turn should be 1")
	}
	if state.Marking[XTurn].Cmp(zero) != 0 {
		t.Error("x_turn should be 0")
	}

	// Check heatmap scores: O play positions should reflect expected rates
	expectedK := [9]float64{3, 2, 3, 2, 4, 2, 3, 2, 3}
	w := ComputeTTTHeatmapStep(state, FixFromFloat(0.01))
	for i := 0; i < 9; i++ {
		score := FixToFloat(w.HeatmapScores[i])
		if i == 4 { // center is occupied
			if score != 0 {
				t.Errorf("cell %d (occupied): expected score 0, got %.6f", i, score)
			}
		} else {
			// On this board, no tactical adjustments apply yet (no 2-in-a-row threats)
			k := expectedK[i]
			if score < k-0.01 || score > k+0.01 {
				t.Errorf("cell %d (empty): expected score ~%.1f (k=%.0f), got %.6f", i, k, k, score)
			}
		}
	}
}

func TestApplyDiscreteMove(t *testing.T) {
	marking := TTTDefaultInitialMarking()
	one := FixFromFloat(1.0)
	zero := FixFromFloat(0.0)

	// Apply x_play_11 (X plays center)
	post := ApplyDiscreteMove(marking, TXPlay11)

	// p11 should go from 1 to 0 (consumed)
	if post[P11].Cmp(zero) != 0 {
		t.Errorf("p11: expected 0, got %s", post[P11].Text(10))
	}
	// x11 should go from 0 to 1 (produced)
	if post[X11].Cmp(one) != 0 {
		t.Errorf("x11: expected 1, got %s", post[X11].Text(10))
	}
	// x_turn should go from 1 to 0
	if post[XTurn].Cmp(zero) != 0 {
		t.Errorf("x_turn: expected 0, got %s", post[XTurn].Text(10))
	}
	// o_turn should go from 0 to 1
	if post[OTurn].Cmp(one) != 0 {
		t.Errorf("o_turn: expected 1, got %s", post[OTurn].Text(10))
	}

	// All other cells should be unchanged
	for i := P00; i <= P22; i++ {
		if i == P11 {
			continue
		}
		if post[i].Cmp(marking[i]) != 0 {
			t.Errorf("%s: expected unchanged, got %s", TTTPlaceNames[i], post[i].Text(10))
		}
	}

	// Verify the discrete post-move board matches BoardToTTTODEState
	board := Board{
		{"", "", ""},
		{"", "X", ""},
		{"", "", ""},
	}
	expected := BoardToTTTODEState(board, "O")
	for p := 0; p < TTTNumPlaces; p++ {
		if post[p].Cmp(expected.Marking[p]) != 0 {
			t.Errorf("%s: ApplyDiscreteMove=%s, BoardToTTTODEState=%s",
				TTTPlaceNames[p], post[p].Text(10), expected.Marking[p].Text(10))
		}
	}

	// Verify root matches
	postRoot := ComputeRoot(post[:])
	if postRoot.Cmp(expected.Root) != 0 {
		t.Errorf("root mismatch: ApplyDiscreteMove=0x%s, BoardToTTTODEState=0x%s",
			postRoot.Text(16), expected.Root.Text(16))
	}

	t.Logf("Discrete post-move root (X@center): 0x%s", postRoot.Text(16))
}

func TestApplyDiscreteMoveOTurn(t *testing.T) {
	// Start from X@center board, O's turn
	marking := TTTDefaultInitialMarking()
	afterX := ApplyDiscreteMove(marking, TXPlay11)

	one := FixFromFloat(1.0)
	zero := FixFromFloat(0.0)

	// Apply o_play_00 (O plays corner)
	post := ApplyDiscreteMove(afterX, TOPlay00)

	// p00 consumed, o00 produced
	if post[P00].Cmp(zero) != 0 {
		t.Errorf("p00: expected 0, got %s", post[P00].Text(10))
	}
	if post[O00].Cmp(one) != 0 {
		t.Errorf("o00: expected 1, got %s", post[O00].Text(10))
	}
	// Turn should swap back to X
	if post[XTurn].Cmp(one) != 0 {
		t.Errorf("x_turn: expected 1, got %s", post[XTurn].Text(10))
	}
	if post[OTurn].Cmp(zero) != 0 {
		t.Errorf("o_turn: expected 0, got %s", post[OTurn].Text(10))
	}

	t.Logf("After O@(0,0) root: 0x%s", ComputeRoot(post[:]).Text(16))
}

func TestNativeTTTStepConservation(t *testing.T) {
	marking := TTTDefaultInitialMarking()
	h := FixFromFloat(0.01)

	preTotal := big.NewInt(0)
	for i := 0; i < 9; i++ {
		preTotal = NativeFixAdd(preTotal, marking[P00+i])
		preTotal = NativeFixAdd(preTotal, marking[X00+i])
		preTotal = NativeFixAdd(preTotal, marking[O00+i])
	}

	post := NativeTTTStep(marking, h)

	postTotal := big.NewInt(0)
	for i := 0; i < 9; i++ {
		postTotal = NativeFixAdd(postTotal, post[P00+i])
		postTotal = NativeFixAdd(postTotal, post[X00+i])
		postTotal = NativeFixAdd(postTotal, post[O00+i])
	}

	preFloat := FixToFloat(preTotal)
	postFloat := FixToFloat(postTotal)

	t.Logf("Cell+piece total: pre=%.10f post=%.10f delta=%.2e", preFloat, postFloat, postFloat-preFloat)

	if diff := postFloat - preFloat; diff < -0.001 || diff > 0.001 {
		t.Errorf("cell+piece conservation violated: delta=%.6f", diff)
	}
}

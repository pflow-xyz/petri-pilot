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

	// After one step from empty board with X's turn:
	// - x_play transitions fire (rate = k * cell * x_turn; k=4 center, 3 corner, 2 edge)
	// - o_play transitions are disabled (o_turn = 0, so rate = k * cell * 0 = 0)
	// - win transitions disabled (no pieces placed, so product of inputs = 0)
	//
	// The empty cells should decrease slightly (consumed by x_play)
	// x_turn should decrease, o_turn should increase
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

func TestComputeTTTStep(t *testing.T) {
	state := NewTTTODEState(TTTDefaultInitialMarking())
	h := FixFromFloat(0.01)

	w := ComputeTTTStep(state, h)

	// Verify witness fields
	if w.PreState.Root.Cmp(state.Root) != 0 {
		t.Error("pre state root mismatch")
	}
	if w.PostState.Root == nil || w.PostState.Root.Sign() == 0 {
		t.Error("post state root should be non-zero")
	}
	if w.StepSize.Cmp(h) != 0 {
		t.Error("step size mismatch")
	}

	// Check that actual rates are computed
	for tr := 0; tr < TTTNumTransitions; tr++ {
		if w.ActualRates[tr] == nil {
			t.Fatalf("actual rate[%d] is nil", tr)
		}
	}

	// x_play_00 rate should be 3.0 (p00=1, x_turn=1, product=1, k=3 for corner)
	xPlayCornerRate := FixToFloat(w.ActualRates[TXPlay00])
	if xPlayCornerRate < 2.99 || xPlayCornerRate > 3.01 {
		t.Errorf("x_play_00 rate: expected ~3.0 (corner k=3), got %.6f", xPlayCornerRate)
	}

	// x_play_11 rate should be 4.0 (p11=1, x_turn=1, product=1, k=4 for center)
	xPlayCenterRate := FixToFloat(w.ActualRates[TXPlay11])
	if xPlayCenterRate < 3.99 || xPlayCenterRate > 4.01 {
		t.Errorf("x_play_11 rate: expected ~4.0 (center k=4), got %.6f", xPlayCenterRate)
	}

	// x_play_01 rate should be 2.0 (p01=1, x_turn=1, product=1, k=2 for edge)
	xPlayEdgeRate := FixToFloat(w.ActualRates[TXPlay01])
	if xPlayEdgeRate < 1.99 || xPlayEdgeRate > 2.01 {
		t.Errorf("x_play_01 rate: expected ~2.0 (edge k=2), got %.6f", xPlayEdgeRate)
	}

	// o_play_00 rate should be 0.0 (p00=1, o_turn=0, product=0, k irrelevant)
	oPlayRate := FixToFloat(w.ActualRates[TOPlay00])
	if oPlayRate != 0 {
		t.Errorf("o_play_00 rate: expected 0.0, got %.6f", oPlayRate)
	}

	// Win transitions should have rate 0 (no pieces placed)
	for i := 0; i < 8; i++ {
		xWinRate := FixToFloat(w.ActualRates[TXWinRow0+i])
		if xWinRate != 0 {
			t.Errorf("x_win_%d rate: expected 0.0, got %.6f", i, xWinRate)
		}
	}
}

func TestTTTRootChaining(t *testing.T) {
	state := NewTTTODEState(TTTDefaultInitialMarking())
	h := FixFromFloat(0.01)

	w1 := ComputeTTTStep(state, h)
	w2 := ComputeTTTStep(w1.PostState, h)

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

func TestToCircuitAssignment(t *testing.T) {
	state := NewTTTODEState(TTTDefaultInitialMarking())
	h := FixFromFloat(0.01)
	w := ComputeTTTStep(state, h)

	assignment := w.ToCircuitAssignment()

	// Verify all fields are populated
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

	for tr := 0; tr < TTTNumTransitions; tr++ {
		if assignment.ActualRates[tr] == nil {
			t.Errorf("ActualRates[%d] is nil", tr)
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

	// Check rates: o_play transitions should be enabled for empty cells
	// Rates now include position-based rate constants (k)
	expectedK := [9]float64{3, 2, 3, 2, 4, 2, 3, 2, 3}
	w := ComputeTTTStep(state, FixFromFloat(0.01))
	for i := 0; i < 9; i++ {
		oPlayRate := FixToFloat(w.ActualRates[TOPlay00+i])
		if i == 4 { // center is occupied
			if oPlayRate != 0 {
				t.Errorf("o_play_%d (occupied cell): expected rate 0, got %.6f", i, oPlayRate)
			}
		} else {
			k := expectedK[i]
			if oPlayRate < k-0.01 || oPlayRate > k+0.01 {
				t.Errorf("o_play_%d (empty cell): expected rate ~%.1f (k=%.0f), got %.6f", i, k, k, oPlayRate)
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
	// In the TTT net, total tokens across cells + pieces should be conserved
	// (each play moves 1 token from cell to piece). Turn tokens swap. Game state changes on win.
	// With small h and no wins possible, the total should be approximately conserved.
	marking := TTTDefaultInitialMarking()
	h := FixFromFloat(0.01)

	// Total tokens in cell + piece pairs
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

	// With play transitions only (no wins), cell+piece tokens are conserved
	// (play removes from cell, adds to piece). The difference should be very small.
	if diff := postFloat - preFloat; diff < -0.001 || diff > 0.001 {
		t.Errorf("cell+piece conservation violated: delta=%.6f", diff)
	}
}

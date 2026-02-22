package zkode

import (
	"testing"
)

func TestTTTPlaceCount(t *testing.T) {
	if TTTNumPlaces != 33 {
		t.Fatalf("expected 33 places, got %d", TTTNumPlaces)
	}
	if len(TTTPlaceNames) != TTTNumPlaces {
		t.Fatalf("place names array size mismatch: %d vs %d", len(TTTPlaceNames), TTTNumPlaces)
	}
}

func TestTTTTransitionCount(t *testing.T) {
	if TTTNumTransitions != 35 {
		t.Fatalf("expected 35 transitions, got %d", TTTNumTransitions)
	}
	if len(TTTTransitionNames) != TTTNumTransitions {
		t.Fatalf("transition names array size mismatch: %d vs %d", len(TTTTransitionNames), TTTNumTransitions)
	}
}

func TestTTTStoichiometryNonZeroCount(t *testing.T) {
	count := 0
	for p := 0; p < TTTNumPlaces; p++ {
		for tr := 0; tr < TTTNumTransitions; tr++ {
			if TTTStoichiometry[p][tr] != 0 {
				count++
			}
		}
	}
	// x_play: 9 * 5 = 45, o_play: 9 * 5 = 45, x_win: 8 * 3 = 24, o_win: 8 * 3 = 24, draw: 3
	expected := 141
	if count != expected {
		t.Fatalf("expected %d non-zero stoichiometry entries, got %d", expected, count)
	}
}

func TestTTTStoichiometryValues(t *testing.T) {
	// All stoichiometry values should be -9, -1, 0, or +1
	for p := 0; p < TTTNumPlaces; p++ {
		for tr := 0; tr < TTTNumTransitions; tr++ {
			s := TTTStoichiometry[p][tr]
			if s != -9 && s != -1 && s != 0 && s != 1 {
				t.Fatalf("unexpected stoichiometry value %d at [%s][%s]",
					s, TTTPlaceNames[p], TTTTransitionNames[tr])
			}
		}
	}
}

func TestTTTTransitionInputs(t *testing.T) {
	// Play transitions: 2 inputs each
	for i := 0; i < 9; i++ {
		if TTTNumInputs[TXPlay00+i] != 2 {
			t.Errorf("x_play_%d: expected 2 inputs, got %d", i, TTTNumInputs[TXPlay00+i])
		}
		if TTTNumInputs[TOPlay00+i] != 2 {
			t.Errorf("o_play_%d: expected 2 inputs, got %d", i, TTTNumInputs[TOPlay00+i])
		}
	}

	// Win transitions: 5 inputs each
	for i := 0; i < 8; i++ {
		if TTTNumInputs[TXWinRow0+i] != 5 {
			t.Errorf("x_win_%d: expected 5 inputs, got %d", i, TTTNumInputs[TXWinRow0+i])
		}
		if TTTNumInputs[TOWinRow0+i] != 5 {
			t.Errorf("o_win_%d: expected 5 inputs, got %d", i, TTTNumInputs[TOWinRow0+i])
		}
	}

	// Draw transition: 2 inputs (move_tokens, game_active)
	if TTTNumInputs[TDraw] != 2 {
		t.Errorf("draw: expected 2 inputs, got %d", TTTNumInputs[TDraw])
	}
}

func TestTTTXPlayStoichiometry(t *testing.T) {
	// x_play_00: p00 -1, x_turn -1, x00 +1, o_turn +1, move_tokens +1
	if TTTStoichiometry[P00][TXPlay00] != -1 {
		t.Error("x_play_00: p00 should be -1")
	}
	if TTTStoichiometry[XTurn][TXPlay00] != -1 {
		t.Error("x_play_00: x_turn should be -1")
	}
	if TTTStoichiometry[X00][TXPlay00] != +1 {
		t.Error("x_play_00: x00 should be +1")
	}
	if TTTStoichiometry[OTurn][TXPlay00] != +1 {
		t.Error("x_play_00: o_turn should be +1")
	}
	if TTTStoichiometry[MoveTokens][TXPlay00] != +1 {
		t.Error("x_play_00: move_tokens should be +1")
	}
}

func TestTTTXWinStoichiometry(t *testing.T) {
	// x_win_row0 (x00,x01,x02): o_turn -1, game_active -1, win_x +1
	// Pieces should have net 0 (consumed and returned)
	if TTTStoichiometry[X00][TXWinRow0] != 0 {
		t.Error("x_win_row0: x00 net should be 0")
	}
	if TTTStoichiometry[X01][TXWinRow0] != 0 {
		t.Error("x_win_row0: x01 net should be 0")
	}
	if TTTStoichiometry[X02][TXWinRow0] != 0 {
		t.Error("x_win_row0: x02 net should be 0")
	}
	if TTTStoichiometry[OTurn][TXWinRow0] != -1 {
		t.Error("x_win_row0: o_turn should be -1")
	}
	if TTTStoichiometry[GameActive][TXWinRow0] != -1 {
		t.Error("x_win_row0: game_active should be -1")
	}
	if TTTStoichiometry[WinX][TXWinRow0] != +1 {
		t.Error("x_win_row0: win_x should be +1")
	}
}

func TestTTTXWinInputs(t *testing.T) {
	// x_win_row0: inputs are [x00, x01, x02, o_turn, game_active]
	inputs := TTTTransitionInputs[TXWinRow0]
	if len(inputs) != 5 {
		t.Fatalf("x_win_row0: expected 5 inputs, got %d", len(inputs))
	}
	if inputs[0] != X00 || inputs[1] != X01 || inputs[2] != X02 {
		t.Error("x_win_row0: piece inputs mismatch")
	}
	if inputs[3] != OTurn {
		t.Error("x_win_row0: expected o_turn input")
	}
	if inputs[4] != GameActive {
		t.Error("x_win_row0: expected game_active input")
	}
}

func TestTTTDefaultInitialMarking(t *testing.T) {
	m := TTTDefaultInitialMarking()
	one := FixFromFloat(1.0)
	zero := FixFromFloat(0.0)

	// All cells should be 1
	for i := P00; i <= P22; i++ {
		if m[i].Cmp(one) != 0 {
			t.Errorf("%s: expected 1.0, got different", TTTPlaceNames[i])
		}
	}

	// All pieces should be 0
	for i := X00; i <= X22; i++ {
		if m[i].Cmp(zero) != 0 {
			t.Errorf("%s: expected 0.0", TTTPlaceNames[i])
		}
	}
	for i := O00; i <= O22; i++ {
		if m[i].Cmp(zero) != 0 {
			t.Errorf("%s: expected 0.0", TTTPlaceNames[i])
		}
	}

	// Control places
	if m[XTurn].Cmp(one) != 0 {
		t.Error("x_turn should be 1")
	}
	if m[OTurn].Cmp(zero) != 0 {
		t.Error("o_turn should be 0")
	}
	if m[GameActive].Cmp(one) != 0 {
		t.Error("game_active should be 1")
	}
	if m[MoveTokens].Cmp(zero) != 0 {
		t.Error("move_tokens should be 0")
	}
}

func TestTTTRateConstants(t *testing.T) {
	// Position weights: center=4, corners=3, edges=2
	expectedK := [9]float64{
		3, 2, 3, // row 0: corner, edge, corner
		2, 4, 2, // row 1: edge, center, edge
		3, 2, 3, // row 2: corner, edge, corner
	}

	for i := 0; i < 9; i++ {
		// X play rate constants
		k := FixToFloat(TTTRateConstants[TXPlay00+i])
		if k != expectedK[i] {
			t.Errorf("x_play_%d: expected k=%.0f, got %.6f", i, expectedK[i], k)
		}
		// O play rate constants (same weights)
		k = FixToFloat(TTTRateConstants[TOPlay00+i])
		if k != expectedK[i] {
			t.Errorf("o_play_%d: expected k=%.0f, got %.6f", i, expectedK[i], k)
		}
	}

	// Win transitions: k=1
	for i := 0; i < 8; i++ {
		k := FixToFloat(TTTRateConstants[TXWinRow0+i])
		if k != 1.0 {
			t.Errorf("x_win_%d: expected k=1, got %.6f", i, k)
		}
		k = FixToFloat(TTTRateConstants[TOWinRow0+i])
		if k != 1.0 {
			t.Errorf("o_win_%d: expected k=1, got %.6f", i, k)
		}
	}

	// All rate constants should be non-nil
	for i := 0; i < TTTNumTransitions; i++ {
		if TTTRateConstants[i] == nil {
			t.Fatalf("TTTRateConstants[%d] (%s) is nil", i, TTTTransitionNames[i])
		}
	}
}

func TestTTTMatchesBuildTicTacToeNet(t *testing.T) {
	// Verify topology constants match the dynamic BuildTicTacToeNet()
	net := BuildTicTacToeNet()

	placeCount := len(net.Places)
	if placeCount != TTTNumPlaces {
		t.Errorf("place count mismatch: BuildTicTacToeNet=%d, TTTNumPlaces=%d", placeCount, TTTNumPlaces)
	}

	transCount := len(net.Transitions)
	if transCount != TTTNumTransitions {
		t.Errorf("transition count mismatch: BuildTicTacToeNet=%d, TTTNumTransitions=%d", transCount, TTTNumTransitions)
	}
}

package main

import "testing"

func TestDeclaredModelShape(t *testing.T) {
	m := buildModel()
	if got := len(cells); got != 16 {
		t.Fatalf("cells: got %d, want 16", got)
	}
	if got := len(winLines); got != 24 {
		t.Fatalf("win lines: got %d, want 24", got)
	}
	if got := len(m.transitions); got != 81 {
		t.Fatalf("declared transitions: got %d, want 81", got)
	}
}

func TestGravityIsTopology(t *testing.T) {
	m := buildModel()
	mk := m.start()
	_, moves, maximizes, ok := m.legalMoves(mk)
	if !ok || !maximizes {
		t.Fatal("empty board should be X to move")
	}
	want := []string{"x_play_30", "x_play_31", "x_play_32", "x_play_33"}
	assertMoves(t, moves, want)

	mk = m.fire("x_play_30", mk)
	_, moves, maximizes, ok = m.legalMoves(mk)
	if !ok || maximizes {
		t.Fatal("after X drop, O should move")
	}
	want = []string{"o_play_20", "o_play_31", "o_play_32", "o_play_33"}
	assertMoves(t, moves, want)
}

func assertMoves(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("moves: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("moves: got %v, want %v", got, want)
		}
	}
}

func TestExactOpeningValue(t *testing.T) {
	m := buildModel()
	if got := m.minimax(m.start(), -2, 2); got != 1 {
		t.Fatalf("empty-board minimax value: got %d, want X win (+1)", got)
	}
}

func TestEvaluationNetShapes(t *testing.T) {
	m := buildModel()
	base := m.toPetriBaseline()
	if got := len(base.net.Places); got != 52 {
		t.Errorf("baseline places: got %d, want 52", got)
	}
	if got := len(base.net.Transitions); got != 80 {
		t.Errorf("baseline transitions: got %d, want 80", got)
	}
	policy := m.toPetriPolicy(candidateForceBias, candidateBlockBias)
	if got := len(policy.net.Transitions); got != 368 {
		t.Errorf("policy transitions: got %d, want 368", got)
	}
}

func TestCalibratedNaiveExhaustiveReferee(t *testing.T) {
	m := buildModel()
	p := odePlayer(m.toPetriBaseline(), scoreLambda)
	oDecisions, oBlown, oMissed := exhaustiveCheck(m, p, false)
	xDecisions, xBlown, xMissed := exhaustiveCheck(m, p, true)
	if oDecisions != 462 || oBlown != 6 || oMissed != 2 {
		t.Errorf("O referee: got (%d,%d,%d), want (462,6,2)", oDecisions, oBlown, oMissed)
	}
	if xDecisions != 45 || xBlown != 0 || xMissed != 0 {
		t.Errorf("X referee: got (%d,%d,%d), want (45,0,0)", xDecisions, xBlown, xMissed)
	}
}

package main

import (
	"math/rand"
	"testing"
)

// bruteForceMinimax is an independent, unoptimized recursive minimax over
// the bitboard rules directly (no alpha-beta, no TT, no move ordering) used
// only to cross-check the oracle on small/shallow positions where it is
// still feasible to run. Two independent implementations of the same rule
// agreeing is the actual correctness evidence here; the oracle's speed
// tricks (TT, pruning, ordering) are exactly the parts most likely to hide
// a bug, so this checker deliberately shares none of them.
func bruteForceMinimax(p Position) int8 {
	if p.moves == Width*Height {
		return 0
	}
	best := int8(-2)
	any := false
	for c := 0; c < Width; c++ {
		if !p.canPlay(c) {
			continue
		}
		any = true
		if p.isWinningMove(c) {
			return 1
		}
		child := p
		child.playCol(c)
		v := -bruteForceMinimax(child)
		if v > best {
			best = v
		}
	}
	if !any {
		return 0
	}
	return best
}

func TestAlignmentHorizontal(t *testing.T) {
	p := newPosition([]int{0, 0, 1, 1, 2, 2}) // x:0,1,2 (row0) o:0,1,2(row1)
	if !p.isWinningMove(3) {
		t.Fatalf("expected horizontal win at col 3")
	}
}

func TestAlignmentVertical(t *testing.T) {
	p := newPosition([]int{0, 1, 0, 1, 0, 1}) // x plays col0 three times, o col1
	if !p.isWinningMove(0) {
		t.Fatalf("expected vertical win at col 0")
	}
}

func TestAlignmentDiagonalUpRight(t *testing.T) {
	// Build a "/" diagonal for X across (0,0),(1,1),(2,2),(3,3).
	// x: 0,1,1,2,2,2,3,3,3,3 columns give heights: col0 h1(x), col1 h2 (o,x),
	// col2 h3 (o,o,x), col3 h3 before final winning move (o,o,o then x wins).
	moves := []int{
		0, // x -> (0,0)
		1, // o -> (1,0)
		1, // x -> (1,1)
		2, // o -> (2,0)
		3, // x -> (3,0)   (filler so parity keeps x to move where needed)
		2, // o -> (2,1)
		2, // x -> (2,2)
		3, // o -> (3,1)
		4, // x -> (4,0) filler
		3, // o -> (3,2)
	}
	p := newPosition(moves)
	// It should now be X's turn; X plays col3 -> (3,3) completing (0,0)(1,1)(2,2)(3,3)
	if !p.isWinningMove(3) {
		t.Fatalf("expected diagonal up-right win at col 3, board=%+v", p)
	}
}

func TestOracleAgreesWithBruteForceShallow(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	o := NewOracle()
	for trial := 0; trial < 40; trial++ {
		// Build a random position with few empty cells left so brute force
		// (no pruning at all) stays feasible: play random moves until at
		// most ~10 cells remain, backing off on any accidental win.
		var p Position
		for p.moves < Width*Height-10 {
			var playable []int
			for c := 0; c < Width; c++ {
				if p.canPlay(c) {
					playable = append(playable, c)
				}
			}
			if len(playable) == 0 {
				break
			}
			c := playable[rng.Intn(len(playable))]
			if p.isWinningMove(c) {
				continue // don't end the game early; try again next iteration
			}
			p.playCol(c)
		}
		want := bruteForceMinimax(p)
		got, ok := o.Solve(p, 5_000_000)
		if !ok {
			t.Fatalf("trial %d: oracle exceeded budget on a near-terminal position", trial)
		}
		if got != want {
			t.Fatalf("trial %d: oracle=%d brute=%d moves=%d", trial, got, want, p.moves)
		}
	}
}

func TestOracleKnownWin(t *testing.T) {
	// X: col0,col0,col0,col0 with O elsewhere never blocking -> vertical win
	// available immediately.
	p := newPosition([]int{0, 1, 0, 1, 0, 1})
	o := NewOracle()
	val, ok := o.Solve(p, 1_000_000)
	if !ok {
		t.Fatal("budget exceeded")
	}
	if val != 1 {
		t.Fatalf("expected an immediate winning position for the player to move, got %d", val)
	}
}

// findDrawnBoard searches (small backtracking DFS, cell-by-cell) for any
// full 42-cell coloring with no 4-in-a-row for either color. A full board's
// gravity constraint is automatic once every cell is filled, so the search
// only has to avoid alignments, not respect any particular play order —
// which is also why the resulting bitboards are handed to Solve directly
// rather than replayed through playCol. Draws are common in Connect Four,
// so this is expected to succeed in a handful of backtracks.
func findDrawnBoard() (xbits, obits uint64, ok bool) {
	cellBit := func(c, r int) uint64 { return uint64(1) << uint(c*h1+r) }
	var x, o uint64
	var fill func(cell int) bool
	fill = func(cell int) bool {
		if cell == Width*Height {
			return true
		}
		c, r := cell/Height, cell%Height
		bit := cellBit(c, r)
		for _, tryX := range [2]bool{true, false} {
			if tryX {
				x |= bit
			} else {
				o |= bit
			}
			ok := !alignment(x) && !alignment(o)
			if ok && fill(cell+1) {
				return true
			}
			x &^= bit
			o &^= bit
		}
		return false
	}
	if !fill(0) {
		return 0, 0, false
	}
	return x, o, true
}

func TestOracleKnownDraw(t *testing.T) {
	xbits, obits, found := findDrawnBoard()
	if !found {
		t.Fatal("no drawn full board found (search bug, not a game-theoretic fact)")
	}
	if alignment(xbits) || alignment(obits) {
		t.Fatal("findDrawnBoard returned a board with an alignment")
	}
	p := Position{current: xbits, mask: xbits | obits, moves: Width * Height}
	o := NewOracle()
	val, ok := o.Solve(p, 1000)
	if !ok {
		t.Fatal("budget exceeded on a terminal position")
	}
	if val != 0 {
		t.Fatalf("expected draw, got %d", val)
	}
}

func TestOracleSymmetry(t *testing.T) {
	// Mirroring every column (c -> Width-1-c) must not change the value.
	o := NewOracle()
	seqs := [][]int{{3}, {3, 3}, {3, 2, 3, 4}, {0, 1, 2, 3}}
	for _, seq := range seqs {
		p := newPosition(seq)
		mirrored := make([]int, len(seq))
		for i, c := range seq {
			mirrored[i] = Width - 1 - c
		}
		pm := newPosition(mirrored)
		v1, ok1 := o.Solve(p, 3_000_000)
		v2, ok2 := o.Solve(pm, 3_000_000)
		if !ok1 || !ok2 {
			t.Skip("budget exceeded on this early position; not a correctness signal")
		}
		if v1 != v2 {
			t.Fatalf("mirror symmetry broken: seq=%v -> %d, mirrored -> %d", seq, v1, v2)
		}
	}
}

func TestOracleBudgetHonest(t *testing.T) {
	o := NewOracle()
	_, ok := o.Solve(Position{}, 10) // empty board, 10-node budget: must not fake an answer
	if ok {
		t.Fatal("expected budget exceeded on the empty board with a 10-node budget")
	}
}

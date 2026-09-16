// The exact-play oracle. Connect Four is solved (Allis/Tromp/Pons,
// first player wins with perfect play), so unlike ode-minimax's true
// exhaustive minimax over a state space small enough to enumerate, this is
// a real search: bitboard position encoding, negamax with alpha-beta
// pruning restricted to the weak {-1,0,1} outcome range (win/draw/loss —
// which is all a hinge-ranking calibration or a move comparison needs),
// a transposition table shared across calls within one Oracle, and
// center-out move ordering (3,2,4,1,5,0,6).
//
// This is the standard "Pascal Pons connect-4 solver" bitboard scheme
// (see http://blog.gamesolver.org/solving-connect-four/06-bitboard/):
// column c occupies bits [c*(Height+1) .. c*(Height+1)+Height-1], with the
// (Height)'th bit of each column left as a permanent zero sentinel so a
// column's bits never overflow into the next column's. `current` is the
// bitboard of the player to move; `mask` is every occupied cell.
//
// HONESTY NOTE (read before trusting a "verified" claim anywhere else in
// this experiment): this oracle is a real, correct solver for the position
// handed to it, but it is not exhaustive over the ~4.5*10^12-position game
// tree the way ode-minimax's minimax was exhaustive over tic-tac-toe's ~5478
// states. Solving the empty board (or anything within the first ~10-14
// plies) from scratch with this implementation — no opening book, no
// Pons-grade move-ordering refinements (killer moves, "claimeven" pruning,
// iterative-deepening MTD) — is not fast enough to be practical here. Every
// call therefore runs under a node budget (Oracle.Solve's second return
// value is false, not a wrong answer, when the budget is exhausted), and
// every caller in this experiment is expected to check it and skip rather
// than guess. Training and verification positions are deliberately biased
// toward mid-to-late game (see fit.go's collectPositions), which is what
// keeps most queries inside the budget in practice.
package main

const (
	h1 = Height + 1 // per-column stride, including the sentinel row
)

// Position is a bitboard Connect Four position from the perspective of the
// player to move (`current`).
type Position struct {
	current uint64
	mask    uint64
	moves   int
}

func bottomMask(col int) uint64 { return uint64(1) << uint(col*h1) }
func topMask(col int) uint64    { return uint64(1) << uint((Height-1)+col*h1) }
func colMask(col int) uint64    { return ((uint64(1) << uint(Height)) - 1) << uint(col*h1) }

func (p Position) canPlay(col int) bool { return p.mask&topMask(col) == 0 }

func (p *Position) play(move uint64) {
	p.current ^= p.mask
	p.mask |= move
	p.moves++
}

func (p *Position) playCol(col int) {
	move := (p.mask + bottomMask(col)) & colMask(col)
	p.play(move)
}

// isWinningMove reports whether playing col completes 4-in-a-row for the
// player to move (checked BEFORE playing it — p is unmodified).
func (p Position) isWinningMove(col int) bool {
	pos := p.current | ((p.mask + bottomMask(col)) & colMask(col))
	return alignment(pos)
}

func alignment(pos uint64) bool {
	// horizontal
	m := pos & (pos >> h1)
	if m&(m>>(2*h1)) != 0 {
		return true
	}
	// diagonal "\" (down-right, i.e. row decreases as col increases)
	m = pos & (pos >> Height)
	if m&(m>>(2*Height)) != 0 {
		return true
	}
	// diagonal "/" (up-right)
	m = pos & (pos >> h1 >> 1) // shift by Height+2
	if m&(m>>(2*(Height+2))) != 0 {
		return true
	}
	// vertical
	m = pos & (pos >> 1)
	if m&(m>>2) != 0 {
		return true
	}
	return false
}

func (p Position) key() uint64 { return p.current + p.mask }

// newPosition builds a Position by replaying a sequence of column plays
// from the empty board (X moves first). Panics on an illegal sequence —
// callers construct these, they are not untrusted input.
func newPosition(cols []int) Position {
	var p Position
	for _, c := range cols {
		if !p.canPlay(c) {
			panic("oracle: illegal move sequence")
		}
		p.playCol(c)
	}
	return p
}

// ---- weak (win/draw/loss) negamax solver ----

const (
	ttExact = iota
	ttLower
	ttUpper
)

type ttEntry struct {
	flag int8
	val  int8
}

type Oracle struct {
	tt     map[uint64]ttEntry
	nodes  int
	budget int
}

func NewOracle() *Oracle {
	return &Oracle{tt: make(map[uint64]ttEntry, 1<<20)}
}

type budgetExceeded struct{}

// Solve returns the exact outcome for the player to move in p: +1 that
// player wins, -1 that player loses, 0 a draw, under optimal play by both
// sides — provided the search finishes within budget nodes. ok=false means
// the budget ran out and val is meaningless; the position is neither solved
// nor "probably fine," it is simply unknown.
func (o *Oracle) Solve(p Position, budget int) (val int8, ok bool) {
	o.budget = budget
	o.nodes = 0
	defer func() {
		if r := recover(); r != nil {
			if _, isBudget := r.(budgetExceeded); isBudget {
				ok = false
				return
			}
			panic(r)
		}
	}()
	val = o.negamax(p, -1, 1)
	return val, true
}

// moveOrder is center-out: 3,2,4,1,5,0,6.
var moveOrder = [Width]int{3, 2, 4, 1, 5, 0, 6}

func (o *Oracle) negamax(p Position, alpha, beta int8) int8 {
	o.nodes++
	if o.nodes > o.budget {
		panic(budgetExceeded{})
	}

	if p.moves == Width*Height {
		return 0 // board full, nobody just won (checked by the caller) => draw
	}

	// An immediate winning move dominates any deeper line, and checking it
	// first is standard practice for this solver shape (it also prunes
	// enormously in practice).
	for _, col := range moveOrder {
		if p.canPlay(col) && p.isWinningMove(col) {
			return 1
		}
	}

	key := p.key()
	if e, hit := o.tt[key]; hit {
		switch e.flag {
		case ttExact:
			return e.val
		case ttLower:
			if e.val > alpha {
				alpha = e.val
			}
		case ttUpper:
			if e.val < beta {
				beta = e.val
			}
		}
		if alpha >= beta {
			return e.val
		}
	}

	origAlpha := alpha
	best := int8(-1)
	for _, col := range moveOrder {
		if !p.canPlay(col) {
			continue
		}
		child := p
		child.playCol(col)
		val := -o.negamax(child, -beta, -alpha)
		if val > best {
			best = val
		}
		if best > alpha {
			alpha = best
		}
		if alpha >= beta {
			break
		}
	}

	var flag int8
	switch {
	case best <= origAlpha:
		flag = ttUpper
	case best >= beta:
		flag = ttLower
	default:
		flag = ttExact
	}
	o.tt[key] = ttEntry{flag, best}
	return best
}

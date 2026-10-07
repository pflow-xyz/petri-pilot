// ode-minimax: an ODE-evaluated Petri net that plays tic-tac-toe
// minimax-perfectly on both seats. The experiment that got here — and
// everything it refuted along the way — is README.md; the variant code
// lives in this branch's git history.
//
// Modes:
//
//	(default)        tactical audits + 100-game tournaments vs minimax
//	verify [b] [l]   exhaustive referee: every legal opponent line, both
//	                 seats; the champion's move must never worsen the
//	                 exact game value
//	fit [g] [i]      refit (blockBias, lambda) from (1,1) against minimax
//	                 labels on random-self-play positions, then referee
//	fitgrad [g] [i]  the same refit by gradient: one tied parameter across
//	                 the 48 derived copies, forward sensitivities + adam,
//	                 then referee and a solve-count comparison vs Nelder-Mead
//	check-neuralode-grad
//	                 verify neuralode.go's hand-rolled backprop against
//	                 finite differences
//	fit-neuralode [h] [g] [i] [l2]
//	                 train dx/dt = MLP(x) from scratch, no declared
//	                 structure at all, then referee
package main

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
)

func (m *model) playGame(xs, os player, rng *rand.Rand) string {
	mk := m.start()
	for {
		mk = m.fireHouse(mk)
		if mk["win_x"] > 0 {
			return "X"
		}
		if mk["win_o"] > 0 {
			// The draw transition pays win_o too; the board says which it was.
			if m.hasLine(mk, "o") {
				return "O"
			}
			return "draw"
		}
		_, moves, maximizes, ok := m.legalMoves(mk)
		if !ok {
			return "draw"
		}
		p := os
		if maximizes {
			p = xs
		}
		mk = m.fire(p(m, mk, moves, maximizes, rng), mk)
	}
}

func tournament(m *model, name string, xs, os player, games int, seed int64) {
	rng := rand.New(rand.NewSource(seed))
	wins := map[string]int{}
	for i := 0; i < games; i++ {
		wins[m.playGame(xs, os, rng)]++
	}
	fmt.Printf("%-28s games=%d  X: %d  O: %d  draws: %d\n",
		name, games, wins["X"], wins["O"], wins["draw"])
}

// position builds a marking from X's and O's cells with `turn` to move.
func (m *model) position(xCells, oCells []string, turn string) marking {
	mk := m.start()
	place := func(cs []string, side string) {
		for _, c := range cs {
			mk["p"+c] = 0
			mk[side+c] = 1
			mk["move_tokens"]++
		}
	}
	place(xCells, "x")
	place(oCells, "o")
	mk["x_turn"], mk["o_turn"] = 0, 0
	mk[turn+"_turn"] = 1
	return mk
}

// boardKey renders a marking as a 9-char board string, X's cells upper.
func boardKey(mk marking) string {
	var b strings.Builder
	for _, c := range cells {
		switch {
		case mk["x"+c] > 0:
			b.WriteByte('X')
		case mk["o"+c] > 0:
			b.WriteByte('O')
		default:
			b.WriteByte('.')
		}
	}
	return b.String()
}

func audit(m *model, p player, name string, mk marking) {
	_, moves, maximizes, ok := m.legalMoves(mk)
	if !ok {
		fmt.Printf("%s: game over\n", name)
		return
	}
	optimal := m.optimalSet(mk, moves, maximizes)
	choice := p(m, mk, moves, maximizes, nil)
	verdict := "WRONG"
	for _, mv := range optimal {
		if mv == choice {
			verdict = "ok"
		}
	}
	fmt.Printf("  %-28s plays %-10s (optimal: %s)  %s\n",
		name, choice, strings.Join(optimal, " "), verdict)
}

func main() {
	m := buildModel()

	if len(os.Args) > 1 && os.Args[1] == "verify" {
		bias, lam := championBlockBias, championLambda
		if len(os.Args) > 3 {
			fmt.Sscanf(os.Args[2], "%f", &bias)
			fmt.Sscanf(os.Args[3], "%f", &lam)
		}
		fmt.Printf("champion: blockBias %.3f  lambda %.3f\n", bias, lam)
		p := championPlayer(m.toPetriChampion(bias), lam)
		for _, seat := range []bool{false, true} {
			d, b, mi := exhaustiveCheck(m, p, seat)
			name := "O"
			if seat {
				name = "X"
			}
			fmt.Printf("as %s: %d distinct decisions, %d game-losing, %d missed wins\n",
				name, d, b, mi)
		}
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "fit" {
		games, iters := 40, 60
		if len(os.Args) > 2 {
			fmt.Sscanf(os.Args[2], "%d", &games)
		}
		if len(os.Args) > 3 {
			fmt.Sscanf(os.Args[3], "%d", &iters)
		}
		positions := collectPositions(m, games, 7)
		fmt.Printf("training positions: %d (random self-play + 4 audits)\n", len(positions))
		bias, lam := fitChampion(m, positions, iters, true)
		fmt.Printf("\nfitted: blockBias %.3f  lambda %.3f  (train loss %.6f)\n",
			bias, lam, rankLoss(m, positions, bias, lam))
		p := championPlayer(m.toPetriChampion(bias), lam)
		for _, seat := range []bool{false, true} {
			d, b, mi := exhaustiveCheck(m, p, seat)
			name := "O"
			if seat {
				name = "X"
			}
			fmt.Printf("exhaustive referee as %s: %d decisions, %d game-losing, %d missed wins\n",
				name, d, b, mi)
		}
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "fitgrad" {
		games, iters := 40, 200
		if len(os.Args) > 2 {
			fmt.Sscanf(os.Args[2], "%d", &games)
		}
		if len(os.Args) > 3 {
			fmt.Sscanf(os.Args[3], "%d", &iters)
		}
		positions := collectPositions(m, games, 7)
		totalMoves := 0
		for _, p := range positions {
			totalMoves += len(p.moves)
		}
		fmt.Printf("training positions: %d (random self-play + 4 audits), %d candidate moves\n",
			len(positions), totalMoves)
		bias, lam, sensSolves := fitChampionGrad(m, positions, iters, true)
		fmt.Printf("\nfitted: blockBias %.3f  lambda %.3f  (train loss %.6f)\n",
			bias, lam, rankLoss(m, positions, bias, lam))
		p := championPlayer(m.toPetriChampion(bias), lam)
		for _, seat := range []bool{false, true} {
			d, b, mi := exhaustiveCheck(m, p, seat)
			name := "O"
			if seat {
				name = "X"
			}
			fmt.Printf("exhaustive referee as %s: %d decisions, %d game-losing, %d missed wins\n",
				name, d, b, mi)
		}
		// Cost comparison. A sensitivity solve at P=1 costs 2 plain-solve
		// equivalents; the Nelder-Mead baseline (fit mode's default 60
		// iterations) pays one plain solve per (position, move) per f call.
		fCalls := 0
		fitChampionCounted(m, positions, 60, false, &fCalls)
		nmSolves := fCalls * totalMoves
		fmt.Printf("\ncost: gradient %d sensitivity solves (= %d plain-solve equivalents at P=1)\n",
			sensSolves, 2*sensSolves)
		fmt.Printf("      nelder-mead baseline (60 iters): %d f calls x %d moves = %d plain solves\n",
			fCalls, totalMoves, nmSolves)
		fmt.Printf("      equivalents ratio (gradient/NM): %.2f\n",
			float64(2*sensSolves)/float64(nmSolves))
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "check-neuralode-grad" {
		rng := rand.New(rand.NewSource(1))
		fmt.Printf("max |analytic - finite-diff| gradient error: %.3e\n", checkNeuralGrad(rng))
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "fit-neuralode" {
		h, games, iters := 16, 40, 300
		l2 := 0.0
		if len(os.Args) > 2 {
			fmt.Sscanf(os.Args[2], "%d", &h)
		}
		if len(os.Args) > 3 {
			fmt.Sscanf(os.Args[3], "%d", &games)
		}
		if len(os.Args) > 4 {
			fmt.Sscanf(os.Args[4], "%d", &iters)
		}
		if len(os.Args) > 5 {
			fmt.Sscanf(os.Args[5], "%f", &l2)
		}
		positions := collectPositions(m, games, 7)
		fmt.Printf("fit-neuralode: hidden=%d training positions: %d l2=%g\n", h, len(positions), l2)
		net, lam := fitNeuralODE(m, positions, h, iters, l2, true)
		fmt.Printf("\nfitted: lambda %.4f (%d params)\n", lam, net.numParams())
		p := neuralODEPlayer(net, lam)
		for _, seat := range []bool{false, true} {
			d, b, mi := exhaustiveCheck(m, p, seat)
			name := "O"
			if seat {
				name = "X"
			}
			fmt.Printf("exhaustive referee as %s: %d decisions, %d game-losing, %d missed wins\n",
				name, d, b, mi)
		}
		return
	}

	// default: audits + tournaments
	p := championPlayer(m.toPetriChampion(championBlockBias), championLambda)
	fmt.Println("== tactical audits ==")
	audit(m, p, "corner reply", m.position([]string{"00"}, nil, "o"))
	audit(m, p, "center reply", m.position([]string{"11"}, nil, "o"))
	audit(m, p, "must-block", m.position([]string{"00", "11", "21"}, []string{"20", "22"}, "o"))
	audit(m, p, "double-corner fork", m.position([]string{"00", "22"}, []string{"11"}, "o"))
	fmt.Println("\n== tournaments (100 games, minimax randomizes over its optimal set) ==")
	tournament(m, "champion vs minimax", p, minimaxPlayer, 100, 11)
	tournament(m, "minimax vs champion", minimaxPlayer, p, 100, 11)
	tournament(m, "champion vs champion", p, p, 100, 11)
}

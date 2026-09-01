// ode-connect3 tests whether policy-in-structure transfers from tic-tac-toe
// to gravity-constrained 4x4 Connect-3.
package main

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
)

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

func (m *model) position(xCells, oCells []string, turn string) marking {
	mk := m.start()
	for _, c := range cells {
		mk["p"+c] = 0
	}
	place := func(given []string, side string) {
		for _, c := range given {
			mk[side+c] = 1
			mk["move_tokens"]++
		}
	}
	place(xCells, "x")
	place(oCells, "o")
	for col := 0; col < boardCols; col++ {
		seenEmpty := false
		landingSet := false
		for row := boardRows - 1; row >= 0; row-- {
			c := fmt.Sprintf("%d%d", row, col)
			occupied := mk["x"+c] > 0 || mk["o"+c] > 0
			if occupied && seenEmpty {
				panic("position contains a floating counter at " + c)
			}
			if !occupied {
				seenEmpty = true
				if !landingSet {
					mk["p"+c] = 1
					landingSet = true
				}
			}
		}
	}
	mk["x_turn"], mk["o_turn"] = 0, 0
	mk[turn+"_turn"] = 1
	return mk
}

func (m *model) playGame(xs, os player, rng *rand.Rand) string {
	mk := m.start()
	for {
		mk = m.fireHouse(mk)
		if mk["win_x"] > 0 {
			return "X"
		}
		if mk["win_o"] > 0 {
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
	fmt.Printf("  %-24s plays %-10s (optimal: %s)  %s\n",
		name, choice, strings.Join(optimal, " "), verdict)
}

func printReferee(m *model, name string, p player) {
	for _, seat := range []bool{false, true} {
		d, blown, missed := exhaustiveCheck(m, p, seat)
		seatName := "O"
		if seat {
			seatName = "X"
		}
		fmt.Printf("%s as %s: %d decisions, %d game-losing, %d missed wins\n",
			name, seatName, d, blown, missed)
	}
}

func printBoard(key string) {
	for row := 0; row < boardRows; row++ {
		fmt.Printf("    %s\n", key[row*boardCols:(row+1)*boardCols])
	}
}

func diagnose(m *model, name string, p player, limit int) {
	for _, seat := range []bool{false, true} {
		_, _, _, failures := runReferee(m, p, seat)
		seatName := "O"
		if seat {
			seatName = "X"
		}
		fmt.Printf("%s as %s: %d failures\n", name, seatName, len(failures))
		for i, failure := range failures {
			if i >= limit {
				break
			}
			optimal := make([]string, 0)
			for _, move := range failure.position.moves {
				if failure.position.optimal[move] {
					optimal = append(optimal, move)
				}
			}
			fmt.Printf("  %s choice=%s exact %d->%d optimal=%s\n",
				failure.key, failure.choice, failure.before, failure.after, strings.Join(optimal, ","))
			next := m.fireHouse(m.fire(failure.choice, failure.position.mk))
			_, replies, _, ok := m.legalMoves(next)
			var terminalReplies []string
			if ok {
				for _, reply := range replies {
					afterReply := m.fireHouse(m.fire(reply, next))
					if afterReply["win_x"] > 0 || m.hasLine(afterReply, "o") {
						terminalReplies = append(terminalReplies, reply)
					}
				}
			}
			fmt.Printf("    opponent immediate wins after choice: %s\n", strings.Join(terminalReplies, ","))
			printBoard(failure.key[:boardRows*boardCols])
		}
	}
}

// groupSchemes are the tuning-only hypotheses fitgrad.go's fitPolicyGrad
// compares, strongest first (see README's fitgrad-policy findings). Each
// scheme's groupKeyFn is applied independently to the win and block
// families — same function, disjoint parameter sets ("win:"/"blk:" prefixes
// in fitgrad.go never collide).
var groupSchemes = []struct {
	name string
	fn   groupKeyFn
}{
	{"row", groupByRow},
	{"parity", groupByRowParity},
	{"linetype", groupByLineType},
	{"cellindex", groupByCellIndex},
	{"linetype-parity", groupByLineTypeRowParity},
}

func groupScheme(name string) (groupKeyFn, bool) {
	for _, s := range groupSchemes {
		if s.name == name {
			return s.fn, true
		}
	}
	return nil, false
}

func main() {
	m := buildModel()
	mode := "quick"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	baseline := odePlayer(m.toPetriBaseline(), scoreLambda)
	policy := odePlayer(m.toPetriPolicy(candidateForceBias, candidateBlockBias), scoreLambda)

	switch mode {
	case "verify-naive":
		lam := scoreLambda
		if len(os.Args) > 2 {
			fmt.Sscanf(os.Args[2], "%f", &odeHorizon)
		}
		if len(os.Args) > 3 {
			fmt.Sscanf(os.Args[3], "%f", &lam)
		}
		fmt.Printf("naive: horizon %.3f lambda %.3f\n", odeHorizon, lam)
		printReferee(m, "naive", odePlayer(m.toPetriBaseline(), lam))
	case "verify-tactical":
		printReferee(m, "naive+tactical", tacticalGuard(baseline))
	case "verify-deep":
		// One real ply of search with the calibrated naive evaluator as
		// leaf, isolated from any structural policy — tests whether
		// lookahead alone (vs. more tuning, finding 7) closes the gap.
		lam := scoreLambda
		if len(os.Args) > 2 {
			fmt.Sscanf(os.Args[2], "%f", &lam)
		}
		fmt.Printf("deep: horizon %.3f lambda %.3f\n", odeHorizon, lam)
		printReferee(m, "deep", odeLookaheadPlayer(m.toPetriBaseline(), lam))
	case "verify-deep-policy":
		winBias, blockBias, lam := candidateForceBias, candidateBlockBias, scoreLambda
		if len(os.Args) > 4 {
			fmt.Sscanf(os.Args[2], "%f", &winBias)
			fmt.Sscanf(os.Args[3], "%f", &blockBias)
			fmt.Sscanf(os.Args[4], "%f", &lam)
		}
		fmt.Printf("deep+policy: winBias %.3f blockBias %.3f lambda %.3f\n", winBias, blockBias, lam)
		printReferee(m, "deep+policy", odeLookaheadPlayer(m.toPetriPolicy(winBias, blockBias), lam))
	case "diagnose-naive":
		diagnose(m, "naive", baseline, 20)
	case "diagnose-deep":
		lam := scoreLambda
		if len(os.Args) > 2 {
			fmt.Sscanf(os.Args[2], "%f", &lam)
		}
		diagnose(m, "deep", odeLookaheadPlayer(m.toPetriBaseline(), lam), 20)
	case "diagnose":
		winBias, blockBias, lam := candidateForceBias, candidateBlockBias, scoreLambda
		if len(os.Args) > 4 {
			fmt.Sscanf(os.Args[2], "%f", &winBias)
			fmt.Sscanf(os.Args[3], "%f", &blockBias)
			fmt.Sscanf(os.Args[4], "%f", &lam)
		}
		diagnose(m, "policy", odePlayer(m.toPetriPolicy(winBias, blockBias), lam), 20)
	case "verify":
		winBias, blockBias, lam := candidateForceBias, candidateBlockBias, scoreLambda
		if len(os.Args) > 4 {
			fmt.Sscanf(os.Args[2], "%f", &winBias)
			fmt.Sscanf(os.Args[3], "%f", &blockBias)
			fmt.Sscanf(os.Args[4], "%f", &lam)
		}
		fmt.Printf("policy: winBias %.3f blockBias %.3f lambda %.3f\n", winBias, blockBias, lam)
		printReferee(m, "policy", odePlayer(m.toPetriPolicy(winBias, blockBias), lam))
	case "fit":
		games, iters := 30, 50
		if len(os.Args) > 2 {
			fmt.Sscanf(os.Args[2], "%d", &games)
		}
		if len(os.Args) > 3 {
			fmt.Sscanf(os.Args[3], "%d", &iters)
		}
		positions := collectPositions(m, games, 7)
		fmt.Printf("training positions: %d\n", len(positions))
		winBias, blockBias, lam := fitPolicy(m, positions, iters, true)
		fmt.Printf("fitted: winBias %.4f blockBias %.4f lambda %.4f loss %.6g\n",
			winBias, blockBias, lam, rankLoss(m, positions, winBias, blockBias, lam))
		printReferee(m, "fitted", odePlayer(m.toPetriPolicy(winBias, blockBias), lam))
	case "fitfail":
		iters := 50
		if len(os.Args) > 2 {
			fmt.Sscanf(os.Args[2], "%d", &iters)
		}
		positions := failurePositions(m, baseline)
		fmt.Printf("training positions: %d exact naive failures\n", len(positions))
		winBias, blockBias, lam := fitPolicy(m, positions, iters, true)
		fmt.Printf("fitted: winBias %.4f blockBias %.4f lambda %.4f loss %.6g\n",
			winBias, blockBias, lam, rankLoss(m, positions, winBias, blockBias, lam))
		printReferee(m, "fitted", odePlayer(m.toPetriPolicy(winBias, blockBias), lam))
	case "fitgrad-policy":
		scheme := "row"
		games, iters := 30, 200
		if len(os.Args) > 2 {
			scheme = os.Args[2]
		}
		if len(os.Args) > 3 {
			fmt.Sscanf(os.Args[3], "%d", &games)
		}
		if len(os.Args) > 4 {
			fmt.Sscanf(os.Args[4], "%d", &iters)
		}
		fn, ok := groupScheme(scheme)
		if !ok {
			names := make([]string, len(groupSchemes))
			for i, s := range groupSchemes {
				names[i] = s.name
			}
			fmt.Printf("unknown scheme %q; choices: %s\n", scheme, strings.Join(names, " "))
			return
		}
		positions := collectPositions(m, games, 7)
		fmt.Printf("scheme=%s training positions: %d\n", scheme, len(positions))
		winRates, blkRates, lam, solves := fitPolicyGrad(m, positions, fn, fn, iters, true)
		fmt.Printf("fitted: %d win-groups, %d block-groups, lambda %.4f (%d sensitivity solves)\n",
			len(winRates), len(blkRates), lam, solves)
		ev := m.toPetriPolicyGrouped(fn, fn, winRates, blkRates)
		printReferee(m, "fitgrad-"+scheme, odePlayer(ev, lam))
	default:
		fmt.Printf("model: %d cells, %d win lines, %d declared transitions\n",
			len(cells), len(winLines), len(m.transitions))
		fmt.Println("== tactical audits ==")
		names := []string{"gravity tempo A", "gravity tempo B", "horizontal block", "vertical block"}
		for i, mk := range auditPositions(m) {
			audit(m, policy, names[i], mk)
		}
		fmt.Println("\n== tournaments ==")
		tournament(m, "naive vs minimax", baseline, minimaxPlayer, 20, 11)
		tournament(m, "policy vs minimax", policy, minimaxPlayer, 20, 11)
		tournament(m, "minimax vs policy", minimaxPlayer, policy, 20, 11)
	}
}

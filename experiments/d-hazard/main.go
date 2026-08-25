// ablation-v-minimax: does next-move elimination (rate-zero ablation),
// scored on the declared objective where a tie counts against X, produce
// perfect play? Exact minimax over the same net is the referee.
//
// Two spotlight positions bound the answer before the tournaments do:
// the must-block (depth-1 tactic, the blog's Example 4 class) and the
// double-corner trap (depth-2 tactic, no threat on the board yet).
package main

import (
	"fmt"
	"math/rand"
	"os"
	"strings"

	"github.com/pflow-xyz/go-pflow/petri"
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

// odeAudit shows the whole ODE measurement at a position: every candidate's
// ablation loss against the minimax-optimal set, under both value modes.
func odeAudit(m *model, net *petri.PetriNet, name string, mk marking) {
	_, moves, maximizes, ok := m.legalMoves(mk)
	if !ok {
		fmt.Printf("%s: game over\n", name)
		return
	}
	optimal := m.optimalSet(mk, moves, maximizes)
	inOptimal := func(mv string) string {
		for _, o := range optimal {
			if o == mv {
				return "optimal"
			}
		}
		return ""
	}
	fmt.Printf("\n%s (minimax-optimal: %s)\n", name, strings.Join(optimal, " "))
	show := func(label string, rows []moveLoss, col string) {
		fmt.Printf("  [%s]\n", label)
		for i, ml := range rows {
			pick := " "
			if i == 0 {
				pick = ">"
			}
			fmt.Printf("  %s %-10s %s=%+.5f  %s\n", pick, ml.move, col, ml.loss, inOptimal(ml.move))
		}
	}
	show("eliminate, value = own win place", m.odeLosses(net, mk, moves, maximizes, objOwn), "loss")
	show("eliminate, value = win_x - win_o", m.odeLosses(net, mk, moves, maximizes, objDiff), "loss")
	show("play, value = win_x - win_o", m.odePlayScores(net, mk, moves, maximizes, objDiff), "score")
}

func audit(m *model, name string, mk marking, players map[string]player) {
	_, moves, maximizes, ok := m.legalMoves(mk)
	if !ok {
		fmt.Printf("%s: game over\n", name)
		return
	}
	optimal := m.optimalSet(mk, moves, maximizes)
	fmt.Printf("\n%s (minimax-optimal: %s)\n", name, strings.Join(optimal, " "))
	rng := rand.New(rand.NewSource(1))
	for label, p := range players {
		choice := p(m, mk, moves, maximizes, rng)
		verdict := "LOSES"
		for _, mv := range optimal {
			if mv == choice {
				verdict = "ok"
			}
		}
		fmt.Printf("  %-16s plays %-10s %s\n", label, choice, verdict)
	}
}

func main() {
	m := buildModel()
	net := m.toPetri()

	ssa := ssaAblation(400)
	ode := odeAblation(net, objOwn)
	odeDiff := odeAblation(net, objDiff)

	if len(os.Args) > 1 && os.Args[1] == "quick" {
		// The algorithm under test: ODE ablation on the full net
		// (move_tokens + draw -> win_o), value read from the mover's own
		// win place. Per-move loss tables at the spotlight positions, then
		// one seeded game per side — perfect play means never losing.
		fmt.Println("\n== ode ablation, per-move losses ==")
		odeAudit(m, net, "opening (X to move)", m.start())
		odeAudit(m, net, "must-block", m.position(
			[]string{"00", "11", "21"}, []string{"20", "22"}, "o"))
		odeAudit(m, net, "double-corner trap", m.position(
			[]string{"00", "22"}, []string{"11"}, "o"))
		fmt.Println("\n== quick check (1 game each) ==")
		tournament(m, "ode-own vs minimax", ode, minimaxPlayer, 1, 11)
		tournament(m, "minimax vs ode-own", minimaxPlayer, ode, 1, 11)
		tournament(m, "ode-diff vs minimax", odeDiff, minimaxPlayer, 1, 11)
		tournament(m, "minimax vs ode-diff", minimaxPlayer, odeDiff, 1, 11)
		play := odePlay(net, objDiff)
		tournament(m, "ode-play vs minimax", play, minimaxPlayer, 1, 11)
		tournament(m, "minimax vs ode-play", minimaxPlayer, play, 1, 11)
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "play100" {
		// The real test for ODE play-scoring: minimax randomizes over its
		// optimal set, so 100 games explore many lines. Perfect play for
		// the ode side = zero losses.
		play := odePlay(net, objDiff)
		tournament(m, "ode-play vs minimax", play, minimaxPlayer, 100, 11)
		tournament(m, "minimax vs ode-play", minimaxPlayer, play, 100, 11)
		tournament(m, "ode-play vs ode-play", play, play, 100, 11)

		// Draw-wiring variants: only the evaluation net changes.
		// B: draw pays the evaluator's opponent — each side plays to win,
		//    a draw is worth a loss.
		netDrawO := m.toPetriDraw([]string{"win_o"}) // X's view: draw = loss
		netDrawX := m.toPetriDraw([]string{"win_x"}) // O's view: draw = loss
		playB := odePlayPerSide(netDrawO, netDrawX, objDiff)
		// C: draw pays both — under win_x - win_o a draw scores 0, giving
		//    both sides the true ranking win > draw > loss.
		netDrawBoth := m.toPetriDraw([]string{"win_x", "win_o"})
		playC := odePlay(netDrawBoth, objDiff)
		fmt.Println()
		tournament(m, "B draw=loss vs minimax", playB, minimaxPlayer, 100, 11)
		tournament(m, "minimax vs B draw=loss", minimaxPlayer, playB, 100, 11)
		tournament(m, "C draw=0 vs minimax", playC, minimaxPlayer, 100, 11)
		tournament(m, "minimax vs C draw=0", minimaxPlayer, playC, 100, 11)

		// D: hazard draw — game_active -> tie in the evaluation net, O
		// maximizes tie + win_o (its true "X does not win" objective).
		playD := odePlayDHazard(m.toPetriDHazard())
		fmt.Println()
		tournament(m, "d-hazard vs minimax", playD, minimaxPlayer, 100, 11)
		tournament(m, "minimax vs d-hazard", minimaxPlayer, playD, 100, 11)
		tournament(m, "d-hazard vs d-hazard", playD, playD, 100, 11)
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "tie" {
		// Net variant D: draw pays a dedicated `tie` place, so the ODE
		// final state exposes three coordinates per candidate —
		// (win_x, win_o, tie) — instead of folding the draw into win_o.
		// Question: does any scoring function over the three separate the
		// tactically forced move?
		if len(os.Args) > 2 {
			fmt.Sscanf(os.Args[2], "%f", &odeHorizon)
			fmt.Printf("(horizon %.0f)\n", odeHorizon)
		}
		nets := []struct {
			label string
			net   *petri.PetriNet
		}{
			{"counter (move_tokens x9 -> tie)", m.toPetriDraw([]string{"tie"})},
			{"d-hazard (game_active -> tie)", m.toPetriDHazard()},
		}
		scan := func(net *petri.PetriNet, name string, mk marking) {
			_, moves, maximizes, _ := m.legalMoves(mk)
			optimal := m.optimalSet(mk, moves, maximizes)
			isOpt := func(mv string) string {
				for _, o := range optimal {
					if o == mv {
						return "optimal"
					}
				}
				return ""
			}
			fmt.Printf("\n%s (minimax-optimal: %s)\n", name, strings.Join(optimal, " "))
			fmt.Println("  move        win_x    win_o    tie      g_act    mv_tok   wx-wo    wx-wo-tie")
			for _, mv := range moves {
				f := m.odeFinal(net, m.fire(mv, mk), m.rates)
				wx, wo, tie := f["win_x"], f["win_o"], f["tie"]
				fmt.Printf("  %-10s  %.5f  %.5f  %.5f  %.5f  %.5f  %+.5f  %+.5f  %s\n",
					mv, wx, wo, tie, f["game_active"], f["move_tokens"],
					wx-wo, wx-wo-tie, isOpt(mv))
			}
		}
		for _, v := range nets {
			fmt.Printf("\n########## %s ##########\n", v.label)
			for _, wr := range []float64{1, 720} {
				for _, t := range m.transitions {
					if len(t) > 5 && (t[:5] == "x_win" || t[:5] == "o_win") {
						m.rates[t] = wr
					}
				}
				fmt.Printf("\n=== win-detector rate %.0f ===\n", wr)
				scan(v.net, "opening (X to move)", m.start())
				scan(v.net, "must-block (O to move)", m.position(
					[]string{"00", "11", "21"}, []string{"20", "22"}, "o"))
				scan(v.net, "double-corner trap (O to move)", m.position(
					[]string{"00", "22"}, []string{"11"}, "o"))
			}
		}
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "sweep" {
		// Does ANY (horizon, win-rate, draw-rate) make ODE play-scoring
		// pass both tactical audits? Each cell: does the top-scored move
		// land in the minimax-optimal set at must-block AND double-corner.
		mustBlock := m.position([]string{"00", "11", "21"}, []string{"20", "22"}, "o")
		trap := m.position([]string{"00", "22"}, []string{"11"}, "o")
		passes := func(mk marking) bool {
			_, moves, maximizes, _ := m.legalMoves(mk)
			optimal := m.optimalSet(mk, moves, maximizes)
			top := m.odePlayScores(net, mk, moves, maximizes, objDiff)[0].move
			for _, o := range optimal {
				if o == top {
					return true
				}
			}
			return false
		}
		fmt.Println("horizon  winRate  drawRate  must-block  double-corner")
		for _, h := range []float64{1, 3, 10} {
			for _, wr := range []float64{1, 10, 720} {
				for _, dr := range []float64{0.1, 1, 10} {
					odeHorizon = h
					for _, t := range m.transitions {
						if len(t) > 5 && (t[:5] == "x_win" || t[:5] == "o_win") {
							m.rates[t] = wr
						}
					}
					m.rates["draw"] = dr
					fmt.Printf("%7.0f  %7.0f  %8.1f  %-10v  %v\n",
						h, wr, dr, passes(mustBlock), passes(trap))
				}
			}
		}
		return
	}

	players := map[string]player{"ssa-ablation@400": ssa, "ode-ablation": ode}
	fmt.Println("== position audits ==")
	// Depth-1: X threatens col1 (x00,x11,x21 vs o20,o22); O must block at 01.
	audit(m, "must-block", m.position(
		[]string{"00", "11", "21"}, []string{"20", "22"}, "o"), players)
	// Depth-2: X in opposite corners, O in center; every edge draws, every
	// corner loses to the fork. No threat on the board yet.
	audit(m, "double-corner trap", m.position(
		[]string{"00", "22"}, []string{"11"}, "o"), players)

	fmt.Println("\n== tournaments (100 games each) ==")
	tournament(m, "minimax vs minimax", minimaxPlayer, minimaxPlayer, 100, 11)
	tournament(m, "ssa-ablation vs minimax", ssa, minimaxPlayer, 100, 11)
	tournament(m, "minimax vs ssa-ablation", minimaxPlayer, ssa, 100, 11)
	tournament(m, "ode-ablation vs minimax", ode, minimaxPlayer, 100, 11)
	tournament(m, "minimax vs ode-ablation", minimaxPlayer, ode, 100, 11)
	tournament(m, "ssa-ablation vs ssa-ablation", ssa, ssa, 100, 11)
}

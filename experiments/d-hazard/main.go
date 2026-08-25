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

	if len(os.Args) > 1 && os.Args[1] == "tempo" {
		// Structural variants vs the fork. Coordinate tables per candidate
		// at the two tactical positions, then a scorer scan: which (net,
		// penalty, alpha) picks a minimax-optimal move at BOTH?
		mustBlock := m.position([]string{"00", "11", "21"}, []string{"20", "22"}, "o")
		trap := m.position([]string{"00", "22"}, []string{"11"}, "o")

		table := func(ev evalNet, name string, mk marking, cols []string) {
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
			fmt.Printf("  %-10s", "move")
			for _, c := range cols {
				fmt.Printf("  %-9s", c)
			}
			fmt.Println()
			for _, mv := range moves {
				f := m.odeFinal(ev.net, m.fire(mv, mk), ev.rates)
				fmt.Printf("  %-10s", mv)
				for _, c := range cols {
					fmt.Printf("  %.5f  ", f[c])
				}
				fmt.Printf("%s\n", isOpt(mv))
			}
		}

		base := []string{"win_x", "win_o", "tie"}
		tempoCols := append(append([]string{}, base...), "threat_x", "threat_o", "fork_x", "fork_o")

		fmt.Println("########## d-hazard baseline ##########")
		dh := evalNet{m.toPetriDHazard(), m.rates}
		table(dh, "must-block (O to move)", mustBlock, base)
		table(dh, "double-corner trap (O to move)", trap, base)

		fmt.Println("\n########## E/F tempo: threat + fork coordinates ##########")
		tempo := m.toPetriTempo()
		table(tempo, "must-block (O to move)", mustBlock, tempoCols)
		table(tempo, "double-corner trap (O to move)", trap, tempoCols)

		for _, bias := range []float64{2, 10, 50} {
			fmt.Printf("\n########## H block-bias %.0f: forced-reply flow ##########\n", bias)
			bb := m.toPetriBlockBias(bias)
			table(bb, "must-block (O to move)", mustBlock, base)
			table(bb, "double-corner trap (O to move)", trap, base)
		}

		// Scorer scan: does any config pass both tactical audits?
		passes := func(p player, mk marking) bool {
			_, moves, maximizes, _ := m.legalMoves(mk)
			optimal := m.optimalSet(mk, moves, maximizes)
			top := p(m, mk, moves, maximizes, nil)
			for _, o := range optimal {
				if o == top {
					return true
				}
			}
			return false
		}
		fmt.Println("\n== scorer scan (must-block / double-corner) ==")
		check := func(label string, p player) {
			fmt.Printf("  %-34s %-6v %v\n", label, passes(p, mustBlock), passes(p, trap))
		}
		check("d-hazard (tie+win_o)", odePlayVariant(dh, 0, ""))
		for _, pen := range []string{"threat_x", "fork_x"} {
			for _, a := range []float64{0.1, 0.3, 1, 3, 10} {
				check(fmt.Sprintf("tempo  -%.1f*%s", a, pen), odePlayVariant(tempo, a, pen))
			}
		}
		for _, bias := range []float64{2, 10, 50} {
			bb := m.toPetriBlockBias(bias)
			check(fmt.Sprintf("block-bias %.0f (tie+win_o)", bias), odePlayVariant(bb, 0, ""))
		}
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "trace" {
		// Replay seeded games (minimax as X vs the variant as O) and report,
		// for each loss, the first position where O's choice left the
		// minimax-optimal set — with the variant's score table there.
		var p player
		label := os.Args[2]
		switch label {
		case "bias":
			b := 6.0
			if len(os.Args) > 3 {
				fmt.Sscanf(os.Args[3], "%f", &b)
			}
			p = odePlayVariant(m.toPetriBlockBias(b), 0, "")
		case "dhazard":
			p = odePlayVariant(evalNet{m.toPetriDHazard(), m.rates}, 0, "")
		default:
			fmt.Println("usage: trace bias <b> | trace dhazard")
			return
		}
		rng := rand.New(rand.NewSource(11))
		losses, shown := 0, 0
		type slip struct {
			mk     marking
			choice string
		}
		seen := map[string]int{}
		for g := 0; g < 100; g++ {
			mk := m.start()
			var slips []slip
			for {
				mk = m.fireHouse(mk)
				if mk["win_x"] > 0 || mk["win_o"] > 0 {
					break
				}
				_, moves, maximizes, ok := m.legalMoves(mk)
				if !ok {
					break
				}
				var mv string
				if maximizes {
					mv = minimaxPlayer(m, mk, moves, maximizes, rng)
				} else {
					mv = p(m, mk, moves, maximizes, rng)
					inOpt := false
					for _, o := range m.optimalSet(mk, moves, maximizes) {
						if o == mv {
							inOpt = true
						}
					}
					if !inOpt {
						slips = append(slips, slip{mk, mv})
					}
				}
				mk = m.fire(mv, mk)
			}
			if mk["win_x"] > 0 && len(slips) > 0 {
				losses++
				s := slips[0]
				key := boardKey(s.mk) + "->" + s.choice
				seen[key]++
				if seen[key] == 1 && shown < 6 {
					shown++
					_, moves, maximizes, _ := m.legalMoves(s.mk)
					optimal := m.optimalSet(s.mk, moves, maximizes)
					fmt.Printf("\nloss %d: first slip at %s\n  chose %s, optimal: %s\n",
						losses, boardKey(s.mk), s.choice, strings.Join(optimal, " "))
				}
			}
		}
		fmt.Printf("\nlosses with a slip: %d; distinct first-slip patterns:\n", losses)
		for k, n := range seen {
			fmt.Printf("  %3dx  %s\n", n, k)
		}
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "verify" {
		// Exhaustive referee: walk EVERY legal opponent line (optimal or
		// not); at each variant decision point, the variant's move must not
		// worsen the exact game value of the position. Zero failures on
		// both seats = perfect play, proven rather than sampled.
		wb, bb, lam := 2.0, 6.0, 1.0
		if len(os.Args) > 3 {
			fmt.Sscanf(os.Args[2], "%f", &wb)
			fmt.Sscanf(os.Args[3], "%f", &bb)
		}
		if len(os.Args) > 4 {
			fmt.Sscanf(os.Args[4], "%f", &lam)
		}
		ev := m.toPetriPolicyBias(wb, bb)
		p := func(m *model, mk marking, moves []string, maximizes bool, _ *rand.Rand) string {
			best, bestScore := "", 0.0
			for i, mv := range moves {
				f := m.odeFinal(ev.net, m.fire(mv, mk), ev.rates)
				score := f["win_x"] - f["win_o"]
				if !maximizes {
					score = f["tie"] + lam*f["win_o"]
				}
				if i == 0 || score > bestScore {
					best, bestScore = mv, score
				}
			}
			return best
		}
		fmt.Printf("policy w%.0f b%.0f, O score = tie + %.1f*win_o\n", wb, bb, lam)
		for _, variantIsX := range []bool{false, true} {
			cache := map[string]string{}
			seen := map[string]bool{}
			decisions, blown, missed := 0, 0, 0
			var walk func(mk marking)
			walk = func(mk marking) {
				mk = m.fireHouse(mk)
				if mk["win_x"] > 0 || mk["win_o"] > 0 {
					return
				}
				_, moves, maximizes, ok := m.legalMoves(mk)
				if !ok {
					return
				}
				if maximizes == variantIsX {
					key := boardKey(mk)
					mv, hit := cache[key]
					if !hit {
						mv = p(m, mk, moves, maximizes, nil)
						cache[key] = mv
					}
					if !seen[key] {
						seen[key] = true
						decisions++
						before := m.minimax(mk, -2, 2)
						after := m.minimax(m.fireHouse(m.fire(mv, mk)), -2, 2)
						worse := after > before
						lost := after == 1 // X wins the position
						if variantIsX {
							worse = after < before
							lost = after == -1
						}
						if worse {
							if lost {
								blown++
								fmt.Printf("  BLOWN %s: plays %s (value %+d -> %+d)\n", key, mv, before, after)
							} else {
								missed++
							}
						}
					}
					walk(m.fire(mv, mk))
					return
				}
				for _, mv := range moves {
					walk(m.fire(mv, mk))
				}
			}
			walk(m.start())
			seat := "O"
			if variantIsX {
				seat = "X"
			}
			fmt.Printf("variant as %s: %d distinct decisions, %d game-losing, %d missed wins\n",
				seat, decisions, blown, missed)
		}
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "bias2d" {
		// Two-knob policy scan: winBias x blockBias against four tactical
		// audits. A cell passing all four earns a tournament.
		audits := []struct {
			name string
			mk   marking
		}{
			{"corner-reply", m.position([]string{"00"}, nil, "o")},
			{"center-reply", m.position([]string{"11"}, nil, "o")},
			{"must-block", m.position([]string{"00", "11", "21"}, []string{"20", "22"}, "o")},
			{"fork", m.position([]string{"00", "22"}, []string{"11"}, "o")},
		}
		pass := func(p player, mk marking) bool {
			_, moves, maximizes, _ := m.legalMoves(mk)
			optimal := m.optimalSet(mk, moves, maximizes)
			top := p(m, mk, moves, maximizes, nil)
			for _, o := range optimal {
				if o == top {
					return true
				}
			}
			return false
		}
		fmt.Println("  winB  blkB   corner center block fork")
		for _, wb := range []float64{0, 2, 4, 8, 16, 32} {
			for _, bb := range []float64{0, 1, 2, 3, 4, 6, 8} {
				p := odePlayVariant(m.toPetriPolicyBias(wb, bb), 0, "")
				row := fmt.Sprintf("  %4.0f  %4.0f  ", wb, bb)
				all := true
				for _, a := range audits {
					ok := pass(p, a.mk)
					all = all && ok
					mark := "-"
					if ok {
						mark = "Y"
					}
					row += fmt.Sprintf("  %-5s", mark)
				}
				if all {
					row += "  <== ALL PASS"
				}
				fmt.Println(row)
			}
		}
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "biasscan" {
		// Map the block-bias window: which bias values pass both tactical
		// audits, and by what margin at the fork?
		mustBlock := m.position([]string{"00", "11", "21"}, []string{"20", "22"}, "o")
		trap := m.position([]string{"00", "22"}, []string{"11"}, "o")
		cornerReply := m.position([]string{"00"}, nil, "o")
		fmt.Println("  bias   corner-reply  must-block  fork    fork margin (best_opt - best_nonopt)")
		for _, b := range []float64{0, 1, 2, 3, 4, 5, 6, 8, 10, 12} {
			ev := m.toPetriBlockBias(b)
			p := odePlayVariant(ev, 0, "")
			pass := func(mk marking) bool {
				_, moves, maximizes, _ := m.legalMoves(mk)
				optimal := m.optimalSet(mk, moves, maximizes)
				top := p(m, mk, moves, maximizes, nil)
				for _, o := range optimal {
					if o == top {
						return true
					}
				}
				return false
			}
			// margin at the fork: best optimal score minus best non-optimal
			_, moves, maximizes, _ := m.legalMoves(trap)
			optimal := map[string]bool{}
			for _, o := range m.optimalSet(trap, moves, maximizes) {
				optimal[o] = true
			}
			bestOpt, bestNon := -1e9, -1e9
			for _, mv := range moves {
				f := m.odeFinal(ev.net, m.fire(mv, trap), ev.rates)
				s := f["tie"] + f["win_o"]
				if optimal[mv] && s > bestOpt {
					bestOpt = s
				}
				if !optimal[mv] && s > bestNon {
					bestNon = s
				}
			}
			fmt.Printf("  %4.0f   %-12v  %-10v  %-6v  %+.5f\n", b, pass(cornerReply), pass(mustBlock), pass(trap), bestOpt-bestNon)
		}
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "tempo100" {
		// Tournament for a chosen variant: tempo100 <variant> [alpha|bias]
		// variant: tempo (needs alpha+penalty via args 3,4) | bias <b>
		var p player
		label := "?"
		switch {
		case len(os.Args) > 2 && os.Args[2] == "bias":
			b := 10.0
			if len(os.Args) > 3 {
				fmt.Sscanf(os.Args[3], "%f", &b)
			}
			p = odePlayVariant(m.toPetriBlockBias(b), 0, "")
			label = fmt.Sprintf("block-bias %.0f", b)
		case len(os.Args) > 2 && os.Args[2] == "policy":
			wb, bb := 2.0, 6.0
			if len(os.Args) > 4 {
				fmt.Sscanf(os.Args[3], "%f", &wb)
				fmt.Sscanf(os.Args[4], "%f", &bb)
			}
			p = odePlayVariant(m.toPetriPolicyBias(wb, bb), 0, "")
			label = fmt.Sprintf("policy w%.0f b%.0f", wb, bb)
		case len(os.Args) > 2 && os.Args[2] == "tempo":
			a, pen := 1.0, "fork_x"
			if len(os.Args) > 3 {
				fmt.Sscanf(os.Args[3], "%f", &a)
			}
			if len(os.Args) > 4 {
				pen = os.Args[4]
			}
			p = odePlayVariant(m.toPetriTempo(), a, pen)
			label = fmt.Sprintf("tempo -%.1f*%s", a, pen)
		default:
			fmt.Println("usage: tempo100 bias <b> | tempo100 tempo <alpha> <penalty>")
			return
		}
		tournament(m, label+" vs minimax", p, minimaxPlayer, 100, 11)
		tournament(m, "minimax vs "+label, minimaxPlayer, p, 100, 11)
		tournament(m, label+" vs "+label, p, p, 100, 11)
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

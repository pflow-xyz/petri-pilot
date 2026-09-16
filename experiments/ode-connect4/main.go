// ode-connect4: can a continuous relaxation of a declared Connect Four
// Petri net pick moves as well as an exact solver? See README.md for the
// full experiment log; this file just drives it.
//
// Modes:
//
//	quick                a handful of tactical audits + small self-play-vs-
//	                      oracle tournaments (naive net, then champion net)
//	verify [b] [w] [p] [l] [g]  sampled referee (see fit.go's file comment
//	                      on what "sampled" means here vs ode-minimax's
//	                      exhaustive walk): every legal move at every
//	                      sampled decision point checked against the
//	                      oracle. g is the self-play game count (default
//	                      25); the README's headline numbers used g=250
//	                      (~1m40s, ~2000 decision points).
//	fit [g] [i]          refit (blockBias, winBias, lambda) from (1,1,1)
//	                      against oracle labels on random-self-play
//	                      positions, then verify (parityBias held at 0)
//	fitparity [g] [i]    README finding 7: calibrate the parity-bias tier
//	                      in both the "alongside blockBias" and "replacing
//	                      blockBias" configurations, referee both against
//	                      the SAME held-out sample as the single-tier
//	                      champion, and report which wins
//	lookahead [depth] [maxDecisions]  README finding 8: N-ply minimax with
//	                      the champion ODE evaluator as the LEAF scorer
//	                      (lookahead.go) — a genuine search+eval hybrid,
//	                      not a search-free evaluator; see that file's
//	                      header for the ply-counting convention. Refereed
//	                      on the SAME 250-game/seed-13 sample as every
//	                      other finding, truncated to the first
//	                      maxDecisions decisions if given (0 = full
//	                      sample) — deeper plies cost roughly 7x more per
//	                      level, so depth 2 needs truncation to finish in
//	                      minutes rather than hours; always states exactly
//	                      how much of the sample it covered.
package main

import (
	"fmt"
	"os"
	"time"
)

const (
	// Fitted values (see README findings 4-7). winBias=0 and parityBias=0
	// are deliberate, not placeholders: finding 6 showed a blind "complete
	// your own line" tier regresses X, and finding 7 checked whether a
	// theory-motivated (row-parity-restricted) version of that tier does
	// better — see README for whether it shipped as the default.
	championBlockBias  = 1.342
	championWinBias    = 0.0
	championParityBias = 0.0
	championLambda     = 0.238

	defaultOracleBudget = 800_000 // nodes per Solve call
	defaultMinDiscs     = 14      // ply floor for sampled positions (see oracle.go's honesty note;
	// 14 discs / 28 empty cells is deep enough that most self-play decision
	// points resolve within the node budget above, and shallow enough that
	// real multi-ply tactics — not just "one move from the end" — show up)
)

func main() {
	m := buildModel()

	switch {
	case len(os.Args) > 1 && os.Args[1] == "verify":
		bias, win, par, lam := championBlockBias, championWinBias, championParityBias, championLambda
		games := 25
		if len(os.Args) > 5 {
			fmt.Sscanf(os.Args[2], "%f", &bias)
			fmt.Sscanf(os.Args[3], "%f", &win)
			fmt.Sscanf(os.Args[4], "%f", &par)
			fmt.Sscanf(os.Args[5], "%f", &lam)
		}
		if len(os.Args) > 6 {
			fmt.Sscanf(os.Args[6], "%d", &games)
		}
		runVerify(m, bias, win, par, lam, games)
		return

	case len(os.Args) > 1 && os.Args[1] == "fit":
		games, iters := 15, 20
		if len(os.Args) > 2 {
			fmt.Sscanf(os.Args[2], "%d", &games)
		}
		if len(os.Args) > 3 {
			fmt.Sscanf(os.Args[3], "%d", &iters)
		}
		runFit(m, games, iters)
		return

	case len(os.Args) > 1 && os.Args[1] == "fitparity":
		games, iters := 15, 20
		if len(os.Args) > 2 {
			fmt.Sscanf(os.Args[2], "%d", &games)
		}
		if len(os.Args) > 3 {
			fmt.Sscanf(os.Args[3], "%d", &iters)
		}
		runFitParity(m, games, iters)
		return

	case len(os.Args) > 1 && os.Args[1] == "lookahead":
		depth, maxDecisions := 1, 0
		if len(os.Args) > 2 {
			fmt.Sscanf(os.Args[2], "%d", &depth)
		}
		if len(os.Args) > 3 {
			fmt.Sscanf(os.Args[3], "%d", &maxDecisions)
		}
		runLookahead(m, depth, maxDecisions)
		return
	}

	runQuick(m)
}

func runQuick(m *model) {
	o := NewOracle()

	fmt.Println("== oracle sanity: known short forced wins ==")
	for _, seq := range [][]int{{0, 1, 0, 1, 0, 1}, {3, 3, 3, 4, 3}} {
		p := newPosition(seq)
		val, ok := o.Solve(p, 2_000_000)
		fmt.Printf("  moves=%v -> value=%v ok=%v\n", seq, val, ok)
	}

	fmt.Println("\n== naive ODE net vs oracle, tactical sample ==")
	naive := evalPlayer(m, evalNet{net: m.toPetriDeclared(), rates: m.rates}, naiveScore)
	positions, skipped := collectPositions(m, o, 6, defaultMinDiscs, defaultOracleBudget, 7)
	fmt.Printf("sampled %d decision points (%d skipped: oracle exceeded budget)\n", len(positions), skipped)
	auditPlayer(m, o, "naive", naive, positions, defaultOracleBudget)

	fmt.Println("\n== champion (three-tier, single active) ODE net vs oracle, same sample ==")
	champ := championPlayer(m, championBlockBias, championWinBias, championParityBias, championLambda)
	auditPlayer(m, o, "champion", champ, positions, defaultOracleBudget)

	fmt.Println("\n== self-play tournaments (small: each ODE solve is expensive on this net) ==")
	oraclePlay, unsolved := oraclePlayer(o, defaultOracleBudget)
	tournament(m, "naive vs oracle", naive, oraclePlay, 3, 11)
	tournament(m, "oracle vs naive", oraclePlay, naive, 3, 11)
	tournament(m, "champion vs oracle", champ, oraclePlay, 3, 11)
	tournament(m, "oracle vs champion", oraclePlay, champ, 3, 11)
	fmt.Printf("(oracle player fell back to a random legal move %d times when its own budget was exceeded)\n", *unsolved)
}

func auditPlayer(m *model, o *Oracle, name string, p player, positions []trainPos, budget int) {
	for _, seat := range []bool{false, true} {
		d, blown, missed, unresolved := sampledCheck(m, o, p, positions, budget, seat)
		label := "O"
		if seat {
			label = "X"
		}
		tacticalTotal := 0
		for _, pos := range positions {
			if pos.maximizes == seat && pos.tactical {
				tacticalTotal++
			}
		}
		fmt.Printf("  %s as %s: %d decisions (%d tactical), %d game-losing, %d missed wins, %d unresolved\n",
			name, label, d, tacticalTotal, blown, missed, unresolved)
	}
}

func runVerify(m *model, bias, win, par, lam float64, verifyGames int) {
	fmt.Printf("champion: blockBias %.3f  winBias %.3f  parityBias %.3f  lambda %.3f\n", bias, win, par, lam)
	o := NewOracle()
	positions, skipped := collectPositions(m, o, verifyGames, defaultMinDiscs, defaultOracleBudget, 13)
	fmt.Printf("sampled %d decision points from %d self-play games (%d skipped: oracle exceeded a %d-node budget)\n",
		len(positions), verifyGames, skipped, defaultOracleBudget)
	tactical := 0
	for _, p := range positions {
		if p.tactical {
			tactical++
		}
	}
	fmt.Printf("of those, %d are \"must-play\" positions (a strictly worse legal alternative exists)\n\n", tactical)

	fmt.Println("naive net:")
	naive := evalPlayer(m, evalNet{net: m.toPetriDeclared(), rates: m.rates}, naiveScore)
	auditPlayer(m, o, "naive", naive, positions, defaultOracleBudget)

	fmt.Println("\nchampion net:")
	champ := championPlayer(m, bias, win, par, lam)
	auditPlayer(m, o, "champion", champ, positions, defaultOracleBudget)

	fmt.Println("\nNOTE: this is a sampled referee, not an exhaustive one. It checks every")
	fmt.Println("legal move against the exact oracle at every SAMPLED decision point, which")
	fmt.Println("is a precise per-position guarantee; it is not a claim of coverage over")
	fmt.Println("Connect Four's full ~4.5*10^12-position tree. See README.md.")
}

func runFit(m *model, games, iters int) {
	o := NewOracle()
	positions, skipped := collectPositions(m, o, games, defaultMinDiscs, defaultOracleBudget, 7)
	fmt.Printf("training positions: %d (%d skipped: oracle exceeded budget) from %d self-play games\n",
		len(positions), skipped, games)
	if len(positions) == 0 {
		fmt.Println("no training positions resolved; raise the oracle budget or lower defaultMinDiscs")
		return
	}
	bias, win, lam := fitChampion(m, positions, iters, true)
	fmt.Printf("\nfitted: blockBias %.3f  winBias %.3f  lambda %.3f  (train loss %.6f)\n",
		bias, win, lam, rankLoss(m, positions, bias, win, 0, lam))

	champ := championPlayer(m, bias, win, 0, lam)
	fmt.Println("\nreferee on the training sample itself (not held-out):")
	auditPlayer(m, o, "champion", champ, positions, defaultOracleBudget)

	fmt.Println("\nreferee on a fresh sample:")
	runVerify(m, bias, win, 0, lam, 25)
}

// runFitParity implements README finding 7: calibrate the parity-bias tier
// in both configurations (alongside blockBias, and replacing it), then
// referee BOTH against the same held-out sample as the existing single-tier
// champion (blockBias=1.342, winBias=0, parityBias=0, lambda=0.238;
// baseline on the 2077-decision/250-game sample: O 129/1003, X 19/1074).
func runFitParity(m *model, games, iters int) {
	o := NewOracle()
	positions, skipped := collectPositions(m, o, games, defaultMinDiscs, defaultOracleBudget, 7)
	fmt.Printf("training positions: %d (%d skipped: oracle exceeded budget) from %d self-play games\n\n",
		len(positions), skipped, games)
	if len(positions) == 0 {
		fmt.Println("no training positions resolved; raise the oracle budget or lower defaultMinDiscs")
		return
	}

	fmt.Println("== configuration A: parityBias ALONGSIDE blockBias ==")
	blockA, parA, lamA := fitChampionBlockParity(m, positions, iters, true)
	fmt.Printf("fitted: blockBias %.3f  parityBias %.3f  lambda %.3f  (train loss %.6f)\n",
		blockA, parA, lamA, rankLoss(m, positions, blockA, 0, parA, lamA))
	champA := championPlayer(m, blockA, 0, parA, lamA)
	fmt.Println("referee on a fresh held-out sample:")
	verifyGames := 250
	oA := NewOracle()
	posA, skippedA := collectPositions(m, oA, verifyGames, defaultMinDiscs, defaultOracleBudget, 13)
	fmt.Printf("  sampled %d decision points from %d self-play games (%d skipped)\n", len(posA), verifyGames, skippedA)
	auditPlayer(m, oA, "config A (block+parity)", champA, posA, defaultOracleBudget)

	fmt.Println("\n== configuration B: parityBias REPLACING blockBias ==")
	parB, lamB := fitChampionParityOnly(m, positions, iters, true)
	fmt.Printf("fitted: parityBias %.3f  lambda %.3f  (train loss %.6f)\n",
		parB, lamB, rankLoss(m, positions, 0, 0, parB, lamB))
	champB := championPlayer(m, 0, 0, parB, lamB)
	fmt.Println("referee on the SAME held-out sample as config A:")
	auditPlayer(m, oA, "config B (parity-only)", champB, posA, defaultOracleBudget)

	fmt.Println("\n== baseline for comparison: single-tier champion (blockBias only) ==")
	champBase := championPlayer(m, championBlockBias, 0, 0, championLambda)
	auditPlayer(m, oA, "baseline (block-only)", champBase, posA, defaultOracleBudget)

	fmt.Println("\nNOTE: sampled referee, not exhaustive — see README.md.")
}

// runLookahead implements README finding 8: N-ply minimax with the
// champion ODE evaluator as the leaf scorer (lookahead.go), refereed on the
// SAME 250-game/seed-13 sample as every other finding (2077 decisions at
// full strength) so results are directly comparable to the 0-ply baseline:
// O 129/1003 (12.9%), X 19/1074 (1.8%). Cost grows roughly 7x per extra
// ply, so maxDecisions truncates the sample when a full run would take
// hours — the truncation, if any, is always stated exactly (how many of
// how many, in original sampling order), never silently substituted for a
// smaller default.
func runLookahead(m *model, depth, maxDecisions int) {
	fmt.Printf("lookahead: depth=%d (see lookahead.go for the ply-counting convention)\n", depth)
	o := NewOracle()
	positions, skipped := collectPositions(m, o, 250, defaultMinDiscs, defaultOracleBudget, 13)
	total := len(positions)
	fmt.Printf("sampled %d decision points from 250 self-play games (%d skipped: oracle exceeded a %d-node budget)\n",
		total, skipped, defaultOracleBudget)
	if maxDecisions > 0 && maxDecisions < total {
		positions = positions[:maxDecisions]
		fmt.Printf("TRUNCATED to the first %d of %d decisions (in original sampling order) to keep this run's\n"+
			"wall-clock time reasonable at this depth — see README finding 8 for exact coverage at each depth.\n",
			maxDecisions, total)
	} else {
		fmt.Printf("using the FULL sample (%d decisions), no truncation.\n", total)
	}

	ev := m.toPetriChampion(championBlockBias, 0, 0)
	p := lookaheadPlayer(ev, depth)

	start := time.Now()
	auditPlayer(m, o, fmt.Sprintf("lookahead depth=%d", depth), p, positions, defaultOracleBudget)
	elapsed := time.Since(start)
	perDecision := time.Duration(0)
	if len(positions) > 0 {
		perDecision = elapsed / time.Duration(len(positions))
	}
	fmt.Printf("\nwall-clock: %v total, %v per decision (%d decisions, includes the oracle re-check of the\n"+
		"chosen move, which is fast/cached relative to the lookahead search itself)\n", elapsed, perDecision, len(positions))
	fmt.Println("\nNOTE: sampled referee, not exhaustive — see README.md. This mode is a search+eval")
	fmt.Println("hybrid, not the search-free evaluator findings 1-7 investigate — see lookahead.go.")
}

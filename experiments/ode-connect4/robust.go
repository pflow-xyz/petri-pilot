// robust.go: README finding 10 — a robustness gate for finding 9's clm_*
// structural claim-account tier. Finding 9 found that config A (clm_*
// alongside blk_*) won the referee on one training fit and lost on a
// second, same recipe, different self-play seed — a sign flip on the exact
// same held-out sample. This file answers the follow-up question finding 9
// left open: is that instability typical, and can a robustness-checked
// fitting discipline (referee every fit on a primary held-out sample,
// gate any apparent win against a SECOND, independent held-out sample,
// and try averaging the fits) rescue a reliable win from a real,
// non-placebo construction. See README finding 10 for the full writeup;
// this file just drives it.
package main

import "fmt"

// claimPoint is one calibrated (blockBias, claimBias, lambda) point for
// config A (clm_* alongside blk_*), tagged with where it came from.
type claimPoint struct {
	label  string
	block  float64
	claim  float64
	lambda float64
}

// refereeResult is one player's seat-by-seat blunder count on a fixed,
// already-oracle-labeled sample.
type refereeResult struct {
	oDecisions, oBlown int
	xDecisions, xBlown int
}

func refereeClaimPoint(m *model, o *Oracle, positions []trainPos, block, claim, lam float64, budget int) refereeResult {
	p := championPlayer(m, block, 0, 0, claim, lam)
	var r refereeResult
	r.oDecisions, r.oBlown, _, _ = sampledCheck(m, o, p, positions, budget, false)
	r.xDecisions, r.xBlown, _, _ = sampledCheck(m, o, p, positions, budget, true)
	return r
}

// beatsBaseline applies this file's stop-condition criterion: O's blunder
// count must fall below the baseline refereed IN THE SAME RUN on the SAME
// sample (the only apples-to-apples comparison available — see README's
// note throughout on run-to-run floating-point/map-iteration noise), and
// X's blunder count must not rise "materially" — defined here as more than
// +3 decisions, consistent with finding 9's own reading of its config A
// fit 1 (+1, accepted as noise-scale) vs. findings 6/7 (+12 to +28,
// rejected outright).
func beatsBaseline(r, baseline refereeResult) bool {
	return r.oBlown < baseline.oBlown && r.xBlown <= baseline.xBlown+3
}

// runRobustClaim implements README finding 10. trainSeeds gives the
// self-play seeds for the new training fits (must not include 7 — that
// seed produced both of finding 9's on-record fits, and the brief
// specifically asks for INDEPENDENTLY seeded samples). valSeed is the
// self-play seed for the second, independent validation sample used only
// for the robustness gate — distinct from 13 (the primary sample every
// other finding in this file referees on) and from every trainSeed.
func runRobustClaim(m *model, games, iters int, trainSeeds []int64, valSeed int64, refereeGames int) {
	fmt.Printf("== finding 10: robustness gate for finding 9 config A (clm_* alongside blk_*) ==\n")
	fmt.Printf("recipe: %d self-play games / %d Nelder-Mead iterations per new fit, %d new independently-seeded fits\n\n",
		games, iters, len(trainSeeds))

	// The two fits already on record (finding 9's config A, both fits),
	// reproduced here verbatim so every point in this table is refereed in
	// the SAME process, on the SAME two held-out samples — the exact
	// apples-to-apples comparison finding 9 itself could not make across
	// separate `go run .` invocations.
	points := []claimPoint{
		{"finding 9 fit 1 (15 games/20 iters, seed 7)", 1.258, 2.202, 0.259},
		{"finding 9 fit 2 (20 games/30 iters, seed 7)", 1.395, 1.845, 0.146},
	}

	for i, seed := range trainSeeds {
		o := NewOracle()
		trainPositions, skipped := collectPositions(m, o, games, defaultMinDiscs, defaultOracleBudget, seed)
		fmt.Printf("fit %d/%d: training seed %d -> %d positions (%d skipped)\n", i+1, len(trainSeeds), seed, len(trainPositions), skipped)
		if len(trainPositions) == 0 {
			fmt.Printf("  no training positions resolved at this seed; skipping\n")
			continue
		}
		block, claim, lam := fitChampionClaim(m, trainPositions, iters, false)
		loss := rankLoss(m, trainPositions, block, 0, 0, claim, lam)
		fmt.Printf("  fitted: block %.3f  claim %.3f  lambda %.3f  (train loss %.4f)\n", block, claim, lam, loss)
		points = append(points, claimPoint{
			label:  fmt.Sprintf("new fit %d (%d games/%d iters, seed %d)", i+1, games, iters, seed),
			block:  block,
			claim:  claim,
			lambda: lam,
		})
	}

	// Ensemble: plain average of the constants across every point above
	// (the 2 on record plus every new fit that resolved).
	var sumBlock, sumClaim, sumLam float64
	for _, p := range points {
		sumBlock += p.block
		sumClaim += p.claim
		sumLam += p.lambda
	}
	n := float64(len(points))
	ensemble := claimPoint{
		label:  fmt.Sprintf("ENSEMBLE: mean of %d fits above", len(points)),
		block:  sumBlock / n,
		claim:  sumClaim / n,
		lambda: sumLam / n,
	}

	// Primary held-out sample: seed 13, refereeGames games — identical to
	// every other finding's `verify`/`verifyclaim` sample, built ONCE and
	// reused for every point so the comparison is apples-to-apples within
	// this one process.
	oPrimary := NewOracle()
	primary, primarySkipped := collectPositions(m, oPrimary, refereeGames, defaultMinDiscs, defaultOracleBudget, 13)
	fmt.Printf("\nprimary held-out sample: seed 13, %d games -> %d decisions (%d skipped)\n", refereeGames, len(primary), primarySkipped)

	// Independent validation sample: a DIFFERENT seed, same size, never
	// used to fit anything and not the sample every other finding
	// compares on — this is the robustness gate itself (step 3 of the
	// brief).
	oVal := NewOracle()
	validation, valSkipped := collectPositions(m, oVal, refereeGames, defaultMinDiscs, defaultOracleBudget, valSeed)
	fmt.Printf("independent validation sample: seed %d, %d games -> %d decisions (%d skipped)\n\n", valSeed, refereeGames, len(validation), valSkipped)

	baseline := refereeClaimPoint(m, oPrimary, primary, championBlockBias, 0, championLambda, defaultOracleBudget)
	baselineVal := refereeClaimPoint(m, oVal, validation, championBlockBias, 0, championLambda, defaultOracleBudget)
	fmt.Printf("baseline (block %.3f, lambda %.3f) -- primary: O %d/%d  X %d/%d | validation: O %d/%d  X %d/%d\n\n",
		championBlockBias, championLambda,
		baseline.oBlown, baseline.oDecisions, baseline.xBlown, baseline.xDecisions,
		baselineVal.oBlown, baselineVal.oDecisions, baselineVal.xBlown, baselineVal.xDecisions)

	beatsPrimaryCount := 0
	survivesGateCount := 0
	report := func(p claimPoint) (pr, vr refereeResult, beatsPrim, survives bool) {
		pr = refereeClaimPoint(m, oPrimary, primary, p.block, p.claim, p.lambda, defaultOracleBudget)
		vr = refereeClaimPoint(m, oVal, validation, p.block, p.claim, p.lambda, defaultOracleBudget)
		beatsPrim = beatsBaseline(pr, baseline)
		survives = beatsPrim && beatsBaseline(vr, baselineVal)
		gate := "N/A (does not beat baseline on primary)"
		if beatsPrim {
			if survives {
				gate = "PASS"
			} else {
				gate = "FAIL"
			}
		}
		fmt.Printf("%-52s block %.3f claim %.3f lambda %.3f | primary O %3d/%d X %3d/%d (beats baseline: %-5v) | validation O %3d/%d X %3d/%d | gate: %s\n",
			p.label, p.block, p.claim, p.lambda,
			pr.oBlown, pr.oDecisions, pr.xBlown, pr.xDecisions, beatsPrim,
			vr.oBlown, vr.oDecisions, vr.xBlown, vr.xDecisions, gate)
		return
	}

	fmt.Println("--- all fitted points, refereed on the SAME primary sample, gated against the SAME independent validation sample ---")
	for _, p := range points {
		_, _, beatsPrim, survives := report(p)
		if beatsPrim {
			beatsPrimaryCount++
		}
		if survives {
			survivesGateCount++
		}
	}

	fmt.Println("\n--- ensemble point (mean of all fits above) ---")
	_, _, ensembleBeatsPrim, ensembleSurvives := report(ensemble)

	fmt.Printf("\nSUMMARY: %d/%d fitted points beat baseline on the primary referee sample; %d/%d of those survive the independent validation gate.\n",
		beatsPrimaryCount, len(points), survivesGateCount, beatsPrimaryCount)
	fmt.Printf("Ensemble point beats baseline on primary: %v; survives validation gate: %v\n", ensembleBeatsPrim, ensembleSurvives)

	fmt.Println("\nNOTE: sampled referee, not exhaustive — see README.md.")
}

// Finite-difference gates for the gradient calibration. The analytic
// derivatives ride an adaptive augmented solve while the FD reference
// runs plain adaptive solves, so the step sequences differ; the gates are
// correspondingly looser than go-pflow/learn's fixed-step ones.
package main

import (
	"math"
	"testing"

	"github.com/pflow-xyz/go-pflow/learn"
)

// auditPositions returns exactly the four tactical audit positions,
// minimax-labeled (collectPositions with zero self-play games).
func auditPositions(m *model) []trainPos {
	return collectPositions(m, 0, 7)
}

// plainScore is the play-time objective: odeFinal on toPetriChampion —
// what championPlayer computes.
func plainScore(m *model, ev evalNet, mk marking, maximizes bool, lam float64) float64 {
	f := m.odeFinal(ev.net, mk, ev.rates)
	if maximizes {
		return f["win_x"] - f["win_o"]
	}
	return f["x_turn"] + f["o_turn"] + lam*f["win_o"]
}

// TestScoreGradBiasVsFD gates ds/dbias through the derived net against a
// central finite difference of the plain-solve score, at both champion
// biases, both seat objectives, on the four audit positions' first moves.
func TestScoreGradBiasVsFD(t *testing.T) {
	m := buildModel()
	positions := auditPositions(m)
	if len(positions) != 4 {
		t.Fatalf("audit positions: got %d, want 4", len(positions))
	}
	const lam = championLambda
	for _, b := range []float64{1, championBlockBias} {
		cg := m.toChampionGrad(b)
		h := 1e-4 * (1 + b)
		evPlus := m.toPetriChampion(b + h)
		evMinus := m.toPetriChampion(b - h)
		for pi, p := range positions {
			mk := m.fire(p.moves[0], p.mk)
			for _, maximizes := range []bool{true, false} {
				_, dsdb, _, ok := cg.scoreGrad(m, mk, maximizes, lam)
				if !ok {
					t.Fatalf("b=%v pos=%d max=%v: scoreGrad failed", b, pi, maximizes)
				}
				fd := (plainScore(m, evPlus, mk, maximizes, lam) -
					plainScore(m, evMinus, mk, maximizes, lam)) / (2 * h)
				if diff := math.Abs(dsdb - fd); diff >= math.Max(1e-3, 1e-2*math.Abs(fd)) {
					t.Errorf("b=%v pos=%d max=%v: ds/dbias analytic %.8g vs FD %.8g (|d|=%.3g)",
						b, pi, maximizes, dsdb, fd, diff)
				}
			}
		}
	}
}

// TestScoreGradLambdaAnalytic pins ds/dlambda == v(win_o) exactly for the
// O objective (and 0 for X), and that an FD in lambda recomputed from the
// SAME cached final state matches to 1e-12 — lambda never enters the
// solve, only the readout.
func TestScoreGradLambdaAnalytic(t *testing.T) {
	m := buildModel()
	positions := auditPositions(m)
	cg := m.toChampionGrad(championBlockBias)
	const lam = championLambda
	for pi, p := range positions {
		mk := m.fire(p.moves[0], p.mk)

		// X objective: lambda absent.
		_, _, dsdlamX, ok := cg.scoreGrad(m, mk, true, lam)
		if !ok {
			t.Fatalf("pos=%d: scoreGrad(X) failed", pi)
		}
		if dsdlamX != 0 {
			t.Errorf("pos=%d: X seat ds/dlambda = %v, want exactly 0", pi, dsdlamX)
		}

		// O objective: ds/dlambda is the win_o read itself.
		ev := m.toPetriChampion(championBlockBias)
		f := m.odeFinal(ev.net, mk, ev.rates)
		_, _, dsdlam, ok := cg.scoreGrad(m, mk, false, lam)
		if !ok {
			t.Fatalf("pos=%d: scoreGrad(O) failed", pi)
		}
		// FD in lambda from the cached finals: exactly f[win_o] in exact
		// arithmetic; 1e-12 covers rounding.
		h := 1e-4
		sPlus := f["x_turn"] + f["o_turn"] + (lam+h)*f["win_o"]
		sMinus := f["x_turn"] + f["o_turn"] + (lam-h)*f["win_o"]
		fd := (sPlus - sMinus) / (2 * h)
		if math.Abs(fd-f["win_o"]) > 1e-12 {
			t.Errorf("pos=%d: cached-final FD %v vs f[win_o] %v", pi, fd, f["win_o"])
		}
		// And the analytic value equals its own solve's win_o read — cross
		// check against the plain solve's within tolerance.
		if diff := math.Abs(dsdlam - f["win_o"]); diff >= math.Max(1e-3, 1e-2*math.Abs(f["win_o"])) {
			t.Errorf("pos=%d: ds/dlambda %v vs plain-solve win_o %v (|d|=%.3g)",
				pi, dsdlam, f["win_o"], diff)
		}
	}
}

// decisionHinge returns one decision's hinge term and its active-set
// signature: whether the margin is violated and which options are the
// (first-max) preferred and non-preferred argmaxes. FD decision-coordinates
// where the signature flips inside +/-h are skipped — the subgradient is
// not the secant there.
func decisionHinge(d learn.RankedDecision) (hinge float64, sig [3]int) {
	iPref, iNon := -1, -1
	n := min(len(d.Scores), len(d.Preferred))
	for i := 0; i < n; i++ {
		if d.Preferred[i] {
			if iPref < 0 || d.Scores[i] > d.Scores[iPref] {
				iPref = i
			}
		} else if iNon < 0 || d.Scores[i] > d.Scores[iNon] {
			iNon = i
		}
	}
	// An inactive decision contributes nothing to loss or gradient, so
	// its argmax indices are irrelevant — recording them would skip
	// coordinates over flips the objective never sees.
	if iPref >= 0 && iNon >= 0 {
		if v := rankMargin + d.Scores[iNon] - d.Scores[iPref]; v > 0 {
			return v, [3]int{1, iPref, iNon}
		}
	}
	return 0, [3]int{0, -1, -1}
}

// TestRankLossGradVsFD gates the assembled hinge subgradient in log space
// against central FD, at u=(0,0) and the champion point — decomposed PER
// DECISION. The loss is a sum of per-decision hinge terms, so each
// decision's term is FD'd on its own against that decision's subgradient
// contribution, and only decisions whose active-set signature flips inside
// +/-h are skipped. A whole-loss FD would skip the entire point whenever
// ANY near-tie flips — with tens of decisions that made the gate
// near-vacuous — so a minimum checked count is asserted too.
func TestRankLossGradVsFD(t *testing.T) {
	m := buildModel()
	positions := auditPositions(m)
	// eval returns per-decision hinge terms and signatures, plus each
	// decision's analytic log-space gradient contribution per coordinate.
	eval := func(u [2]float64) (hinges []float64, sigs [][3]int, contrib [][2]float64) {
		b, l := math.Exp(u[0]), math.Exp(u[1])
		cg := m.toChampionGrad(b)
		decisions, dbs, dlams, _, ok := evalDecisions(m, cg, positions, l)
		if !ok {
			t.Fatal("evalDecisions failed")
		}
		hinges = make([]float64, len(decisions))
		sigs = make([][3]int, len(decisions))
		contrib = make([][2]float64, len(decisions))
		sum := 0.0
		for di, d := range decisions {
			hinges[di], sigs[di] = decisionHinge(d)
			sum += hinges[di]
			if sigs[di][0] == 1 {
				iPref, iNon := sigs[di][1], sigs[di][2]
				contrib[di] = [2]float64{
					b * (dbs[di][iNon] - dbs[di][iPref]),
					l * (dlams[di][iNon] - dlams[di][iPref]),
				}
			}
		}
		// Pin the decomposition: the per-decision terms sum to the loss,
		// and the per-decision contributions to hingeSubgrad's assembly.
		if loss := learn.HingeRankLoss(decisions, rankMargin); math.Abs(sum-loss) > 1e-12*(1+math.Abs(loss)) {
			t.Fatalf("u=%v: per-decision hinges sum %.12g vs HingeRankLoss %.12g", u, sum, loss)
		}
		dLdb, dLdlam := hingeSubgrad(decisions, dbs, dlams, rankMargin)
		var cb, cl float64
		for _, c := range contrib {
			cb += c[0]
			cl += c[1]
		}
		if math.Abs(cb-b*dLdb) > 1e-12*(1+math.Abs(cb)) || math.Abs(cl-l*dLdlam) > 1e-12*(1+math.Abs(cl)) {
			t.Fatalf("u=%v: contribution sums (%.12g, %.12g) vs hingeSubgrad (%.12g, %.12g)",
				u, cb, cl, b*dLdb, l*dLdlam)
		}
		return hinges, sigs, contrib
	}
	const h = 1e-4
	checked, skipped := 0, 0
	for _, u0 := range [][2]float64{{0, 0}, {math.Log(championBlockBias), math.Log(championLambda)}} {
		_, s0, contrib := eval(u0)
		for c := 0; c < 2; c++ {
			up, um := u0, u0
			up[c] += h
			um[c] -= h
			hp, sp, _ := eval(up)
			hm, sm, _ := eval(um)
			for di := range hp {
				if s0[di] != sp[di] || sp[di] != sm[di] {
					skipped++
					t.Logf("u=%v coord %d decision %d: active set flips inside +/-h, skipped", u0, c, di)
					continue
				}
				checked++
				fd := (hp[di] - hm[di]) / (2 * h)
				den := math.Max(math.Abs(fd), 1e-8)
				if rel := math.Abs(contrib[di][c]-fd) / den; rel >= 1e-2 {
					t.Errorf("u=%v coord %d decision %d: subgrad %.8g vs FD %.8g (rel %.3g >= 1e-2)",
						u0, c, di, contrib[di][c], fd, rel)
				}
			}
		}
	}
	t.Logf("decision-coordinates: %d checked, %d skipped", checked, skipped)
	// 2 points x 2 coordinates x 4 decisions = 16 terms; the gate must
	// never silently go vacuous again.
	if checked < 8 {
		t.Errorf("only %d decision-coordinates survived the skip rule (want >= 8): gate is near-vacuous", checked)
	}
}

// TestHingeSubgradDefinitions pins the kink and tie conventions on
// synthetic fixtures, independent of any solve.
func TestHingeSubgradDefinitions(t *testing.T) {
	// Kink: margin exactly satisfied (v_d == 0) contributes zero.
	kink := []learn.RankedDecision{{
		Scores:    []float64{1.0, 1.0 - rankMargin},
		Preferred: []bool{true, false},
	}}
	dLdb, dLdlam := hingeSubgrad(kink, [][]float64{{3, 5}}, [][]float64{{7, 11}}, rankMargin)
	if dLdb != 0 || dLdlam != 0 {
		t.Errorf("kink (v_d == 0): got (%v, %v), want (0, 0)", dLdb, dLdlam)
	}

	// Tied argmax: options 1 and 2 are both non-preferred maximizers; the
	// smaller index (1) supplies the sensitivities. Active violation:
	// bestNon 2.0 > bestPref 1.0.
	tie := []learn.RankedDecision{{
		Scores:    []float64{1.0, 2.0, 2.0},
		Preferred: []bool{true, false, false},
	}}
	dLdb, dLdlam = hingeSubgrad(tie, [][]float64{{1, 10, 100}}, [][]float64{{2, 20, 200}}, rankMargin)
	if dLdb != 10-1 || dLdlam != 20-2 {
		t.Errorf("tied argmax: got (%v, %v), want (9, 18) — smaller index wins", dLdb, dLdlam)
	}

	// Tied preferred argmax too: options 0 and 1 both preferred at 1.0;
	// index 0 supplies the sensitivities.
	tiePref := []learn.RankedDecision{{
		Scores:    []float64{1.0, 1.0, 2.0},
		Preferred: []bool{true, true, false},
	}}
	dLdb, dLdlam = hingeSubgrad(tiePref, [][]float64{{4, 40, 400}}, [][]float64{{5, 50, 500}}, rankMargin)
	if dLdb != 400-4 || dLdlam != 500-5 {
		t.Errorf("tied preferred argmax: got (%v, %v), want (396, 495)", dLdb, dLdlam)
	}
}

// TestChampionGradDerivationParity pins toChampionGrad's net to
// toPetriChampion's: identical place, transition and arc sets, and every
// RateFunc evaluated at the shared bias equals the rates map entry.
func TestChampionGradDerivationParity(t *testing.T) {
	m := buildModel()
	const b = championBlockBias
	ev := m.toPetriChampion(b)
	cg := m.toChampionGrad(b)

	if len(cg.net.Places) != len(ev.net.Places) {
		t.Errorf("place count: grad %d vs champion %d", len(cg.net.Places), len(ev.net.Places))
	}
	for p := range ev.net.Places {
		if _, ok := cg.net.Places[p]; !ok {
			t.Errorf("place %q missing from grad net", p)
		}
	}
	if len(cg.net.Transitions) != len(ev.net.Transitions) {
		t.Errorf("transition count: grad %d vs champion %d",
			len(cg.net.Transitions), len(ev.net.Transitions))
	}
	for tr := range ev.net.Transitions {
		if _, ok := cg.net.Transitions[tr]; !ok {
			t.Errorf("transition %q missing from grad net", tr)
		}
	}
	evArcs := map[string]int{}
	for _, a := range ev.net.Arcs {
		evArcs[a.Source+"->"+a.Target] += int(a.GetWeightSum())
	}
	cgArcs := map[string]int{}
	for _, a := range cg.net.Arcs {
		cgArcs[a.Source+"->"+a.Target] += int(a.GetWeightSum())
	}
	if len(evArcs) != len(cgArcs) {
		t.Errorf("arc set size: grad %d vs champion %d", len(cgArcs), len(evArcs))
	}
	for k, w := range evArcs {
		if cgArcs[k] != w {
			t.Errorf("arc %s: grad weight %d vs champion %d", k, cgArcs[k], w)
		}
	}

	if len(cg.rfs) != len(ev.rates) {
		t.Errorf("rate entries: grad %d vs champion %d", len(cg.rfs), len(ev.rates))
	}
	for tr, want := range ev.rates {
		rf, ok := cg.rfs[tr]
		if !ok {
			t.Errorf("transition %q has no RateFunc", tr)
			continue
		}
		if got := rf.Eval(nil, 0); got != want {
			t.Errorf("rate at %q: RateFunc %v vs rates map %v", tr, got, want)
		}
	}
	// The 48 blk_* entries all share the ONE *SharedScalar.
	blkCount := 0
	for tr, rf := range cg.rfs {
		if len(tr) > 4 && tr[:4] == "blk_" {
			blkCount++
			if rf != learn.RateFunc(cg.bias) {
				t.Errorf("blk transition %q does not share cg.bias", tr)
			}
		}
	}
	if blkCount != 48 {
		t.Errorf("blk_* transitions: %d, want 48", blkCount)
	}
}

// TestFitGradChampion is the acceptance run: gradient fit from (1,1) on
// the four tactical audit positions, then the exhaustive referee on both
// seats, plus the solve-counter arithmetic for one hand-counted
// rankLossGrad call.
//
// The training set is the audit positions ON PURPOSE, not self-play.
// Measured on this branch: on a 12-game self-play set (73 positions) adam
// converges by ~iter 80 to (bias 2.67, lambda 1.11) at train loss 0.100 —
// BELOW the champion point's 0.166 on the same set — and that point fails
// the referee (2 game-losing, 10 missed wins as O). The hinge minimum on
// that set does not identify perfect play at any iteration budget, and at
// ~19s per iteration the run also blows any test timeout. On the audit
// set the fit reaches hinge loss exactly 0 in a handful of iterations at
// (bias ~2.03, lambda ~1.84), and that point passes the referee 0/0 on
// both seats — an acceptance that is both true and affordable.
func TestFitGradChampion(t *testing.T) {
	if testing.Short() {
		t.Skip("gradient fit + exhaustive referee: skipped in -short")
	}
	m := buildModel()
	positions := auditPositions(m)

	// Counter arithmetic: one rankLossGrad call costs exactly one
	// sensitivity solve per (position, candidate move).
	totalMoves := 0
	for _, p := range positions {
		totalMoves += len(p.moves)
	}
	cg := m.toChampionGrad(1)
	_, _, _, solves := rankLossGrad(m, cg, positions, 1)
	if solves != totalMoves {
		t.Fatalf("rankLossGrad solves = %d, want sum len(moves) = %d", solves, totalMoves)
	}

	bias, lam, sensSolves := fitChampionGrad(m, positions, 200, false)
	trainLoss := rankLoss(m, positions, bias, lam)
	t.Logf("fitted: bias %.3f lambda %.3f train loss %.6g (%d sensitivity solves)",
		bias, lam, trainLoss, sensSolves)
	if trainLoss > 1e-6 {
		t.Errorf("audit-set train loss %.6g after fit, want ~0", trainLoss)
	}
	if bias <= 1 {
		t.Errorf("fitted bias %.3f did not move above its start 1", bias)
	}
	p := championPlayer(m.toPetriChampion(bias), lam)
	for _, seat := range []bool{false, true} {
		d, blown, missed := exhaustiveCheck(m, p, seat)
		name := "O"
		if seat {
			name = "X"
		}
		t.Logf("as %s: %d decisions, %d game-losing, %d missed wins", name, d, blown, missed)
		if blown != 0 || missed != 0 {
			t.Errorf("referee as %s: %d game-losing, %d missed wins; want 0/0", name, blown, missed)
		}
	}
}

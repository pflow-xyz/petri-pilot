// Oracle labeling, calibration, and the exhaustive referee.
package main

import (
	"math"
	"math/rand"

	"github.com/pflow-xyz/go-pflow/learn"
)

type trainPos struct {
	mk        marking
	moves     []string
	maximizes bool
	optimal   map[string]bool
}

func auditPositions(m *model) []marking {
	return []marking{
		m.position([]string{"01", "12", "21", "31", "33"}, []string{"11", "22", "30", "32"}, "o"),
		m.position([]string{"12", "20", "21", "31", "33"}, []string{"11", "22", "30", "32"}, "o"),
		m.position([]string{"30", "31"}, []string{"33"}, "o"),
		m.position([]string{"30", "20"}, []string{"31"}, "o"),
	}
}

func collectPositions(m *model, games int, seed int64) []trainPos {
	rng := rand.New(rand.NewSource(seed))
	seen := map[string]bool{}
	var out []trainPos
	add := func(mk marking) {
		mk = m.fireHouse(mk)
		key := boardKey(mk)
		if seen[key] {
			return
		}
		_, moves, maximizes, ok := m.legalMoves(mk)
		if !ok {
			return
		}
		seen[key] = true
		optimal := map[string]bool{}
		for _, mv := range m.optimalSet(mk, moves, maximizes) {
			optimal[mv] = true
		}
		out = append(out, trainPos{mk, moves, maximizes, optimal})
	}
	for _, mk := range auditPositions(m) {
		add(mk)
	}
	for g := 0; g < games; g++ {
		mk := m.start()
		for {
			mk = m.fireHouse(mk)
			_, moves, _, ok := m.legalMoves(mk)
			if !ok {
				break
			}
			add(mk)
			mk = m.fire(moves[rng.Intn(len(moves))], mk)
		}
	}
	return out
}

func rankLoss(m *model, positions []trainPos, winBias, blockBias, lam float64) float64 {
	ev := m.toPetriPolicy(winBias, blockBias)
	decisions := make([]learn.RankedDecision, 0, len(positions))
	for _, p := range positions {
		d := learn.RankedDecision{
			Scores: make([]float64, len(p.moves)), Preferred: make([]bool, len(p.moves)),
		}
		for i, mv := range p.moves {
			f := m.odeFinal(ev.net, m.fire(mv, p.mk), ev.rates)
			score := f["win_x"] - f["win_o"]
			if !p.maximizes {
				score = f["x_turn"] + f["o_turn"] + lam*f["win_o"]
			}
			d.Scores[i], d.Preferred[i] = score, p.optimal[mv]
		}
		decisions = append(decisions, d)
	}
	return learn.HingeRankLoss(decisions, 0.0005)
}

func fitPolicy(m *model, positions []trainPos, iters int, verbose bool) (winBias, blockBias, lam float64) {
	f := func(logp []float64) float64 {
		return rankLoss(m, positions, math.Exp(logp[0]), math.Exp(logp[1]), math.Exp(logp[2]))
	}
	opts := learn.DefaultFitOptions()
	opts.MaxIters, opts.Tolerance, opts.Verbose = iters, 1e-9, verbose
	result, err := learn.Minimize(f, []float64{0, 0, 0}, opts)
	if err != nil {
		panic(err)
	}
	return math.Exp(result.Params[0]), math.Exp(result.Params[1]), math.Exp(result.Params[2])
}

// exhaustiveCheck follows every legal opponent continuation while the ODE
// evaluator occupies one fixed seat. Every distinct evaluator decision is
// compared with the exact oracle; only equal game value passes.
type refereeFailure struct {
	position trainPos
	key      string
	choice   string
	before   int
	after    int
}

func runReferee(m *model, p player, variantIsX bool) (decisions, blown, missed int, failures []refereeFailure) {
	choiceCache := map[string]string{}
	seenDecision := map[string]bool{}
	seenWalk := map[string]bool{}
	var walk func(marking)
	walk = func(mk marking) {
		mk = m.fireHouse(mk)
		_, moves, maximizes, ok := m.legalMoves(mk)
		if !ok {
			return
		}
		walkKey := boardKey(mk)
		if maximizes {
			walkKey += "X"
		} else {
			walkKey += "O"
		}
		if seenWalk[walkKey] {
			return
		}
		seenWalk[walkKey] = true
		if maximizes == variantIsX {
			mv, hit := choiceCache[walkKey]
			if !hit {
				mv = p(m, mk, moves, maximizes, nil)
				choiceCache[walkKey] = mv
			}
			if !seenDecision[walkKey] {
				seenDecision[walkKey] = true
				decisions++
				before := m.minimax(mk, -2, 2)
				after := m.minimax(m.fireHouse(m.fire(mv, mk)), -2, 2)
				worse, lost := after > before, after == 1
				if variantIsX {
					worse, lost = after < before, after == -1
				}
				if worse {
					optimal := map[string]bool{}
					for _, move := range m.optimalSet(mk, moves, maximizes) {
						optimal[move] = true
					}
					failures = append(failures, refereeFailure{
						position: trainPos{mk, moves, maximizes, optimal},
						key:      walkKey, choice: mv, before: before, after: after,
					})
					if lost {
						blown++
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
	return
}

func exhaustiveCheck(m *model, p player, variantIsX bool) (decisions, blown, missed int) {
	decisions, blown, missed, _ = runReferee(m, p, variantIsX)
	return
}

func failurePositions(m *model, p player) []trainPos {
	seen := map[string]bool{}
	var out []trainPos
	for _, seat := range []bool{false, true} {
		_, _, _, failures := runReferee(m, p, seat)
		for _, failure := range failures {
			if !seen[failure.key] {
				seen[failure.key] = true
				out = append(out, failure.position)
			}
		}
	}
	return out
}

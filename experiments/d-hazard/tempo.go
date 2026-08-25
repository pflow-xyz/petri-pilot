// Structural variants aimed at the last 7%: the double-corner fork, where
// the final-state coordinates (win_x, win_o, tie) of the d-hazard net are
// provably insufficient — the losing corners dominate the optimal edges on
// all three. Two families of structural adjustment:
//
//   E/F "tempo": add places whose final mass carries sequencing
//   information. Per-line threat accumulators (2 marks + open third cell,
//   all catalytic, gated on game_active) integrate how much live threat
//   each side held over the trajectory; line-pair fork detectors multiply
//   two distinct line-threat masses, so fork_<side> accrues only when two
//   different lines are threatened together.
//
//   H "block bias": shape the flow instead of the score. For each line
//   and each of its cells, a second copy of the play transition whose
//   rate is catalyzed by the opponent's two marks on that line — the ODE's
//   implicit policy then answers threats first, the way a real opponent
//   does. Sequencing enters the trajectory, so the ORIGINAL coordinates
//   can change their ranking; no new score term needed.
//
// All evaluation-only, built on the d-hazard base; the discrete game is
// untouched.
package main

import (
	"math/rand"
	"sort"
	"strconv"

	"github.com/pflow-xyz/go-pflow/petri"
)

func sortedLineNames() []string {
	names := make([]string, 0, len(winLines))
	for name := range winLines {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// evalNet is an evaluation net plus the rates for its extra transitions —
// variant transitions are net-only, so m.rates alone cannot drive them.
type evalNet struct {
	net   *petri.PetriNet
	rates map[string]float64
}

func (m *model) cloneRates() map[string]float64 {
	rates := make(map[string]float64, len(m.rates)*2)
	for k, v := range m.rates {
		rates[k] = v
	}
	return rates
}

// toPetriTempo: d-hazard + threat accumulators + fork detectors.
func (m *model) toPetriTempo() evalNet {
	net := m.toPetriDHazard()
	rates := m.cloneRates()
	names := sortedLineNames()

	read := func(t, p string, w int) {
		net.AddArc(p, t, w, false)
		net.AddArc(t, p, w, false)
	}
	for _, side := range []string{"x", "o"} {
		net.AddPlace("threat_"+side, 0, nil, 0, 0, nil)
		net.AddPlace("fork_"+side, 0, nil, 0, 0, nil)
		for _, name := range names {
			net.AddPlace("tl_"+side+"_"+name, 0, nil, 0, 0, nil)
		}
		for _, name := range names {
			line := winLines[name]
			for i := range line {
				t := "thr_" + side + "_" + name + "_" + strconv.Itoa(i)
				net.AddTransition(t, "", 0, 0, nil)
				rates[t] = 1
				for j, c := range line {
					if j == i {
						read(t, "p"+c, 1) // the open completing square
					} else {
						read(t, side+c, 1)
					}
				}
				read(t, "game_active", 1)
				net.AddArc(t, "threat_"+side, 1, false)
				net.AddArc(t, "tl_"+side+"_"+name, 1, false)
			}
		}
		for a := 0; a < len(names); a++ {
			for b := a + 1; b < len(names); b++ {
				t := "forkdet_" + side + "_" + names[a] + "_" + names[b]
				net.AddTransition(t, "", 0, 0, nil)
				rates[t] = 1
				read(t, "tl_"+side+"_"+names[a], 1)
				read(t, "tl_"+side+"_"+names[b], 1)
				read(t, "game_active", 1)
				net.AddArc(t, "fork_"+side, 1, false)
			}
		}
	}
	return evalNet{net, rates}
}

// toPetriPolicyBias: d-hazard + a two-knob policy in the flow. For each
// line, each cell c in it, and each side, two extra copies of the play
// transition: one catalyzed by the OPPONENT holding the other two cells
// (blockBias — answer threats) and one catalyzed by the side's OWN two
// cells there (winBias — convert threats). Together they approximate the
// tactical priority win > block > neutral that a real opponent plays by.
func (m *model) toPetriPolicyBias(winBias, blockBias float64) evalNet {
	net := m.toPetriDHazard()
	rates := m.cloneRates()
	names := sortedLineNames()

	addBias := func(kind, side, name string, i int, catalyst string, rate float64) {
		if rate == 0 {
			return
		}
		line := winLines[name]
		c := line[i]
		opp := "o"
		if side == "o" {
			opp = "x"
		}
		t := kind + "_" + side + "_" + name + "_" + strconv.Itoa(i)
		net.AddTransition(t, "", 0, 0, nil)
		rates[t] = rate
		net.AddArc("p"+c, t, 1, false)
		net.AddArc(side+"_turn", t, 1, false)
		net.AddArc(t, side+c, 1, false)
		net.AddArc(t, opp+"_turn", 1, false)
		net.AddArc(t, "move_tokens", 1, false)
		for j, oc := range line {
			if j != i {
				net.AddArc(catalyst+oc, t, 1, false)
				net.AddArc(t, catalyst+oc, 1, false)
			}
		}
	}
	for _, name := range names {
		line := winLines[name]
		for i := range line {
			for _, side := range []string{"x", "o"} {
				opp := "o"
				if side == "o" {
					opp = "x"
				}
				addBias("blk", side, name, i, opp, blockBias)
				addBias("winb", side, name, i, side, winBias)
			}
		}
	}
	return evalNet{net, rates}
}

// applyBlockBias adds the forced-reply transitions to an existing
// evaluation net — for each line, cell and side, a copy of the play
// transition catalyzed by the opponent's two marks on that line.
func applyBlockBias(ev evalNet, bias float64) evalNet {
	if bias == 0 {
		return ev
	}
	for _, name := range sortedLineNames() {
		line := winLines[name]
		for i, c := range line {
			for _, side := range []string{"x", "o"} {
				opp := "o"
				if side == "o" {
					opp = "x"
				}
				t := "blk_" + side + "_" + name + "_" + strconvItoa(i)
				ev.net.AddTransition(t, "", 0, 0, nil)
				ev.rates[t] = bias
				ev.net.AddArc("p"+c, t, 1, false)
				ev.net.AddArc(side+"_turn", t, 1, false)
				ev.net.AddArc(t, side+c, 1, false)
				ev.net.AddArc(t, opp+"_turn", 1, false)
				if _, ok := ev.net.Places["move_tokens"]; ok {
					ev.net.AddArc(t, "move_tokens", 1, false)
				}
				// match the base play transition's total deposit weight
				// (possibly split over top-up arcs), so an incidence-
				// weighted net biases consistently
				total := 0
				for _, a := range ev.net.Arcs {
					if a.Source == side+"_play_"+c && a.Target == side+c {
						total += int(a.GetWeightSum())
					}
				}
				if total > 1 {
					ev.net.AddArc(t, side+c, total-1, false)
				}
				for j, oc := range line {
					if j != i {
						ev.net.AddArc(opp+oc, t, 1, false)
						ev.net.AddArc(t, opp+oc, 1, false)
					}
				}
			}
		}
	}
	return ev
}

func strconvItoa(i int) string { return strconv.Itoa(i) }

// toPetriNoDraw is the declared net with the draw transition removed
// entirely: undecided mass simply stays in game_active.
func (m *model) toPetriNoDraw() evalNet {
	net := petri.NewPetriNet()
	for _, p := range m.places {
		net.AddPlace(p, m.initial[p], nil, 0, 0, nil)
	}
	for _, t := range m.transitions {
		if t == "draw" {
			continue
		}
		net.AddTransition(t, "", 0, 0, nil)
		for _, a := range m.inputs[t] {
			net.AddArc(a.from, t, a.weight, false)
		}
		for _, a := range m.outputs[t] {
			net.AddArc(t, a.to, a.weight, false)
		}
	}
	return evalNet{net, m.cloneRates()}
}

// toPetriMinimal: no draw transition AND no move_tokens place — with the
// draw gone, the counter is a write-only accumulator no rate ever reads,
// so removing it must be behavior-neutral for the ODE. The minimal
// evaluation net: cells, turns, game_active, win detectors, win places.
func (m *model) toPetriMinimal() evalNet {
	net := petri.NewPetriNet()
	for _, p := range m.places {
		if p == "move_tokens" {
			continue
		}
		net.AddPlace(p, m.initial[p], nil, 0, 0, nil)
	}
	for _, t := range m.transitions {
		if t == "draw" {
			continue
		}
		net.AddTransition(t, "", 0, 0, nil)
		for _, a := range m.inputs[t] {
			net.AddArc(a.from, t, a.weight, false)
		}
		for _, a := range m.outputs[t] {
			if a.to == "move_tokens" {
				continue
			}
			net.AddArc(t, a.to, a.weight, false)
		}
	}
	return evalNet{net, m.cloneRates()}
}

// toPetriOneShot: the minimal net with CONSUMING win detectors — the
// cell-return arcs (x_win_anti -> x11 etc.) are dropped, so a detector
// firing consumes the line it detected. Each line pays its win once
// instead of pumping continuously; threats self-extinguish as they are
// counted. Evaluation only, as ever.
func (m *model) toPetriOneShot() evalNet {
	net := petri.NewPetriNet()
	for _, p := range m.places {
		if p == "move_tokens" {
			continue
		}
		net.AddPlace(p, m.initial[p], nil, 0, 0, nil)
	}
	isDetector := func(t string) bool {
		return len(t) > 5 && (t[:5] == "x_win" || t[:5] == "o_win")
	}
	for _, t := range m.transitions {
		if t == "draw" {
			continue
		}
		net.AddTransition(t, "", 0, 0, nil)
		for _, a := range m.inputs[t] {
			net.AddArc(a.from, t, a.weight, false)
		}
		for _, a := range m.outputs[t] {
			if a.to == "move_tokens" {
				continue
			}
			if isDetector(t) && a.to != "win_x" && a.to != "win_o" {
				continue // no cell return: the detector consumes the line
			}
			net.AddArc(t, a.to, a.weight, false)
		}
	}
	return evalNet{net, m.cloneRates()}
}

// toPetriIncidence: play deposits weighted by the incidence matrix — a
// mark on cell c lands count(c) tokens, where count(c) is the number of
// win lines through c (center 4, corners 3, edges 2). The incidence
// prior enters the stoichiometry itself instead of being recovered
// numerically by the solver. Built on the one-shot (consuming detector)
// or minimal (looped detector) base.
func (m *model) toPetriIncidence(oneShot bool) evalNet {
	count := map[string]int{}
	for _, line := range winLines {
		for _, c := range line {
			count[c]++
		}
	}
	base := m.toPetriMinimal()
	if oneShot {
		base = m.toPetriOneShot()
	}
	// raise each play deposit from 1 to count(c) by topping up
	for _, c := range cells {
		for _, side := range []string{"x", "o"} {
			if count[c] > 1 {
				base.net.AddArc(side+"_play_"+c, side+c, count[c]-1, false)
			}
		}
	}
	return base
}

// drawVariant is one answer to "which draw structure does the evaluation
// net carry?" — build the base net, and name O's undecided coordinate.
type drawVariant struct {
	name   string
	build  func(m *model) evalNet
	oscore func(f map[string]float64, lam float64) float64
}

var drawVariants = []drawVariant{
	{"dhazard", func(m *model) evalNet { return evalNet{m.toPetriDHazard(), m.cloneRates()} },
		func(f map[string]float64, lam float64) float64 { return f["tie"] + lam*f["win_o"] }},
	{"nodraw", func(m *model) evalNet { return m.toPetriNoDraw() },
		func(f map[string]float64, lam float64) float64 { return f["game_active"] + lam*f["win_o"] }},
	{"counter", func(m *model) evalNet { return evalNet{m.toPetriDraw(nil), m.cloneRates()} },
		func(f map[string]float64, lam float64) float64 { return f["game_active"] + lam*f["win_o"] }},
	{"minimal", func(m *model) evalNet { return m.toPetriMinimal() },
		func(f map[string]float64, lam float64) float64 { return f["game_active"] + lam*f["win_o"] }},
	{"oneshot", func(m *model) evalNet { return m.toPetriOneShot() },
		func(f map[string]float64, lam float64) float64 { return f["game_active"] + lam*f["win_o"] }},
	{"inc-oneshot", func(m *model) evalNet { return m.toPetriIncidence(true) },
		func(f map[string]float64, lam float64) float64 { return f["game_active"] + lam*f["win_o"] }},
	{"inc-minimal", func(m *model) evalNet { return m.toPetriIncidence(false) },
		func(f map[string]float64, lam float64) float64 { return f["game_active"] + lam*f["win_o"] }},
}

// odePlayScored: X maximizes win_x - win_o, O maximizes oscore(final, lam).
func odePlayScored(ev evalNet, oscore func(map[string]float64, float64) float64, lam float64) player {
	return func(m *model, mk marking, moves []string, maximizes bool, _ *rand.Rand) string {
		best, bestScore := "", 0.0
		for i, mv := range moves {
			f := m.odeFinal(ev.net, m.fire(mv, mk), ev.rates)
			score := f["win_x"] - f["win_o"]
			if !maximizes {
				score = oscore(f, lam)
			}
			if i == 0 || score > bestScore {
				best, bestScore = mv, score
			}
		}
		return best
	}
}

// toPetriBlockBias is the one-knob form: block bias only.
func (m *model) toPetriBlockBias(bias float64) evalNet {
	net := m.toPetriDHazard()
	rates := m.cloneRates()
	names := sortedLineNames()

	for _, name := range names {
		line := winLines[name]
		for i, c := range line {
			for _, side := range []string{"x", "o"} {
				opp := "o"
				if side == "o" {
					opp = "x"
				}
				t := "blk_" + side + "_" + name + "_" + strconv.Itoa(i)
				net.AddTransition(t, "", 0, 0, nil)
				rates[t] = bias
				// same firing effect as <side>_play_<c> ...
				net.AddArc("p"+c, t, 1, false)
				net.AddArc(side+"_turn", t, 1, false)
				net.AddArc(t, side+c, 1, false)
				net.AddArc(t, opp+"_turn", 1, false)
				net.AddArc(t, "move_tokens", 1, false)
				// ... catalyzed by the opponent's two marks on the line
				for j, oc := range line {
					if j != i {
						net.AddArc(opp+oc, t, 1, false)
						net.AddArc(t, opp+oc, 1, false)
					}
				}
			}
		}
	}
	return evalNet{net, rates}
}

// odePlayVariant scores each candidate on the variant net. X maximizes
// win_x - win_o; O maximizes tie + win_o - alpha*penalty (penalty "" means
// none — the plain d-hazard objective).
func odePlayVariant(ev evalNet, alpha float64, penalty string) player {
	return func(m *model, mk marking, moves []string, maximizes bool, _ *rand.Rand) string {
		best, bestScore := "", 0.0
		for i, mv := range moves {
			f := m.odeFinal(ev.net, m.fire(mv, mk), ev.rates)
			var score float64
			if maximizes {
				score = f["win_x"] - f["win_o"]
			} else {
				score = f["tie"] + f["win_o"]
				if penalty != "" {
					score -= alpha * f[penalty]
				}
			}
			if i == 0 || score > bestScore {
				best, bestScore = mv, score
			}
		}
		return best
	}
}

// odePlayPolicy is THE evaluator the experiment resolves on: play-scoring
// over the policy-biased d-hazard net, X maximizing win_x - win_o and O
// maximizing tie + lam*win_o. At (winBias 2, blockBias 6, lam 1.8) the
// exhaustive referee reports zero value-losing moves and zero missed wins
// on both seats.
func odePlayPolicy(ev evalNet, lam float64) player {
	return func(m *model, mk marking, moves []string, maximizes bool, _ *rand.Rand) string {
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
}

// The three players.
//
// minimaxPlayer: exact alpha-beta over the net's discrete semantics.
// Leaves score win +1, loss -1, draw 0 — the three outcomes must rank for
// "perfect play" to mean anything (the declared objective folds draw into
// the defender's win, making X indifferent between drawing and losing).
//
// ssaAblation: next-move elimination on exact discrete rollouts. Each
// player maximizes its own win place — X scores win_x, O scores win_o (a
// called draw pays win_o, so O counts draws as its own). Baseline =
// expected win-place tokens over N uniform-random playouts from the
// current position; per candidate, N more playouts with that transition's
// rate zeroed (the move is never chosen, by anyone, for the rest of the
// rollout). Common random numbers: every candidate reuses the baseline's
// seeds, so a difference measures the move, not the dice. Play the move
// whose removal costs the mover most.
//
// odeAblation: the same ablation on the mass-action relaxation — one
// go-pflow solve with all rates, one per candidate with the rate zeroed.
package main

import (
	"math/rand"
	"sort"

	"github.com/pflow-xyz/go-pflow/petri"
	"github.com/pflow-xyz/go-pflow/solver"
)

type player func(m *model, mk marking, moves []string, maximizes bool, rng *rand.Rand) string

// odeHorizon is the ODE solve span; the sweep varies it.
var odeHorizon = 3.0

// ---- exact minimax ----

func (m *model) minimax(mk marking, alpha, beta int) int {
	mk = m.fireHouse(mk)
	prefix, moves, maximizes, ok := "", []string(nil), false, false
	prefix, moves, maximizes, ok = m.legalMoves(mk)
	_ = prefix
	if !ok {
		// win +1, loss -1, draw 0. The folded net pays a called draw into
		// win_o, so the referee reads the board rather than the objective:
		// win_o without a completed O line is a draw.
		if mk["win_x"] > 0 {
			return 1
		}
		if mk["win_o"] > 0 && m.hasLine(mk, "o") {
			return -1
		}
		return 0
	}
	if maximizes {
		v := -2
		for _, mv := range moves {
			if w := m.minimax(m.fire(mv, mk), alpha, beta); w > v {
				v = w
			}
			if v > alpha {
				alpha = v
			}
			if alpha >= beta {
				break
			}
		}
		return v
	}
	v := 2
	for _, mv := range moves {
		if w := m.minimax(m.fire(mv, mk), alpha, beta); w < v {
			v = w
		}
		if v < beta {
			beta = v
		}
		if alpha >= beta {
			break
		}
	}
	return v
}

// optimalSet returns every move achieving the minimax value.
func (m *model) optimalSet(mk marking, moves []string, maximizes bool) []string {
	var best []string
	bestV := 0
	for i, mv := range moves {
		v := m.minimax(m.fire(mv, mk), -2, 2)
		if !maximizes {
			v = -v
		}
		if i == 0 || v > bestV {
			best, bestV = []string{mv}, v
		} else if v == bestV {
			best = append(best, mv)
		}
	}
	return best
}

func minimaxPlayer(m *model, mk marking, moves []string, maximizes bool, rng *rand.Rand) string {
	best := m.optimalSet(mk, moves, maximizes)
	return best[rng.Intn(len(best))]
}

// ---- SSA ablation ----

// rollout plays uniform random legal moves (skipping any zeroed transition)
// with the house refereeing, and returns the tokens in the mover's own win
// place.
func (m *model) rollout(mk marking, zeroed, obj string, rng *rand.Rand) int {
	for {
		mk = m.fireHouse(mk)
		_, moves, _, ok := m.legalMoves(mk)
		if !ok {
			return mk[obj]
		}
		if zeroed != "" {
			kept := moves[:0]
			for _, mv := range moves {
				if mv != zeroed {
					kept = append(kept, mv)
				}
			}
			moves = kept
			if len(moves) == 0 {
				// Only the ablated move remains: the game cannot continue.
				// Score what is on the board.
				return mk[obj]
			}
		}
		mk = m.fire(moves[rng.Intn(len(moves))], mk)
	}
}

func (m *model) expectedObjective(mk marking, zeroed, obj string, n int, seed int64) float64 {
	total := 0
	for i := 0; i < n; i++ {
		// Common random numbers: seed per realization, shared across candidates.
		rng := rand.New(rand.NewSource(seed + int64(i)))
		total += m.rollout(mk, zeroed, obj, rng)
	}
	return float64(total) / float64(n)
}

// winPlace is the mover's own objective: win_x for X, win_o for O.
func winPlace(maximizes bool) string {
	if maximizes {
		return "win_x"
	}
	return "win_o"
}

// objMode selects what the ODE ablation reads as value.
type objMode int

const (
	objOwn  objMode = iota // the mover's own win place only
	objDiff                // win_x - win_o, signed for the mover: denial counts
)

func odeValue(final map[string]float64, mode objMode, maximizes bool) float64 {
	if mode == objOwn {
		return final[winPlace(maximizes)]
	}
	v := final["win_x"] - final["win_o"]
	if !maximizes {
		v = -v
	}
	return v
}

func ssaAblation(realizations int) player {
	return func(m *model, mk marking, moves []string, maximizes bool, rng *rand.Rand) string {
		obj := winPlace(maximizes)
		seed := rng.Int63()
		baseline := m.expectedObjective(mk, "", obj, realizations, seed)
		best, bestLoss := "", 0.0
		for i, mv := range moves {
			score := m.expectedObjective(mk, mv, obj, realizations, seed)
			loss := baseline - score
			if i == 0 || loss > bestLoss {
				best, bestLoss = mv, loss
			}
		}
		return best
	}
}

// ---- ODE ablation ----

func (m *model) toPetri() *petri.PetriNet {
	return m.toPetriDraw(nil)
}

// toPetriDraw builds the solver net, optionally redirecting the draw
// transition's output to the given places instead of the model's own
// (win_o). A target place the model does not declare (e.g. "tie") is
// added empty. The discrete game is untouched — this varies only what
// the ODE evaluator believes a draw is worth.
func (m *model) toPetriDraw(drawTargets []string) *petri.PetriNet {
	net := petri.NewPetriNet()
	for _, p := range m.places {
		net.AddPlace(p, m.initial[p], nil, 0, 0, nil)
	}
	for _, p := range drawTargets {
		if _, exists := m.initial[p]; !exists {
			net.AddPlace(p, 0, nil, 0, 0, nil)
		}
	}
	for _, t := range m.transitions {
		net.AddTransition(t, "", 0, 0, nil)
	}
	for _, t := range m.transitions {
		for _, a := range m.inputs[t] {
			net.AddArc(a.from, t, a.weight, false)
		}
		if t == "draw" && drawTargets != nil {
			for _, p := range drawTargets {
				net.AddArc(t, p, 1, false)
			}
			continue
		}
		for _, a := range m.outputs[t] {
			net.AddArc(t, a.to, a.weight, false)
		}
	}
	return net
}

// odeFinal solves the net from mk and returns the final continuous state.
func (m *model) odeFinal(net *petri.PetriNet, mk marking, rates map[string]float64) map[string]float64 {
	state := make(map[string]float64, len(mk)+1)
	for k, v := range mk {
		state[k] = float64(v)
	}
	// Places the net declares beyond the model's marking (e.g. "tie")
	// must still enter the solve, or their mass is silently dropped.
	for p := range net.Places {
		if _, ok := state[p]; !ok {
			state[p] = 0
		}
	}
	prob := solver.NewProblem(net, state, [2]float64{0, odeHorizon}, rates)
	opts := &solver.Options{
		Dt: 0.2, Dtmin: 1e-4, Dtmax: 1.0,
		Abstol: 1e-4, Reltol: 1e-3, Maxiters: 1000, Adaptive: true,
	}
	return solver.Solve(prob, solver.Tsit5(), opts).GetFinalState()
}

func (m *model) odeObjective(net *petri.PetriNet, mk marking, rates map[string]float64, mode objMode, maximizes bool) float64 {
	state := make(map[string]float64, len(mk))
	for k, v := range mk {
		state[k] = float64(v)
	}
	prob := solver.NewProblem(net, state, [2]float64{0, odeHorizon}, rates)
	opts := &solver.Options{
		Dt: 0.2, Dtmin: 1e-4, Dtmax: 1.0,
		Abstol: 1e-4, Reltol: 1e-3, Maxiters: 1000, Adaptive: true,
	}
	sol := solver.Solve(prob, solver.Tsit5(), opts)
	final := sol.GetFinalState()
	return odeValue(final, mode, maximizes)
}

// moveLoss is one candidate's ablation result: baseline minus the payout
// with the move's rate zeroed.
type moveLoss struct {
	move string
	loss float64
}

// odeLosses runs the ablation once and returns every candidate's loss,
// best first — the whole measurement, not just the argmax.
func (m *model) odeLosses(net *petri.PetriNet, mk marking, moves []string, maximizes bool, mode objMode) []moveLoss {
	baseline := m.odeObjective(net, mk, m.rates, mode, maximizes)
	out := make([]moveLoss, 0, len(moves))
	for _, mv := range moves {
		zeroed := make(map[string]float64, len(m.rates))
		for k, v := range m.rates {
			zeroed[k] = v
		}
		zeroed[mv] = 0
		out = append(out, moveLoss{mv, baseline - m.odeObjective(net, mk, zeroed, mode, maximizes)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].loss > out[j].loss })
	return out
}

func odeAblation(net *petri.PetriNet, mode objMode) player {
	return func(m *model, mk marking, moves []string, maximizes bool, _ *rand.Rand) string {
		return m.odeLosses(net, mk, moves, maximizes, mode)[0].move
	}
}

// ---- ODE play (the blog's example-4 scorer) ----

// odePlayScores fires each candidate hypothetically and solves forward from
// the RESULTING marking; the score is the mover's final objective. Unlike
// elimination, the move actually happens, so a block removes the threat
// from every future the solve explores. Best first.
func (m *model) odePlayScores(net *petri.PetriNet, mk marking, moves []string, maximizes bool, mode objMode) []moveLoss {
	out := make([]moveLoss, 0, len(moves))
	for _, mv := range moves {
		out = append(out, moveLoss{mv, m.odeObjective(net, m.fire(mv, mk), m.rates, mode, maximizes)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].loss > out[j].loss })
	return out
}

func odePlay(net *petri.PetriNet, mode objMode) player {
	return func(m *model, mk marking, moves []string, maximizes bool, _ *rand.Rand) string {
		return m.odePlayScores(net, mk, moves, maximizes, mode)[0].move
	}
}

// odePlayPerSide scores each mover on its own evaluation net — the draw
// wiring can differ per perspective while the discrete game stays one net.
// toPetriDHazard builds an evaluation net where the draw is a HAZARD:
// game_active -> tie at the draw's rate, no move counter. The weight-9
// move_tokens arc is unreachable by continuous flow (the binomial rate
// C(move_tokens, 9) is zero below 9 tokens, and detector leakage starves
// the counter), so the discrete draw semantics cannot be expressed in the
// relaxation; a hazard can. Evaluation only — the discrete game keeps the
// exact counter.
func (m *model) toPetriDHazard() *petri.PetriNet {
	net := petri.NewPetriNet()
	for _, p := range m.places {
		net.AddPlace(p, m.initial[p], nil, 0, 0, nil)
	}
	net.AddPlace("tie", 0, nil, 0, 0, nil)
	for _, t := range m.transitions {
		net.AddTransition(t, "", 0, 0, nil)
	}
	for _, t := range m.transitions {
		if t == "draw" {
			net.AddArc("game_active", t, 1, false)
			net.AddArc(t, "tie", 1, false)
			continue
		}
		for _, a := range m.inputs[t] {
			net.AddArc(a.from, t, a.weight, false)
		}
		for _, a := range m.outputs[t] {
			net.AddArc(t, a.to, a.weight, false)
		}
	}
	return net
}

// odePlayDHazard scores on the hazard net: X maximizes win_x - win_o;
// O maximizes tie + win_o — its true objective, "X does not win",
// finally measurable because the hazard makes the tie channel live.
func odePlayDHazard(net *petri.PetriNet) player {
	return func(m *model, mk marking, moves []string, maximizes bool, _ *rand.Rand) string {
		best, bestScore := "", 0.0
		for i, mv := range moves {
			f := m.odeFinal(net, m.fire(mv, mk), m.rates)
			score := f["win_x"] - f["win_o"]
			if !maximizes {
				score = f["tie"] + f["win_o"]
			}
			if i == 0 || score > bestScore {
				best, bestScore = mv, score
			}
		}
		return best
	}
}

func odePlayPerSide(netX, netO *petri.PetriNet, mode objMode) player {
	return func(m *model, mk marking, moves []string, maximizes bool, _ *rand.Rand) string {
		net := netO
		if maximizes {
			net = netX
		}
		return m.odePlayScores(net, mk, moves, maximizes, mode)[0].move
	}
}

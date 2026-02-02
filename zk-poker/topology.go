package zkpoker

// ArcDef represents input and output arcs for a transition.
// All arc weights are 1 in this model.
type ArcDef struct {
	Inputs  []int // places consumed (arc weight = 1)
	Outputs []int // places produced (arc weight = 1)
}

// Topology defines the Petri net arcs for Texas Hold'em.
// This encodes the game rules as a state machine.
var Topology [NumTransitions]ArcDef

func init() {
	buildDealTransitions()
	buildActionTransitions()
	buildSkipTransitions()
	buildPhaseTransitions()
}

// buildDealTransitions creates transitions for dealing cards.
// Each deal transition: consumes card from deck, produces card in dealt pile.
func buildDealTransitions() {
	for i := 0; i < 52; i++ {
		Topology[DealTransitionStart+i] = ArcDef{
			Inputs:  []int{DeckPlace(i)},
			Outputs: []int{DealtPlace(i)},
		}
	}
}

// buildActionTransitions creates transitions for player betting actions.
// Each player has: fold, check, call, raise
//
// Fold: consumes turn + active, produces next_turn (active token is lost)
// Check: consumes turn + active, produces next_turn + active
// Call: consumes turn + active, produces next_turn + active
// Raise: consumes turn + active, produces next_turn + active
func buildActionTransitions() {
	for p := 0; p < NumPlayers; p++ {
		nextPlayer := (p + 1) % NumPlayers

		// For last player, betting round ends instead of going to next player
		var nextTurnOutput int
		if p == NumPlayers-1 {
			nextTurnOutput = PlaceBettingComplete
		} else {
			nextTurnOutput = TurnPlace(nextPlayer)
		}

		// Fold: lose active status
		Topology[ActionTransition(p, ActionFold)] = ArcDef{
			Inputs:  []int{TurnPlace(p), ActivePlace(p)},
			Outputs: []int{nextTurnOutput, FoldedPlace(p)},
		}

		// Check: keep active status
		Topology[ActionTransition(p, ActionCheck)] = ArcDef{
			Inputs:  []int{TurnPlace(p), ActivePlace(p)},
			Outputs: []int{nextTurnOutput, ActivePlace(p)},
		}

		// Call: keep active status
		Topology[ActionTransition(p, ActionCall)] = ArcDef{
			Inputs:  []int{TurnPlace(p), ActivePlace(p)},
			Outputs: []int{nextTurnOutput, ActivePlace(p)},
		}

		// Raise: keep active status
		Topology[ActionTransition(p, ActionRaise)] = ArcDef{
			Inputs:  []int{TurnPlace(p), ActivePlace(p)},
			Outputs: []int{nextTurnOutput, ActivePlace(p)},
		}
	}
}

// buildSkipTransitions creates transitions for skipping folded/allin players.
// Skip: consumes turn, produces next_turn (doesn't require active token)
func buildSkipTransitions() {
	for p := 0; p < NumPlayers; p++ {
		nextPlayer := (p + 1) % NumPlayers

		var nextTurnOutput int
		if p == NumPlayers-1 {
			nextTurnOutput = PlaceBettingComplete
		} else {
			nextTurnOutput = TurnPlace(nextPlayer)
		}

		Topology[SkipTransition(p)] = ArcDef{
			Inputs:  []int{TurnPlace(p)},
			Outputs: []int{nextTurnOutput},
		}
	}
}

// buildPhaseTransitions creates transitions for game phase changes.
func buildPhaseTransitions() {
	// StartHand: waiting -> preflop, give turn to player 0
	Topology[TransitionStartHand] = ArcDef{
		Inputs:  []int{PhaseWaiting},
		Outputs: []int{PhasePreflop, TurnPlace(0)},
	}

	// DealFlop: preflop + betting_complete -> flop, give turn to player 0
	Topology[TransitionDealFlop] = ArcDef{
		Inputs:  []int{PhasePreflop, PlaceBettingComplete},
		Outputs: []int{PhaseFlop, TurnPlace(0)},
	}

	// DealTurn: flop + betting_complete -> turn, give turn to player 0
	Topology[TransitionDealTurn] = ArcDef{
		Inputs:  []int{PhaseFlop, PlaceBettingComplete},
		Outputs: []int{PhaseTurn, TurnPlace(0)},
	}

	// DealRiver: turn + betting_complete -> river, give turn to player 0
	Topology[TransitionDealRiver] = ArcDef{
		Inputs:  []int{PhaseTurn, PlaceBettingComplete},
		Outputs: []int{PhaseRiver, TurnPlace(0)},
	}

	// ToShowdown: river + betting_complete -> showdown
	Topology[TransitionToShowdown] = ArcDef{
		Inputs:  []int{PhaseRiver, PlaceBettingComplete},
		Outputs: []int{PhaseShowdown},
	}

	// DetermineWinner: showdown -> hand_complete
	Topology[TransitionDetermineWinner] = ArcDef{
		Inputs:  []int{PhaseShowdown},
		Outputs: []int{PlaceHandComplete},
	}

	// EndHand: hand_complete -> waiting, reset all players to active
	Topology[TransitionEndHand] = ArcDef{
		Inputs: []int{PlaceHandComplete},
		Outputs: []int{
			PhaseWaiting,
			ActivePlace(0), ActivePlace(1), ActivePlace(2), ActivePlace(3), ActivePlace(4),
		},
	}
}

// IsEnabled checks if a transition can fire with the current marking.
func IsEnabled(m Marking, t int) bool {
	if t < 0 || t >= NumTransitions {
		return false
	}
	for _, p := range Topology[t].Inputs {
		if m[p] < 1 {
			return false
		}
	}
	return true
}

// Fire applies a transition and returns the new marking.
func Fire(m Marking, t int) (Marking, bool) {
	if !IsEnabled(m, t) {
		return m, false
	}

	newM := m
	// Consume from input places
	for _, p := range Topology[t].Inputs {
		newM[p]--
	}
	// Produce to output places
	for _, p := range Topology[t].Outputs {
		newM[p]++
	}
	return newM, true
}

// EnabledTransitions returns all transitions that can fire.
func EnabledTransitions(m Marking) []int {
	var enabled []int
	for t := 0; t < NumTransitions; t++ {
		if IsEnabled(m, t) {
			enabled = append(enabled, t)
		}
	}
	return enabled
}

// TransitionName returns a human-readable name for a transition.
func TransitionName(t int) string {
	switch {
	case t >= DealTransitionStart && t <= DealTransitionEnd:
		card := Card(t - DealTransitionStart)
		return "deal_" + card.String()
	case t >= ActionStart && t < SkipStart:
		player := (t - ActionStart) / 4
		action := (t - ActionStart) % 4
		actionNames := []string{"fold", "check", "call", "raise"}
		return "p" + string('0'+byte(player)) + "_" + actionNames[action]
	case t >= SkipStart && t <= SkipEnd:
		player := t - SkipStart
		return "p" + string('0'+byte(player)) + "_skip"
	case t == TransitionStartHand:
		return "start_hand"
	case t == TransitionDealFlop:
		return "deal_flop"
	case t == TransitionDealTurn:
		return "deal_turn"
	case t == TransitionDealRiver:
		return "deal_river"
	case t == TransitionToShowdown:
		return "to_showdown"
	case t == TransitionDetermineWinner:
		return "determine_winner"
	case t == TransitionEndHand:
		return "end_hand"
	default:
		return "unknown"
	}
}

// PlaceName returns a human-readable name for a place.
func PlaceName(p int) string {
	switch {
	case p >= DeckStart && p <= DeckEnd:
		card := Card(p - DeckStart)
		return "deck_" + card.String()
	case p >= DealtStart && p <= DealtEnd:
		card := Card(p - DealtStart)
		return "dealt_" + card.String()
	case p >= HoleStart && p <= HoleEnd:
		idx := p - HoleStart
		player := idx / 2
		slot := idx % 2
		return "p" + string('0'+byte(player)) + "_hole" + string('0'+byte(slot))
	case p >= CommunityStart && p <= CommunityEnd:
		slot := p - CommunityStart
		names := []string{"flop1", "flop2", "flop3", "turn", "river"}
		return "community_" + names[slot]
	case p >= ActiveStart && p <= ActiveEnd:
		player := p - ActiveStart
		return "p" + string('0'+byte(player)) + "_active"
	case p >= FoldedStart && p <= FoldedEnd:
		player := p - FoldedStart
		return "p" + string('0'+byte(player)) + "_folded"
	case p >= AllInStart && p <= AllInEnd:
		player := p - AllInStart
		return "p" + string('0'+byte(player)) + "_allin"
	case p >= TurnStart && p <= TurnEnd:
		player := p - TurnStart
		return "p" + string('0'+byte(player)) + "_turn"
	case p == PhaseWaiting:
		return "phase_waiting"
	case p == PhasePreflop:
		return "phase_preflop"
	case p == PhaseFlop:
		return "phase_flop"
	case p == PhaseTurn:
		return "phase_turn"
	case p == PhaseRiver:
		return "phase_river"
	case p == PhaseShowdown:
		return "phase_showdown"
	case p == PlaceBettingComplete:
		return "betting_complete"
	case p == PlaceHandComplete:
		return "hand_complete"
	default:
		return "unknown"
	}
}

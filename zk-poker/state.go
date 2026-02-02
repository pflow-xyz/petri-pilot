package zkpoker

import (
	"fmt"
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/mimc"
)

// NumPlayers is the number of players at the table.
const NumPlayers = 5

// NumPlaces is the total number of places in the poker Petri net.
// Breakdown:
//   - 52 deck cards (card in deck)
//   - 52 dealt cards (card has been dealt)
//   - 10 hole card slots (5 players × 2 cards each)
//   - 5 community card slots
//   - 5 player active states
//   - 5 player folded states
//   - 5 player allin states
//   - 5 player turn tokens
//   - 6 round phases (waiting, preflop, flop, turn, river, showdown)
//   - 1 betting_complete
//   - 1 hand_complete
const NumPlaces = 147

// NumTransitions is the total number of transitions.
// Breakdown:
//   - 52 deal transitions (one per card)
//   - 5×4 = 20 player actions (fold, check, call, raise per player)
//   - 5 skip transitions (for folded/allin players)
//   - 7 phase transitions (start_hand, deal_flop, deal_turn, deal_river, to_showdown, determine_winner, end_hand)
const NumTransitions = 84

// =============================================================================
// Place indices
// =============================================================================

// Deck places: card is in deck (0-51)
const (
	DeckStart = 0
	DeckEnd   = 51
)

// Dealt places: card has been dealt (52-103)
const (
	DealtStart = 52
	DealtEnd   = 103
)

// Hole card commitment places (104-113)
// Each player has 2 slots for their hole cards
const (
	HoleStart = 104 // P0 card 1
	HoleEnd   = 113 // P4 card 2
)

// Community card places (114-118)
const (
	CommunityStart = 114
	CommunityEnd   = 118
)

// Player state places
const (
	// Active: player is in the hand (119-123)
	ActiveStart = 119
	ActiveEnd   = 123

	// Folded: player has folded (124-128)
	FoldedStart = 124
	FoldedEnd   = 128

	// AllIn: player is all-in (129-133)
	AllInStart = 129
	AllInEnd   = 133

	// Turn: it's this player's turn (134-138)
	TurnStart = 134
	TurnEnd   = 138
)

// Phase places (139-144)
const (
	PhaseWaiting  = 139
	PhasePreflop  = 140
	PhaseFlop     = 141
	PhaseTurn     = 142
	PhaseRiver    = 143
	PhaseShowdown = 144
)

// Control places (145-146)
const (
	PlaceBettingComplete = 145
	PlaceHandComplete    = 146
)

// =============================================================================
// Transition indices
// =============================================================================

// Deal transitions (0-51): deal card i from deck
const (
	DealTransitionStart = 0
	DealTransitionEnd   = 51
)

// Player action transitions
// Each player has: fold, check, call, raise (4 actions)
// Player 0: 52-55, Player 1: 56-59, ..., Player 4: 68-71
const (
	ActionStart = 52

	// Action offsets within a player's block
	ActionFold  = 0
	ActionCheck = 1
	ActionCall  = 2
	ActionRaise = 3
)

// Skip transitions (72-76): skip player who is folded/allin
const (
	SkipStart = 72
	SkipEnd   = 76
)

// Phase transitions (77-83)
const (
	TransitionStartHand       = 77
	TransitionDealFlop        = 78
	TransitionDealTurn        = 79
	TransitionDealRiver       = 80
	TransitionToShowdown      = 81
	TransitionDetermineWinner = 82
	TransitionEndHand         = 83
)

// =============================================================================
// Helper functions
// =============================================================================

// DeckPlace returns the place index for card i in deck (0-51).
func DeckPlace(card int) int {
	return DeckStart + card
}

// DealtPlace returns the place index for card i being dealt (0-51).
func DealtPlace(card int) int {
	return DealtStart + card
}

// HolePlace returns the place index for player p's hole card slot s (0 or 1).
func HolePlace(player, slot int) int {
	return HoleStart + player*2 + slot
}

// CommunityPlace returns the place index for community card slot (0-4).
func CommunityPlace(slot int) int {
	return CommunityStart + slot
}

// ActivePlace returns the place index for player p's active state.
func ActivePlace(player int) int {
	return ActiveStart + player
}

// FoldedPlace returns the place index for player p's folded state.
func FoldedPlace(player int) int {
	return FoldedStart + player
}

// AllInPlace returns the place index for player p's all-in state.
func AllInPlace(player int) int {
	return AllInStart + player
}

// TurnPlace returns the place index for player p's turn token.
func TurnPlace(player int) int {
	return TurnStart + player
}

// ActionTransition returns the transition index for player p's action a.
// a: 0=fold, 1=check, 2=call, 3=raise
func ActionTransition(player, action int) int {
	return ActionStart + player*4 + action
}

// SkipTransition returns the transition index for skipping player p.
func SkipTransition(player int) int {
	return SkipStart + player
}

// =============================================================================
// Marking and state
// =============================================================================

// Marking represents the token counts for all places.
type Marking [NumPlaces]uint8

// InitialMarking returns the initial state for a new hand.
func InitialMarking() Marking {
	var m Marking

	// All cards start in deck
	for i := 0; i < 52; i++ {
		m[DeckPlace(i)] = 1
	}

	// All players start active
	for p := 0; p < NumPlayers; p++ {
		m[ActivePlace(p)] = 1
	}

	// Game starts in waiting phase
	m[PhaseWaiting] = 1

	return m
}

// ComputeMarkingRoot computes MiMC hash of the full marking.
func ComputeMarkingRoot(m Marking) *big.Int {
	h := mimc.NewMiMC()
	for _, tokens := range m {
		var elem fr.Element
		elem.SetUint64(uint64(tokens))
		b := elem.Bytes()
		h.Write(b[:])
	}
	sum := h.Sum(nil)
	return new(big.Int).SetBytes(sum)
}

// =============================================================================
// Card representation
// =============================================================================

// Card represents a playing card (0-51).
// Encoding: card = rank*4 + suit
// Ranks: 0=2, 1=3, ..., 8=T, 9=J, 10=Q, 11=K, 12=A
// Suits: 0=clubs, 1=diamonds, 2=hearts, 3=spades
type Card uint8

// Rank returns the rank of the card (0-12).
func (c Card) Rank() int {
	return int(c) / 4
}

// Suit returns the suit of the card (0-3).
func (c Card) Suit() int {
	return int(c) % 4
}

// RankName returns the rank as a string.
func (c Card) RankName() string {
	ranks := []string{"2", "3", "4", "5", "6", "7", "8", "9", "T", "J", "Q", "K", "A"}
	return ranks[c.Rank()]
}

// SuitName returns the suit as a string.
func (c Card) SuitName() string {
	suits := []string{"c", "d", "h", "s"}
	return suits[c.Suit()]
}

func (c Card) String() string {
	return c.RankName() + c.SuitName()
}

// NewCard creates a card from rank (0-12) and suit (0-3).
func NewCard(rank, suit int) Card {
	return Card(rank*4 + suit)
}

// ParseCard parses a card string like "Ah" or "2c".
func ParseCard(s string) (Card, error) {
	if len(s) != 2 {
		return 0, fmt.Errorf("invalid card: %s", s)
	}

	rankChar := s[0]
	suitChar := s[1]

	var rank int
	switch rankChar {
	case '2', '3', '4', '5', '6', '7', '8', '9':
		rank = int(rankChar - '2')
	case 'T', 't':
		rank = 8
	case 'J', 'j':
		rank = 9
	case 'Q', 'q':
		rank = 10
	case 'K', 'k':
		rank = 11
	case 'A', 'a':
		rank = 12
	default:
		return 0, fmt.Errorf("invalid rank: %c", rankChar)
	}

	var suit int
	switch suitChar {
	case 'c', 'C':
		suit = 0
	case 'd', 'D':
		suit = 1
	case 'h', 'H':
		suit = 2
	case 's', 'S':
		suit = 3
	default:
		return 0, fmt.Errorf("invalid suit: %c", suitChar)
	}

	return NewCard(rank, suit), nil
}

// =============================================================================
// Deck commitment (trusted dealer model)
// =============================================================================

// DeckCommitment represents a committed shuffled deck.
type DeckCommitment struct {
	// Permutation is the shuffled order of cards (indices 0-51)
	Permutation [52]Card

	// Salt is the random value used in the commitment
	Salt *big.Int

	// Root is the MiMC hash of the permutation and salt
	Root *big.Int
}

// ComputeDeckCommitment creates a commitment to a shuffled deck.
func ComputeDeckCommitment(permutation [52]Card, salt *big.Int) *DeckCommitment {
	h := mimc.NewMiMC()

	// Hash each card in order
	for _, card := range permutation {
		var elem fr.Element
		elem.SetUint64(uint64(card))
		b := elem.Bytes()
		h.Write(b[:])
	}

	// Hash the salt
	var saltElem fr.Element
	saltElem.SetBigInt(salt)
	saltBytes := saltElem.Bytes()
	h.Write(saltBytes[:])

	sum := h.Sum(nil)
	root := new(big.Int).SetBytes(sum)

	return &DeckCommitment{
		Permutation: permutation,
		Salt:        salt,
		Root:        root,
	}
}

// HoleCardCommitment represents a player's committed hole cards.
type HoleCardCommitment struct {
	Card1 Card
	Card2 Card
	Salt  *big.Int
	Root  *big.Int
}

// ComputeHoleCommitment creates a commitment to hole cards.
func ComputeHoleCommitment(card1, card2 Card, salt *big.Int) *HoleCardCommitment {
	h := mimc.NewMiMC()

	var c1Elem, c2Elem, saltElem fr.Element
	c1Elem.SetUint64(uint64(card1))
	c2Elem.SetUint64(uint64(card2))
	saltElem.SetBigInt(salt)

	c1Bytes := c1Elem.Bytes()
	c2Bytes := c2Elem.Bytes()
	saltBytes := saltElem.Bytes()
	h.Write(c1Bytes[:])
	h.Write(c2Bytes[:])
	h.Write(saltBytes[:])

	sum := h.Sum(nil)
	root := new(big.Int).SetBytes(sum)

	return &HoleCardCommitment{
		Card1: card1,
		Card2: card2,
		Salt:  salt,
		Root:  root,
	}
}

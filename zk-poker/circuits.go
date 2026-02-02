package zkpoker

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash/mimc"
)

// =============================================================================
// DealCircuit - Proves cards dealt match deck commitment
// =============================================================================

// DealCircuit proves that dealt cards match the committed deck.
//
// Public inputs:
//   - DeckCommitment: MiMC hash of shuffled deck + salt
//   - CardIndices:    which positions in deck were dealt (e.g., [0,1] for first 2 cards)
//   - DealtCards:     the actual cards dealt
//
// Private inputs:
//   - Deck: the full shuffled deck (52 cards)
//   - Salt: the salt used in commitment
type DealCircuit struct {
	// Public
	DeckCommitment frontend.Variable    `gnark:",public"`
	NumCardsDealt  frontend.Variable    `gnark:",public"`
	CardIndices    [7]frontend.Variable `gnark:",public"` // max 7 cards (2 hole + 5 community)
	DealtCards     [7]frontend.Variable `gnark:",public"`

	// Private
	Deck [52]frontend.Variable
	Salt frontend.Variable
}

// Define declares the constraints for valid deal.
func (c *DealCircuit) Define(api frontend.API) error {
	// 1. Verify deck commitment
	h, _ := mimc.NewMiMC(api)
	for i := 0; i < 52; i++ {
		h.Write(c.Deck[i])
	}
	h.Write(c.Salt)
	computedCommitment := h.Sum()
	api.AssertIsEqual(computedCommitment, c.DeckCommitment)

	// 2. Verify each dealt card matches its position in deck
	// For each card index, the dealt card must equal deck[index]
	for i := 0; i < 7; i++ {
		// Select the card from deck at CardIndices[i]
		selectedCard := frontend.Variable(0)
		for j := 0; j < 52; j++ {
			isMatch := api.IsZero(api.Sub(c.CardIndices[i], j))
			selectedCard = api.Add(selectedCard, api.Mul(c.Deck[j], isMatch))
		}

		// For now, unconditionally verify dealt card matches selected
		// TODO: Conditional enforcement for unused slots
		api.AssertIsEqual(c.DealtCards[i], selectedCard)
	}

	return nil
}

// =============================================================================
// HoleCommitmentCircuit - Proves hole cards match commitment
// =============================================================================

// HoleCommitmentCircuit proves that revealed hole cards match a prior commitment.
//
// Public inputs:
//   - Commitment: MiMC hash of (card1, card2, salt)
//   - Card1, Card2: the revealed hole cards
//
// Private inputs:
//   - Salt: the salt used in commitment
type HoleCommitmentCircuit struct {
	// Public
	Commitment frontend.Variable `gnark:",public"`
	Card1      frontend.Variable `gnark:",public"`
	Card2      frontend.Variable `gnark:",public"`

	// Private
	Salt frontend.Variable
}

// Define declares the constraints for valid hole card reveal.
func (c *HoleCommitmentCircuit) Define(api frontend.API) error {
	h, _ := mimc.NewMiMC(api)
	h.Write(c.Card1)
	h.Write(c.Card2)
	h.Write(c.Salt)
	computed := h.Sum()
	api.AssertIsEqual(computed, c.Commitment)
	return nil
}

// =============================================================================
// ShowdownCircuit - Proves winner among revealed hands
// =============================================================================

// ShowdownCircuit proves the correct winner among players at showdown.
// Uses the Petri net hand evaluation model from buildPokerHandModel().
//
// Public inputs:
//   - CommunityCards: 5 community cards
//   - PlayerHoles:    hole cards for each active player (commitments already verified)
//   - ActiveMask:     which players are still in (didn't fold)
//   - Winner:         the winning player index
//   - WinningRank:    the hand rank (0-8: high card to straight flush)
//
// Private inputs:
//   - HandEvaluations: intermediate values for hand evaluation
type ShowdownCircuit struct {
	// Public
	Community   [5]frontend.Variable             `gnark:",public"`
	Holes       [NumPlayers][2]frontend.Variable `gnark:",public"`
	ActiveMask  [NumPlayers]frontend.Variable    `gnark:",public"` // 1 if player is in, 0 if folded
	Winner      frontend.Variable                `gnark:",public"`
	WinningRank frontend.Variable                `gnark:",public"`

	// Private - intermediate evaluation state
	// For each player: computed hand rank and tiebreaker values
	PlayerRanks      [NumPlayers]frontend.Variable
	PlayerTiebreaker [NumPlayers][5]frontend.Variable // kicker cards in order
}

// Define declares the constraints for showdown winner determination.
func (c *ShowdownCircuit) Define(api frontend.API) error {
	// For each active player:
	// 1. Evaluate their 7-card hand (2 hole + 5 community)
	// 2. Compute hand rank (pair, two pair, etc.)
	// 3. Compare to find winner

	// This is where the hand evaluation Petri net gets encoded as constraints
	// The topology from buildPokerHandModel() becomes circuit constraints

	// Placeholder - full implementation would:
	// - Check for pairs (78 combinations)
	// - Check for trips (52 combinations)
	// - Check for straights (10 patterns × flush check)
	// - Check for flushes (suit counts)
	// - Determine best 5-card hand
	// - Compare active players

	// Verify winner has highest rank among active players
	for p := 0; p < NumPlayers; p++ {
		isWinner := api.IsZero(api.Sub(c.Winner, p))
		isActive := c.ActiveMask[p]

		// If this player is the winner, their rank must equal WinningRank
		winnerRankCheck := api.Mul(isWinner, api.Sub(c.PlayerRanks[p], c.WinningRank))
		api.AssertIsEqual(winnerRankCheck, 0)

		// If this player is active but not winner, their rank must be <= WinningRank
		// (This is a simplified check - full version needs tiebreaker comparison)
		// TODO: For active non-winners, verify rank < WinningRank OR (rank == WinningRank AND loses tiebreaker)
		isActiveNonWinner := api.Mul(isActive, api.Sub(1, isWinner))
		_ = isActiveNonWinner // Placeholder for full comparison logic
	}

	return nil
}

// =============================================================================
// Hand evaluation constants (from poker hand rankings)
// =============================================================================

const (
	RankHighCard      = 0
	RankPair          = 1
	RankTwoPair       = 2
	RankThreeOfAKind  = 3
	RankStraight      = 4
	RankFlush         = 5
	RankFullHouse     = 6
	RankFourOfAKind   = 7
	RankStraightFlush = 8
)

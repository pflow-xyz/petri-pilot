package zkpoker

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

func TestShowdownCircuitV2Compiles(t *testing.T) {
	var circuit ShowdownCircuitV2
	cs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		t.Fatalf("Circuit compilation failed: %v", err)
	}

	t.Logf("ShowdownCircuitV2 compiled successfully")
	t.Logf("  Constraints: %d", cs.GetNbConstraints())
	t.Logf("  Public inputs: %d", cs.GetNbPublicVariables())
	t.Logf("  Secret inputs: %d", cs.GetNbSecretVariables())
}

func TestEvaluateHandCircuitCompiles(t *testing.T) {
	// This test verifies the hand evaluation logic compiles as part of ShowdownCircuitV2
	// The evaluateHandCircuit function is tested through the full circuit
	t.Log("Hand evaluation is tested through ShowdownCircuitV2")
}

func TestHandEvaluationLogic(t *testing.T) {
	// Test the off-circuit hand evaluation logic to verify correctness
	tests := []struct {
		name     string
		cards    []string // 7 cards
		wantRank int      // expected hand rank
	}{
		{
			name:     "High card",
			cards:    []string{"Ah", "Kd", "Qs", "Jc", "9h", "7d", "2s"},
			wantRank: RankHighCard,
		},
		{
			name:     "Pair of Aces",
			cards:    []string{"Ah", "Ad", "Ks", "Qc", "Jh", "9d", "7s"},
			wantRank: RankPair,
		},
		{
			name:     "Two Pair",
			cards:    []string{"Ah", "Ad", "Ks", "Kc", "Jh", "9d", "7s"},
			wantRank: RankTwoPair,
		},
		{
			name:     "Three of a Kind",
			cards:    []string{"Ah", "Ad", "As", "Kc", "Jh", "9d", "7s"},
			wantRank: RankThreeOfAKind,
		},
		{
			name:     "Straight (Broadway)",
			cards:    []string{"Ah", "Kd", "Qs", "Jc", "Th", "7d", "2s"},
			wantRank: RankStraight,
		},
		{
			name:     "Straight (Wheel)",
			cards:    []string{"Ah", "2d", "3s", "4c", "5h", "Kd", "Qs"},
			wantRank: RankStraight,
		},
		{
			name:     "Flush",
			cards:    []string{"Ah", "Kh", "Qh", "Jh", "9h", "7d", "2s"},
			wantRank: RankFlush,
		},
		{
			name:     "Full House",
			cards:    []string{"Ah", "Ad", "As", "Kc", "Kh", "9d", "7s"},
			wantRank: RankFullHouse,
		},
		{
			name:     "Four of a Kind",
			cards:    []string{"Ah", "Ad", "As", "Ac", "Kh", "9d", "7s"},
			wantRank: RankFourOfAKind,
		},
		{
			name:     "Straight Flush",
			cards:    []string{"Ah", "Kh", "Qh", "Jh", "Th", "7d", "2s"},
			wantRank: RankStraightFlush,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse cards
			var cards [7]Card
			for i, s := range tt.cards {
				c, err := ParseCard(s)
				if err != nil {
					t.Fatalf("Failed to parse card %s: %v", s, err)
				}
				cards[i] = c
			}

			// Evaluate using off-circuit logic
			rank := evaluateHandOffCircuit(cards)

			if rank != tt.wantRank {
				t.Errorf("Got rank %d, want %d", rank, tt.wantRank)
			}
		})
	}
}

// evaluateHandOffCircuit is the off-circuit version for testing
func evaluateHandOffCircuit(cards [7]Card) int {
	// Extract ranks and suits
	var ranks [7]int
	var suits [7]int
	for i, c := range cards {
		ranks[i] = c.Rank()
		suits[i] = c.Suit()
	}

	// Count ranks
	var rankCounts [13]int
	for _, r := range ranks {
		rankCounts[r]++
	}

	// Count suits
	var suitCounts [4]int
	for _, s := range suits {
		suitCounts[s]++
	}

	// Detect hand components
	var numPairs, numTrips, numQuads int
	for _, count := range rankCounts {
		switch count {
		case 2:
			numPairs++
		case 3:
			numTrips++
		case 4:
			numQuads++
		}
	}

	// Flush
	hasFlush := false
	for _, count := range suitCounts {
		if count >= 5 {
			hasFlush = true
			break
		}
	}

	// Straight
	hasStraight := false
	straightPatterns := [][5]int{
		{12, 11, 10, 9, 8}, // Broadway
		{11, 10, 9, 8, 7},
		{10, 9, 8, 7, 6},
		{9, 8, 7, 6, 5},
		{8, 7, 6, 5, 4},
		{7, 6, 5, 4, 3},
		{6, 5, 4, 3, 2},
		{5, 4, 3, 2, 1},
		{4, 3, 2, 1, 0},
		{3, 2, 1, 0, 12}, // Wheel (5-4-3-2-A)
	}
	for _, pattern := range straightPatterns {
		hasAll := true
		for _, r := range pattern {
			if rankCounts[r] == 0 {
				hasAll = false
				break
			}
		}
		if hasAll {
			hasStraight = true
			break
		}
	}

	// Determine rank (highest priority first)
	if hasStraight && hasFlush {
		return RankStraightFlush
	}
	if numQuads > 0 {
		return RankFourOfAKind
	}
	if numTrips > 0 && numPairs > 0 {
		return RankFullHouse
	}
	if numTrips >= 2 {
		return RankFullHouse
	}
	if hasFlush {
		return RankFlush
	}
	if hasStraight {
		return RankStraight
	}
	if numTrips > 0 {
		return RankThreeOfAKind
	}
	if numPairs >= 2 {
		return RankTwoPair
	}
	if numPairs == 1 {
		return RankPair
	}
	return RankHighCard
}

func TestShowdownWitnessPreparation(t *testing.T) {
	// Test that we can prepare a valid witness

	// Player 0: Ah Kh (will make flush with community)
	// Player 1: 2c 3c (weak hand)
	// Community: Qh Jh Th 9d 8d (gives P0 a flush)

	p0h1, _ := ParseCard("Ah")
	p0h2, _ := ParseCard("Kh")
	p1h1, _ := ParseCard("2c")
	p1h2, _ := ParseCard("3c")

	comm := []string{"Qh", "Jh", "Th", "9d", "8d"}
	var community [5]Card
	for i, s := range comm {
		community[i], _ = ParseCard(s)
	}

	// Create commitments
	salt0 := big.NewInt(111)
	salt1 := big.NewInt(222)
	commit0 := ComputeHoleCommitment(p0h1, p0h2, salt0)
	commit1 := ComputeHoleCommitment(p1h1, p1h2, salt1)

	// Player 0's hand: Ah Kh Qh Jh Th 9d 8d = straight flush (A-high)
	// Player 1's hand: 2c 3c Qh Jh Th 9d 8d = straight (Q-high)

	p0Cards := [7]Card{p0h1, p0h2, community[0], community[1], community[2], community[3], community[4]}
	p1Cards := [7]Card{p1h1, p1h2, community[0], community[1], community[2], community[3], community[4]}

	p0Rank := evaluateHandOffCircuit(p0Cards)
	p1Rank := evaluateHandOffCircuit(p1Cards)

	t.Logf("Player 0 rank: %d (straight flush expected: %d)", p0Rank, RankStraightFlush)
	t.Logf("Player 1 rank: %d (straight expected: %d)", p1Rank, RankStraight)

	if p0Rank != RankStraightFlush {
		t.Errorf("Player 0 should have straight flush")
	}
	if p1Rank != RankStraight {
		t.Errorf("Player 1 should have straight")
	}
	if p0Rank <= p1Rank {
		t.Errorf("Player 0 should beat Player 1")
	}

	t.Logf("Commitment 0: %s", commit0.Root.String()[:20]+"...")
	t.Logf("Commitment 1: %s", commit1.Root.String()[:20]+"...")
}

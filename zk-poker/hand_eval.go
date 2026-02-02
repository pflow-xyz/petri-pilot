package zkpoker

// evaluateHandOffCircuit evaluates a 7-card poker hand off-circuit.
// Returns the hand rank (0-8).
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

// EvaluateHand is the exported version for external use.
func EvaluateHand(cards []Card) (rank int, rankName string) {
	if len(cards) < 5 || len(cards) > 7 {
		return -1, "invalid"
	}

	var hand [7]Card
	for i := 0; i < len(cards) && i < 7; i++ {
		hand[i] = cards[i]
	}

	rank = evaluateHandOffCircuit(hand)
	rankNames := []string{
		"High Card", "Pair", "Two Pair", "Three of a Kind",
		"Straight", "Flush", "Full House", "Four of a Kind", "Straight Flush",
	}
	return rank, rankNames[rank]
}

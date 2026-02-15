package serve

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

// HandRank represents standard poker hand rankings
type HandRank int

const (
	HighCard HandRank = iota
	Pair
	TwoPair
	ThreeOfAKind
	Straight
	Flush
	FullHouse
	FourOfAKind
	StraightFlush
)

func (r HandRank) String() string {
	names := []string{
		"High Card", "Pair", "Two Pair", "Three of a Kind",
		"Straight", "Flush", "Full House", "Four of a Kind", "Straight Flush",
	}
	if int(r) < len(names) {
		return names[r]
	}
	return "Unknown"
}

// PetriNetStrength returns the strength value used by the Petri net model
func (r HandRank) PetriNetStrength() int {
	// Mapping from buildPokerHandModel:
	// pair=2, two_pair=3, trips=4, straight=5, flush=6, full_house=7, quads=8, straight_flush=9
	strengths := []int{0, 2, 3, 4, 5, 6, 7, 8, 9}
	if int(r) < len(strengths) {
		return strengths[r]
	}
	return 0
}

// Card represents a playing card
type Card struct {
	Rank string // A, K, Q, J, T, 9, 8, 7, 6, 5, 4, 3, 2
	Suit string // h, d, c, s
}

// parseCard parses a card string like "Ah" or "Ks" into a Card
func parseCard(s string) Card {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return Card{}
	}
	return Card{
		Rank: string(s[0]),
		Suit: string(s[len(s)-1]),
	}
}

// parseHand parses a comma-separated list of cards
func parseHand(s string) []Card {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	cards := make([]Card, 0, len(parts))
	for _, p := range parts {
		if c := parseCard(p); c.Rank != "" {
			cards = append(cards, c)
		}
	}
	return cards
}

// isCardPlace checks if a place ID represents a card in hand (e.g., "A♥", "K♦")
// Card places have format: Rank + Suit symbol (2 runes total)
// Deck places have format: "deck_" + Rank + Suit symbol
func isCardPlace(id string) bool {
	if strings.HasPrefix(id, "deck_") {
		return false
	}
	// Card places are exactly 2 runes: rank + suit symbol
	runes := []rune(id)
	if len(runes) != 2 {
		return false
	}
	// First rune should be a rank
	ranks := "AKQJT98765432"
	if !strings.ContainsRune(ranks, runes[0]) {
		return false
	}
	// Second rune should be a suit symbol
	suitSymbols := "♥♦♣♠"
	return strings.ContainsRune(suitSymbols, runes[1])
}

// evaluateStandardHand evaluates a poker hand using standard rules
// Returns the HandRank for the best 5-card hand from the given cards
func evaluateStandardHand(cards []Card) HandRank {
	if len(cards) < 5 {
		// Need at least 5 cards to make a hand
		// Check for partial hands
		rankCounts := make(map[string]int)
		for _, c := range cards {
			rankCounts[c.Rank]++
		}

		maxCount := 0
		pairs := 0
		for _, count := range rankCounts {
			if count > maxCount {
				maxCount = count
			}
			if count >= 2 {
				pairs++
			}
		}

		if maxCount >= 4 {
			return FourOfAKind
		}
		if maxCount >= 3 && pairs >= 2 {
			return FullHouse
		}
		if maxCount >= 3 {
			return ThreeOfAKind
		}
		if pairs >= 2 {
			return TwoPair
		}
		if pairs >= 1 {
			return Pair
		}
		return HighCard
	}

	// Count ranks and suits
	rankCounts := make(map[string]int)
	suitCounts := make(map[string]int)
	for _, c := range cards {
		rankCounts[c.Rank]++
		suitCounts[c.Suit]++
	}

	// Check for flush (5+ cards of same suit)
	hasFlush := false
	flushSuit := ""
	for suit, count := range suitCounts {
		if count >= 5 {
			hasFlush = true
			flushSuit = suit
			break
		}
	}

	// Check for straight
	hasStraight := false
	straightPatterns := [][]string{
		{"A", "K", "Q", "J", "T"},
		{"K", "Q", "J", "T", "9"},
		{"Q", "J", "T", "9", "8"},
		{"J", "T", "9", "8", "7"},
		{"T", "9", "8", "7", "6"},
		{"9", "8", "7", "6", "5"},
		{"8", "7", "6", "5", "4"},
		{"7", "6", "5", "4", "3"},
		{"6", "5", "4", "3", "2"},
		{"5", "4", "3", "2", "A"}, // Wheel
	}

	for _, pattern := range straightPatterns {
		hasAll := true
		for _, rank := range pattern {
			if rankCounts[rank] == 0 {
				hasAll = false
				break
			}
		}
		if hasAll {
			hasStraight = true
			break
		}
	}

	// Check for straight flush
	if hasFlush {
		// Get cards in flush suit
		flushCards := make([]Card, 0)
		for _, c := range cards {
			if c.Suit == flushSuit {
				flushCards = append(flushCards, c)
			}
		}
		flushRanks := make(map[string]bool)
		for _, c := range flushCards {
			flushRanks[c.Rank] = true
		}

		for _, pattern := range straightPatterns {
			hasAll := true
			for _, rank := range pattern {
				if !flushRanks[rank] {
					hasAll = false
					break
				}
			}
			if hasAll {
				return StraightFlush
			}
		}
	}

	// Count pairs, trips, quads
	pairs := 0
	trips := 0
	quads := 0
	for _, count := range rankCounts {
		switch {
		case count == 4:
			quads++
		case count == 3:
			trips++
		case count == 2:
			pairs++
		}
	}

	// Determine hand rank (highest first)
	if quads > 0 {
		return FourOfAKind
	}
	if trips > 0 && pairs > 0 {
		return FullHouse
	}
	if trips >= 2 {
		// Two trips also makes a full house (use one trip as pair)
		return FullHouse
	}
	if hasFlush {
		return Flush
	}
	if hasStraight {
		return Straight
	}
	if trips > 0 {
		return ThreeOfAKind
	}
	if pairs >= 2 {
		return TwoPair
	}
	if pairs == 1 {
		return Pair
	}
	return HighCard
}

// TestPokerHandModelClassification tests that the poker-hand Petri net model
// correctly classifies hands according to standard poker rankings
func TestPokerHandModelClassification(t *testing.T) {
	testCases := []struct {
		name      string
		hole      string
		community string
		expected  HandRank
	}{
		// Basic hands
		{"High card", "Ah,Kd", "Qs,Jc,9h", HighCard},
		{"Pair of Aces", "Ah,Ad", "", Pair},
		{"Pair with board", "Ah,Kd", "As,Jc,9h", Pair},
		{"Two pair", "Ah,Kh", "Ad,Kd,9s", TwoPair},
		{"Three of a kind", "Ah,Ad", "As,Kd,Qc", ThreeOfAKind},
		{"Full house Aces over Kings", "Ah,Ad", "As,Kd,Kc", FullHouse},
		{"Four of a kind", "Ah,Ad", "As,Ac,Kd", FourOfAKind},

		// Straights
		{"Broadway straight", "Ah,Kd", "Qs,Jc,Th", Straight},
		{"Wheel straight", "Ah,2d", "3s,4c,5h", Straight},
		{"Middle straight", "9h,8d", "7s,6c,5h", Straight},

		// Flushes
		{"Ace-high flush", "Ah,Kh", "Qh,Jh,9h", Flush},
		{"Low flush", "7h,6h", "5h,4h,2h", Flush},

		// Straight flush
		{"Royal flush", "Ah,Kh", "Qh,Jh,Th", StraightFlush},
		{"Steel wheel", "Ah,2h", "3h,4h,5h", StraightFlush},
		{"Middle straight flush", "9h,8h", "7h,6h,5h", StraightFlush},

		// Edge cases
		{"Trips over pair = full house", "Ah,Ad", "As,Kd,Kc", FullHouse},
		{"Two trips = full house", "Ah,Ad", "As,Kd,Kc,Ks", FullHouse},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Evaluate using standard algorithm
			cards := append(parseHand(tc.hole), parseHand(tc.community)...)
			standardRank := evaluateStandardHand(cards)

			// Build Petri net model and check its classification
			model := buildPokerHandModel(tc.hole, tc.community)

			// Extract the description which contains the hand classification
			description := model["description"].(string)

			// Parse the model to verify places and initial state
			places := model["places"].([]map[string]any)

			// Count cards in hand based on initial tokens
			cardsInHand := 0
			for _, place := range places {
				id := place["id"].(string)
				initial := place["initial"].(int)
				// Card places are like "A♥" (2 runes: rank + suit symbol)
				if initial == 1 && isCardPlace(id) {
					cardsInHand++
				}
			}

			expectedCards := len(cards)
			if cardsInHand != expectedCards {
				t.Errorf("Expected %d cards in hand, got %d", expectedCards, cardsInHand)
			}

			// Verify standard algorithm result
			if standardRank != tc.expected {
				t.Errorf("Standard algorithm: expected %s, got %s", tc.expected, standardRank)
			}

			// Check that description contains expected hand type
			expectedInDesc := tc.expected.String()
			if !strings.Contains(description, expectedInDesc) {
				t.Errorf("Model description should contain '%s', got: %s", expectedInDesc, description)
			}

			t.Logf("%s: standard=%s, model_desc=%s", tc.name, standardRank, description)
		})
	}
}

// TestPokerHandModelStructure verifies the Petri net model structure
func TestPokerHandModelStructure(t *testing.T) {
	model := buildPokerHandModel("Ah,Ad", "As,Kd,Kc")

	places := model["places"].([]map[string]any)
	transitions := model["transitions"].([]map[string]any)
	arcs := model["arcs"].([]map[string]any)

	// Should have 52 card places + 52 deck places + hand type places + hand_strength + pair_count
	// Hand type places: pair, two_pair, three_kind, straight, flush, full_house, four_kind, straight_flush
	expectedHandPlaces := 8
	expectedCardPlaces := 52 * 2 // hand + deck
	minPlaces := expectedCardPlaces + expectedHandPlaces + 2 // +2 for hand_strength, pair_count

	if len(places) < minPlaces {
		t.Errorf("Expected at least %d places, got %d", minPlaces, len(places))
	}

	// Check for required hand type places
	requiredPlaces := []string{"pair", "two_pair", "three_kind", "straight", "flush", "full_house", "four_kind", "straight_flush", "hand_strength"}
	placeIDs := make(map[string]bool)
	for _, p := range places {
		placeIDs[p["id"].(string)] = true
	}
	for _, required := range requiredPlaces {
		if !placeIDs[required] {
			t.Errorf("Missing required place: %s", required)
		}
	}

	// Should have deal transitions for all 52 cards
	dealTransitions := 0
	for _, tr := range transitions {
		id := tr["id"].(string)
		if strings.HasPrefix(id, "deal_") {
			dealTransitions++
		}
	}
	if dealTransitions != 52 {
		t.Errorf("Expected 52 deal transitions, got %d", dealTransitions)
	}

	// Should have pair detection transitions (78 = 6 combos × 13 ranks)
	pairTransitions := 0
	for _, tr := range transitions {
		id := tr["id"].(string)
		if strings.HasPrefix(id, "pair_") {
			pairTransitions++
		}
	}
	if pairTransitions != 78 {
		t.Errorf("Expected 78 pair transitions, got %d", pairTransitions)
	}

	t.Logf("Model structure: %d places, %d transitions, %d arcs", len(places), len(transitions), len(arcs))
}

// TestPokerHandRankingOrder verifies that hand rankings are correctly ordered
func TestPokerHandRankingOrder(t *testing.T) {
	// Test that our standard evaluation correctly orders hands
	testHands := []struct {
		hole      string
		community string
		expected  HandRank
	}{
		// Order from worst to best
		{"Ah,Kd", "Qs,Jc,9h,7s,3c", HighCard},      // 0
		{"Ah,Ad", "Qs,Jc,9h", Pair},                 // 1
		{"Ah,Kh", "Ad,Kd,9s", TwoPair},              // 2
		{"Ah,Ad", "As,Kd,Qc", ThreeOfAKind},         // 3
		{"Ah,Kd", "Qs,Jc,Th", Straight},             // 4
		{"Ah,Kh", "Qh,Jh,9h", Flush},                // 5
		{"Ah,Ad", "As,Kd,Kc", FullHouse},            // 6
		{"Ah,Ad", "As,Ac,Kd", FourOfAKind},          // 7
		{"Ah,Kh", "Qh,Jh,Th", StraightFlush},        // 8
	}

	prevRank := HandRank(-1)
	for _, tc := range testHands {
		cards := append(parseHand(tc.hole), parseHand(tc.community)...)
		rank := evaluateStandardHand(cards)

		if rank != tc.expected {
			t.Errorf("Hand %s+%s: expected %s, got %s", tc.hole, tc.community, tc.expected, rank)
		}

		if rank <= prevRank {
			t.Errorf("Rankings not in ascending order: %s <= %s", rank, prevRank)
		}

		// Verify Petri net strength values are also ordered
		if tc.expected.PetriNetStrength() < int(prevRank)+1 && prevRank >= 0 {
			t.Errorf("Petri net strengths not ordered: %d for %s", tc.expected.PetriNetStrength(), tc.expected)
		}

		prevRank = rank
		t.Logf("%s: rank=%d (%s), petri_strength=%d", tc.hole+"+"+tc.community, rank, rank, rank.PetriNetStrength())
	}
}

// TestPokerHandModelTransitions tests specific transition firings
func TestPokerHandModelTransitions(t *testing.T) {
	// Test that the model correctly sets up transitions for pattern detection
	model := buildPokerHandModel("Ah,Ad", "")

	transitions := model["transitions"].([]map[string]any)
	arcs := model["arcs"].([]map[string]any)

	// Find pair_A_hd transition (pair of Aces, hearts and diamonds)
	var pairAhdFound bool
	for _, tr := range transitions {
		if tr["id"].(string) == "pair_A_hd" {
			pairAhdFound = true
			break
		}
	}
	if !pairAhdFound {
		t.Error("Expected pair_A_hd transition for pair of Aces (hearts, diamonds)")
	}

	// Verify arcs connect the pair transition correctly
	// Should have input arcs from A♥ and A♦ to pair_A_hd
	pairAhdInputs := make(map[string]bool)
	pairAhdOutput := ""
	for _, arc := range arcs {
		from := arc["from"].(string)
		to := arc["to"].(string)
		if to == "pair_A_hd" {
			pairAhdInputs[from] = true
		}
		if from == "pair_A_hd" {
			pairAhdOutput = to
		}
	}

	if !pairAhdInputs["A♥"] {
		t.Error("pair_A_hd should have input from A♥")
	}
	if !pairAhdInputs["A♦"] {
		t.Error("pair_A_hd should have input from A♦")
	}
	if pairAhdOutput != "pair" {
		t.Errorf("pair_A_hd should output to 'pair' place, got %s", pairAhdOutput)
	}

	t.Logf("Pair transition verified: inputs=%v, output=%s", pairAhdInputs, pairAhdOutput)
}

// TestAllPairCombinations verifies all 78 pair detection transitions exist
func TestAllPairCombinations(t *testing.T) {
	model := buildPokerHandModel("", "")
	transitions := model["transitions"].([]map[string]any)

	ranks := []string{"A", "K", "Q", "J", "T", "9", "8", "7", "6", "5", "4", "3", "2"}
	suitCombos := [][2]string{{"h", "d"}, {"h", "c"}, {"h", "s"}, {"d", "c"}, {"d", "s"}, {"c", "s"}}

	transitionIDs := make(map[string]bool)
	for _, tr := range transitions {
		transitionIDs[tr["id"].(string)] = true
	}

	missing := []string{}
	for _, rank := range ranks {
		for _, combo := range suitCombos {
			expected := "pair_" + rank + "_" + combo[0] + combo[1]
			if !transitionIDs[expected] {
				missing = append(missing, expected)
			}
		}
	}

	if len(missing) > 0 {
		t.Errorf("Missing pair transitions: %v", missing)
	}

	expectedCount := len(ranks) * len(suitCombos) // 13 * 6 = 78
	t.Logf("All %d pair combinations verified", expectedCount)
}

// TestTripsCombinations verifies trips detection transitions
func TestTripsCombinations(t *testing.T) {
	model := buildPokerHandModel("", "")
	transitions := model["transitions"].([]map[string]any)

	ranks := []string{"A", "K", "Q", "J", "T", "9", "8", "7", "6", "5", "4", "3", "2"}
	// C(4,3) = 4 ways to choose 3 suits from 4
	tripsCombos := [][3]string{{"h", "d", "c"}, {"h", "d", "s"}, {"h", "c", "s"}, {"d", "c", "s"}}

	transitionIDs := make(map[string]bool)
	for _, tr := range transitions {
		transitionIDs[tr["id"].(string)] = true
	}

	missing := []string{}
	for _, rank := range ranks {
		for _, combo := range tripsCombos {
			expected := "trips_" + rank + "_" + combo[0] + combo[1] + combo[2]
			if !transitionIDs[expected] {
				missing = append(missing, expected)
			}
		}
	}

	if len(missing) > 0 {
		t.Errorf("Missing trips transitions: %v", missing)
	}

	expectedCount := len(ranks) * len(tripsCombos) // 13 * 4 = 52
	t.Logf("All %d trips combinations verified", expectedCount)
}

// TestQuadsCombinations verifies quads detection transitions
func TestQuadsCombinations(t *testing.T) {
	model := buildPokerHandModel("", "")
	transitions := model["transitions"].([]map[string]any)

	ranks := []string{"A", "K", "Q", "J", "T", "9", "8", "7", "6", "5", "4", "3", "2"}

	transitionIDs := make(map[string]bool)
	for _, tr := range transitions {
		transitionIDs[tr["id"].(string)] = true
	}

	missing := []string{}
	for _, rank := range ranks {
		expected := "quads_" + rank
		if !transitionIDs[expected] {
			missing = append(missing, expected)
		}
	}

	if len(missing) > 0 {
		t.Errorf("Missing quads transitions: %v", missing)
	}

	t.Logf("All %d quads combinations verified", len(ranks))
}

// TestStraightFlushTransitions verifies straight flush detection
func TestStraightFlushTransitions(t *testing.T) {
	model := buildPokerHandModel("", "")
	transitions := model["transitions"].([]map[string]any)

	// 10 straight types × 4 suits = 40 straight flush transitions
	straightStarts := []string{"A", "K", "Q", "J", "T", "9", "8", "7", "6", "5"}
	suits := []string{"h", "d", "c", "s"}

	transitionIDs := make(map[string]bool)
	for _, tr := range transitions {
		transitionIDs[tr["id"].(string)] = true
	}

	missing := []string{}
	for _, start := range straightStarts {
		for _, suit := range suits {
			expected := "sf_" + start + "_" + suit
			if !transitionIDs[expected] {
				missing = append(missing, expected)
			}
		}
	}

	if len(missing) > 0 {
		t.Errorf("Missing straight flush transitions: %v", missing)
	}

	expectedCount := len(straightStarts) * len(suits) // 10 * 4 = 40
	t.Logf("All %d straight flush combinations verified", expectedCount)
}

// TestModelJSONSerialization verifies the model can be serialized to JSON
func TestModelJSONSerialization(t *testing.T) {
	model := buildPokerHandModel("Ah,Ad", "As,Kd,Kc")

	// Should serialize without error
	jsonBytes, err := json.Marshal(model)
	if err != nil {
		t.Fatalf("Failed to serialize model: %v", err)
	}

	// Should deserialize back
	var parsed map[string]any
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("Failed to deserialize model: %v", err)
	}

	// Verify key fields
	if parsed["name"] != "poker-hand" {
		t.Errorf("Expected name 'poker-hand', got %v", parsed["name"])
	}

	t.Logf("Model serialized to %d bytes JSON", len(jsonBytes))
}

// TestPartialHandRanking tests hands with fewer than 5 cards
func TestPartialHandRanking(t *testing.T) {
	testCases := []struct {
		name      string
		hole      string
		community string
		expected  HandRank
		desc      string
	}{
		// 2 cards (preflop)
		{"Pocket Aces", "Ah,Ad", "", Pair, "Best preflop hand"},
		{"Pocket Kings", "Kh,Kd", "", Pair, "Second best preflop"},
		{"Ace King suited", "Ah,Kh", "", HighCard, "No pair yet"},
		{"Ace King offsuit", "Ah,Kd", "", HighCard, "No pair"},
		{"Seven Two offsuit", "7h,2d", "", HighCard, "Worst starting hand"},

		// 3 cards (hole + 1 community - unusual but valid)
		{"Trips on first card", "Ah,Ad", "As", ThreeOfAKind, "Flopped set"},
		{"Pair with kicker", "Ah,Kd", "As", Pair, "Paired the ace"},

		// 4 cards (hole + 2 community)
		{"Two pair early", "Ah,Kh", "Ad,Kd", TwoPair, "Two pair formed"},
		{"Quads early", "Ah,Ad", "As,Ac", FourOfAKind, "Flopped quads"},
		{"Trips with kicker", "Ah,Ad", "As,Kd", ThreeOfAKind, "Trip aces"},

		// 5 cards (hole + flop) - minimum for straights/flushes
		{"Flopped flush", "Ah,Kh", "Qh,Jh,9h", Flush, "Flush on flop"},
		{"Flopped straight", "Ah,Kd", "Qs,Jc,Th", Straight, "Broadway on flop"},
		{"Flopped full house", "Ah,Ad", "As,Kd,Kc", FullHouse, "Boat on flop"},

		// 6 cards (hole + turn)
		{"Turn improves to flush", "Ah,Kh", "Qh,Jh,9s,Th", StraightFlush, "Royal on turn"},
		{"Turn makes full house", "Ah,Ad", "As,Kd,9c,Kc", FullHouse, "Boat on turn"},

		// 7 cards (hole + river) - full hand
		{"River completes straight", "Ah,2d", "3s,4c,5h,Kd,Qc", Straight, "Wheel on river"},
		{"Seven card flush", "Ah,Kh", "Qh,Jh,9h,8h,7h", Flush, "7-card flush (best 5)"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cards := append(parseHand(tc.hole), parseHand(tc.community)...)

			// Standard algorithm evaluation
			standardRank := evaluateStandardHand(cards)

			// Model description check
			model := buildPokerHandModel(tc.hole, tc.community)
			description := model["description"].(string)

			if standardRank != tc.expected {
				t.Errorf("%s: expected %s, got %s (cards: %d)",
					tc.desc, tc.expected, standardRank, len(cards))
			}

			// Verify model description matches
			expectedInDesc := tc.expected.String()
			if !strings.Contains(description, expectedInDesc) {
				t.Errorf("Model should classify as '%s', got: %s", expectedInDesc, description)
			}

			t.Logf("%s (%d cards): %s → %s", tc.name, len(cards), tc.hole+"+"+tc.community, standardRank)
		})
	}
}

// TestPreflopHandStrength tests preflop hand categorization
func TestPreflopHandStrength(t *testing.T) {
	// Test that pocket pairs are correctly identified
	pocketPairs := []string{"AA", "KK", "QQ", "JJ", "TT", "99", "88", "77", "66", "55", "44", "33", "22"}
	suits := []string{"h", "d"}

	for _, pair := range pocketPairs {
		rank := string(pair[0])
		hole := rank + suits[0] + "," + rank + suits[1]

		cards := parseHand(hole)
		result := evaluateStandardHand(cards)

		if result != Pair {
			t.Errorf("Pocket %s should be Pair, got %s", pair, result)
		}

		// Also verify model
		model := buildPokerHandModel(hole, "")
		desc := model["description"].(string)
		if !strings.Contains(desc, "Pair") {
			t.Errorf("Model should identify pocket %s as Pair: %s", pair, desc)
		}
	}

	t.Logf("All %d pocket pairs correctly identified", len(pocketPairs))
}

// Benchmark for model generation
func BenchmarkBuildPokerHandModel(b *testing.B) {
	for i := 0; i < b.N; i++ {
		buildPokerHandModel("Ah,Ad", "As,Kd,Kc")
	}
}

// TestHandComparisonMatrix tests a matrix of hand comparisons
func TestHandComparisonMatrix(t *testing.T) {
	hands := []struct {
		name      string
		hole      string
		community string
	}{
		{"Royal flush", "Ah,Kh", "Qh,Jh,Th"},
		{"Steel wheel SF", "Ah,2h", "3h,4h,5h"},
		{"Quad Aces", "Ah,Ad", "As,Ac,Kd"},
		{"Aces full of Kings", "Ah,Ad", "As,Kd,Kc"},
		{"Ace-high flush", "Ah,Kh", "Qh,Jh,9h"},
		{"Broadway straight", "Ah,Kd", "Qs,Jc,Th"},
		{"Trip Aces", "Ah,Ad", "As,Kd,Qc"},
		{"Aces and Kings", "Ah,Kh", "Ad,Kd,9s"},
		{"Pair of Aces", "Ah,Ad", "Ks,Qc,Jh"},
		{"High card Ace", "Ah,Kd", "Qs,Jc,9h"},
	}

	// Evaluate all hands
	results := make([]struct {
		name     string
		rank     HandRank
		strength int
	}, len(hands))

	for i, h := range hands {
		cards := append(parseHand(h.hole), parseHand(h.community)...)
		rank := evaluateStandardHand(cards)
		results[i] = struct {
			name     string
			rank     HandRank
			strength int
		}{h.name, rank, rank.PetriNetStrength()}
	}

	// Sort by strength (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].strength > results[j].strength
	})

	// Verify ordering
	t.Log("Hand rankings (best to worst):")
	for i, r := range results {
		t.Logf("  %d. %s: %s (strength=%d)", i+1, r.name, r.rank, r.strength)

		// Verify each hand beats all hands below it
		for j := i + 1; j < len(results); j++ {
			if r.strength < results[j].strength {
				t.Errorf("%s (strength=%d) should beat %s (strength=%d)",
					r.name, r.strength, results[j].name, results[j].strength)
			}
		}
	}
}

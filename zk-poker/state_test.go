package zkpoker

import (
	"math/big"
	"testing"
)

func TestCardEncoding(t *testing.T) {
	tests := []struct {
		input    string
		wantRank int
		wantSuit int
	}{
		{"2c", 0, 0},
		{"2d", 0, 1},
		{"2h", 0, 2},
		{"2s", 0, 3},
		{"Ac", 12, 0},
		{"Ad", 12, 1},
		{"Ah", 12, 2},
		{"As", 12, 3},
		{"Tc", 8, 0},
		{"Jh", 9, 2},
		{"Qs", 10, 3},
		{"Kd", 11, 1},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			card, err := ParseCard(tt.input)
			if err != nil {
				t.Fatalf("ParseCard(%q) error: %v", tt.input, err)
			}
			if card.Rank() != tt.wantRank {
				t.Errorf("Rank() = %d, want %d", card.Rank(), tt.wantRank)
			}
			if card.Suit() != tt.wantSuit {
				t.Errorf("Suit() = %d, want %d", card.Suit(), tt.wantSuit)
			}
			// Round-trip test
			if card.String() != tt.input {
				t.Errorf("String() = %q, want %q", card.String(), tt.input)
			}
		})
	}
}

func TestInitialMarking(t *testing.T) {
	m := InitialMarking()

	// All 52 cards should be in deck
	for i := 0; i < 52; i++ {
		if m[DeckPlace(i)] != 1 {
			t.Errorf("Deck card %d should have 1 token, got %d", i, m[DeckPlace(i)])
		}
	}

	// No cards should be dealt
	for i := 0; i < 52; i++ {
		if m[DealtPlace(i)] != 0 {
			t.Errorf("Dealt card %d should have 0 tokens, got %d", i, m[DealtPlace(i)])
		}
	}

	// All 5 players should be active
	for p := 0; p < NumPlayers; p++ {
		if m[ActivePlace(p)] != 1 {
			t.Errorf("Player %d should be active", p)
		}
	}

	// Game should be in waiting phase
	if m[PhaseWaiting] != 1 {
		t.Error("Game should start in waiting phase")
	}
}

func TestDeckCommitment(t *testing.T) {
	// Create a simple permutation (identity for testing)
	var perm [52]Card
	for i := 0; i < 52; i++ {
		perm[i] = Card(i)
	}

	salt := big.NewInt(12345)

	commitment := ComputeDeckCommitment(perm, salt)

	// Verify root is non-nil and non-zero
	if commitment.Root == nil {
		t.Fatal("Commitment root is nil")
	}
	if commitment.Root.Sign() == 0 {
		t.Error("Commitment root is zero")
	}

	// Same inputs should produce same commitment
	commitment2 := ComputeDeckCommitment(perm, salt)
	if commitment.Root.Cmp(commitment2.Root) != 0 {
		t.Error("Same inputs should produce same commitment")
	}

	// Different salt should produce different commitment
	commitment3 := ComputeDeckCommitment(perm, big.NewInt(99999))
	if commitment.Root.Cmp(commitment3.Root) == 0 {
		t.Error("Different salt should produce different commitment")
	}

	// Different permutation should produce different commitment
	perm2 := perm
	perm2[0], perm2[1] = perm2[1], perm2[0] // swap first two cards
	commitment4 := ComputeDeckCommitment(perm2, salt)
	if commitment.Root.Cmp(commitment4.Root) == 0 {
		t.Error("Different permutation should produce different commitment")
	}
}

func TestHoleCommitment(t *testing.T) {
	card1, _ := ParseCard("Ah")
	card2, _ := ParseCard("Kh")
	salt := big.NewInt(67890)

	commitment := ComputeHoleCommitment(card1, card2, salt)

	if commitment.Root == nil {
		t.Fatal("Commitment root is nil")
	}
	if commitment.Root.Sign() == 0 {
		t.Error("Commitment root is zero")
	}

	// Same inputs should produce same commitment
	commitment2 := ComputeHoleCommitment(card1, card2, salt)
	if commitment.Root.Cmp(commitment2.Root) != 0 {
		t.Error("Same inputs should produce same commitment")
	}

	// Different cards should produce different commitment
	card3, _ := ParseCard("2c")
	commitment3 := ComputeHoleCommitment(card3, card2, salt)
	if commitment.Root.Cmp(commitment3.Root) == 0 {
		t.Error("Different cards should produce different commitment")
	}

	// Order matters
	commitment4 := ComputeHoleCommitment(card2, card1, salt)
	if commitment.Root.Cmp(commitment4.Root) == 0 {
		t.Error("Card order should matter in commitment")
	}
}

func TestPlaceHelpers(t *testing.T) {
	// Test that helpers return values in expected ranges
	for i := 0; i < 52; i++ {
		if p := DeckPlace(i); p < DeckStart || p > DeckEnd {
			t.Errorf("DeckPlace(%d) = %d, out of range [%d, %d]", i, p, DeckStart, DeckEnd)
		}
		if p := DealtPlace(i); p < DealtStart || p > DealtEnd {
			t.Errorf("DealtPlace(%d) = %d, out of range [%d, %d]", i, p, DealtStart, DealtEnd)
		}
	}

	for p := 0; p < NumPlayers; p++ {
		for s := 0; s < 2; s++ {
			if hp := HolePlace(p, s); hp < HoleStart || hp > HoleEnd {
				t.Errorf("HolePlace(%d, %d) = %d, out of range", p, s, hp)
			}
		}

		if ap := ActivePlace(p); ap < ActiveStart || ap > ActiveEnd {
			t.Errorf("ActivePlace(%d) = %d, out of range", p, ap)
		}

		if fp := FoldedPlace(p); fp < FoldedStart || fp > FoldedEnd {
			t.Errorf("FoldedPlace(%d) = %d, out of range", p, fp)
		}

		if aip := AllInPlace(p); aip < AllInStart || aip > AllInEnd {
			t.Errorf("AllInPlace(%d) = %d, out of range", p, aip)
		}

		if tp := TurnPlace(p); tp < TurnStart || tp > TurnEnd {
			t.Errorf("TurnPlace(%d) = %d, out of range", p, tp)
		}
	}

	for s := 0; s < 5; s++ {
		if cp := CommunityPlace(s); cp < CommunityStart || cp > CommunityEnd {
			t.Errorf("CommunityPlace(%d) = %d, out of range", s, cp)
		}
	}
}

func TestTransitionHelpers(t *testing.T) {
	for p := 0; p < NumPlayers; p++ {
		for a := 0; a < 4; a++ {
			at := ActionTransition(p, a)
			if at < ActionStart || at >= SkipStart {
				t.Errorf("ActionTransition(%d, %d) = %d, out of range", p, a, at)
			}
		}

		st := SkipTransition(p)
		if st < SkipStart || st > SkipEnd {
			t.Errorf("SkipTransition(%d) = %d, out of range", p, st)
		}
	}
}

func TestMarkingRoot(t *testing.T) {
	m := InitialMarking()
	root := ComputeMarkingRoot(m)

	if root == nil {
		t.Fatal("Marking root is nil")
	}
	if root.Sign() == 0 {
		t.Error("Marking root is zero")
	}

	// Same marking should produce same root
	root2 := ComputeMarkingRoot(m)
	if root.Cmp(root2) != 0 {
		t.Error("Same marking should produce same root")
	}

	// Different marking should produce different root
	m2 := m
	m2[0] = 0 // Remove first card from deck
	root3 := ComputeMarkingRoot(m2)
	if root.Cmp(root3) == 0 {
		t.Error("Different marking should produce different root")
	}
}

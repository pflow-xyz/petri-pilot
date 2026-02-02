package zkpoker

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/test"
)

func TestDealCircuitCompiles(t *testing.T) {
	var circuit DealCircuit
	cs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		t.Fatalf("Circuit compilation failed: %v", err)
	}

	t.Logf("DealCircuit compiled successfully")
	t.Logf("  Constraints: %d", cs.GetNbConstraints())
	t.Logf("  Public inputs: %d", cs.GetNbPublicVariables())
	t.Logf("  Secret inputs: %d", cs.GetNbSecretVariables())
}

func TestHoleCommitmentCircuitCompiles(t *testing.T) {
	var circuit HoleCommitmentCircuit
	cs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		t.Fatalf("Circuit compilation failed: %v", err)
	}

	t.Logf("HoleCommitmentCircuit compiled successfully")
	t.Logf("  Constraints: %d", cs.GetNbConstraints())
	t.Logf("  Public inputs: %d", cs.GetNbPublicVariables())
	t.Logf("  Secret inputs: %d", cs.GetNbSecretVariables())
}

func TestShowdownCircuitCompiles(t *testing.T) {
	var circuit ShowdownCircuit
	cs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		t.Fatalf("Circuit compilation failed: %v", err)
	}

	t.Logf("ShowdownCircuit compiled successfully")
	t.Logf("  Constraints: %d", cs.GetNbConstraints())
	t.Logf("  Public inputs: %d", cs.GetNbPublicVariables())
	t.Logf("  Secret inputs: %d", cs.GetNbSecretVariables())
}

func TestHoleCommitmentCircuitValid(t *testing.T) {
	// Create a commitment
	card1, _ := ParseCard("Ah")
	card2, _ := ParseCard("Kh")
	salt := big.NewInt(12345)

	commitment := ComputeHoleCommitment(card1, card2, salt)

	// Create witness
	witness := &HoleCommitmentCircuit{
		Commitment: commitment.Root,
		Card1:      int(card1),
		Card2:      int(card2),
		Salt:       salt,
	}

	// Test the circuit
	assert := test.NewAssert(t)
	assert.ProverSucceeded(&HoleCommitmentCircuit{}, witness, test.WithBackends(backend.GROTH16), test.WithCurves(ecc.BN254))
}

func TestHoleCommitmentCircuitInvalid(t *testing.T) {
	// Create a commitment
	card1, _ := ParseCard("Ah")
	card2, _ := ParseCard("Kh")
	salt := big.NewInt(12345)

	commitment := ComputeHoleCommitment(card1, card2, salt)

	// Try to claim different cards
	wrongCard1, _ := ParseCard("2c")

	witness := &HoleCommitmentCircuit{
		Commitment: commitment.Root,
		Card1:      int(wrongCard1), // Wrong card!
		Card2:      int(card2),
		Salt:       salt,
	}

	// Should fail
	assert := test.NewAssert(t)
	assert.ProverFailed(&HoleCommitmentCircuit{}, witness, test.WithBackends(backend.GROTH16), test.WithCurves(ecc.BN254))
}

func TestHoleCommitmentCircuitWrongSalt(t *testing.T) {
	// Create a commitment
	card1, _ := ParseCard("Ah")
	card2, _ := ParseCard("Kh")
	salt := big.NewInt(12345)

	commitment := ComputeHoleCommitment(card1, card2, salt)

	// Try wrong salt
	wrongSalt := big.NewInt(99999)

	witness := &HoleCommitmentCircuit{
		Commitment: commitment.Root,
		Card1:      int(card1),
		Card2:      int(card2),
		Salt:       wrongSalt, // Wrong salt!
	}

	// Should fail
	assert := test.NewAssert(t)
	assert.ProverFailed(&HoleCommitmentCircuit{}, witness, test.WithBackends(backend.GROTH16), test.WithCurves(ecc.BN254))
}

func TestDealCircuitValid(t *testing.T) {
	// Create a deck (identity permutation for simplicity)
	var deck [52]Card
	for i := 0; i < 52; i++ {
		deck[i] = Card(i)
	}

	salt := big.NewInt(67890)
	commitment := ComputeDeckCommitment(deck, salt)

	// Deal cards at positions 0, 1, 2, 3, 4, 5, 6 (first 7 cards)
	var witness DealCircuit
	witness.DeckCommitment = commitment.Root
	witness.NumCardsDealt = 7
	witness.Salt = salt

	// Copy deck
	for i := 0; i < 52; i++ {
		witness.Deck[i] = int(deck[i])
	}

	// Card indices and dealt cards
	for i := 0; i < 7; i++ {
		witness.CardIndices[i] = i
		witness.DealtCards[i] = int(deck[i])
	}

	assert := test.NewAssert(t)
	assert.ProverSucceeded(&DealCircuit{}, &witness, test.WithBackends(backend.GROTH16), test.WithCurves(ecc.BN254))
}

func TestDealCircuitWrongCard(t *testing.T) {
	// Create a deck
	var deck [52]Card
	for i := 0; i < 52; i++ {
		deck[i] = Card(i)
	}

	salt := big.NewInt(67890)
	commitment := ComputeDeckCommitment(deck, salt)

	var witness DealCircuit
	witness.DeckCommitment = commitment.Root
	witness.NumCardsDealt = 7
	witness.Salt = salt

	for i := 0; i < 52; i++ {
		witness.Deck[i] = int(deck[i])
	}

	for i := 0; i < 7; i++ {
		witness.CardIndices[i] = i
		witness.DealtCards[i] = int(deck[i])
	}

	// Lie about the first card
	witness.DealtCards[0] = 51 // Claim As when it should be 2c

	assert := test.NewAssert(t)
	assert.ProverFailed(&DealCircuit{}, &witness, test.WithBackends(backend.GROTH16), test.WithCurves(ecc.BN254))
}

func TestDealCircuitWrongCommitment(t *testing.T) {
	// Create a deck
	var deck [52]Card
	for i := 0; i < 52; i++ {
		deck[i] = Card(i)
	}

	salt := big.NewInt(67890)
	commitment := ComputeDeckCommitment(deck, salt)

	// Swap two cards in the deck we provide
	swappedDeck := deck
	swappedDeck[0], swappedDeck[1] = swappedDeck[1], swappedDeck[0]

	var witness DealCircuit
	witness.DeckCommitment = commitment.Root // Original commitment
	witness.NumCardsDealt = 7
	witness.Salt = salt

	// But provide swapped deck
	for i := 0; i < 52; i++ {
		witness.Deck[i] = int(swappedDeck[i])
	}

	for i := 0; i < 7; i++ {
		witness.CardIndices[i] = i
		witness.DealtCards[i] = int(swappedDeck[i])
	}

	// Should fail because deck doesn't match commitment
	assert := test.NewAssert(t)
	assert.ProverFailed(&DealCircuit{}, &witness, test.WithBackends(backend.GROTH16), test.WithCurves(ecc.BN254))
}

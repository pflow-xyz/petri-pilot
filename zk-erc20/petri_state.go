package zkerc20

import (
	"fmt"
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/mimc"
)

// NumPlaces is the number of places in the 2-account ERC-20 Petri net.
const NumPlaces = 5

// NumTransitions is the number of transitions in the 2-account ERC-20 Petri net.
const NumTransitions = 10

// Place indices for the ERC-20 Petri net.
const (
	PlaceTotalSupply = 0 // Aggregate token supply
	PlaceBalance0    = 1 // Alice's balance
	PlaceBalance1    = 2 // Bob's balance
	PlaceAllow01     = 3 // Alice allows Bob to spend
	PlaceAllow10     = 4 // Bob allows Alice to spend
)

// PlaceNames maps place indices to human-readable names.
var PlaceNames = [NumPlaces]string{
	"totalSupply",
	"balance_0",
	"balance_1",
	"allowance_01",
	"allowance_10",
}

// Transition indices for the ERC-20 Petri net.
const (
	TTransfer01     = 0 // Alice → Bob transfer
	TTransfer10     = 1 // Bob → Alice transfer
	TApprove01      = 2 // Alice approves Bob
	TApprove10      = 3 // Bob approves Alice
	TTransferFrom01 = 4 // Bob spends Alice's tokens → Bob
	TTransferFrom10 = 5 // Alice spends Bob's tokens → Alice
	TMint0          = 6 // Mint to Alice
	TMint1          = 7 // Mint to Bob
	TBurn0          = 8 // Burn from Alice
	TBurn1          = 9 // Burn from Bob
)

// TransitionNames maps transition indices to human-readable names.
var TransitionNames = [NumTransitions]string{
	"transfer_01",
	"transfer_10",
	"approve_01",
	"approve_10",
	"transferFrom_01",
	"transferFrom_10",
	"mint_0",
	"mint_1",
	"burn_0",
	"burn_1",
}

// ArcDef represents input and output arcs for a transition.
// Arc weights are multiplied by the Amount parameter at firing time.
// IsApprove indicates set semantics: the output amount is absolute, not a delta.
type ArcDef struct {
	Inputs    []int // places consumed (amount-weighted)
	Outputs   []int // places produced (amount-weighted)
	IsApprove bool  // approve transitions use set semantics
}

// Topology defines the Petri net arcs for the 2-account ERC-20.
var Topology = [NumTransitions]ArcDef{
	// transfer_01: Alice sends Amount to Bob
	TTransfer01: {Inputs: []int{PlaceBalance0}, Outputs: []int{PlaceBalance1}},
	// transfer_10: Bob sends Amount to Alice
	TTransfer10: {Inputs: []int{PlaceBalance1}, Outputs: []int{PlaceBalance0}},

	// approve_01: Alice sets Bob's allowance to Amount (set semantics)
	TApprove01: {Inputs: []int{PlaceAllow01}, Outputs: []int{PlaceAllow01}, IsApprove: true},
	// approve_10: Bob sets Alice's allowance to Amount (set semantics)
	TApprove10: {Inputs: []int{PlaceAllow10}, Outputs: []int{PlaceAllow10}, IsApprove: true},

	// transferFrom_01: Bob spends from Alice's allowance → Bob gets tokens
	TTransferFrom01: {Inputs: []int{PlaceBalance0, PlaceAllow01}, Outputs: []int{PlaceBalance1}},
	// transferFrom_10: Alice spends from Bob's allowance → Alice gets tokens
	TTransferFrom10: {Inputs: []int{PlaceBalance1, PlaceAllow10}, Outputs: []int{PlaceBalance0}},

	// mint_0: Mint Amount tokens to Alice (increases totalSupply)
	TMint0: {Inputs: []int{}, Outputs: []int{PlaceBalance0, PlaceTotalSupply}},
	// mint_1: Mint Amount tokens to Bob (increases totalSupply)
	TMint1: {Inputs: []int{}, Outputs: []int{PlaceBalance1, PlaceTotalSupply}},

	// burn_0: Burn Amount tokens from Alice (decreases totalSupply)
	TBurn0: {Inputs: []int{PlaceBalance0, PlaceTotalSupply}, Outputs: []int{}},
	// burn_1: Burn Amount tokens from Bob (decreases totalSupply)
	TBurn1: {Inputs: []int{PlaceBalance1, PlaceTotalSupply}, Outputs: []int{}},
}

// Marking represents the token counts for all places in the Petri net.
// Uses int64 for larger token amounts (vs uint8 in tic-tac-toe).
type Marking [NumPlaces]int64

// InitialMarking returns the initial marking (all zeros — no tokens minted yet).
func InitialMarking() Marking {
	return Marking{}
}

// ComputeMarkingRoot computes a MiMC hash of the full marking.
func ComputeMarkingRoot(m Marking) *big.Int {
	h := mimc.NewMiMC()
	for _, tokens := range m {
		var elem fr.Element
		if tokens >= 0 {
			elem.SetUint64(uint64(tokens))
		} else {
			// Negative values should not occur in valid states
			elem.SetUint64(0)
		}
		b := elem.Bytes()
		h.Write(b[:])
	}
	sum := h.Sum(nil)
	return new(big.Int).SetBytes(sum)
}

// IsEnabled checks if a transition can fire with the current marking and amount.
func IsEnabled(m Marking, t int, amount int64) bool {
	if t < 0 || t >= NumTransitions {
		return false
	}
	if amount <= 0 {
		return false
	}

	arc := Topology[t]

	if arc.IsApprove {
		// Approve transitions: the allowance place must have >= 0 tokens (always true for non-negative marking)
		// and the amount can be any non-negative value (set semantics).
		// We just need the input place to exist and have a non-negative count.
		return true
	}

	// Standard transitions: each input place must have >= amount tokens
	for _, p := range arc.Inputs {
		if m[p] < amount {
			return false
		}
	}
	return true
}

// Fire applies a transition with the given amount and returns the new marking.
// Returns an error if the transition is not enabled.
func Fire(m Marking, t int, amount int64) (Marking, error) {
	if t < 0 || t >= NumTransitions {
		return m, fmt.Errorf("invalid transition index: %d", t)
	}
	if !IsEnabled(m, t, amount) {
		return m, fmt.Errorf("transition %s is not enabled with amount %d", TransitionNames[t], amount)
	}

	newM := m
	arc := Topology[t]

	if arc.IsApprove {
		// Approve: set semantics — consume all current allowance, produce Amount
		// The allowance place appears in both Inputs and Outputs
		allowancePlace := arc.Inputs[0] // same as Outputs[0] for approve
		newM[allowancePlace] = amount
		return newM, nil
	}

	// Standard: delta = Amount * direction
	for _, p := range arc.Inputs {
		newM[p] -= amount
	}
	for _, p := range arc.Outputs {
		newM[p] += amount
	}
	return newM, nil
}

// EnabledTransitions returns all transitions that can fire with the given amount.
func EnabledTransitions(m Marking, amount int64) []int {
	var enabled []int
	for t := 0; t < NumTransitions; t++ {
		if IsEnabled(m, t, amount) {
			enabled = append(enabled, t)
		}
	}
	return enabled
}

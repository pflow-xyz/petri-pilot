package zkode

import "math/big"

// NumPlaces is the number of places in the MVP cascade net (A → B → C).
const NumPlaces = 3

// NumTransitions is the number of transitions (A→B, B→C).
const NumTransitions = 2

// Place indices.
const (
	PlaceA = 0
	PlaceB = 1
	PlaceC = 2
)

// PlaceNames maps indices to human-readable names.
var PlaceNames = [NumPlaces]string{"A", "B", "C"}

// Stoichiometry matrix S[place][transition] = net change in place when transition fires.
// S = Output - Input for the cascade A→B→C:
//
//	        t0(A→B)  t1(B→C)
//	A:       -1        0
//	B:       +1       -1
//	C:        0       +1
var Stoichiometry = [NumPlaces][NumTransitions]int{
	{-1, 0},  // A
	{+1, -1}, // B
	{0, +1},  // C
}

// InputPlaces maps each transition to its input place index.
// Mass-action rate for transition t = rate_constant[t] * marking[InputPlace[t]].
// This is first-order kinetics (single input per transition).
var InputPlaces = [NumTransitions]int{
	PlaceA, // t0: A → B (rate depends on A)
	PlaceB, // t1: B → C (rate depends on B)
}

// DefaultRates returns default rate constants (k=1.0 for both transitions),
// as fixed-point field elements.
func DefaultRates() [NumTransitions]*big.Int {
	return [NumTransitions]*big.Int{
		FixFromFloat(1.0),
		FixFromFloat(1.0),
	}
}

// DefaultInitialMarking returns [1, 0, 0] as fixed-point field elements
// (all tokens start in place A).
func DefaultInitialMarking() [NumPlaces]*big.Int {
	return [NumPlaces]*big.Int{
		FixFromFloat(1.0),
		FixFromFloat(0.0),
		FixFromFloat(0.0),
	}
}

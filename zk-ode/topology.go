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

// Tsit5 Butcher tableau coefficients, pre-converted to fixed-point field elements.
// Reference: Tsitouras 5(4) RK method.

// tsit5C are the node coefficients.
var tsit5C = [7]*big.Int{
	FixFromFloat(0),
	FixFromFloat(0.161),
	FixFromFloat(0.327),
	FixFromFloat(0.9),
	FixFromFloat(0.9800255409045097),
	FixFromFloat(1),
	FixFromFloat(1),
}

// tsit5A is the Runge-Kutta coefficient matrix. Only lower-triangular entries
// are non-zero. Stored as a ragged array matching the reference implementation.
var tsit5A = [7][]*big.Int{
	{},
	{FixFromFloat(0.161)},
	{FixFromFloat(-0.008480655492356924), FixFromFloat(0.335480655492357)},
	{FixFromFloat(2.8971530571054935), FixFromFloat(-6.359448489975075), FixFromFloat(4.362295432869581)},
	{FixFromFloat(5.325864828439257), FixFromFloat(-11.748883564062828), FixFromFloat(7.4955393428898365), FixFromFloat(-0.09249506636175525)},
	{FixFromFloat(5.86145544294642), FixFromFloat(-12.92096931784711), FixFromFloat(8.159367898576159), FixFromFloat(-0.071584973281401), FixFromFloat(-0.028269050394068383)},
	{FixFromFloat(0.09646076681806523), FixFromFloat(0.01), FixFromFloat(0.4798896504144996), FixFromFloat(1.379008574103742), FixFromFloat(-3.290069515436081), FixFromFloat(2.324710524099774)},
}

// tsit5B are the 5th-order solution weights.
var tsit5B = [7]*big.Int{
	FixFromFloat(0.09646076681806523),
	FixFromFloat(0.01),
	FixFromFloat(0.4798896504144996),
	FixFromFloat(1.379008574103742),
	FixFromFloat(-3.290069515436081),
	FixFromFloat(2.324710524099774),
	FixFromFloat(0),
}

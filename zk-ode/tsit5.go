package zkode

// Tsit5 Butcher tableau coefficients, pre-converted to fixed-point field elements.
// Reference: Tsitouras 5(4) RK method.
//
// These are exported so that generated ZK ODE packages can import them.

import "math/big"

// Tsit5C are the node coefficients.
var Tsit5C = [7]*big.Int{
	FixFromFloat(0),
	FixFromFloat(0.161),
	FixFromFloat(0.327),
	FixFromFloat(0.9),
	FixFromFloat(0.9800255409045097),
	FixFromFloat(1),
	FixFromFloat(1),
}

// Tsit5A is the Runge-Kutta coefficient matrix. Only lower-triangular entries
// are non-zero. Stored as a ragged array matching the reference implementation.
var Tsit5A = [7][]*big.Int{
	{},
	{FixFromFloat(0.161)},
	{FixFromFloat(-0.008480655492356924), FixFromFloat(0.335480655492357)},
	{FixFromFloat(2.8971530571054935), FixFromFloat(-6.359448489975075), FixFromFloat(4.362295432869581)},
	{FixFromFloat(5.325864828439257), FixFromFloat(-11.748883564062828), FixFromFloat(7.4955393428898365), FixFromFloat(-0.09249506636175525)},
	{FixFromFloat(5.86145544294642), FixFromFloat(-12.92096931784711), FixFromFloat(8.159367898576159), FixFromFloat(-0.071584973281401), FixFromFloat(-0.028269050394068383)},
	{FixFromFloat(0.09646076681806523), FixFromFloat(0.01), FixFromFloat(0.4798896504144996), FixFromFloat(1.379008574103742), FixFromFloat(-3.290069515436081), FixFromFloat(2.324710524099774)},
}

// Tsit5B are the 5th-order solution weights.
var Tsit5B = [7]*big.Int{
	FixFromFloat(0.09646076681806523),
	FixFromFloat(0.01),
	FixFromFloat(0.4798896504144996),
	FixFromFloat(1.379008574103742),
	FixFromFloat(-3.290069515436081),
	FixFromFloat(2.324710524099774),
	FixFromFloat(0),
}

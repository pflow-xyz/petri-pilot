// Package zkode implements a ZK circuit that proves correct execution of a Tsit5
// ODE solver over encrypted Petri net state. The private witness is the token
// marking (place values). The public inputs are the net topology rate constants
// and step size. Proofs chain via MiMC hash commitments.
package zkode

import (
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
)

func init() {
	solver.RegisterHint(fixMulHint)
}

// Scale is 10^18, matching standard fixed-point precision for ZK circuits.
// All continuous values (markings, rates, coefficients) are scaled by this factor.
var Scale = new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

// scaleBits is the number of bits needed to represent Scale (60 bits > log2(10^18)).
const scaleBits = 60

// fieldModulus is the BN254 scalar field prime.
var fieldModulus = fr.Modulus()

// halfField is p/2, used to interpret field elements as signed values.
var halfField = new(big.Int).Rsh(fieldModulus, 1)

// FixMul computes fixed-point multiplication inside a gnark circuit.
// Uses a hint for integer division: a * b = q * SCALE + r, with 0 <= r < 2^60.
// Returns q (the quotient).
func FixMul(api frontend.API, a, b frontend.Variable) frontend.Variable {
	outputs, err := api.NewHint(fixMulHint, 2, a, b)
	if err != nil {
		panic("FixMul hint failed: " + err.Error())
	}
	q := outputs[0] // quotient
	r := outputs[1] // remainder

	// Verify: a * b == q * SCALE + r (in the field)
	lhs := api.Mul(a, b)
	rhs := api.Add(api.Mul(q, Scale), r)
	api.AssertIsEqual(lhs, rhs)

	// Range check: 0 <= r < 2^60 (sufficient since SCALE < 2^60)
	api.ToBinary(r, scaleBits)

	return q
}

// fixMulHint computes the quotient and remainder of signed fixed-point multiplication.
// Field elements > p/2 are interpreted as negative values.
// Outputs: [quotient, remainder] where product = quotient * SCALE + remainder, 0 <= remainder < SCALE.
func fixMulHint(mod *big.Int, inputs []*big.Int, outputs []*big.Int) error {
	halfMod := new(big.Int).Rsh(mod, 1)

	// Interpret field elements as signed
	a := new(big.Int).Set(inputs[0])
	if a.Cmp(halfMod) > 0 {
		a.Sub(a, mod)
	}
	b := new(big.Int).Set(inputs[1])
	if b.Cmp(halfMod) > 0 {
		b.Sub(b, mod)
	}

	product := new(big.Int).Mul(a, b)

	// Euclidean division: product = q * Scale + r, 0 <= r < Scale
	r := new(big.Int).Mod(product, Scale)
	q := new(big.Int).Sub(product, r)
	q.Div(q, Scale)

	// Reduce quotient to field
	outputs[0] = new(big.Int).Mod(q, mod)
	outputs[1] = r // always non-negative from big.Int.Mod with positive modulus
	return nil
}

// FixFromFloat converts a float64 to a fixed-point *big.Int (scaled by 10^18).
// The result is reduced to the BN254 scalar field, so negative values become
// field elements (p - |x * SCALE|).
func FixFromFloat(f float64) *big.Int {
	// Use big.Float for precise scaling
	bf := new(big.Float).SetPrec(128).SetFloat64(f)
	scaleBf := new(big.Float).SetPrec(128).SetInt(Scale)
	bf.Mul(bf, scaleBf)

	result := new(big.Int)
	bf.Int(result) // truncates toward zero

	// Reduce to field: negative values become p - |val|
	result.Mod(result, fieldModulus)
	return result
}

// NativeFixMul computes fixed-point multiplication using math/big with signed
// interpretation. This mirrors the circuit's FixMul exactly (integer division,
// not field division).
func NativeFixMul(a, b *big.Int) *big.Int {
	// Interpret field elements as signed
	aSign := new(big.Int).Set(a)
	if aSign.Cmp(halfField) > 0 {
		aSign.Sub(aSign, fieldModulus)
	}
	bSign := new(big.Int).Set(b)
	if bSign.Cmp(halfField) > 0 {
		bSign.Sub(bSign, fieldModulus)
	}

	product := new(big.Int).Mul(aSign, bSign)

	// Euclidean division: product = q * Scale + r, 0 <= r < Scale
	r := new(big.Int).Mod(product, Scale)
	q := new(big.Int).Sub(product, r)
	q.Div(q, Scale)

	// Reduce to field
	q.Mod(q, fieldModulus)
	return q
}

// NativeFixAdd computes (a + b) mod p.
func NativeFixAdd(a, b *big.Int) *big.Int {
	result := new(big.Int).Add(a, b)
	result.Mod(result, fieldModulus)
	return result
}

// NativeFixSub computes (a - b) mod p.
func NativeFixSub(a, b *big.Int) *big.Int {
	result := new(big.Int).Sub(a, b)
	result.Mod(result, fieldModulus)
	return result
}

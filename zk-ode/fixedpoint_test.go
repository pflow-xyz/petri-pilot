package zkode

import (
	"math"
	"math/big"
	"testing"
)

func TestFixFromFloat(t *testing.T) {
	tests := []struct {
		name  string
		input float64
	}{
		{"zero", 0},
		{"one", 1.0},
		{"half", 0.5},
		{"small positive", 0.01},
		{"negative", -3.290069515436081},
		{"large positive", 5.86145544294642},
		{"tiny negative", -0.028269050394068383},
		{"tsit5 B0", 0.09646076681806523},
		{"tsit5 B3", 1.379008574103742},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FixFromFloat(tt.input)

			// Verify result is in field
			if result.Sign() < 0 || result.Cmp(fieldModulus) >= 0 {
				t.Errorf("result out of field range: %s", result)
			}

			// For positive values, verify approximate magnitude
			if tt.input > 0 {
				expected := new(big.Float).SetPrec(128).SetFloat64(tt.input)
				expected.Mul(expected, new(big.Float).SetPrec(128).SetInt(Scale))
				expectedInt := new(big.Int)
				expected.Int(expectedInt)

				diff := new(big.Int).Sub(result, expectedInt)
				diff.Abs(diff)

				// Allow 1 unit of error from truncation
				if diff.Cmp(big.NewInt(1)) > 0 {
					t.Errorf("FixFromFloat(%g) = %s, expected ~%s (diff=%s)",
						tt.input, result, expectedInt, diff)
				}
			}
		})
	}
}

func TestNativeFixMul(t *testing.T) {
	tests := []struct {
		name string
		a, b float64
		want float64
	}{
		{"1 * 1", 1.0, 1.0, 1.0},
		{"2 * 3", 2.0, 3.0, 6.0},
		{"0.5 * 4", 0.5, 4.0, 2.0},
		{"0.1 * 0.1", 0.1, 0.1, 0.01},
		{"rate * marking", 0.5, 100.0, 50.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := FixFromFloat(tt.a)
			b := FixFromFloat(tt.b)
			result := NativeFixMul(a, b)

			// Convert back to float for comparison
			resultFloat := new(big.Float).SetPrec(128).SetInt(result)
			scaleFloat := new(big.Float).SetPrec(128).SetInt(Scale)
			resultFloat.Quo(resultFloat, scaleFloat)
			got, _ := resultFloat.Float64()

			if math.Abs(got-tt.want) > 1e-10 {
				t.Errorf("NativeFixMul(%g, %g) = %g, want %g", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestNativeFixMulNegative(t *testing.T) {
	// Test that negative coefficient multiplication works correctly in field
	a := FixFromFloat(-3.290069515436081) // negative Tsit5 B[4]
	b := FixFromFloat(1.0)                // unit marking

	result := NativeFixMul(a, b)

	// The result should be -3.29... in the field (p - 3.29e18)
	// Adding the positive version should give ~0
	pos := FixFromFloat(3.290069515436081)
	sum := NativeFixAdd(result, pos)

	// Sum should be very close to 0 (within rounding)
	if sum.Cmp(big.NewInt(2)) > 0 && sum.Cmp(new(big.Int).Sub(fieldModulus, big.NewInt(2))) < 0 {
		t.Errorf("negative*positive + positive != 0, got %s", sum)
	}
}

func TestTsit5CoefficientsPreserved(t *testing.T) {
	// Verify all Butcher tableau B coefficients convert and recover accurately
	bFloats := []float64{
		0.09646076681806523,
		0.01,
		0.4798896504144996,
		1.379008574103742,
		-3.290069515436081,
		2.324710524099774,
		0,
	}

	for i, bf := range bFloats {
		scaled := FixFromFloat(bf)

		// For non-zero positive values, convert back and check
		if bf > 0 {
			resultFloat := new(big.Float).SetPrec(128).SetInt(scaled)
			scaleFloat := new(big.Float).SetPrec(128).SetInt(Scale)
			resultFloat.Quo(resultFloat, scaleFloat)
			got, _ := resultFloat.Float64()

			relErr := math.Abs(got-bf) / math.Abs(bf)
			if relErr > 1e-15 {
				t.Errorf("B[%d]: FixFromFloat(%g) recovers as %g (relErr=%e)", i, bf, got, relErr)
			}
		}

		if bf == 0 {
			if scaled.Sign() != 0 {
				t.Errorf("B[%d]: FixFromFloat(0) = %s, want 0", i, scaled)
			}
		}
	}
}

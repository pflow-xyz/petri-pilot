package zkode

import (
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/mimc"
)

// ODEState tracks the current marking and its MiMC state root.
type ODEState struct {
	Marking [NumPlaces]*big.Int // Fixed-point field elements
	Root    *big.Int            // MiMC hash of Marking
	Step    int                 // Step number (0 = genesis)
}

// NewODEState creates an initial state from a marking.
func NewODEState(marking [NumPlaces]*big.Int) *ODEState {
	s := &ODEState{
		Marking: marking,
		Step:    0,
	}
	s.Root = ComputeRoot(marking[:])
	return s
}

// ComputeRoot computes a MiMC hash over the marking values.
// Uses the BN254 MiMC implementation to match the gnark circuit.
func ComputeRoot(values []*big.Int) *big.Int {
	h := mimc.NewMiMC()
	for _, v := range values {
		var elem fr.Element
		elem.SetBigInt(v)
		b := elem.Bytes()
		h.Write(b[:])
	}
	return new(big.Int).SetBytes(h.Sum(nil))
}

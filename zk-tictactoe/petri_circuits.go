package zktictactoe

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash/mimc"
)

// PetriTransitionCircuit proves that firing a transition on the Petri net is valid.
//
// This encodes the FULL Petri net state (33 places), not just the board.
// Win detection, turn control, and all game logic comes from the net topology.
//
// Public inputs:
//   - PreStateRoot:  MiMC hash of marking before transition
//   - PostStateRoot: MiMC hash of marking after transition
//   - Transition:    which transition fired (0-34)
//
// Private inputs:
//   - PreMarking:  token counts for all 33 places before firing
//   - PostMarking: token counts for all 33 places after firing
type PetriTransitionCircuit struct {
	// Public
	PreStateRoot  frontend.Variable `gnark:",public"`
	PostStateRoot frontend.Variable `gnark:",public"`
	Transition    frontend.Variable `gnark:",public"`

	// Private
	PreMarking  [NumPlaces]frontend.Variable
	PostMarking [NumPlaces]frontend.Variable
}

// Define declares the constraints for valid Petri net transition firing.
func (c *PetriTransitionCircuit) Define(api frontend.API) error {
	// 1. Verify pre-state root matches the private marking
	preRoot := petriMimcHash(api, c.PreMarking[:])
	api.AssertIsEqual(preRoot, c.PreStateRoot)

	// 2. Verify post-state root matches the private marking
	postRoot := petriMimcHash(api, c.PostMarking[:])
	api.AssertIsEqual(postRoot, c.PostStateRoot)

	// 3. Compute expected input/output deltas based on which transition fired
	// We build delta[p] = output[p] - input[p] for the selected transition
	var deltas [NumPlaces]frontend.Variable
	for p := 0; p < NumPlaces; p++ {
		deltas[p] = frontend.Variable(0)
	}

	// For each transition, conditionally add its effect
	for t := 0; t < NumTransitions; t++ {
		isThis := api.IsZero(api.Sub(c.Transition, t))

		// Subtract 1 from input places
		for _, p := range Topology[t].Inputs {
			deltas[p] = api.Sub(deltas[p], isThis)
		}
		// Add 1 to output places
		for _, p := range Topology[t].Outputs {
			deltas[p] = api.Add(deltas[p], isThis)
		}
	}

	// 4. Verify the marking change matches the computed deltas
	// post[p] = pre[p] + delta[p]
	for p := 0; p < NumPlaces; p++ {
		expected := api.Add(c.PreMarking[p], deltas[p])
		api.AssertIsEqual(c.PostMarking[p], expected)
	}

	// 5. Verify enabledness: for the selected transition, all input places must have >= 1 token
	// We compute: for each place, if it's an input to the selected transition, pre[p] >= 1
	for p := 0; p < NumPlaces; p++ {
		// Check if this place is an input to any transition that matches
		isInput := frontend.Variable(0)
		for t := 0; t < NumTransitions; t++ {
			isThis := api.IsZero(api.Sub(c.Transition, t))
			for _, inp := range Topology[t].Inputs {
				if inp == p {
					isInput = api.Add(isInput, isThis)
				}
			}
		}

		// If isInput > 0, then pre[p] must be >= 1
		// We check: isInput * (1 - isPositive(pre[p])) == 0
		// Simpler: if isInput != 0, assert pre[p] >= 1
		// Use: pre[p] - 1 must be non-negative when isInput is set
		//
		// Actually, we already validated post = pre + delta, and if pre[p] < input weight,
		// then post[p] would be negative (huge field element).
		// We can detect this by checking post[p] fits in reasonable bits.

		// For places that are inputs to the selected transition:
		// pre[p] must be at least 1, so pre[p] - 1 >= 0
		// We decompose (pre[p] - isInput) into bits - if pre[p] < isInput, this wraps to huge value
		diff := api.Sub(c.PreMarking[p], isInput)
		// Only constrain if isInput could be 1
		// ToBinary will fail if diff is negative (wraps to large field element)
		api.ToBinary(diff, 8) // 8 bits is enough for small token counts
	}

	return nil
}

// petriMimcHash computes MiMC hash of marking values.
func petriMimcHash(api frontend.API, values []frontend.Variable) frontend.Variable {
	h, _ := mimc.NewMiMC(api)
	for _, v := range values {
		h.Write(v)
	}
	return h.Sum()
}

// PetriWinCircuit proves that a player has won by checking the win_x or win_o place.
//
// This is simpler than the board-based win circuit because the Petri net
// topology already encodes win detection - we just check if tokens arrived
// at the win place.
type PetriWinCircuit struct {
	// Public
	StateRoot frontend.Variable `gnark:",public"`
	Winner    frontend.Variable `gnark:",public"` // PlaceWinX (29) or PlaceWinO (30)

	// Private
	Marking [NumPlaces]frontend.Variable
}

// Define declares the constraints for win verification.
func (c *PetriWinCircuit) Define(api frontend.API) error {
	// 1. Verify state root matches the private marking
	root := petriMimcHash(api, c.Marking[:])
	api.AssertIsEqual(root, c.StateRoot)

	// 2. Verify the claimed winner place has at least 1 token
	// Winner should be PlaceWinX (29) or PlaceWinO (30)

	// Get token count at the winner place using selector
	winTokens := frontend.Variable(0)
	for p := 0; p < NumPlaces; p++ {
		isWinnerPlace := api.IsZero(api.Sub(c.Winner, p))
		winTokens = api.Add(winTokens, api.Mul(c.Marking[p], isWinnerPlace))
	}

	// Assert winTokens >= 1
	// winTokens - 1 must be non-negative
	api.ToBinary(api.Sub(winTokens, 1), 8)

	// 3. Verify Winner is a valid win place (29 or 30)
	isXWin := api.IsZero(api.Sub(c.Winner, PlaceWinX))
	isOWin := api.IsZero(api.Sub(c.Winner, PlaceWinO))
	validWinner := api.Add(isXWin, isOWin)
	api.AssertIsEqual(validWinner, 1)

	return nil
}

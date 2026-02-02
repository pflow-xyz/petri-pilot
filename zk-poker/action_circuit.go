package zkpoker

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash/mimc"
)

// ActionCircuit proves that firing a transition on the poker Petri net is valid.
//
// This follows the same pattern as tic-tac-toe's PetriTransitionCircuit but
// for the larger poker state (147 places, 84 transitions).
//
// Public inputs:
//   - PreStateRoot:  MiMC hash of marking before transition
//   - PostStateRoot: MiMC hash of marking after transition
//   - Transition:    which transition fired (0-83)
//
// Private inputs:
//   - PreMarking:  token counts for all 147 places before firing
//   - PostMarking: token counts for all 147 places after firing
type ActionCircuit struct {
	// Public
	PreStateRoot  frontend.Variable `gnark:",public"`
	PostStateRoot frontend.Variable `gnark:",public"`
	Transition    frontend.Variable `gnark:",public"`

	// Private
	PreMarking  [NumPlaces]frontend.Variable
	PostMarking [NumPlaces]frontend.Variable
}

// Define declares the constraints for valid Petri net transition firing.
func (c *ActionCircuit) Define(api frontend.API) error {
	// 1. Verify pre-state root matches the private marking
	preRoot := actionMimcHash(api, c.PreMarking[:])
	api.AssertIsEqual(preRoot, c.PreStateRoot)

	// 2. Verify post-state root matches the private marking
	postRoot := actionMimcHash(api, c.PostMarking[:])
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
		// pre[p] - isInput must be non-negative (>= 0)
		// We decompose to bits - if negative, wraps to huge field element
		diff := api.Sub(c.PreMarking[p], isInput)
		// ToBinary will fail if diff is negative (wraps to large field element)
		// 8 bits is enough for small token counts (max 255)
		api.ToBinary(diff, 8)
	}

	return nil
}

// actionMimcHash computes MiMC hash of marking values.
func actionMimcHash(api frontend.API, values []frontend.Variable) frontend.Variable {
	h, _ := mimc.NewMiMC(api)
	for _, v := range values {
		h.Write(v)
	}
	return h.Sum()
}

// PrepareActionWitness creates the witness data for proving a transition.
func PrepareActionWitness(preMark, postMark Marking, transition int) *ActionCircuit {
	var circuit ActionCircuit

	// Set transition
	circuit.Transition = transition

	// Copy markings
	for p := 0; p < NumPlaces; p++ {
		circuit.PreMarking[p] = preMark[p]
		circuit.PostMarking[p] = postMark[p]
	}

	// Compute state roots
	circuit.PreStateRoot = ComputeMarkingRoot(preMark)
	circuit.PostStateRoot = ComputeMarkingRoot(postMark)

	return &circuit
}

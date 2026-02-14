package zkerc20

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash/mimc"
)

// ERC20TransitionCircuit proves that firing a transition on the ERC-20 Petri net is valid.
//
// Unlike the tic-tac-toe circuit (unit arc weights), this circuit supports parametric
// amounts: a transfer of 100 tokens debits 100 from sender and credits 100 to receiver.
// Approve transitions use set semantics (absolute value, not delta).
//
// Public inputs:
//   - PreStateRoot:  MiMC hash of marking before transition
//   - PostStateRoot: MiMC hash of marking after transition
//   - Transition:    which transition fired (0-9)
//   - Amount:        token amount for the operation
//
// Private inputs:
//   - PreMarking:  token counts for all 5 places before firing
//   - PostMarking: token counts for all 5 places after firing
type ERC20TransitionCircuit struct {
	// Public
	PreStateRoot  frontend.Variable `gnark:",public"`
	PostStateRoot frontend.Variable `gnark:",public"`
	Transition    frontend.Variable `gnark:",public"`
	Amount        frontend.Variable `gnark:",public"`

	// Private
	PreMarking  [NumPlaces]frontend.Variable
	PostMarking [NumPlaces]frontend.Variable
}

// Define declares the constraints for valid ERC-20 transition firing.
func (c *ERC20TransitionCircuit) Define(api frontend.API) error {
	// 1. Verify pre-state root matches the private marking
	preRoot := petriMimcHash(api, c.PreMarking[:])
	api.AssertIsEqual(preRoot, c.PreStateRoot)

	// 2. Verify post-state root matches the private marking
	postRoot := petriMimcHash(api, c.PostMarking[:])
	api.AssertIsEqual(postRoot, c.PostStateRoot)

	// 3. Compute expected deltas for each place based on the selected transition.
	//
	// For standard transitions (transfer, transferFrom, mint, burn):
	//   delta[p] = isThisTransition * Amount * direction
	//   where direction is -1 for inputs, +1 for outputs
	//
	// For approve transitions (set semantics):
	//   post[allowance] = Amount (absolute set)
	//   delta[allowance] = Amount - pre[allowance]
	//
	// We compute the expected post marking and compare.

	var expectedPost [NumPlaces]frontend.Variable
	for p := 0; p < NumPlaces; p++ {
		expectedPost[p] = c.PreMarking[p]
	}

	for t := 0; t < NumTransitions; t++ {
		isThis := api.IsZero(api.Sub(c.Transition, t))
		arc := Topology[t]

		if arc.IsApprove {
			// Approve: set semantics — post[place] = Amount when this transition fires
			// The allowance place appears in both Inputs and Outputs
			allowancePlace := arc.Inputs[0]

			// delta = Amount - pre[allowancePlace]
			// But only when isThis == 1
			// conditionalDelta = isThis * (Amount - pre[allowancePlace])
			setDelta := api.Sub(c.Amount, c.PreMarking[allowancePlace])
			conditionalDelta := api.Mul(isThis, setDelta)
			expectedPost[allowancePlace] = api.Add(expectedPost[allowancePlace], conditionalDelta)
		} else {
			// Standard: delta[p] = isThis * Amount * direction
			for _, p := range arc.Inputs {
				delta := api.Mul(isThis, c.Amount)
				expectedPost[p] = api.Sub(expectedPost[p], delta)
			}
			for _, p := range arc.Outputs {
				delta := api.Mul(isThis, c.Amount)
				expectedPost[p] = api.Add(expectedPost[p], delta)
			}
		}
	}

	// 4. Verify the actual post marking matches expected
	for p := 0; p < NumPlaces; p++ {
		api.AssertIsEqual(c.PostMarking[p], expectedPost[p])
	}

	// 5. Verify enabledness: input places must have sufficient tokens.
	// For each place, compute how much is consumed by the selected transition.
	// Then verify pre[p] >= consumed[p] via bit decomposition.
	for p := 0; p < NumPlaces; p++ {
		consumed := frontend.Variable(0)
		for t := 0; t < NumTransitions; t++ {
			isThis := api.IsZero(api.Sub(c.Transition, t))
			arc := Topology[t]

			if arc.IsApprove {
				// Approve doesn't consume — it sets. No enabledness check needed
				// (any non-negative allowance is valid).
				continue
			}

			for _, inp := range arc.Inputs {
				if inp == p {
					consumed = api.Add(consumed, api.Mul(isThis, c.Amount))
				}
			}
		}

		// pre[p] - consumed >= 0
		// Decompose into bits; wraps to huge field value if negative
		diff := api.Sub(c.PreMarking[p], consumed)
		api.ToBinary(diff, 64) // 64 bits for token amounts up to ~1.8 * 10^19
	}

	return nil
}

// ERC20InvariantCircuit proves a conservation law: sum(balances) == totalSupply.
//
// This circuit verifies that the token state satisfies the fundamental ERC-20
// invariant without revealing the individual balances.
//
// Public inputs:
//   - StateRoot: MiMC hash of the marking
//
// Private inputs:
//   - Marking: token counts for all 5 places
type ERC20InvariantCircuit struct {
	// Public
	StateRoot frontend.Variable `gnark:",public"`

	// Private
	Marking [NumPlaces]frontend.Variable
}

// Define declares the constraints for the ERC-20 conservation invariant.
func (c *ERC20InvariantCircuit) Define(api frontend.API) error {
	// 1. Verify state root matches the private marking
	root := petriMimcHash(api, c.Marking[:])
	api.AssertIsEqual(root, c.StateRoot)

	// 2. sum(balances) == totalSupply
	sumBal := api.Add(c.Marking[PlaceBalance0], c.Marking[PlaceBalance1])
	api.AssertIsEqual(sumBal, c.Marking[PlaceTotalSupply])

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

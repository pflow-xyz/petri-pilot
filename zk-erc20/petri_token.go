package zkerc20

import (
	"fmt"
	"math/big"
)

// TokenState tracks the full Petri net state for ZK proof generation.
type TokenState struct {
	Marking Marking
	Roots   []*big.Int // state root after each transition
}

// NewTokenState creates a new token state with the initial marking (all zeros).
func NewTokenState() *TokenState {
	m := InitialMarking()
	root := ComputeMarkingRoot(m)
	return &TokenState{
		Marking: m,
		Roots:   []*big.Int{root},
	}
}

// CurrentRoot returns the current marking's state root.
func (s *TokenState) CurrentRoot() *big.Int {
	return s.Roots[len(s.Roots)-1]
}

// ERC20TransitionWitness contains all values needed to generate a ZK proof.
type ERC20TransitionWitness struct {
	PreStateRoot  *big.Int
	PostStateRoot *big.Int
	Transition    int
	Amount        int64
	PreMarking    Marking
	PostMarking   Marking
}

// fireTransition fires a transition and returns the witness for proof generation.
func (s *TokenState) fireTransition(t int, amount int64) (*ERC20TransitionWitness, error) {
	if t < 0 || t >= NumTransitions {
		return nil, fmt.Errorf("invalid transition index: %d", t)
	}

	preMarking := s.Marking
	preRoot := s.CurrentRoot()

	newMarking, err := Fire(s.Marking, t, amount)
	if err != nil {
		return nil, fmt.Errorf("transition %s failed: %w", TransitionNames[t], err)
	}

	postRoot := ComputeMarkingRoot(newMarking)

	witness := &ERC20TransitionWitness{
		PreStateRoot:  preRoot,
		PostStateRoot: postRoot,
		Transition:    t,
		Amount:        amount,
		PreMarking:    preMarking,
		PostMarking:   newMarking,
	}

	s.Marking = newMarking
	s.Roots = append(s.Roots, postRoot)

	return witness, nil
}

// Transfer moves tokens between accounts.
// from/to: 0=Alice, 1=Bob
func (s *TokenState) Transfer(from, to int, amount int64) (*ERC20TransitionWitness, error) {
	if from == to {
		return nil, fmt.Errorf("cannot transfer to self")
	}

	var t int
	if from == 0 && to == 1 {
		t = TTransfer01
	} else if from == 1 && to == 0 {
		t = TTransfer10
	} else {
		return nil, fmt.Errorf("invalid from/to: %d/%d", from, to)
	}

	return s.fireTransition(t, amount)
}

// Approve sets the spending allowance.
// owner grants spender permission to spend up to amount tokens.
func (s *TokenState) Approve(owner, spender int, amount int64) (*ERC20TransitionWitness, error) {
	if owner == spender {
		return nil, fmt.Errorf("cannot approve self")
	}

	var t int
	if owner == 0 && spender == 1 {
		t = TApprove01
	} else if owner == 1 && spender == 0 {
		t = TApprove10
	} else {
		return nil, fmt.Errorf("invalid owner/spender: %d/%d", owner, spender)
	}

	return s.fireTransition(t, amount)
}

// TransferFrom spends tokens from an approved allowance.
// spender moves amount from 'from' to 'spender's account.
func (s *TokenState) TransferFrom(from, to, spender int, amount int64) (*ERC20TransitionWitness, error) {
	if from == to {
		return nil, fmt.Errorf("cannot transferFrom to same account")
	}

	// Validate spender is the 'to' account (topology encodes this)
	var t int
	if from == 0 && to == 1 && spender == 1 {
		t = TTransferFrom01 // Bob spends Alice's tokens → Bob
	} else if from == 1 && to == 0 && spender == 0 {
		t = TTransferFrom10 // Alice spends Bob's tokens → Alice
	} else {
		return nil, fmt.Errorf("invalid transferFrom: from=%d to=%d spender=%d", from, to, spender)
	}

	return s.fireTransition(t, amount)
}

// Mint creates new tokens and credits them to an account.
func (s *TokenState) Mint(to int, amount int64) (*ERC20TransitionWitness, error) {
	var t int
	switch to {
	case 0:
		t = TMint0
	case 1:
		t = TMint1
	default:
		return nil, fmt.Errorf("invalid account: %d", to)
	}

	return s.fireTransition(t, amount)
}

// Burn destroys tokens from an account.
func (s *TokenState) Burn(from int, amount int64) (*ERC20TransitionWitness, error) {
	var t int
	switch from {
	case 0:
		t = TBurn0
	case 1:
		t = TBurn1
	default:
		return nil, fmt.Errorf("invalid account: %d", from)
	}

	return s.fireTransition(t, amount)
}

// ToTransitionAssignment converts a witness to a circuit assignment.
func (w *ERC20TransitionWitness) ToTransitionAssignment() *ERC20TransitionCircuit {
	c := &ERC20TransitionCircuit{
		PreStateRoot:  w.PreStateRoot,
		PostStateRoot: w.PostStateRoot,
		Transition:    w.Transition,
		Amount:        w.Amount,
	}
	for i := 0; i < NumPlaces; i++ {
		c.PreMarking[i] = w.PreMarking[i]
		c.PostMarking[i] = w.PostMarking[i]
	}
	return c
}

// ERC20InvariantWitness contains values for proving the conservation law.
type ERC20InvariantWitness struct {
	StateRoot *big.Int
	Marking   Marking
}

// GetInvariantWitness returns a witness for proving the conservation invariant.
func (s *TokenState) GetInvariantWitness() *ERC20InvariantWitness {
	return &ERC20InvariantWitness{
		StateRoot: s.CurrentRoot(),
		Marking:   s.Marking,
	}
}

// ToInvariantAssignment converts a witness to a circuit assignment.
func (w *ERC20InvariantWitness) ToInvariantAssignment() *ERC20InvariantCircuit {
	c := &ERC20InvariantCircuit{
		StateRoot: w.StateRoot,
	}
	for i := 0; i < NumPlaces; i++ {
		c.Marking[i] = w.Marking[i]
	}
	return c
}

// String returns a human-readable representation of the marking.
func (m Marking) String() string {
	return fmt.Sprintf(
		"totalSupply=%d  balance[0]=%d  balance[1]=%d  allowance[0→1]=%d  allowance[1→0]=%d",
		m[PlaceTotalSupply], m[PlaceBalance0], m[PlaceBalance1],
		m[PlaceAllow01], m[PlaceAllow10],
	)
}

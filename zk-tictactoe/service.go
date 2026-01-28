package zktictactoe

import (
	"fmt"
	"log/slog"
	"strconv"
	"sync"

	"github.com/consensys/gnark/frontend"
	"github.com/pflow-xyz/go-pflow/prover"
)

// TicTacToeWitnessFactory converts raw JSON witness maps into typed circuit assignments.
type TicTacToeWitnessFactory struct{}

// CreateAssignment builds a circuit assignment from a witness map.
//
// For "move" circuit, expected witness keys:
//
//	pre_state_root, post_state_root, position, player,
//	board_0..board_8, turn_count
//
// For "win" circuit, expected witness keys:
//
//	state_root, winner, board_0..board_8
func (f *TicTacToeWitnessFactory) CreateAssignment(circuitName string, witness map[string]string) (frontend.Circuit, error) {
	switch circuitName {
	case "move":
		return createMoveAssignment(witness)
	case "win":
		return createWinAssignment(witness)
	default:
		return nil, fmt.Errorf("unknown circuit: %s", circuitName)
	}
}

func createMoveAssignment(w map[string]string) (*MoveCircuit, error) {
	c := &MoveCircuit{}
	var err error

	c.PreStateRoot, err = parseField(w, "pre_state_root")
	if err != nil {
		return nil, err
	}
	c.PostStateRoot, err = parseField(w, "post_state_root")
	if err != nil {
		return nil, err
	}
	c.Position, err = parseField(w, "position")
	if err != nil {
		return nil, err
	}
	c.Player, err = parseField(w, "player")
	if err != nil {
		return nil, err
	}
	c.TurnCount, err = parseField(w, "turn_count")
	if err != nil {
		return nil, err
	}
	for i := 0; i < 9; i++ {
		c.Board[i], err = parseField(w, fmt.Sprintf("board_%d", i))
		if err != nil {
			return nil, err
		}
	}
	return c, nil
}

func createWinAssignment(w map[string]string) (*WinCircuit, error) {
	c := &WinCircuit{}
	var err error

	c.StateRoot, err = parseField(w, "state_root")
	if err != nil {
		return nil, err
	}
	c.Winner, err = parseField(w, "winner")
	if err != nil {
		return nil, err
	}
	for i := 0; i < 9; i++ {
		c.Board[i], err = parseField(w, fmt.Sprintf("board_%d", i))
		if err != nil {
			return nil, err
		}
	}
	return c, nil
}

// parseField extracts a field from the witness map, parsing hex or decimal.
func parseField(w map[string]string, key string) (interface{}, error) {
	val, ok := w[key]
	if !ok {
		return nil, fmt.Errorf("missing witness field: %s", key)
	}
	return prover.ParseBigInt(val)
}

// NewTicTacToeService creates a prover.Service with "move" and "win" circuits registered.
func NewTicTacToeService() (*prover.Service, error) {
	p := prover.NewProver()

	slog.Info("Compiling tic-tac-toe circuits...")

	// Register circuits in parallel
	type result struct {
		name string
		cc   *prover.CompiledCircuit
		err  error
	}
	results := make(chan result, 2)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		cc, err := p.CompileCircuit("move", &MoveCircuit{})
		results <- result{"move", cc, err}
	}()

	go func() {
		defer wg.Done()
		cc, err := p.CompileCircuit("win", &WinCircuit{})
		results <- result{"win", cc, err}
	}()

	wg.Wait()
	close(results)

	for r := range results {
		if r.err != nil {
			return nil, fmt.Errorf("failed to compile %s circuit: %w", r.name, r.err)
		}
		p.StoreCircuit(r.name, r.cc)
		slog.Info("Circuit compiled",
			"name", r.name,
			"constraints", r.cc.Constraints,
			"public", r.cc.PublicVars,
			"private", r.cc.PrivateVars,
		)
	}

	factory := &TicTacToeWitnessFactory{}
	return prover.NewService(p, factory), nil
}

// parseInt parses a decimal string to int.
func parseInt(s string) (int, error) {
	return strconv.Atoi(s)
}

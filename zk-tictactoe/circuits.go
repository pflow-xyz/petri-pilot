package zktictactoe

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash/mimc"
)

// mimcHash computes a MiMC hash of the given inputs inside a circuit.
func mimcHash(api frontend.API, inputs ...frontend.Variable) frontend.Variable {
	h, _ := mimc.NewMiMC(api)
	for _, v := range inputs {
		h.Write(v)
	}
	return h.Sum()
}

// MoveCircuit proves that a tic-tac-toe move is valid.
//
// Public inputs:
//   - PreStateRoot:  MiMC hash of the board before the move
//   - PostStateRoot: MiMC hash of the board after the move
//   - Position:      cell index (0-8)
//   - Player:        who is moving (1=X, 2=O)
//
// Private inputs:
//   - Board:     the 9 cells of the pre-move board
//   - TurnCount: how many moves have been made (0-indexed)
type MoveCircuit struct {
	// Public
	PreStateRoot  frontend.Variable `gnark:",public"`
	PostStateRoot frontend.Variable `gnark:",public"`
	Position      frontend.Variable `gnark:",public"`
	Player        frontend.Variable `gnark:",public"`

	// Private
	Board     [9]frontend.Variable
	TurnCount frontend.Variable
}

// Define declares the constraints for a valid move.
func (c *MoveCircuit) Define(api frontend.API) error {
	// 1. Verify pre-state root matches the private board.
	preRoot := mimcHash(api, c.Board[:]...)
	api.AssertIsEqual(preRoot, c.PreStateRoot)

	// 2. Verify the target cell is empty (Board[Position] == 0).
	//    We compute the value at Position using a linear scan selector:
	//    val = sum_i( Board[i] * (Position == i) )
	cellValue := frontend.Variable(0)
	for i := 0; i < 9; i++ {
		isTarget := api.IsZero(api.Sub(c.Position, i))
		cellValue = api.Add(cellValue, api.Mul(c.Board[i], isTarget))
	}
	api.AssertIsEqual(cellValue, 0)

	// 3. Verify correct player's turn.
	//    Even turns (0,2,4,...) → X (1), Odd turns (1,3,5,...) → O (2).
	//    turnMod2 = TurnCount mod 2, computed via bit decomposition.
	//    expectedPlayer = 1 + turnMod2  (0→1=X, 1→2=O)
	bits := api.ToBinary(c.TurnCount, 4) // 4 bits is enough for 0-8
	turnMod2 := bits[0]                  // LSB = parity
	expectedPlayer := api.Add(1, turnMod2)
	api.AssertIsEqual(c.Player, expectedPlayer)

	// 4. Compute post-state board: replace Board[Position] with Player.
	var postBoard [9]frontend.Variable
	for i := 0; i < 9; i++ {
		isTarget := api.IsZero(api.Sub(c.Position, i))
		// postBoard[i] = isTarget * Player + (1 - isTarget) * Board[i]
		postBoard[i] = api.Add(
			api.Mul(isTarget, c.Player),
			api.Mul(api.Sub(1, isTarget), c.Board[i]),
		)
	}

	// 5. Verify post-state root.
	postRoot := mimcHash(api, postBoard[:]...)
	api.AssertIsEqual(postRoot, c.PostStateRoot)

	return nil
}

// WinCircuit proves that a player has won the game.
//
// Public inputs:
//   - StateRoot: MiMC hash of the board
//   - Winner:    the winning player (1=X, 2=O)
//
// Private inputs:
//   - Board: the 9 cells of the board
type WinCircuit struct {
	// Public
	StateRoot frontend.Variable `gnark:",public"`
	Winner    frontend.Variable `gnark:",public"`

	// Private
	Board [9]frontend.Variable
}

// Define declares the constraints for win verification.
func (c *WinCircuit) Define(api frontend.API) error {
	// 1. Verify state root matches the private board.
	root := mimcHash(api, c.Board[:]...)
	api.AssertIsEqual(root, c.StateRoot)

	// 2. Check all 8 win patterns. At least one must match Winner.
	//    For each pattern (a,b,c):
	//      matchesWinner = (Board[a] == Winner) AND (Board[b] == Winner) AND (Board[c] == Winner)
	//    We accumulate: anyWin = sum of all matchesWinner values.
	//    Then assert anyWin != 0.
	anyWin := frontend.Variable(0)
	for _, pat := range WinPatterns {
		a := api.IsZero(api.Sub(c.Board[pat[0]], c.Winner))
		b := api.IsZero(api.Sub(c.Board[pat[1]], c.Winner))
		c2 := api.IsZero(api.Sub(c.Board[pat[2]], c.Winner))
		lineMatch := api.Mul(a, api.Mul(b, c2))
		anyWin = api.Add(anyWin, lineMatch)
	}
	// anyWin >= 1 iff at least one line matches. Assert it's nonzero.
	api.AssertIsDifferent(anyWin, 0)

	return nil
}

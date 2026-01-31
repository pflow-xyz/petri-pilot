package zktictactoe

import (
	"fmt"
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/mimc"
)

// Cell values on the board.
const (
	Empty = 0
	X     = 1
	O     = 2
)

// BoardState represents a 3x3 tic-tac-toe board.
// Cells are indexed 0-8 (row-major): [0,1,2,3,4,5,6,7,8].
type BoardState [9]uint8

// ComputeStateRoot computes a MiMC hash of the board state.
// This produces a field element compatible with the gnark circuit.
func ComputeStateRoot(board BoardState) *big.Int {
	h := mimc.NewMiMC()
	for _, cell := range board {
		var elem fr.Element
		elem.SetUint64(uint64(cell))
		b := elem.Bytes()
		h.Write(b[:])
	}
	sum := h.Sum(nil)
	return new(big.Int).SetBytes(sum)
}

// ApplyMove places a player's mark at the given position and returns
// the new board and its state root. Returns an error if the move is invalid.
func ApplyMove(board BoardState, pos int, player uint8) (BoardState, *big.Int, error) {
	if pos < 0 || pos > 8 {
		return board, nil, fmt.Errorf("position %d out of range [0,8]", pos)
	}
	if player != X && player != O {
		return board, nil, fmt.Errorf("invalid player %d, must be %d (X) or %d (O)", player, X, O)
	}
	if board[pos] != Empty {
		return board, nil, fmt.Errorf("cell %d is already occupied by %d", pos, board[pos])
	}
	newBoard := board
	newBoard[pos] = player
	root := ComputeStateRoot(newBoard)
	return newBoard, root, nil
}

// WinPatterns lists all 8 winning lines (rows, columns, diagonals).
var WinPatterns = [8][3]int{
	{0, 1, 2}, // row 0
	{3, 4, 5}, // row 1
	{6, 7, 8}, // row 2
	{0, 3, 6}, // col 0
	{1, 4, 7}, // col 1
	{2, 5, 8}, // col 2
	{0, 4, 8}, // diagonal
	{2, 4, 6}, // anti-diagonal
}

// CheckWinner returns the winner (X or O) or Empty if no winner.
func CheckWinner(board BoardState) uint8 {
	for _, pat := range WinPatterns {
		a, b, c := board[pat[0]], board[pat[1]], board[pat[2]]
		if a != Empty && a == b && b == c {
			return a
		}
	}
	return Empty
}

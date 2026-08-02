package core

import (
	"fmt"
	"sort"
)

// Findings is what the generation-time model-check learned about the net.
// The Lean proof template bakes these into theorems that the Lean kernel
// re-derives by evaluation — so a wrong finding is not a wrong comment, it is
// a file that does not compile.
type Findings struct {
	// StateCount is the number of reachable markings.
	StateCount int

	// Fuel bounds the emitted Lean BFS. It exceeds StateCount by enough
	// that the closure theorem, not the fuel, is what guarantees the search
	// finished.
	Fuel int

	// Bounds is the per-place maximum token count observed across all
	// reachable markings, aligned with Model.Places (sorted by name).
	Bounds []int

	// Deadlocks are the reachable markings with no enabled transition,
	// aligned with Model.Places and sorted lexicographically.
	Deadlocks [][]int

	// Tactic is the Lean tactic the emitted theorems use: "decide" (pure
	// kernel reduction) for small state spaces, "native_decide" (compiled
	// evaluation, larger trusted base) when kernel reduction would crawl.
	Tactic string
}

// Model-check limits. Kernel reduction (`decide`) is the proof engine on the
// Lean side, and it evaluates the whole search — these bounds keep that
// tractable and catch unbounded nets.
const (
	maxStates = 4096
	maxTokens = 4096
)

type marking []int

func (m marking) key() string {
	return fmt.Sprint([]int(m))
}

// ModelCheck explores the net's full reachable state space under the same
// semantics the generated code uses (inputs and reads satisfied, inhibitors
// clear, bounded outputs have room) and reports what it found. It refuses
// nets whose state space exceeds maxStates markings or whose places exceed
// maxTokens tokens — the former makes the Lean kernel's job intractable, the
// latter means the net is effectively unbounded.
func ModelCheck(m Model) (*Findings, error) {
	placeIndex := make(map[string]int, len(m.Places))
	for i, p := range m.Places {
		placeIndex[p.Name] = i
	}

	initial := make(marking, len(m.Places))
	for i, p := range m.Places {
		initial[i] = p.Initial
	}

	enabled := func(mk marking, t Transition) bool {
		for _, a := range t.Inputs {
			if mk[placeIndex[a.Place]] < a.Weight {
				return false
			}
		}
		for _, a := range t.Reads {
			if mk[placeIndex[a.Place]] < a.Weight {
				return false
			}
		}
		for _, a := range t.Inhibits {
			if mk[placeIndex[a.Place]] >= a.Weight {
				return false
			}
		}
		for _, a := range t.Outputs {
			if a.Capacity > 0 && mk[placeIndex[a.Place]]+a.Weight > a.Capacity {
				return false
			}
		}
		return true
	}

	fire := func(mk marking, t Transition) marking {
		next := make(marking, len(mk))
		copy(next, mk)
		for _, a := range t.Inputs {
			next[placeIndex[a.Place]] -= a.Weight
		}
		for _, a := range t.Outputs {
			next[placeIndex[a.Place]] += a.Weight
		}
		return next
	}

	seen := map[string]bool{initial.key(): true}
	frontier := []marking{initial}
	findings := &Findings{Bounds: make([]int, len(m.Places))}
	var deadlocks []marking

	for len(frontier) > 0 {
		mk := frontier[0]
		frontier = frontier[1:]
		findings.StateCount++

		for i, count := range mk {
			if count > maxTokens {
				return nil, fmt.Errorf("place %q reaches %d tokens: the net looks unbounded (limit %d)", m.Places[i].Name, count, maxTokens)
			}
			if count > findings.Bounds[i] {
				findings.Bounds[i] = count
			}
		}

		fired := false
		for _, t := range m.Transitions {
			if !enabled(mk, t) {
				continue
			}
			fired = true
			next := fire(mk, t)
			if !seen[next.key()] {
				if len(seen) >= maxStates {
					return nil, fmt.Errorf("state space exceeds %d markings: too large for the proof form's kernel evaluation", maxStates)
				}
				seen[next.key()] = true
				frontier = append(frontier, next)
			}
		}
		if !fired {
			deadlocks = append(deadlocks, mk)
		}
	}

	sort.Slice(deadlocks, func(i, j int) bool {
		for k := range deadlocks[i] {
			if deadlocks[i][k] != deadlocks[j][k] {
				return deadlocks[i][k] < deadlocks[j][k]
			}
		}
		return false
	})
	for _, d := range deadlocks {
		findings.Deadlocks = append(findings.Deadlocks, []int(d))
	}

	findings.Fuel = findings.StateCount + 4
	findings.Tactic = "decide"
	if findings.StateCount > 128 {
		findings.Tactic = "native_decide"
	}
	return findings, nil
}

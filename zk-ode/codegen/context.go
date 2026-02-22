package codegen

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

// Context holds all computed data needed by templates.
type Context struct {
	PackageName string
	ModelName   string

	NumPlaces      int
	NumTransitions int

	Places      []PlaceInfo
	Transitions []TransitionInfo

	// Stoichiometry[place][transition] = net token change.
	Stoichiometry [][]int

	// MaxInputsPerTransition is the maximum number of input places across all transitions.
	MaxInputsPerTransition int

	// HasScoring indicates whether scoring templates should be generated.
	HasScoring bool
	Scoring    *ScoringContext
}

// PlaceInfo holds per-place metadata for templates.
type PlaceInfo struct {
	ID        string
	Index     int
	ConstName string  // Go constant name (e.g., "PlaceP00")
	Initial   float64 // Initial marking as float
}

// TransitionInfo holds per-transition metadata for templates.
type TransitionInfo struct {
	ID        string
	Index     int
	ConstName string  // Go constant name (e.g., "TransXPlay00")
	Rate      float64 // Rate constant (default 1.0)
	Inputs    []int   // Indices of input places
	NumInputs int
}

// ScoringContext holds resolved scoring information for templates.
type ScoringContext struct {
	Candidates    []int   // Indices of candidate transitions
	Targets       []int   // Indices of target transitions
	Bonus         float64 // Score bonus for enabling a target
	Penalty       float64 // Score penalty for opponent threat
	NumCandidates int
	NumTargets    int
}

// NewContext builds a template context from a metamodel.Model.
func NewContext(model *metamodel.Model, pkg string, scoring *ScoringConfig) (*Context, error) {
	if len(model.Places) == 0 {
		return nil, fmt.Errorf("model has no places")
	}
	if len(model.Transitions) == 0 {
		return nil, fmt.Errorf("model has no transitions")
	}

	ctx := &Context{
		PackageName:    pkg,
		ModelName:      model.Name,
		NumPlaces:      len(model.Places),
		NumTransitions: len(model.Transitions),
	}

	// Build place index map
	placeIndex := make(map[string]int)
	for i, p := range model.Places {
		placeIndex[p.ID] = i
		ctx.Places = append(ctx.Places, PlaceInfo{
			ID:        p.ID,
			Index:     i,
			ConstName: toConstName("Place", p.ID),
			Initial:   float64(p.Initial),
		})
	}

	// Build transition index map and extract rates
	transIndex := make(map[string]int)
	var transIDs []string
	for i, t := range model.Transitions {
		transIndex[t.ID] = i
		transIDs = append(transIDs, t.ID)

		rate := 1.0
		if t.Rate > 0 {
			rate = t.Rate
		}
		// Override with simulation solver rates if present
		if model.Simulation != nil && model.Simulation.Solver != nil {
			if r, ok := model.Simulation.Solver.Rates[t.ID]; ok {
				rate = r
			}
		}

		ctx.Transitions = append(ctx.Transitions, TransitionInfo{
			ID:        t.ID,
			Index:     i,
			ConstName: toConstName("Trans", t.ID),
			Rate:      rate,
		})
	}

	// Build stoichiometry matrix and transition inputs from arcs
	ctx.Stoichiometry = make([][]int, ctx.NumPlaces)
	for p := range ctx.Stoichiometry {
		ctx.Stoichiometry[p] = make([]int, ctx.NumTransitions)
	}

	// Track inputs per transition
	inputSets := make([]map[int]bool, ctx.NumTransitions)
	for t := range inputSets {
		inputSets[t] = make(map[int]bool)
	}

	for _, arc := range model.Arcs {
		fromPlace, fromIsPlace := placeIndex[arc.From]
		fromTrans, fromIsTrans := transIndex[arc.From]
		toPlace, toIsPlace := placeIndex[arc.To]
		toTrans, toIsTrans := transIndex[arc.To]

		weight := arc.Weight
		if weight == 0 {
			weight = 1
		}

		if arc.IsInhibitor() {
			// Inhibitor arcs: the place is a read-only input to the transition
			if fromIsPlace && toIsTrans {
				inputSets[toTrans][fromPlace] = true
			}
			continue
		}

		if fromIsPlace && toIsTrans {
			// Place → Transition: input arc (consumes tokens)
			ctx.Stoichiometry[fromPlace][toTrans] -= weight
			inputSets[toTrans][fromPlace] = true
		} else if fromIsTrans && toIsPlace {
			// Transition → Place: output arc (produces tokens)
			ctx.Stoichiometry[toPlace][fromTrans] += weight
		} else {
			return nil, fmt.Errorf("invalid arc from %q to %q: arcs must connect places to transitions or vice versa", arc.From, arc.To)
		}
	}

	// Populate transition inputs
	for t := 0; t < ctx.NumTransitions; t++ {
		var inputs []int
		for p := 0; p < ctx.NumPlaces; p++ {
			if inputSets[t][p] {
				inputs = append(inputs, p)
			}
		}
		ctx.Transitions[t].Inputs = inputs
		ctx.Transitions[t].NumInputs = len(inputs)
		if len(inputs) > ctx.MaxInputsPerTransition {
			ctx.MaxInputsPerTransition = len(inputs)
		}
	}

	// Build scoring context if config provided
	if scoring != nil {
		candidates := MatchGlobs(transIDs, scoring.Candidates)
		targets := MatchGlobs(transIDs, scoring.Targets)

		if len(candidates) == 0 {
			return nil, fmt.Errorf("scoring: no transitions matched candidate globs %v", scoring.Candidates)
		}
		if len(targets) == 0 {
			return nil, fmt.Errorf("scoring: no transitions matched target globs %v", scoring.Targets)
		}

		ctx.HasScoring = true
		ctx.Scoring = &ScoringContext{
			Candidates:    candidates,
			Targets:       targets,
			Bonus:         scoring.Bonus,
			Penalty:       scoring.Penalty,
			NumCandidates: len(candidates),
			NumTargets:    len(targets),
		}

		// Auto-derive rate constants for candidates from target connectivity.
		// For each candidate, count how many targets it connects to through
		// its unique output places (places not shared by ALL candidates).
		ctx.deriveRatesFromTopology(candidates, targets)
	}

	return ctx, nil
}

// deriveRatesFromTopology computes rate constants for candidate transitions
// based on their connectivity to target transitions through the stoichiometry
// matrix. Each candidate's rate = number of targets reachable through its
// unique output places (places not produced by any other candidate).
// Explicit model rates (non-default) are preserved.
func (ctx *Context) deriveRatesFromTopology(candidates, targets []int) {
	// Find output places for each candidate: places where S[p][c] > 0
	candidateOutputs := make(map[int]map[int]bool)
	for _, cIdx := range candidates {
		candidateOutputs[cIdx] = make(map[int]bool)
		for p := 0; p < ctx.NumPlaces; p++ {
			if ctx.Stoichiometry[p][cIdx] > 0 {
				candidateOutputs[cIdx][p] = true
			}
		}
	}

	// For each candidate, find outputs unique to it (not produced by any
	// other candidate). This filters control-flow places like turn tokens
	// that are shared across same-player transitions.
	for _, cIdx := range candidates {
		// Skip if model specified an explicit rate
		if ctx.Transitions[cIdx].Rate != 1.0 {
			continue
		}

		uniqueOutputs := make(map[int]bool)
		for p := range candidateOutputs[cIdx] {
			unique := true
			for _, otherIdx := range candidates {
				if otherIdx == cIdx {
					continue
				}
				if candidateOutputs[otherIdx][p] {
					unique = false
					break
				}
			}
			if unique {
				uniqueOutputs[p] = true
			}
		}

		count := 0
		for _, tIdx := range targets {
			for _, inp := range ctx.Transitions[tIdx].Inputs {
				if uniqueOutputs[inp] {
					count++
					break // count each target once
				}
			}
		}

		if count > 1 {
			ctx.Transitions[cIdx].Rate = float64(count)
		}
	}
}

// toConstName converts a prefix and ID like "Place" + "x_play_00" to "PlaceXPlay00".
func toConstName(prefix, id string) string {
	parts := strings.FieldsFunc(id, func(r rune) bool {
		return r == '_' || r == '-' || r == '.'
	})
	var b strings.Builder
	b.WriteString(prefix)
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		b.WriteString(string(runes))
	}
	return b.String()
}

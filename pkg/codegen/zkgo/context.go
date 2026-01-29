package zkgo

import (
	"fmt"
	"strings"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

// Context provides data for ZK code generation templates.
type Context struct {
	PackageName string
	ModelName   string

	// Places
	NumPlaces  int
	Places     []PlaceInfo
	PlaceIndex map[string]int // place ID -> index

	// Transitions
	NumTransitions  int
	Transitions     []TransitionInfo
	TransitionIndex map[string]int // transition ID -> index

	// Initial marking
	InitialMarking []int // token count per place
}

// PlaceInfo contains information about a place.
type PlaceInfo struct {
	ID      string
	Index   int
	Initial int
	VarName string // Go constant name (e.g., "PlaceXTurn")
}

// TransitionInfo contains information about a transition.
type TransitionInfo struct {
	ID      string
	Index   int
	VarName string // Go constant name (e.g., "TXPlay00")
	Inputs  []int  // input place indices
	Outputs []int  // output place indices
}

// NewContext creates a new Context from a Petri net model.
func NewContext(model *metamodel.Model, pkgName string) (*Context, error) {
	if pkgName == "" {
		pkgName = sanitizePackageName(model.Name)
	}

	ctx := &Context{
		PackageName:     pkgName,
		ModelName:       model.Name,
		PlaceIndex:      make(map[string]int),
		TransitionIndex: make(map[string]int),
	}

	// Build place list (use order from model)
	for i, place := range model.Places {
		ctx.Places = append(ctx.Places, PlaceInfo{
			ID:      place.ID,
			Index:   i,
			Initial: place.Initial,
			VarName: placeVarName(place.ID),
		})
		ctx.PlaceIndex[place.ID] = i
		ctx.InitialMarking = append(ctx.InitialMarking, place.Initial)
	}
	ctx.NumPlaces = len(ctx.Places)

	// Build transition list (use order from model)
	for i, trans := range model.Transitions {
		ctx.Transitions = append(ctx.Transitions, TransitionInfo{
			ID:      trans.ID,
			Index:   i,
			VarName: transitionVarName(trans.ID),
			Inputs:  []int{},
			Outputs: []int{},
		})
		ctx.TransitionIndex[trans.ID] = i
	}
	ctx.NumTransitions = len(ctx.Transitions)

	// Build arc topology
	for _, arc := range model.Arcs {
		fromPlace, fromIsPlace := ctx.PlaceIndex[arc.From]
		toPlace, toIsPlace := ctx.PlaceIndex[arc.To]
		fromTrans, fromIsTrans := ctx.TransitionIndex[arc.From]
		toTrans, toIsTrans := ctx.TransitionIndex[arc.To]

		if fromIsPlace && toIsTrans {
			// Input arc: place -> transition
			ctx.Transitions[toTrans].Inputs = append(ctx.Transitions[toTrans].Inputs, fromPlace)
		} else if fromIsTrans && toIsPlace {
			// Output arc: transition -> place
			ctx.Transitions[fromTrans].Outputs = append(ctx.Transitions[fromTrans].Outputs, toPlace)
		}
	}

	return ctx, nil
}

// placeVarName converts a place ID to a Go constant name.
func placeVarName(id string) string {
	// Convert snake_case to PascalCase with "Place" prefix
	parts := strings.Split(id, "_")
	var sb strings.Builder
	sb.WriteString("Place")
	for _, part := range parts {
		if len(part) > 0 {
			sb.WriteString(strings.ToUpper(part[:1]))
			if len(part) > 1 {
				sb.WriteString(part[1:])
			}
		}
	}
	return sb.String()
}

// transitionVarName converts a transition ID to a Go constant name.
func transitionVarName(id string) string {
	// Convert snake_case to PascalCase with "T" prefix
	parts := strings.Split(id, "_")
	var sb strings.Builder
	sb.WriteString("T")
	for _, part := range parts {
		if len(part) > 0 {
			sb.WriteString(strings.ToUpper(part[:1]))
			if len(part) > 1 {
				sb.WriteString(part[1:])
			}
		}
	}
	return sb.String()
}

func sanitizePackageName(name string) string {
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ToLower(name)
	return name
}

// HasInitialTokens returns true if the place has initial tokens.
func (p PlaceInfo) HasInitialTokens() bool {
	return p.Initial > 0
}

// InputsStr returns the inputs as a Go slice literal.
func (t TransitionInfo) InputsStr() string {
	if len(t.Inputs) == 0 {
		return "[]int{}"
	}
	parts := make([]string, len(t.Inputs))
	for i, idx := range t.Inputs {
		parts[i] = fmt.Sprintf("%d", idx)
	}
	return fmt.Sprintf("[]int{%s}", strings.Join(parts, ", "))
}

// OutputsStr returns the outputs as a Go slice literal.
func (t TransitionInfo) OutputsStr() string {
	if len(t.Outputs) == 0 {
		return "[]int{}"
	}
	parts := make([]string, len(t.Outputs))
	for i, idx := range t.Outputs {
		parts[i] = fmt.Sprintf("%d", idx)
	}
	return fmt.Sprintf("[]int{%s}", strings.Join(parts, ", "))
}

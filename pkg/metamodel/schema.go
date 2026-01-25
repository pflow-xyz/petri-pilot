// Package metamodel provides the core metamodel types for Petri net execution.
// This is a local fork of go-pflow/metamodel, extended for petri-pilot's needs.
package metamodel

import goflowmodel "github.com/pflow-xyz/go-pflow/metamodel"

// Kind discriminates between token-counting and data-holding states.
type Kind string

const (
	// TokenState holds an integer count (classic Petri net "tokens-as-money").
	// Firing semantics: input arcs decrement count, output arcs increment count.
	// Used for control flow, synchronization, enablement conditions.
	TokenState Kind = "token"

	// DataState holds typed structured data ("tokens-as-data").
	// Firing semantics: arcs specify keys for map access and transformation.
	// Used for state: balances, owners, allowances, approvals.
	DataState Kind = "data"
)

// State represents a named container in a goflowmodel.
// The Kind field determines whether this is a countable token state
// or a structured data state.
type State struct {
	ID   string `json:"id"`
	Kind Kind   `json:"kind,omitempty"` // "token" or "data" (default: "data")

	// For TokenState: Initial is an int (token count)
	// For DataState: Initial can be any value (map, struct, etc.)
	Initial any `json:"initial,omitempty"`

	// Type describes the state's type goflowmodel.
	// For TokenState: typically empty or "int"
	// For DataState: e.g., "map[string]int64", "string", "int64"
	Type string `json:"type,omitempty"`

	// Exported states are externally visible.
	Exported bool `json:"exported,omitempty"`

	// Description provides human-readable documentation.
	Description string `json:"description,omitempty"`
}

// IsToken returns true if this is a token-counting state.
func (s *State) IsToken() bool {
	return s.Kind == TokenState
}

// IsData returns true if this is a data-holding state.
func (s *State) IsData() bool {
	return s.Kind == DataState || s.Kind == "" // default to data
}

// InitialTokens returns the initial token count (for TokenState).
// Returns 0 if not a token state or if Initial is not numeric.
func (s *State) InitialTokens() int {
	if !s.IsToken() {
		return 0
	}
	switch v := s.Initial.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

// IsSimpleType returns true if this data state holds a simple type (string, int64, etc.)
// rather than a collection type (map).
func (s *State) IsSimpleType() bool {
	if !s.IsData() {
		return false
	}
	switch s.Type {
	case "string", "int64", "int", "float64", "bool", "time.Time":
		return true
	default:
		return false
	}
}

// IsMapType returns true if this data state holds a map type.
func (s *State) IsMapType() bool {
	if !s.IsData() {
		return false
	}
	return len(s.Type) > 4 && s.Type[:4] == "map["
}

// Action represents a state-changing operation.
// Generalizes Petri net "Transition" to support arbitrary transformations.
type Action struct {
	ID    string `json:"id"`
	Guard string `json:"guard,omitempty"` // precondition expression

	// Description provides human-readable documentation.
	Description string `json:"description,omitempty"`

	// Bindings maps parameter names to types for this action.
	Bindings map[string]string `json:"bindings,omitempty"`
}

// ArcType discriminates between normal and inhibitor arcs.
type ArcType string

const (
	// NormalArc consumes tokens from input places and produces tokens to output places.
	NormalArc ArcType = ""

	// InhibitorArc prevents firing if the source place has tokens.
	// Inhibitor arcs are read-only - they don't consume or produce tokens.
	InhibitorArc ArcType = "inhibitor"
)

// Arc connects states and actions, defining state transformation flow.
// Semantics depend on the connected state's Kind:
//   - TokenState: arc weight is 1, decrement on input, increment on output
//   - DataState: Keys specify map access path, Value specifies the binding name
//   - InhibitorArc: prevents firing if source has tokens (read-only)
type Arc struct {
	Source string   `json:"source"`           // state or action ID
	Target string   `json:"target"`           // state or action ID
	Keys   []string `json:"keys,omitempty"`   // for DataState: binding names for map keys
	Value  string   `json:"value,omitempty"`  // for DataState: binding name for value (default: "amount")
	Weight int      `json:"weight,omitempty"` // for TokenState: arc weight (default: 1)
	Type   ArcType  `json:"type,omitempty"`   // arc type: "" (normal) or "inhibitor"
}

// IsInhibitor returns true if this is an inhibitor arc.
func (a *Arc) IsInhibitor() bool {
	return a.Type == InhibitorArc
}

// Constraint represents a property that must hold across all snapshots.
// Constraints are checked after each action executes (unless disabled).
type Constraint struct {
	ID   string `json:"id"`
	Expr string `json:"expr"` // expression over snapshot (e.g., "totalSupply >= 0")
}

// Simulation configures ODE-based simulation for move evaluation and AI.
// When present, enables strategic analysis using the Guard DSL for scoring.
type Simulation struct {
	// Objective is a numeric expression evaluated against the marking.
	// Examples: "win_x - win_o", "tokens('goal')", "sum('score')"
	// The DSL supports: +, -, *, /, comparisons, and aggregate functions.
	Objective string `json:"objective,omitempty"`

	// Players defines the agents in the simulation and their goals.
	// Each player has a perspective on the objective (maximize or minimize).
	Players map[string]Player `json:"players,omitempty"`

	// Solver configures ODE simulation parameters.
	Solver *SolverConfig `json:"solver,omitempty"`
}

// Player represents an agent in the simulation.
type Player struct {
	// Maximizes indicates whether this player tries to maximize the objective.
	// If false, the player minimizes (opponent perspective).
	Maximizes bool `json:"maximizes"`

	// TurnPlace is the place ID that indicates it's this player's turn.
	// Used for turn-based games to determine whose move it is.
	TurnPlace string `json:"turnPlace,omitempty"`

	// Transitions lists which transitions this player can fire.
	// If empty, inferred from TurnPlace input arcs.
	Transitions []string `json:"transitions,omitempty"`
}

// SolverConfig contains ODE solver parameters.
type SolverConfig struct {
	// Tspan is the simulation time span [start, end].
	// Default: [0, 10]
	Tspan [2]float64 `json:"tspan,omitempty"`

	// Dt is the initial time step. Default: 0.01
	Dt float64 `json:"dt,omitempty"`

	// Rates maps transition IDs to firing rates.
	// Default: all transitions have rate 1.0
	Rates map[string]float64 `json:"rates,omitempty"`
}

// Schema is a complete metamodel definition.
// It defines the structure and behavior of a formal model.
type Schema struct {
	Name        string       `json:"name,omitempty"`
	Version     string       `json:"version,omitempty"`
	Description string       `json:"description,omitempty"`
	States      []State      `json:"states"`
	Actions     []Action     `json:"actions"`
	Arcs        []Arc        `json:"arcs"`
	Constraints []Constraint `json:"constraints,omitempty"`
	Views       []View       `json:"views,omitempty"`  // UI views for forms and data display
	Simulation  *Simulation  `json:"simulation,omitempty"` // ODE simulation config for AI/evaluation
}

// NewSchema creates a new empty goflowmodel.
func NewSchema(name string) *Schema {
	return &Schema{
		Name:        name,
		Version:     "1.0.0",
		States:      make([]State, 0),
		Actions:     make([]Action, 0),
		Arcs:        make([]Arc, 0),
		Constraints: make([]Constraint, 0),
		Views:       make([]View, 0),
	}
}

// AddState adds a state to the goflowmodel.
func (s *Schema) AddState(st State) *Schema {
	s.States = append(s.States, st)
	return s
}

// AddTokenState adds a token-counting state to the goflowmodel.
func (s *Schema) AddTokenState(id string, initial int) *Schema {
	return s.AddState(State{
		ID:      id,
		Kind:    TokenState,
		Initial: initial,
		Type:    "int",
	})
}

// AddDataState adds a data-holding state to the goflowmodel.
func (s *Schema) AddDataState(id string, typ string, initial any, exported bool) *Schema {
	return s.AddState(State{
		ID:       id,
		Kind:     DataState,
		Type:     typ,
		Initial:  initial,
		Exported: exported,
	})
}

// AddAction adds an action to the goflowmodel.
func (s *Schema) AddAction(a Action) *Schema {
	s.Actions = append(s.Actions, a)
	return s
}

// AddArc adds an arc to the goflowmodel.
func (s *Schema) AddArc(a Arc) *Schema {
	s.Arcs = append(s.Arcs, a)
	return s
}

// AddConstraint adds a constraint to the goflowmodel.
func (s *Schema) AddConstraint(c Constraint) *Schema {
	s.Constraints = append(s.Constraints, c)
	return s
}

// StateByID returns a state by its ID, or nil if not found.
func (s *Schema) StateByID(id string) *State {
	for i := range s.States {
		if s.States[i].ID == id {
			return &s.States[i]
		}
	}
	return nil
}

// StateIsExported returns true if the state is exported.
func (s *Schema) StateIsExported(id string) bool {
	if st := s.StateByID(id); st != nil {
		return st.Exported
	}
	return false
}

// TokenStates returns all token-counting states.
func (s *Schema) TokenStates() []State {
	var result []State
	for _, st := range s.States {
		if st.IsToken() {
			result = append(result, st)
		}
	}
	return result
}

// DataStates returns all data-holding states.
func (s *Schema) DataStates() []State {
	var result []State
	for _, st := range s.States {
		if st.IsData() {
			result = append(result, st)
		}
	}
	return result
}

// ActionByID returns an action by its ID, or nil if not found.
func (s *Schema) ActionByID(id string) *Action {
	for i := range s.Actions {
		if s.Actions[i].ID == id {
			return &s.Actions[i]
		}
	}
	return nil
}

// InputArcs returns all arcs flowing into an action.
func (s *Schema) InputArcs(actionID string) []Arc {
	var result []Arc
	for _, arc := range s.Arcs {
		if arc.Target == actionID {
			result = append(result, arc)
		}
	}
	return result
}

// OutputArcs returns all arcs flowing out of an action.
func (s *Schema) OutputArcs(actionID string) []Arc {
	var result []Arc
	for _, arc := range s.Arcs {
		if arc.Source == actionID {
			result = append(result, arc)
		}
	}
	return result
}

// ToModel converts the local metamodel.Schema to goflowmodel.Model for code generation.
func (s *Schema) ToModel() *goflowmodel.Model {
	model := &goflowmodel.Model{
		Name:        s.Name,
		Version:     s.Version,
		Description: s.Description,
	}

	// Convert states to places
	for _, state := range s.States {
		place := goflowmodel.Place{
			ID:          state.ID,
			Description: state.Description,
			Type:        state.Type,
			Exported:    state.Exported,
		}

		if state.IsToken() {
			place.Kind = goflowmodel.TokenKind
			place.Initial = state.InitialTokens()
		} else {
			place.Kind = goflowmodel.DataKind
			place.Initial = 0
		}

		model.Places = append(model.Places, place)
	}

	// Convert actions to transitions
	for _, action := range s.Actions {
		transition := goflowmodel.Transition{
			ID:          action.ID,
			Description: action.Description,
			Guard:       action.Guard,
			Bindings:    mapToBindings(action.Bindings),
		}
		model.Transitions = append(model.Transitions, transition)
	}

	// Convert arcs
	for _, arc := range s.Arcs {
		modelArc := goflowmodel.Arc{
			From:   arc.Source,
			To:     arc.Target,
			Weight: arc.Weight,
			Keys:   arc.Keys,
			Value:  arc.Value,
			Type:   goflowmodel.ArcType(arc.Type),
		}
		if modelArc.Weight == 0 {
			modelArc.Weight = 1
		}
		model.Arcs = append(model.Arcs, modelArc)
	}

	// Convert constraints
	for _, constraint := range s.Constraints {
		model.Constraints = append(model.Constraints, goflowmodel.Constraint{
			ID:   constraint.ID,
			Expr: constraint.Expr,
		})
	}

	return model
}

// mapToBindings converts map[string]string to []goflowmodel.Binding.
func mapToBindings(m map[string]string) []goflowmodel.Binding {
	if len(m) == 0 {
		return nil
	}
	var result []goflowmodel.Binding
	for name, typ := range m {
		result = append(result, goflowmodel.Binding{
			Name: name,
			Type: typ,
		})
	}
	return result
}

package metamodel

import (
	"errors"
	"fmt"
)

// Errors for metamodel operations.
var (
	// Schema validation errors
	ErrEmptyID              = errors.New("metamodel: element has empty ID")
	ErrDuplicateID          = errors.New("metamodel: duplicate element ID")
	ErrInvalidArcSource     = errors.New("metamodel: arc source not found")
	ErrInvalidArcTarget     = errors.New("metamodel: arc target not found")
	ErrInvalidArcConnection = errors.New("metamodel: arcs must connect states to actions")

	// Execution errors
	ErrActionNotFound     = errors.New("metamodel: action not found")
	ErrInsufficientTokens = errors.New("metamodel: insufficient tokens to execute")
	ErrGuardNotSatisfied  = errors.New("metamodel: action guard not satisfied")
	ErrGuardEvaluation    = errors.New("metamodel: guard evaluation error")
	ErrActionNotEnabled   = errors.New("metamodel: action not enabled")

	// Constraint errors
	ErrConstraintViolated   = errors.New("metamodel: constraint violated")
	ErrConstraintEvaluation = errors.New("metamodel: constraint evaluation error")
)

// ConstraintViolation describes a failed constraint check.
type ConstraintViolation struct {
	Constraint Constraint
	Snapshot   *Snapshot
	Err        error // nil if constraint evaluated to false; non-nil if evaluation failed
}

// Snapshot represents the current state of all states in a schema.
// It separates token counts (Petri net semantics) from data values (structured state).
type Snapshot struct {
	// Tokens holds integer counts for TokenState places.
	// Key: state ID, Value: token count
	// Firing semantics: input arcs decrement, output arcs increment.
	Tokens map[string]int `json:"tokens,omitempty"`

	// Data holds typed values for DataState places.
	// Key: state ID, Value: typed data (maps, structs, etc.)
	// Firing semantics: arcs specify keys for map transformation.
	Data map[string]any `json:"data,omitempty"`
}

// NewSnapshot creates an empty snapshot.
func NewSnapshot() *Snapshot {
	return &Snapshot{
		Tokens: make(map[string]int),
		Data:   make(map[string]any),
	}
}

// NewSnapshotFromSchema creates a snapshot initialized from schema defaults.
func NewSnapshotFromSchema(s *Schema) *Snapshot {
	snap := NewSnapshot()
	for _, st := range s.States {
		if st.IsToken() {
			snap.Tokens[st.ID] = st.InitialTokens()
		} else {
			if st.Initial != nil {
				snap.Data[st.ID] = st.Initial
			} else if st.IsMapType() {
				// Initialize empty map for map types
				snap.Data[st.ID] = make(map[string]any)
			} else {
				// For simple types, use zero value
				snap.Data[st.ID] = zeroValue(st.Type)
			}
		}
	}
	return snap
}

// zeroValue returns the zero value for a type.
func zeroValue(typ string) any {
	switch typ {
	case "string":
		return ""
	case "int", "int64":
		return int64(0)
	case "float64":
		return float64(0)
	case "bool":
		return false
	default:
		return nil
	}
}

// Clone creates a deep copy of the snapshot.
func (s *Snapshot) Clone() *Snapshot {
	clone := NewSnapshot()

	for k, v := range s.Tokens {
		clone.Tokens[k] = v
	}

	for k, v := range s.Data {
		// Deep copy maps
		if m, ok := v.(map[string]any); ok {
			clonedMap := make(map[string]any, len(m))
			for mk, mv := range m {
				// Handle nested maps
				if nested, ok := mv.(map[string]any); ok {
					clonedNested := make(map[string]any, len(nested))
					for nk, nv := range nested {
						clonedNested[nk] = nv
					}
					clonedMap[mk] = clonedNested
				} else {
					clonedMap[mk] = mv
				}
			}
			clone.Data[k] = clonedMap
		} else if m, ok := v.(map[string]int64); ok {
			clonedMap := make(map[string]int64, len(m))
			for mk, mv := range m {
				clonedMap[mk] = mv
			}
			clone.Data[k] = clonedMap
		} else if m, ok := v.(map[string]map[string]int64); ok {
			clonedMap := make(map[string]map[string]int64, len(m))
			for mk, mv := range m {
				clonedNested := make(map[string]int64, len(mv))
				for nk, nv := range mv {
					clonedNested[nk] = nv
				}
				clonedMap[mk] = clonedNested
			}
			clone.Data[k] = clonedMap
		} else {
			clone.Data[k] = v
		}
	}

	return clone
}

// GetTokens returns the token count for a TokenState.
func (s *Snapshot) GetTokens(stateID string) int {
	return s.Tokens[stateID]
}

// SetTokens sets the token count for a TokenState.
func (s *Snapshot) SetTokens(stateID string, count int) {
	s.Tokens[stateID] = count
}

// AddTokens adds to the token count for a TokenState.
func (s *Snapshot) AddTokens(stateID string, delta int) {
	s.Tokens[stateID] += delta
}

// GetData returns the data value for a DataState.
func (s *Snapshot) GetData(stateID string) any {
	return s.Data[stateID]
}

// SetData sets the data value for a DataState.
func (s *Snapshot) SetData(stateID string, value any) {
	s.Data[stateID] = value
}

// GetDataMap returns the data value as a map, or nil if not a map.
func (s *Snapshot) GetDataMap(stateID string) map[string]any {
	if v, ok := s.Data[stateID].(map[string]any); ok {
		return v
	}
	return nil
}

// GetDataMapValue returns a value from a DataState map.
func (s *Snapshot) GetDataMapValue(stateID, key string) any {
	if m := s.GetDataMap(stateID); m != nil {
		return m[key]
	}
	return nil
}

// SetDataMapValue sets a value in a DataState map.
func (s *Snapshot) SetDataMapValue(stateID, key string, value any) {
	m := s.GetDataMap(stateID)
	if m == nil {
		m = make(map[string]any)
		s.Data[stateID] = m
	}
	m[key] = value
}

// Bindings holds variable bindings for parameterized action execution.
// Contains function parameters (from, to, amount, owner, spender, etc.)
type Bindings map[string]any

// Clone creates a deep copy of the bindings.
func (b Bindings) Clone() Bindings {
	clone := make(Bindings, len(b))
	for k, v := range b {
		clone[k] = v
	}
	return clone
}

// Get retrieves a value from the bindings, returning nil if not found.
func (b Bindings) Get(key string) any {
	return b[key]
}

// GetString returns the value as a string, or empty string if not found.
func (b Bindings) GetString(key string) string {
	if v, ok := b[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetInt returns the value as an int, defaulting to 0 if not found.
func (b Bindings) GetInt(key string) int {
	if v, ok := b[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		}
	}
	return 0
}

// GetInt64 returns the value as an int64, defaulting to 0 if not found.
func (b Bindings) GetInt64(key string) int64 {
	if v, ok := b[key]; ok {
		switch n := v.(type) {
		case int:
			return int64(n)
		case int64:
			return n
		case float64:
			return int64(n)
		case string:
			// Support string amounts for large numbers
			var result int64
			for _, c := range n {
				if c >= '0' && c <= '9' {
					result = result*10 + int64(c-'0')
				}
			}
			return result
		}
	}
	return 0
}

// GetBool returns the value as a bool, defaulting to false if not found.
func (b Bindings) GetBool(key string) bool {
	if v, ok := b[key]; ok {
		if bv, ok := v.(bool); ok {
			return bv
		}
	}
	return false
}

// GuardFunc is a function that can be called from guard expressions.
type GuardFunc func(args ...any) (any, error)

// GuardEvaluator evaluates guard expressions.
// This interface allows the metamodel to be independent of the guard DSL implementation.
type GuardEvaluator interface {
	// Evaluate evaluates a guard expression with bindings and returns true if satisfied.
	Evaluate(expr string, bindings Bindings, funcs map[string]GuardFunc) (bool, error)
	// EvaluateConstraint evaluates a constraint expression against token counts.
	EvaluateConstraint(expr string, tokens map[string]int) (bool, error)
}

// Runtime holds the execution state of a schema.
type Runtime struct {
	Schema           *Schema
	Snapshot         *Snapshot
	Sequence         uint64
	CheckConstraints bool           // If true, check constraints after each Execute (default: true)
	GuardEvaluator   GuardEvaluator // Optional guard evaluator; nil disables guard checking
}

// NewRuntime creates a new execution runtime from a schema.
func NewRuntime(s *Schema) *Runtime {
	return &Runtime{
		Schema:           s,
		Snapshot:         NewSnapshotFromSchema(s),
		Sequence:         0,
		CheckConstraints: true, // Auto-check by default
	}
}

// Clone creates a deep copy of the runtime.
func (r *Runtime) Clone() *Runtime {
	return &Runtime{
		Schema:           r.Schema,
		Snapshot:         r.Snapshot.Clone(),
		Sequence:         r.Sequence,
		CheckConstraints: r.CheckConstraints,
		GuardEvaluator:   r.GuardEvaluator,
	}
}

// Tokens returns the token count at a TokenState.
func (r *Runtime) Tokens(stateID string) int {
	return r.Snapshot.GetTokens(stateID)
}

// SetTokens sets the token count at a TokenState.
func (r *Runtime) SetTokens(stateID string, count int) {
	r.Snapshot.SetTokens(stateID, count)
}

// Data returns the data value at a DataState.
func (r *Runtime) Data(stateID string) any {
	return r.Snapshot.GetData(stateID)
}

// SetData sets the data value at a DataState.
func (r *Runtime) SetData(stateID string, value any) {
	r.Snapshot.SetData(stateID, value)
}

// DataMap returns the data value as a map.
func (r *Runtime) DataMap(stateID string) map[string]any {
	return r.Snapshot.GetDataMap(stateID)
}

// Enabled returns true if an action can execute.
// For TokenState inputs: checks token count >= weight (default 1)
// For DataState inputs: always enabled (data transformation doesn't consume)
func (r *Runtime) Enabled(actionID string) bool {
	a := r.Schema.ActionByID(actionID)
	if a == nil {
		return false
	}

	// Check all input arcs from TokenStates have sufficient tokens
	for _, arc := range r.Schema.InputArcs(actionID) {
		st := r.Schema.StateByID(arc.Source)
		if st != nil && st.IsToken() {
			weight := arc.Weight
			if weight == 0 {
				weight = 1
			}
			if r.Tokens(arc.Source) < weight {
				return false
			}
		}
	}

	return true
}

// EnabledActions returns all actions that can execute.
func (r *Runtime) EnabledActions() []string {
	var enabled []string
	for _, a := range r.Schema.Actions {
		if r.Enabled(a.ID) {
			enabled = append(enabled, a.ID)
		}
	}
	return enabled
}

// Execute runs an action.
// For TokenStates: consumes/produces tokens (Petri net semantics)
// For DataStates: no automatic transformation (use ExecuteWithBindings for data)
func (r *Runtime) Execute(actionID string) error {
	if !r.Enabled(actionID) {
		return ErrActionNotEnabled
	}

	// Process input arcs
	for _, arc := range r.Schema.InputArcs(actionID) {
		st := r.Schema.StateByID(arc.Source)
		if st != nil && st.IsToken() {
			weight := arc.Weight
			if weight == 0 {
				weight = 1
			}
			r.Snapshot.AddTokens(arc.Source, -weight)
		}
	}

	// Process output arcs
	for _, arc := range r.Schema.OutputArcs(actionID) {
		st := r.Schema.StateByID(arc.Target)
		if st != nil && st.IsToken() {
			weight := arc.Weight
			if weight == 0 {
				weight = 1
			}
			r.Snapshot.AddTokens(arc.Target, weight)
		}
	}

	r.Sequence++

	// Check constraints if enabled
	if r.CheckConstraints {
		if violations := r.Constraints(); len(violations) > 0 {
			v := violations[0]
			if v.Err != nil {
				return fmt.Errorf("%w: %s: %v", ErrConstraintEvaluation, v.Constraint.ID, v.Err)
			}
			return fmt.Errorf("%w: %s", ErrConstraintViolated, v.Constraint.ID)
		}
	}

	return nil
}

// ExecuteWithBindings runs an action with variable bindings.
// This applies data transformations based on arc Keys and Value specifications.
func (r *Runtime) ExecuteWithBindings(actionID string, bindings Bindings) error {
	a := r.Schema.ActionByID(actionID)
	if a == nil {
		return ErrActionNotFound
	}

	// Evaluate guard if present and evaluator is set
	if a.Guard != "" && r.GuardEvaluator != nil {
		ok, err := r.GuardEvaluator.Evaluate(a.Guard, bindings, nil)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrGuardEvaluation, err)
		}
		if !ok {
			return ErrGuardNotSatisfied
		}
	}

	// Check enablement
	if !r.Enabled(actionID) {
		return ErrActionNotEnabled
	}

	// Apply arc transformations
	r.applyArcs(actionID, bindings)

	r.Sequence++

	// Check constraints if enabled
	if r.CheckConstraints {
		if violations := r.Constraints(); len(violations) > 0 {
			v := violations[0]
			if v.Err != nil {
				return fmt.Errorf("%w: %s: %v", ErrConstraintEvaluation, v.Constraint.ID, v.Err)
			}
			return fmt.Errorf("%w: %s", ErrConstraintViolated, v.Constraint.ID)
		}
	}

	return nil
}

// applyArcs processes input and output arcs for an action.
func (r *Runtime) applyArcs(actionID string, bindings Bindings) {
	// Process input arcs (consume from source states)
	for _, arc := range r.Schema.InputArcs(actionID) {
		st := r.Schema.StateByID(arc.Source)
		if st == nil {
			continue
		}

		if st.IsToken() {
			weight := arc.Weight
			if weight == 0 {
				weight = 1
			}
			r.Snapshot.AddTokens(arc.Source, -weight)
		} else {
			// DataState: subtract from map using arc keys
			r.applyDataArc(arc.Source, arc, bindings, false)
		}
	}

	// Process output arcs (produce at target states)
	for _, arc := range r.Schema.OutputArcs(actionID) {
		st := r.Schema.StateByID(arc.Target)
		if st == nil {
			continue
		}

		if st.IsToken() {
			weight := arc.Weight
			if weight == 0 {
				weight = 1
			}
			r.Snapshot.AddTokens(arc.Target, weight)
		} else {
			// DataState: add to map using arc keys
			r.applyDataArc(arc.Target, arc, bindings, true)
		}
	}
}

// applyDataArc applies a data transformation to a DataState.
// For input arcs (add=false): subtracts the value
// For output arcs (add=true): adds the value or sets directly for simple types
func (r *Runtime) applyDataArc(stateID string, arc Arc, bindings Bindings, add bool) {
	st := r.Schema.StateByID(stateID)
	if st == nil {
		return
	}

	// Get the value binding name
	valueName := arc.Value
	if valueName == "" {
		valueName = "amount" // default
	}

	// Handle simple types (direct assignment from binding)
	if st.IsSimpleType() {
		if add {
			// Output arc: set the value from binding
			r.Snapshot.SetData(stateID, bindings.Get(valueName))
		}
		// Input arcs on simple types are read-only (no state change)
		return
	}

	// Handle map types
	amount := bindings.GetInt64(valueName)

	// Get or create the data map
	dataMap := r.Snapshot.GetDataMap(stateID)
	if dataMap == nil {
		dataMap = make(map[string]any)
		r.Snapshot.SetData(stateID, dataMap)
	}

	// Build the key from arc.Keys and bindings
	if len(arc.Keys) == 0 {
		return // No key specified, nothing to do
	}

	// Single key: direct map access
	if len(arc.Keys) == 1 {
		key := bindings.GetString(arc.Keys[0])
		if key == "" {
			return
		}

		current := getMapInt64(dataMap, key)
		if add {
			dataMap[key] = current + amount
		} else {
			dataMap[key] = current - amount
		}
		return
	}

	// Multiple keys: nested map access (e.g., allowances[owner][spender])
	if len(arc.Keys) == 2 {
		key1 := bindings.GetString(arc.Keys[0])
		key2 := bindings.GetString(arc.Keys[1])
		if key1 == "" || key2 == "" {
			return
		}

		// Get or create nested map
		nested, ok := dataMap[key1].(map[string]any)
		if !ok {
			nested = make(map[string]any)
			dataMap[key1] = nested
		}

		current := getMapInt64(nested, key2)
		if add {
			nested[key2] = current + amount
		} else {
			nested[key2] = current - amount
		}
	}
}

// getMapInt64 extracts an int64 value from a map, handling various numeric types.
func getMapInt64(m map[string]any, key string) int64 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case int:
		return int64(n)
	case int64:
		return n
	case float64:
		return int64(n)
	case string:
		var result int64
		for _, c := range n {
			if c >= '0' && c <= '9' {
				result = result*10 + int64(c-'0')
			}
		}
		return result
	default:
		return 0
	}
}

// ExecuteWithGuardFuncs runs an action with bindings and custom guard functions.
func (r *Runtime) ExecuteWithGuardFuncs(actionID string, bindings Bindings, funcs map[string]GuardFunc) error {
	a := r.Schema.ActionByID(actionID)
	if a == nil {
		return ErrActionNotFound
	}

	// Evaluate guard if present and evaluator is set
	if a.Guard != "" && r.GuardEvaluator != nil {
		ok, err := r.GuardEvaluator.Evaluate(a.Guard, bindings, funcs)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrGuardEvaluation, err)
		}
		if !ok {
			return ErrGuardNotSatisfied
		}
	}

	// Check enablement
	if !r.Enabled(actionID) {
		return ErrActionNotEnabled
	}

	// Apply arc transformations
	r.applyArcs(actionID, bindings)

	r.Sequence++

	// Check constraints if enabled
	if r.CheckConstraints {
		if violations := r.Constraints(); len(violations) > 0 {
			v := violations[0]
			if v.Err != nil {
				return fmt.Errorf("%w: %s: %v", ErrConstraintEvaluation, v.Constraint.ID, v.Err)
			}
			return fmt.Errorf("%w: %s", ErrConstraintViolated, v.Constraint.ID)
		}
	}

	return nil
}

// Constraints checks all schema constraints against the current snapshot.
// Returns a slice of violations (empty if all constraints hold).
func (r *Runtime) Constraints() []ConstraintViolation {
	var violations []ConstraintViolation

	if r.GuardEvaluator == nil {
		return violations // No evaluator, no constraint checking
	}

	for _, c := range r.Schema.Constraints {
		ok, err := r.GuardEvaluator.EvaluateConstraint(c.Expr, r.Snapshot.Tokens)
		if err != nil {
			violations = append(violations, ConstraintViolation{
				Constraint: c,
				Snapshot:   r.Snapshot.Clone(),
				Err:        err,
			})
		} else if !ok {
			violations = append(violations, ConstraintViolation{
				Constraint: c,
				Snapshot:   r.Snapshot.Clone(),
				Err:        nil,
			})
		}
	}

	return violations
}

// CanReach returns true if the target token state is reachable from current state.
// This is a simple BFS; complex reachability requires more sophisticated analysis.
func (r *Runtime) CanReach(targetTokens map[string]int, maxSteps int) bool {
	visited := make(map[string]bool)
	queue := []*Runtime{r.Clone()}

	for len(queue) > 0 && maxSteps > 0 {
		current := queue[0]
		queue = queue[1:]
		maxSteps--

		key := current.tokenKey()
		if visited[key] {
			continue
		}
		visited[key] = true

		if current.matchesTokens(targetTokens) {
			return true
		}

		for _, aid := range current.EnabledActions() {
			next := current.Clone()
			next.Execute(aid)
			queue = append(queue, next)
		}
	}

	return false
}

func (r *Runtime) tokenKey() string {
	result := ""
	for _, st := range r.Schema.TokenStates() {
		result += fmt.Sprintf("%s:%d;", st.ID, r.Snapshot.Tokens[st.ID])
	}
	return result
}

func (r *Runtime) matchesTokens(target map[string]int) bool {
	for k, v := range target {
		if r.Snapshot.Tokens[k] != v {
			return false
		}
	}
	return true
}

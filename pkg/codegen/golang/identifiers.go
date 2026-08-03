package golang

import (
	"strconv"
	"unicode"
)

// identScope hands out collision-free identifier stems for element IDs.
//
// Sanitizing an ID is not enough on its own: ToPascalCase is many-to-one
// (it drops separators and folds case), so a flattened bundle model can hold
// two distinct IDs — "orders/ship_now" and "orders_ship/now", or "orders/ready"
// and "orders/Ready" — that sanitize to the same stem. Every name derived from
// a stem (ConstName, FieldName, FuncName, HandlerName, …) then collides too,
// and the collision is silent: go/format accepts a const block that declares
// the same identifier twice, so generation "succeeds" and the failure surfaces
// later as "redeclared in this block" at go build time, pointing at generated
// code instead of at the model that produced it.
//
// A scope is per-kind (one for places, one for transitions) because the derived
// names of the two kinds live in different Go scopes and carry different
// prefixes; sharing one scope would renumber models that legitimately use the
// same ID for a place and a transition.
type identScope struct {
	byID map[string]string
	used map[string]bool
}

// identScopes are the per-model scopes shared by every context builder, so an
// arc, a state field and the place they all name resolve to one identifier.
type identScopes struct {
	place      *identScope
	transition *identScope
	// event is separate from transition because an event struct name may be
	// author-supplied (Transition.EventType) rather than derived from the ID.
	event *identScope
}

func newIdentScopes() *identScopes {
	return &identScopes{place: newIdentScope(), transition: newIdentScope(), event: newIdentScope()}
}

func newIdentScope() *identScope {
	return &identScope{byID: map[string]string{}, used: map[string]bool{}}
}

// Stem returns the unique PascalCase stem for id, allocating it on first use.
// Allocation order is the model's declaration order, so the disambiguating
// suffix is stable across runs for a given model.
//
// A nil scope falls back to the raw sanitizer, so helpers that have no scope
// to hand (tests, one-off previews) keep working.
func (s *identScope) Stem(id string) string {
	base := ToPascalCase(id)
	if base == "" {
		// An ID made entirely of separators ("::", "/") sanitizes away.
		// Anything valid will do — uniquification below keeps it distinct.
		base = "Element"
	}
	return s.Unique(id, base)
}

// Unique is Stem for names that are not derived from the ID by ToPascalCase.
// An event struct name, for instance, is sanitized by IdentifierFrom, which
// preserves interior case — running it through ToPascalCase instead would
// rename every already-generated app. The allocation is still keyed by element
// ID, so one element always gets one name.
func (s *identScope) Unique(id, base string) string {
	if s == nil {
		return base
	}
	if name, ok := s.byID[id]; ok {
		return name
	}
	name := base
	for n := 2; s.used[name]; n++ {
		name = base + strconv.Itoa(n)
	}
	s.byID[id] = name
	s.used[name] = true
	return name
}

// lowerFirst lowercases the leading rune, turning an exported stem into its
// unexported spelling. Equivalent to ToCamelCase(ToPascalCase(id)) but applied
// to an already-allocated stem, so the disambiguating suffix is preserved.
func lowerFirst(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

// eventTypeFromStem is ToEventTypeName over an allocated stem.
func eventTypeFromStem(stem string) string {
	if stem == "" {
		return ""
	}
	if stem[len(stem)-1] == 'e' {
		return stem + "d"
	}
	return stem + "ed"
}

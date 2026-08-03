package metamodel

import "fmt"

// ValidateArcs checks each arc's type and, for read arcs, its direction.
//
// It mirrors go-pflow's metamodel.ValidateArcs, and for the same reason: an
// unknown arc type is an error rather than something to skip past, because
// every reader that does not recognise a type falls back to treating the arc
// as a normal consuming one. That turns a constraint into token theft, and
// nothing downstream can tell the difference. Failing loudly is the only way a
// schema written against a newer ArcType cannot be silently mis-executed here.
//
// The returned errors wrap ErrUnknownArcType and ErrReadArcDirection so
// callers can classify them with errors.Is.
func (s *Schema) ValidateArcs() []error {
	var out []error
	for i := range s.Arcs {
		a := &s.Arcs[i]
		label := a.Source + " -> " + a.Target

		if !IsKnownArcType(a.Type) {
			out = append(out, fmt.Errorf("%w: arc %s has type %q; this build understands %s",
				ErrUnknownArcType, label, a.Type, knownArcTypeList()))
			continue
		}

		// A read arc tests a state's marking, so only state -> action carries
		// meaning; the reverse would be testing an action, which holds no
		// tokens.
		//
		// Only read arcs are checked. A reversed INHIBITOR is pflow-xyz's
		// long-standing spelling for a guard (pkg/codegen/core reads an
		// action -> state inhibitor as a read), so rejecting it here would
		// invalidate models that have always been accepted.
		if a.IsRead() {
			if s.StateByID(a.Source) == nil || s.ActionByID(a.Target) == nil {
				out = append(out, fmt.Errorf("%w: read arc %s must run state -> action",
					ErrReadArcDirection, label))
			}
		}
	}
	return out
}

// knownArcTypeList renders the known types for an error message, in a fixed
// order so the message is reproducible.
func knownArcTypeList() string {
	return `"" (normal), "inhibitor", "read"`
}

package golang

import "sort"

// A composed application has two kinds of firing rule. Most transitions are
// entirely local: everything the rule reads lives in one entity's own marking,
// so that entity's aggregate — which replays only its own log — is a complete
// authority. The rest are not, and this file names them.
//
// A transition is a CROSS-ENTITY COMMAND when the composition gave it a rule
// its owning entity cannot evaluate:
//
//   - an EventLink fused it with transitions in other entities, so firing it
//     must append to several logs at once; or
//   - a GuardLink gave it a guard over places in other entities, so deciding
//     whether it may fire requires reading a marking the entity cannot see.
//
// Either way the entity API must refuse it. The alternative — letting the
// entity fire what it can see and hoping the rest holds — is exactly the bug
// this type exists to close: warehouse's order.ship carried a guard on
// inventory.reserved that the order aggregate never consulted, so shipping
// unreserved stock succeeded.
//
// The refusal is total: CanFire says false, EnabledTransitions omits it, Fire
// and Execute return an error naming the command. Reporting a transition
// enabled and then refusing it is the Enabled/Execute divergence we just
// closed elsewhere, and re-opening it here would be worse — the divergence
// would be invisible to a single-entity test.

// CrossEntityTransition marks one of a model's transitions as owned by a
// composition-root command. It is set only in bundle mode; a standalone app
// generates with an empty list and is byte-identical to before this existed.
type CrossEntityTransition struct {
	// TransitionID is the transition's ID inside this entity's own model.
	TransitionID string

	// Command is the transition's ID in the flattened model — the name of the
	// root-package method and HTTP route that may fire it.
	Command string

	// Reason is why the composition took it over: "fused" (an EventLink joined
	// it to transitions in other entities) or "guarded" (a GuardLink gave it a
	// precondition over another entity's places).
	Reason string
}

// CrossEntityContext is the template-facing form: the same facts plus the Go
// constant the entity package declares for the transition.
type CrossEntityContext struct {
	TransitionID string
	ConstName    string
	Command      string
	Reason       string
}

// buildCrossEntityContexts resolves each cross-entity transition to the
// constant the entity package declares for it, and sorts by transition ID so
// the generated map literal does not depend on link ordering in the bundle.
//
// A transition the caller names but the model does not declare is dropped
// rather than emitted: it would generate a reference to a constant that does
// not exist, turning a bundle-wiring mistake into a compile error in the
// generated tree instead of at the point that made it.
func buildCrossEntityContexts(xs []CrossEntityTransition, transitions []TransitionContext) []CrossEntityContext {
	if len(xs) == 0 {
		return nil
	}
	constByID := make(map[string]string, len(transitions))
	for _, t := range transitions {
		constByID[t.ID] = t.ConstName
	}
	out := make([]CrossEntityContext, 0, len(xs))
	for _, x := range xs {
		constName, ok := constByID[x.TransitionID]
		if !ok {
			continue
		}
		out = append(out, CrossEntityContext{
			TransitionID: x.TransitionID,
			ConstName:    constName,
			Command:      x.Command,
			Reason:       x.Reason,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TransitionID < out[j].TransitionID })
	return out
}

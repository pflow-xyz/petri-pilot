package bundle

import (
	"fmt"
	"sort"

	metamodel "github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/petri-pilot/pkg/extensions"
)

// ActionRef names one entity's action.
type ActionRef struct {
	Entity string `json:"entity"`
	Action string `json:"action"`
}

// Fusion declares a cross-entity rendezvous: the named actions fire together
// as one atomic transition. Compiled to a chain of EventLinks (fusion is by
// equivalence class upstream, so the chain and the clique are the same
// thing).
type Fusion struct {
	ID      string      `json:"id"`
	Members []ActionRef `json:"members"`
}

// Reference is a cross-entity foreign key surviving compilation as
// metadata. It is NOT compiled to a DataLink today: the referring entity
// writes its FK field, and upstream DataLink fusion requires the observer to
// carry no non-inhibitor arcs on the fused place (E_DATALINK_CONSUMES) —
// the correct primitive is a read arc, which metamodel.Arc does not have
// yet. Until then, referential integrity and OnDelete behavior are the
// generated application's job, driven by this record.
type Reference struct {
	FromEntity string
	FromField  string
	ToEntity   string
	ToField    string // empty means the target's identity
	OnDelete   string // cascade, restrict, set_null
}

// ApplicationInput is what the application compiler consumes: the
// petri_application entity spec plus explicit cross-entity fusions.
type ApplicationInput struct {
	Name     string
	Entities []extensions.Entity
	Fusions  []Fusion
}

// Compiled is the compiler's output: the Bundle, plus everything the
// application layer needs that the Bundle cannot carry.
type Compiled struct {
	Bundle     *metamodel.Bundle
	References []Reference
}

// CompileApplication compiles an entity spec into a validated Bundle:
// one subnet per entity (via extensions.EntityToModel — the canonical
// Entity→Model conversion), EventLinks for declared fusions, and reference
// metadata for foreign keys. Transition event IDs are prefixed
// "<entity>.<action>" so that upstream event merging can never fuse two
// entities' same-named events, and so a fused transition's Emits list is
// unambiguous about which entity each event belongs to.
func CompileApplication(app ApplicationInput) (*Compiled, error) {
	if app.Name == "" {
		return nil, fmt.Errorf("application name is required")
	}
	if len(app.Entities) == 0 {
		return nil, fmt.Errorf("at least one entity is required")
	}

	b := metamodel.NewBundle(app.Name)
	entityByID := map[string]*extensions.Entity{}

	for i := range app.Entities {
		e := &app.Entities[i]
		if _, dup := entityByID[e.ID]; dup {
			return nil, fmt.Errorf("duplicate entity %q", e.ID)
		}
		entityByID[e.ID] = e

		model := extensions.EntityToModel(*e)
		// Prefix event identity per entity. EntityToModel leaves
		// Transition.Event empty, in which case fusion falls back to the
		// bare transition ID — which collides across entities ("create"
		// twice). Naming it here makes every event globally attributable.
		for ti := range model.Transitions {
			if model.Transitions[ti].Event == "" {
				model.Transitions[ti].Event = e.ID + "." + model.Transitions[ti].ID
			}
		}
		b.AddSubnet(metamodel.Subnet{
			ID: e.ID,
			// Entity lifecycles are workflow-shaped: token places are
			// states, transitions are commands. WorkflowNet permits
			// EventLinks and forbids TokenLinks — exactly the composition
			// surface entities should have.
			NetType: metamodel.WorkflowNet,
			Model:   model,
		})
	}

	// Fusions → EventLink chains.
	for _, fusion := range app.Fusions {
		if len(fusion.Members) < 2 {
			return nil, fmt.Errorf("fusion %q: needs at least two members", fusion.ID)
		}
		for _, m := range fusion.Members {
			entity, ok := entityByID[m.Entity]
			if !ok {
				return nil, fmt.Errorf("fusion %q: unknown entity %q", fusion.ID, m.Entity)
			}
			if !entityHasAction(entity, m.Action) {
				return nil, fmt.Errorf("fusion %q: entity %q has no action %q", fusion.ID, m.Entity, m.Action)
			}
		}
		for i := 0; i+1 < len(fusion.Members); i++ {
			linkID := fusion.ID
			if len(fusion.Members) > 2 {
				linkID = fmt.Sprintf("%s_%d", fusion.ID, i)
			}
			b.AddLink(metamodel.Link{
				ID:   linkID,
				Kind: metamodel.EventLink,
				From: metamodel.Endpoint{Subnet: fusion.Members[i].Entity, Transition: fusion.Members[i].Action},
				To:   metamodel.Endpoint{Subnet: fusion.Members[i+1].Entity, Transition: fusion.Members[i+1].Action},
			})
		}
	}

	// Foreign keys → validated reference metadata.
	var refs []Reference
	for _, e := range app.Entities {
		for _, f := range e.Fields {
			if f.Reference == nil {
				continue
			}
			target, ok := entityByID[f.Reference.Entity]
			if !ok {
				return nil, fmt.Errorf("entity %q field %q: references unknown entity %q", e.ID, f.ID, f.Reference.Entity)
			}
			if f.Reference.Field != "" && !entityHasField(target, f.Reference.Field) {
				return nil, fmt.Errorf("entity %q field %q: target entity %q has no field %q", e.ID, f.ID, target.ID, f.Reference.Field)
			}
			switch f.Reference.OnDelete {
			case "", "cascade", "restrict", "set_null":
			default:
				return nil, fmt.Errorf("entity %q field %q: unknown on_delete %q", e.ID, f.ID, f.Reference.OnDelete)
			}
			refs = append(refs, Reference{
				FromEntity: e.ID,
				FromField:  f.ID,
				ToEntity:   f.Reference.Entity,
				ToField:    f.Reference.Field,
				OnDelete:   f.Reference.OnDelete,
			})
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].FromEntity != refs[j].FromEntity {
			return refs[i].FromEntity < refs[j].FromEntity
		}
		return refs[i].FromField < refs[j].FromField
	})

	if err := b.MustValidate(); err != nil {
		return nil, fmt.Errorf("compiled bundle: %w", err)
	}
	return &Compiled{Bundle: b, References: refs}, nil
}

func entityHasAction(e *extensions.Entity, id string) bool {
	for _, a := range e.Actions {
		if a.ID == id {
			return true
		}
	}
	return false
}

func entityHasField(e *extensions.Entity, id string) bool {
	for _, f := range e.Fields {
		if f.ID == id {
			return true
		}
	}
	return false
}

package bundle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	metamodel "github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/petri-pilot/pkg/extensions"
)

// shopEntities is the two-entity fixture the plan names: orders and
// inventory, with a fused place_order/reserve_stock rendezvous and an FK
// from order.item_id to inventory.
func shopEntities() ApplicationInput {
	return ApplicationInput{
		Name: "shop",
		Entities: []extensions.Entity{
			{
				ID: "order",
				Fields: []extensions.Field{
					{ID: "customer", Type: extensions.FieldTypeString},
					{ID: "item_id", Type: extensions.FieldTypeReference,
						Reference: &extensions.FieldReference{Entity: "inventory", OnDelete: "restrict"}},
				},
				States: []extensions.EntityState{
					{ID: "draft", Initial: true}, {ID: "placed"}, {ID: "shipped", Terminal: true},
				},
				Actions: []extensions.EntityAction{
					{ID: "place_order", FromStates: []string{"draft"}, ToState: "placed"},
					{ID: "ship", FromStates: []string{"placed"}, ToState: "shipped"},
				},
			},
			{
				ID: "inventory",
				Fields: []extensions.Field{
					{ID: "stock", Type: extensions.FieldTypeInt64},
				},
				States: []extensions.EntityState{
					{ID: "available", Initial: true}, {ID: "reserved"},
				},
				Actions: []extensions.EntityAction{
					{ID: "reserve_stock", FromStates: []string{"available"}, ToState: "reserved"},
					{ID: "release", FromStates: []string{"reserved"}, ToState: "available"},
				},
			},
		},
		Fusions: []Fusion{
			{ID: "order_reserves_stock", Members: []ActionRef{
				{Entity: "order", Action: "place_order"},
				{Entity: "inventory", Action: "reserve_stock"},
			}},
		},
	}
}

func TestCompileApplicationShape(t *testing.T) {
	compiled, err := CompileApplication(shopEntities())
	if err != nil {
		t.Fatal(err)
	}
	b := compiled.Bundle

	if len(b.Subnets) != 2 {
		t.Fatalf("subnets = %d, want 2", len(b.Subnets))
	}
	for _, sn := range b.Subnets {
		if sn.NetType != metamodel.WorkflowNet {
			t.Errorf("subnet %s: net type %q, want workflow", sn.ID, sn.NetType)
		}
		for _, tr := range sn.Model.Transitions {
			want := sn.ID + "." + tr.ID
			if tr.Event != want {
				t.Errorf("subnet %s transition %s: event %q, want %q", sn.ID, tr.ID, tr.Event, want)
			}
		}
	}
	if len(b.Links) != 1 || b.Links[0].Kind != metamodel.EventLink {
		t.Fatalf("links = %+v, want one EventLink", b.Links)
	}

	if len(compiled.References) != 1 {
		t.Fatalf("references = %+v, want one", compiled.References)
	}
	ref := compiled.References[0]
	if ref.FromEntity != "order" || ref.FromField != "item_id" || ref.ToEntity != "inventory" || ref.OnDelete != "restrict" {
		t.Errorf("reference = %+v", ref)
	}
}

// The compiled bundle must flatten, and the FlattenMap must carry what
// per-entity codegen needs: total per-subnet maps, the fused-transition
// group, and one event per member on the fused transition.
func TestCompiledBundleFlattens(t *testing.T) {
	compiled, err := CompileApplication(shopEntities())
	if err != nil {
		t.Fatal(err)
	}
	flat, fm, err := compiled.Bundle.FlattenWithMap()
	if err != nil {
		t.Fatal(err)
	}

	for _, sid := range []string{"order", "inventory"} {
		if len(fm.Transition[sid]) == 0 || len(fm.Place[sid]) == 0 {
			t.Fatalf("FlattenMap missing subnet %q", sid)
		}
	}

	fusedFlat := fm.Transition["order"]["place_order"]
	if fusedFlat != fm.Transition["inventory"]["reserve_stock"] {
		t.Fatalf("place_order (%s) and reserve_stock (%s) did not fuse",
			fusedFlat, fm.Transition["inventory"]["reserve_stock"])
	}
	members := fm.FusedGroups[fusedFlat]
	if len(members) != 2 {
		t.Fatalf("fused group = %v, want 2 members", members)
	}
	events := fm.MemberEvents[fusedFlat]
	if len(events) != 2 {
		t.Fatalf("member events = %v, want 2", events)
	}
	joined := strings.Join(events, ",")
	for _, want := range []string{"order.place_order", "inventory.reserve_stock"} {
		if !strings.Contains(joined, want) {
			t.Errorf("member events %v missing %q", events, want)
		}
	}

	// Every flat transition resolves in the flat model.
	flatTransitions := map[string]bool{}
	for _, tr := range flat.Transitions {
		flatTransitions[tr.ID] = true
	}
	for _, sid := range []string{"order", "inventory"} {
		for local, flatID := range fm.Transition[sid] {
			if !flatTransitions[flatID] {
				t.Errorf("%s/%s maps to %q, absent from flat model", sid, local, flatID)
			}
		}
	}
}

func TestCompileErrors(t *testing.T) {
	app := shopEntities()
	app.Fusions[0].Members[1].Action = "nope"
	if _, err := CompileApplication(app); err == nil || !strings.Contains(err.Error(), "no action") {
		t.Errorf("unknown fusion action: %v", err)
	}

	app = shopEntities()
	app.Entities[0].Fields[1].Reference.Entity = "ghost"
	if _, err := CompileApplication(app); err == nil || !strings.Contains(err.Error(), "unknown entity") {
		t.Errorf("unknown reference target: %v", err)
	}

	app = shopEntities()
	app.Entities[0].Fields[1].Reference.OnDelete = "explode"
	if _, err := CompileApplication(app); err == nil || !strings.Contains(err.Error(), "on_delete") {
		t.Errorf("bad on_delete: %v", err)
	}
}

func TestLoadFileWithModelRef(t *testing.T) {
	dir := t.TempDir()

	model := &metamodel.Model{
		Name:        "gate",
		Places:      []metamodel.Place{{ID: "open", Initial: 1}},
		Transitions: []metamodel.Transition{{ID: "shut"}},
		Arcs:        []metamodel.Arc{{From: "open", To: "shut"}},
	}
	raw, _ := json.Marshal(model)
	if err := os.WriteFile(filepath.Join(dir, "gate.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	doc := `{"name": "solo", "subnets": [{"id": "gate", "model_ref": "gate.json"}]}`
	docPath := filepath.Join(dir, "solo.bundle.json")
	if err := os.WriteFile(docPath, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}

	b, err := LoadFile(docPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Subnets) != 1 || b.Subnets[0].Model.Name != "gate" {
		t.Fatalf("loaded bundle = %+v", b)
	}

	// Identity case: one subnet, no links still flattens (unprefixed IDs).
	flat, fm, err := b.FlattenWithMap()
	if err != nil {
		t.Fatal(err)
	}
	if fm.Transition["gate"]["shut"] != "shut" {
		t.Errorf("identity flatten renamed shut to %q", fm.Transition["gate"]["shut"])
	}
	if len(flat.Transitions) != 1 {
		t.Errorf("flat transitions = %d", len(flat.Transitions))
	}
}

func TestLoadRejectsBadDocs(t *testing.T) {
	cases := map[string]string{
		"no name":        `{"subnets": [{"id": "a", "model": {"name": "a"}}]}`,
		"no subnets":     `{"name": "x"}`,
		"both model+ref": `{"name": "x", "subnets": [{"id": "a", "model": {"name": "a"}, "model_ref": "a.json"}]}`,
		"neither":        `{"name": "x", "subnets": [{"id": "a"}]}`,
	}
	for label, doc := range cases {
		if _, err := Load([]byte(doc), nil); err == nil {
			t.Errorf("%s: expected error", label)
		}
	}
}

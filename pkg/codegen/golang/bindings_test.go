package golang

import (
	"strings"
	"testing"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

// erc20Model mirrors services/erc20.pflow: binding names live on data arcs
// (keys and values), not on declared Transition.Bindings.
func erc20Model() *metamodel.Model {
	return &metamodel.Model{
		Name: "erc20",
		Places: []metamodel.Place{
			{ID: "balances", Kind: metamodel.DataKind, Type: "map[string]int64", Exported: true},
			{ID: "allowances", Kind: metamodel.DataKind, Type: "map[string]map[string]int64", Exported: true},
			{ID: "total_supply", Kind: metamodel.DataKind, Type: "int64", Initial: 0, Exported: true},
		},
		Transitions: []metamodel.Transition{
			{ID: "transfer", Guard: "balances[from] >= amount && amount > 0"},
			{ID: "approve", Guard: "amount >= 0"},
			{ID: "mint", Guard: "amount > 0"},
		},
		Arcs: []metamodel.Arc{
			{From: "balances", To: "transfer", Keys: []string{"from"}, Value: "amount"},
			{From: "transfer", To: "balances", Keys: []string{"to"}, Value: "amount"},
			{From: "allowances", To: "approve", Keys: []string{"owner", "spender"}},
			{From: "approve", To: "allowances", Keys: []string{"owner", "spender"}, Value: "amount"},
			{From: "mint", To: "balances", Keys: []string{"to"}, Value: "amount"},
			{From: "mint", To: "total_supply", Value: "amount"},
		},
	}
}

// TestBindingFieldsDerivedFromModel is the regression for the Bindings
// struct previously being hardcoded to the ERC-20 vocabulary in
// aggregate.tmpl: the fields must come from the model — arc keys as strings,
// arc values as numerics — and nothing else (no phantom "caller").
func TestBindingFieldsDerivedFromModel(t *testing.T) {
	ctx, err := NewContext(erc20Model(), ContextOptions{PackageName: "erc20"})
	if err != nil {
		t.Fatal(err)
	}

	got := map[string]BindingFieldContext{}
	var order []string
	for _, f := range ctx.BindingFields {
		got[f.Name] = f
		order = append(order, f.Name)
	}

	want := map[string]bool{ // name -> numeric
		"amount": true, "from": false, "to": false, "owner": false, "spender": false,
	}
	if len(got) != len(want) {
		t.Fatalf("fields = %v, want exactly %d of %v", order, len(want), want)
	}
	for name, numeric := range want {
		f, ok := got[name]
		if !ok {
			t.Errorf("missing binding field %q", name)
			continue
		}
		if f.IsNumeric != numeric {
			t.Errorf("%s: IsNumeric = %v, want %v", name, f.IsNumeric, numeric)
		}
		wantType := "string"
		if numeric {
			wantType = "U256JSON"
		}
		if f.GoType != wantType {
			t.Errorf("%s: GoType = %q, want %q", name, f.GoType, wantType)
		}
		if f.FieldName != ToPascalCase(name) {
			t.Errorf("%s: FieldName = %q", name, f.FieldName)
		}
	}
	if _, hasCaller := got["caller"]; hasCaller {
		t.Error("phantom 'caller' field derived from nowhere")
	}

	// Sorted by name — deterministic output.
	for i := 1; i < len(order); i++ {
		if order[i-1] >= order[i] {
			t.Errorf("fields not sorted: %v", order)
			break
		}
	}
}

// Declared Transition.Bindings must contribute fields too — that is the
// contract the bundle work builds on.
func TestBindingFieldsFromDeclaredBindings(t *testing.T) {
	m := &metamodel.Model{
		Name:   "declared",
		Places: []metamodel.Place{{ID: "queue", Kind: metamodel.DataKind, Type: "map[string]int64", Exported: true}},
		Transitions: []metamodel.Transition{{
			ID:    "enqueue",
			Guard: "priority > 0",
			Bindings: []metamodel.Binding{
				{Name: "job_id", Type: "string"},
				{Name: "priority", Type: "int64"},
			},
		}},
		Arcs: []metamodel.Arc{{From: "enqueue", To: "queue", Keys: []string{"job_id"}, Value: "priority"}},
	}
	ctx, err := NewContext(m, ContextOptions{PackageName: "declared"})
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, f := range ctx.BindingFields {
		names = append(names, f.Name+":"+f.GoType)
	}
	joined := strings.Join(names, ",")
	if joined != "job_id:string,priority:U256JSON" {
		t.Errorf("fields = %s, want job_id:string,priority:U256JSON", joined)
	}
}

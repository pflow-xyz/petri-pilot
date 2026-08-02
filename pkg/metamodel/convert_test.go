package metamodel

import "testing"

// The JSON-shaped conversion must carry the fields the bundle compiler
// depends on — most importantly the foreign-key reference.
func TestEntityToExtensions(t *testing.T) {
	e := Entity{
		ID: "order",
		Fields: []Field{
			{ID: "customer", Type: "string", Required: true},
			{ID: "item_id", Type: "reference",
				Reference: &FieldReference{Entity: "inventory", OnDelete: "cascade"}},
		},
		States:  []EntityState{{ID: "draft", Initial: true}},
		Actions: []EntityAction{{ID: "place", FromStates: []string{"draft"}, ToState: "placed"}},
	}
	out, err := e.ToExtensions()
	if err != nil {
		t.Fatal(err)
	}
	if out.ID != "order" || len(out.Fields) != 2 || len(out.Actions) != 1 {
		t.Fatalf("converted = %+v", out)
	}
	ref := out.Fields[1].Reference
	if ref == nil || ref.Entity != "inventory" || ref.OnDelete != "cascade" {
		t.Errorf("reference lost in conversion: %+v", ref)
	}
	if !out.Fields[0].Required || !out.States[0].Initial {
		t.Error("flags lost in conversion")
	}
}

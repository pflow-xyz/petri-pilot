package extensions

import (
	"encoding/json"
	"testing"

	goflowmodel "github.com/pflow-xyz/go-pflow/metamodel"
)

func TestEntityExtension(t *testing.T) {
	t.Run("basic entity", func(t *testing.T) {
		ext := NewEntityExtension()
		ext.AddEntity(Entity{
			ID:   "order",
			Name: "Order",
			Fields: []Field{
				{ID: "total", Type: FieldTypeInt64},
				{ID: "status", Type: FieldTypeString},
			},
			States: []EntityState{
				{ID: "pending", Initial: true},
				{ID: "confirmed"},
				{ID: "shipped"},
			},
			Actions: []EntityAction{
				{ID: "confirm", FromStates: []string{"pending"}, ToState: "confirmed"},
				{ID: "ship", FromStates: []string{"confirmed"}, ToState: "shipped"},
			},
		})

		if len(ext.Entities) != 1 {
			t.Errorf("expected 1 entity, got %d", len(ext.Entities))
		}

		order := ext.EntityByID("order")
		if order == nil {
			t.Fatal("expected to find order entity")
		}
		if order.Name != "Order" {
			t.Errorf("expected name 'Order', got %q", order.Name)
		}
	})

	t.Run("validation duplicate entity", func(t *testing.T) {
		ext := NewEntityExtension()
		ext.AddEntity(Entity{ID: "order"})
		ext.AddEntity(Entity{ID: "order"}) // duplicate

		err := ext.Validate(&goflowmodel.Model{})
		if err == nil {
			t.Error("expected validation error for duplicate entity")
		}
	})

	t.Run("validation duplicate field", func(t *testing.T) {
		ext := NewEntityExtension()
		ext.AddEntity(Entity{
			ID: "order",
			Fields: []Field{
				{ID: "total"},
				{ID: "total"}, // duplicate
			},
		})

		err := ext.Validate(&goflowmodel.Model{})
		if err == nil {
			t.Error("expected validation error for duplicate field")
		}
	})

	t.Run("validation unknown state", func(t *testing.T) {
		ext := NewEntityExtension()
		ext.AddEntity(Entity{
			ID: "order",
			States: []EntityState{
				{ID: "pending"},
			},
			Actions: []EntityAction{
				{ID: "confirm", FromStates: []string{"unknown"}},
			},
		})

		err := ext.Validate(&goflowmodel.Model{})
		if err == nil {
			t.Error("expected validation error for unknown state")
		}
	})

	t.Run("JSON roundtrip", func(t *testing.T) {
		ext := NewEntityExtension()
		ext.AddEntity(Entity{
			ID: "order",
			Fields: []Field{
				{ID: "total", Type: FieldTypeInt64, Required: true},
			},
		})

		data, err := ext.MarshalJSON()
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}

		ext2 := NewEntityExtension()
		if err := ext2.UnmarshalJSON(data); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		if len(ext2.Entities) != 1 {
			t.Errorf("expected 1 entity after roundtrip, got %d", len(ext2.Entities))
		}
	})
}

func TestRoleExtension(t *testing.T) {
	t.Run("basic role", func(t *testing.T) {
		ext := NewRoleExtension()
		ext.AddRole(Role{ID: "user", Name: "User"})
		ext.AddRole(Role{ID: "admin", Name: "Administrator", Inherits: []string{"user"}})

		if len(ext.Roles) != 2 {
			t.Errorf("expected 2 roles, got %d", len(ext.Roles))
		}

		admin := ext.RoleByID("admin")
		if admin == nil {
			t.Fatal("expected to find admin role")
		}
		if len(admin.Inherits) != 1 || admin.Inherits[0] != "user" {
			t.Error("expected admin to inherit from user")
		}
	})

	t.Run("flatten hierarchy", func(t *testing.T) {
		ext := NewRoleExtension()
		ext.AddRole(Role{ID: "base"})
		ext.AddRole(Role{ID: "user", Inherits: []string{"base"}})
		ext.AddRole(Role{ID: "admin", Inherits: []string{"user"}})

		flat := ext.FlattenHierarchy("admin")
		if len(flat) != 3 {
			t.Errorf("expected 3 roles in hierarchy, got %d", len(flat))
		}
	})

	t.Run("validation duplicate role", func(t *testing.T) {
		ext := NewRoleExtension()
		ext.AddRole(Role{ID: "admin"})
		ext.AddRole(Role{ID: "admin"}) // duplicate

		err := ext.Validate(&goflowmodel.Model{})
		if err == nil {
			t.Error("expected validation error for duplicate role")
		}
	})

	t.Run("validation unknown parent", func(t *testing.T) {
		ext := NewRoleExtension()
		ext.AddRole(Role{ID: "admin", Inherits: []string{"unknown"}})

		err := ext.Validate(&goflowmodel.Model{})
		if err == nil {
			t.Error("expected validation error for unknown parent")
		}
	})
}

func TestPageExtension(t *testing.T) {
	t.Run("basic page", func(t *testing.T) {
		ext := NewPageExtension()
		ext.AddPage(Page{
			ID:   "orders",
			Name: "Orders",
			Path: "/orders",
			Layout: PageLayout{
				Type:   "list",
				Entity: "order",
			},
		})

		if len(ext.Pages) != 1 {
			t.Errorf("expected 1 page, got %d", len(ext.Pages))
		}

		page := ext.PageByPath("/orders")
		if page == nil {
			t.Fatal("expected to find page by path")
		}
	})

	t.Run("navigation", func(t *testing.T) {
		ext := NewPageExtension()
		ext.SetNavigation(Navigation{
			Brand: "OrderApp",
			Items: []NavigationItem{
				{Label: "Orders", Path: "/orders"},
				{Label: "Admin", Path: "/admin", Roles: []string{"admin"}},
			},
		})

		if ext.Navigation == nil {
			t.Fatal("expected navigation to be set")
		}
		if ext.Navigation.Brand != "OrderApp" {
			t.Errorf("expected brand 'OrderApp', got %q", ext.Navigation.Brand)
		}
	})

	t.Run("validation duplicate path", func(t *testing.T) {
		ext := NewPageExtension()
		ext.AddPage(Page{ID: "p1", Path: "/orders"})
		ext.AddPage(Page{ID: "p2", Path: "/orders"}) // duplicate path

		err := ext.Validate(&goflowmodel.Model{})
		if err == nil {
			t.Error("expected validation error for duplicate path")
		}
	})
}

func TestWorkflowExtension(t *testing.T) {
	t.Run("basic workflow", func(t *testing.T) {
		ext := NewWorkflowExtension()
		ext.AddWorkflow(Workflow{
			ID:   "order-fulfillment",
			Name: "Order Fulfillment",
			Trigger: WorkflowTrigger{
				Type:   "event",
				Entity: "order",
				Action: "create",
			},
			Steps: []WorkflowStep{
				{ID: "notify", Type: "action", Action: "send_notification"},
				{ID: "wait", Type: "wait", Duration: "1h", OnSuccess: "process"},
				{ID: "process", Type: "action", Action: "process_order"},
			},
		})

		if len(ext.Workflows) != 1 {
			t.Errorf("expected 1 workflow, got %d", len(ext.Workflows))
		}

		wf := ext.WorkflowByID("order-fulfillment")
		if wf == nil {
			t.Fatal("expected to find workflow")
		}
		if len(wf.Steps) != 3 {
			t.Errorf("expected 3 steps, got %d", len(wf.Steps))
		}
	})

	t.Run("validation invalid trigger", func(t *testing.T) {
		ext := NewWorkflowExtension()
		ext.AddWorkflow(Workflow{
			ID:      "wf1",
			Trigger: WorkflowTrigger{Type: "invalid"},
		})

		err := ext.Validate(&goflowmodel.Model{})
		if err == nil {
			t.Error("expected validation error for invalid trigger type")
		}
	})

	t.Run("validation unknown step reference", func(t *testing.T) {
		ext := NewWorkflowExtension()
		ext.AddWorkflow(Workflow{
			ID:      "wf1",
			Trigger: WorkflowTrigger{Type: "manual"},
			Steps: []WorkflowStep{
				{ID: "step1", OnSuccess: "nonexistent"},
			},
		})

		err := ext.Validate(&goflowmodel.Model{})
		if err == nil {
			t.Error("expected validation error for unknown step reference")
		}
	})
}

func TestViewExtension(t *testing.T) {
	t.Run("basic view", func(t *testing.T) {
		ext := NewViewExtension()
		ext.AddView(View{
			ID:   "order-detail",
			Name: "Order Detail",
			Kind: "detail",
			Groups: []ViewGroup{
				{
					ID:   "info",
					Name: "Order Information",
					Fields: []ViewField{
						{Binding: "total", Label: "Total", Type: "number"},
						{Binding: "status", Label: "Status", Type: "text"},
					},
				},
			},
		})

		if len(ext.Views) != 1 {
			t.Errorf("expected 1 view, got %d", len(ext.Views))
		}

		view := ext.ViewByID("order-detail")
		if view == nil {
			t.Fatal("expected to find view")
		}
		if len(view.Groups) != 1 {
			t.Errorf("expected 1 group, got %d", len(view.Groups))
		}
	})

	t.Run("admin config", func(t *testing.T) {
		ext := NewViewExtension()
		ext.SetAdmin(Admin{
			Enabled:  true,
			Path:     "/admin",
			Roles:    []string{"admin"},
			Features: []string{"list", "detail", "history"},
		})

		if ext.Admin == nil {
			t.Fatal("expected admin to be set")
		}
		if !ext.Admin.Enabled {
			t.Error("expected admin to be enabled")
		}
	})

	t.Run("validation invalid kind", func(t *testing.T) {
		ext := NewViewExtension()
		ext.AddView(View{ID: "v1", Kind: "invalid"})

		err := ext.Validate(&goflowmodel.Model{})
		if err == nil {
			t.Error("expected validation error for invalid kind")
		}
	})

	t.Run("validation unknown action", func(t *testing.T) {
		ext := NewViewExtension()
		ext.AddView(View{
			ID:      "v1",
			Actions: []string{"nonexistent"},
		})

		model := &goflowmodel.Model{
			Transitions: []goflowmodel.Transition{
				{ID: "confirm"},
			},
		}

		err := ext.Validate(model)
		if err == nil {
			t.Error("expected validation error for unknown action")
		}
	})
}

func TestApplicationSpec(t *testing.T) {
	t.Run("create and add extensions", func(t *testing.T) {
		model := &goflowmodel.Model{
			Name: "order-workflow",
			Places: []goflowmodel.Place{
				{ID: "pending", Initial: 1},
				{ID: "confirmed"},
			},
			Transitions: []goflowmodel.Transition{
				{ID: "confirm"},
			},
			Arcs: []goflowmodel.Arc{
				{From: "pending", To: "confirm"},
				{From: "confirm", To: "confirmed"},
			},
		}

		app := NewApplicationSpec(model)

		// Add entities
		entities := NewEntityExtension()
		entities.AddEntity(Entity{
			ID:   "order",
			Name: "Order",
			Fields: []Field{
				{ID: "total", Type: FieldTypeInt64},
			},
		})
		if err := app.WithEntities(entities); err != nil {
			t.Fatalf("error adding entities: %v", err)
		}

		// Add roles
		roles := NewRoleExtension()
		roles.AddRole(Role{ID: "admin", Name: "Administrator"})
		if err := app.WithRoles(roles); err != nil {
			t.Fatalf("error adding roles: %v", err)
		}

		// Verify
		if !app.HasEntities() {
			t.Error("expected HasEntities to be true")
		}
		if !app.HasRoles() {
			t.Error("expected HasRoles to be true")
		}
		if app.HasPages() {
			t.Error("expected HasPages to be false")
		}
	})

	t.Run("JSON roundtrip", func(t *testing.T) {
		model := &goflowmodel.Model{
			Name: "test-workflow",
			Places: []goflowmodel.Place{
				{ID: "start", Initial: 1},
			},
			Transitions: []goflowmodel.Transition{
				{ID: "run"},
			},
			Arcs: []goflowmodel.Arc{
				{From: "start", To: "run"},
			},
		}

		app := NewApplicationSpec(model)

		entities := NewEntityExtension()
		entities.AddEntity(Entity{ID: "task", Name: "Task"})
		app.WithEntities(entities)

		roles := NewRoleExtension()
		roles.AddRole(Role{ID: "user", Name: "User"})
		app.WithRoles(roles)

		// Serialize
		data, err := app.ToJSON()
		if err != nil {
			t.Fatalf("error serializing: %v", err)
		}

		// Deserialize
		app2, err := NewApplicationSpecFromJSON(data)
		if err != nil {
			t.Fatalf("error deserializing: %v", err)
		}

		// Verify
		if app2.Net.Name != "test-workflow" {
			t.Errorf("expected name 'test-workflow', got %q", app2.Net.Name)
		}
		if !app2.HasEntities() {
			t.Error("expected HasEntities to be true after roundtrip")
		}
		if !app2.HasRoles() {
			t.Error("expected HasRoles to be true after roundtrip")
		}
	})

	t.Run("helper methods", func(t *testing.T) {
		model := &goflowmodel.Model{
			Name: "test",
			Places: []goflowmodel.Place{
				{ID: "p1"},
				{ID: "p2"},
			},
			Transitions: []goflowmodel.Transition{
				{ID: "t1"},
				{ID: "t2"},
			},
		}

		app := NewApplicationSpec(model)

		placeIDs := app.AllPlaceIDs()
		if len(placeIDs) != 2 {
			t.Errorf("expected 2 place IDs, got %d", len(placeIDs))
		}

		transIDs := app.AllTransitionIDs()
		if len(transIDs) != 2 {
			t.Errorf("expected 2 transition IDs, got %d", len(transIDs))
		}
	})
}

func TestExtensionRegistration(t *testing.T) {
	// Verify all extensions are registered
	registry := goflowmodel.DefaultRegistry

	tests := []string{
		EntitiesExtensionName,
		RolesExtensionName,
		PagesExtensionName,
		WorkflowsExtensionName,
		ViewsExtensionName,
	}

	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			ext, err := registry.Create(name)
			if err != nil {
				t.Fatalf("failed to create extension %s: %v", name, err)
			}
			if ext.Name() != name {
				t.Errorf("expected name %q, got %q", name, ext.Name())
			}
		})
	}
}

func TestFullApplicationJSON(t *testing.T) {
	// Test the full JSON structure as specified in the plan
	jsonData := `{
		"version": "2.0",
		"net": {
			"name": "order-workflow",
			"places": [
				{"id": "pending", "initial": 1, "kind": "token"},
				{"id": "order_data", "kind": "data", "type": "Order"}
			],
			"transitions": [
				{"id": "create"},
				{"id": "confirm"}
			],
			"arcs": [
				{"from": "create", "to": "pending"},
				{"from": "pending", "to": "confirm"}
			]
		},
		"extensions": {
			"petri-pilot/entities": [
				{
					"id": "order",
					"name": "Order",
					"fields": [
						{"id": "total", "type": "int64", "required": true},
						{"id": "customer", "type": "string"}
					]
				}
			],
			"petri-pilot/roles": [
				{"id": "customer", "name": "Customer"},
				{"id": "admin", "name": "Administrator", "inherits": ["customer"]}
			]
		}
	}`

	app, err := NewApplicationSpecFromJSON([]byte(jsonData))
	if err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// Verify model
	if app.Net.Name != "order-workflow" {
		t.Errorf("expected name 'order-workflow', got %q", app.Net.Name)
	}

	// Verify entities
	entities := app.Entities()
	if entities == nil {
		t.Fatal("expected entities extension")
	}
	if len(entities.Entities) != 1 {
		t.Errorf("expected 1 entity, got %d", len(entities.Entities))
	}
	order := entities.EntityByID("order")
	if order == nil {
		t.Fatal("expected to find order entity")
	}
	if len(order.Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(order.Fields))
	}

	// Verify roles
	roles := app.Roles()
	if roles == nil {
		t.Fatal("expected roles extension")
	}
	if len(roles.Roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(roles.Roles))
	}
	admin := roles.RoleByID("admin")
	if admin == nil {
		t.Fatal("expected to find admin role")
	}
	if len(admin.Inherits) != 1 || admin.Inherits[0] != "customer" {
		t.Error("expected admin to inherit from customer")
	}

	// Re-serialize and verify structure
	output, err := json.MarshalIndent(app.ExtendedModel, "", "  ")
	if err != nil {
		t.Fatalf("failed to serialize: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(output, &parsed); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if parsed["version"] != "2.0" {
		t.Errorf("expected version '2.0', got %v", parsed["version"])
	}
}

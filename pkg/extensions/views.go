package extensions

import (
	"encoding/json"
	"fmt"

	goflowmodel "github.com/pflow-xyz/go-pflow/metamodel"
)

const (
	// ViewsExtensionName is the extension name for view definitions.
	ViewsExtensionName = "petri-pilot/views"
)

// ViewExtension adds UI view definitions to a Petri net model.
type ViewExtension struct {
	goflowmodel.BaseExtension
	Views []View `json:"views"`
	Admin *Admin `json:"admin,omitempty"`
}

// View represents a UI view definition for presenting workflow data.
type View struct {
	ID          string      `json:"id"`
	Name        string      `json:"name,omitempty"`
	Kind        string      `json:"kind,omitempty"` // form, card, table, detail
	Description string      `json:"description,omitempty"`
	Groups      []ViewGroup `json:"groups,omitempty"`
	Actions     []string    `json:"actions,omitempty"` // Transition IDs that can be triggered
}

// ViewGroup represents a logical grouping of fields within a view.
type ViewGroup struct {
	ID     string      `json:"id"`
	Name   string      `json:"name,omitempty"`
	Fields []ViewField `json:"fields"`
}

// ViewField represents a single field within a view group.
type ViewField struct {
	Binding     string `json:"binding"`
	Label       string `json:"label,omitempty"`
	Type        string `json:"type,omitempty"` // text, number, select, date, etc.
	Required    bool   `json:"required,omitempty"`
	ReadOnly    bool   `json:"readonly,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
}

// Admin represents admin dashboard configuration.
type Admin struct {
	Enabled  bool     `json:"enabled"`
	Path     string   `json:"path"`
	Roles    []string `json:"roles"`
	Features []string `json:"features"` // list, detail, history, transitions
}

// NewViewExtension creates a new ViewExtension.
func NewViewExtension() *ViewExtension {
	return &ViewExtension{
		BaseExtension: goflowmodel.NewBaseExtension(ViewsExtensionName),
		Views:         make([]View, 0),
	}
}

// Validate checks that all views are valid and consistent with the model.
func (v *ViewExtension) Validate(model *goflowmodel.Model) error {
	seen := make(map[string]bool)
	for _, view := range v.Views {
		if seen[view.ID] {
			return fmt.Errorf("duplicate view ID: %s", view.ID)
		}
		seen[view.ID] = true

		// Validate view kind
		validKinds := map[string]bool{
			"form": true, "card": true, "table": true, "detail": true, "": true,
		}
		if !validKinds[view.Kind] {
			return fmt.Errorf("view %s: invalid kind: %s", view.ID, view.Kind)
		}

		// Validate actions reference existing transitions
		transitionIDs := make(map[string]bool)
		for _, t := range model.Transitions {
			transitionIDs[t.ID] = true
		}
		for _, actionID := range view.Actions {
			if !transitionIDs[actionID] {
				return fmt.Errorf("view %s: action references unknown transition: %s",
					view.ID, actionID)
			}
		}
	}
	return nil
}

// MarshalJSON serializes the views and admin config.
func (v *ViewExtension) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Views []View `json:"views"`
		Admin *Admin `json:"admin,omitempty"`
	}{
		Views: v.Views,
		Admin: v.Admin,
	})
}

// UnmarshalJSON deserializes the views and admin config.
func (v *ViewExtension) UnmarshalJSON(data []byte) error {
	var raw struct {
		Views []View `json:"views"`
		Admin *Admin `json:"admin,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	v.Views = raw.Views
	v.Admin = raw.Admin
	return nil
}

// AddView adds a view to the extension.
func (v *ViewExtension) AddView(view View) {
	v.Views = append(v.Views, view)
}

// ViewByID returns a view by ID, or nil if not found.
func (v *ViewExtension) ViewByID(id string) *View {
	for i := range v.Views {
		if v.Views[i].ID == id {
			return &v.Views[i]
		}
	}
	return nil
}

// SetAdmin sets the admin configuration.
func (v *ViewExtension) SetAdmin(admin Admin) {
	v.Admin = &admin
}

// init registers the view extension with the default registry.
func init() {
	goflowmodel.Register(ViewsExtensionName, func() goflowmodel.ModelExtension {
		return NewViewExtension()
	})
}

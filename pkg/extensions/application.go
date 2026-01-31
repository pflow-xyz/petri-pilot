// Package extensions provides petri-pilot application-level extensions
// that work with go-pflow's metamodel extension system.
//
// Extensions allow application-level constructs (entities, roles, pages,
// workflows) to be added to a Petri net model without modifying the core
// go-pflow metamodel package.
//
// Usage:
//
//	// Create an ApplicationSpec from a model
//	app := NewApplicationSpec(model)
//
//	// Add entities
//	entities := NewEntityExtension()
//	entities.AddEntity(Entity{ID: "order", ...})
//	app.AddExtension(entities)
//
//	// Add roles
//	roles := NewRoleExtension()
//	roles.AddRole(Role{ID: "admin", ...})
//	app.AddExtension(roles)
//
//	// Serialize to JSON
//	data, _ := json.Marshal(app)
package extensions

import (
	"encoding/json"

	goflowmodel "github.com/pflow-xyz/go-pflow/metamodel"
)

// ApplicationSpec represents a complete application specification.
// It wraps a Petri net model with petri-pilot extensions.
type ApplicationSpec struct {
	// Model is the underlying ExtendedModel from go-pflow.
	*goflowmodel.ExtendedModel

	// Debug configuration (not an extension, simple config)
	DebugConfig *goflowmodel.Debug `json:"debug,omitempty"`

	// GraphQL configuration (not an extension, simple config)
	GraphQLConfig *goflowmodel.GraphQLConfig `json:"graphql,omitempty"`
}

// NewApplicationSpec creates a new ApplicationSpec from a model.
func NewApplicationSpec(model *goflowmodel.Model) *ApplicationSpec {
	return &ApplicationSpec{
		ExtendedModel: goflowmodel.NewExtendedModel(model),
	}
}

// NewApplicationSpecFromJSON parses an ApplicationSpec from JSON.
func NewApplicationSpecFromJSON(data []byte) (*ApplicationSpec, error) {
	var extended goflowmodel.ExtendedModel
	if err := json.Unmarshal(data, &extended); err != nil {
		return nil, err
	}
	return &ApplicationSpec{ExtendedModel: &extended}, nil
}

// Entities returns the entity extension, or nil if not present.
func (a *ApplicationSpec) Entities() *EntityExtension {
	ext := a.GetExtension(EntitiesExtensionName)
	if ext == nil {
		return nil
	}
	return ext.(*EntityExtension)
}

// Roles returns the role extension, or nil if not present.
func (a *ApplicationSpec) Roles() *RoleExtension {
	ext := a.GetExtension(RolesExtensionName)
	if ext == nil {
		return nil
	}
	return ext.(*RoleExtension)
}

// Pages returns the page extension, or nil if not present.
func (a *ApplicationSpec) Pages() *PageExtension {
	ext := a.GetExtension(PagesExtensionName)
	if ext == nil {
		return nil
	}
	return ext.(*PageExtension)
}

// Workflows returns the workflow extension, or nil if not present.
func (a *ApplicationSpec) Workflows() *WorkflowExtension {
	ext := a.GetExtension(WorkflowsExtensionName)
	if ext == nil {
		return nil
	}
	return ext.(*WorkflowExtension)
}

// Views returns the view extension, or nil if not present.
func (a *ApplicationSpec) Views() *ViewExtension {
	ext := a.GetExtension(ViewsExtensionName)
	if ext == nil {
		return nil
	}
	return ext.(*ViewExtension)
}

// HasEntities returns true if the application has entities.
func (a *ApplicationSpec) HasEntities() bool {
	return a.Entities() != nil && len(a.Entities().Entities) > 0
}

// HasRoles returns true if the application has roles.
func (a *ApplicationSpec) HasRoles() bool {
	return a.Roles() != nil && len(a.Roles().Roles) > 0
}

// HasPages returns true if the application has pages.
func (a *ApplicationSpec) HasPages() bool {
	return a.Pages() != nil && len(a.Pages().Pages) > 0
}

// HasWorkflows returns true if the application has workflows.
func (a *ApplicationSpec) HasWorkflows() bool {
	return a.Workflows() != nil && len(a.Workflows().Workflows) > 0
}

// HasViews returns true if the application has views.
func (a *ApplicationSpec) HasViews() bool {
	return a.Views() != nil && len(a.Views().Views) > 0
}

// HasAdmin returns true if admin is enabled.
func (a *ApplicationSpec) HasAdmin() bool {
	views := a.Views()
	return views != nil && views.Admin != nil && views.Admin.Enabled
}

// HasNavigation returns true if navigation is configured.
func (a *ApplicationSpec) HasNavigation() bool {
	pages := a.Pages()
	return pages != nil && pages.Navigation != nil
}

// HasDebug returns true if debug is enabled.
func (a *ApplicationSpec) HasDebug() bool {
	return a.DebugConfig != nil && a.DebugConfig.Enabled
}

// HasGraphQL returns true if GraphQL is enabled.
func (a *ApplicationSpec) HasGraphQL() bool {
	return a.GraphQLConfig != nil && a.GraphQLConfig.Enabled
}

// Debug returns the debug config.
func (a *ApplicationSpec) Debug() *goflowmodel.Debug {
	return a.DebugConfig
}

// GraphQL returns the GraphQL config.
func (a *ApplicationSpec) GraphQL() *goflowmodel.GraphQLConfig {
	return a.GraphQLConfig
}

// SetDebug sets the debug config.
func (a *ApplicationSpec) SetDebug(debug *goflowmodel.Debug) {
	a.DebugConfig = debug
}

// SetGraphQL sets the GraphQL config.
func (a *ApplicationSpec) SetGraphQL(graphql *goflowmodel.GraphQLConfig) {
	a.GraphQLConfig = graphql
}

// WithEntities adds or replaces the entity extension.
func (a *ApplicationSpec) WithEntities(entities *EntityExtension) error {
	return a.AddExtension(entities)
}

// WithRoles adds or replaces the role extension.
func (a *ApplicationSpec) WithRoles(roles *RoleExtension) error {
	return a.AddExtension(roles)
}

// WithPages adds or replaces the page extension.
func (a *ApplicationSpec) WithPages(pages *PageExtension) error {
	return a.AddExtension(pages)
}

// WithWorkflows adds or replaces the workflow extension.
func (a *ApplicationSpec) WithWorkflows(workflows *WorkflowExtension) error {
	return a.AddExtension(workflows)
}

// WithViews adds or replaces the view extension.
func (a *ApplicationSpec) WithViews(views *ViewExtension) error {
	return a.AddExtension(views)
}

// ToJSON serializes the application spec to JSON.
func (a *ApplicationSpec) ToJSON() ([]byte, error) {
	return json.MarshalIndent(a.ExtendedModel, "", "  ")
}

// AllTransitionIDs returns all transition IDs from the model.
func (a *ApplicationSpec) AllTransitionIDs() []string {
	if a.Net == nil {
		return nil
	}
	ids := make([]string, len(a.Net.Transitions))
	for i, t := range a.Net.Transitions {
		ids[i] = t.ID
	}
	return ids
}

// AllPlaceIDs returns all place IDs from the model.
func (a *ApplicationSpec) AllPlaceIDs() []string {
	if a.Net == nil {
		return nil
	}
	ids := make([]string, len(a.Net.Places))
	for i, p := range a.Net.Places {
		ids[i] = p.ID
	}
	return ids
}

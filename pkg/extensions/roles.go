package extensions

import (
	"encoding/json"
	"fmt"

	goflowmodel "github.com/pflow-xyz/go-pflow/metamodel"
)

const (
	// RolesExtensionName is the extension name for role definitions.
	RolesExtensionName = "petri-pilot/roles"
)

// RoleExtension adds role-based access control to a Petri net model.
type RoleExtension struct {
	goflowmodel.BaseExtension
	Roles []Role `json:"roles"`
}

// Role defines an access control role.
type Role struct {
	ID           string   `json:"id"`
	Name         string   `json:"name,omitempty"`
	Description  string   `json:"description,omitempty"`
	Inherits     []string `json:"inherits,omitempty"`      // Parent roles
	DynamicGrant string   `json:"dynamic_grant,omitempty"` // Expression to dynamically grant role
}

// NewRoleExtension creates a new RoleExtension.
func NewRoleExtension() *RoleExtension {
	return &RoleExtension{
		BaseExtension: goflowmodel.NewBaseExtension(RolesExtensionName),
		Roles:         make([]Role, 0),
	}
}

// Validate checks that all roles are valid.
func (r *RoleExtension) Validate(model *goflowmodel.Model) error {
	seen := make(map[string]bool)
	for _, role := range r.Roles {
		if seen[role.ID] {
			return fmt.Errorf("duplicate role ID: %s", role.ID)
		}
		seen[role.ID] = true
	}

	// Validate inheritance references existing roles
	for _, role := range r.Roles {
		for _, parent := range role.Inherits {
			if !seen[parent] {
				return fmt.Errorf("role %s inherits unknown role: %s", role.ID, parent)
			}
		}
	}

	return nil
}

// MarshalJSON serializes the roles.
func (r *RoleExtension) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.Roles)
}

// UnmarshalJSON deserializes the roles.
func (r *RoleExtension) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &r.Roles)
}

// AddRole adds a role to the extension.
func (r *RoleExtension) AddRole(role Role) {
	r.Roles = append(r.Roles, role)
}

// RoleByID returns a role by ID, or nil if not found.
func (r *RoleExtension) RoleByID(id string) *Role {
	for i := range r.Roles {
		if r.Roles[i].ID == id {
			return &r.Roles[i]
		}
	}
	return nil
}

// FlattenHierarchy returns all roles that a given role inherits (including itself).
func (r *RoleExtension) FlattenHierarchy(roleID string) []string {
	result := make(map[string]bool)
	r.collectInheritedRoles(roleID, result)

	flat := make([]string, 0, len(result))
	for id := range result {
		flat = append(flat, id)
	}
	return flat
}

func (r *RoleExtension) collectInheritedRoles(roleID string, collected map[string]bool) {
	if collected[roleID] {
		return // Already processed (prevents cycles)
	}
	collected[roleID] = true

	role := r.RoleByID(roleID)
	if role == nil {
		return
	}

	for _, parent := range role.Inherits {
		r.collectInheritedRoles(parent, collected)
	}
}

// init registers the role extension with the default registry.
func init() {
	goflowmodel.Register(RolesExtensionName, func() goflowmodel.ModelExtension {
		return NewRoleExtension()
	})
}

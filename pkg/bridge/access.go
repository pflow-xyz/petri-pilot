// Package bridge provides code generation helpers for petri-pilot.
// This file adds access control extraction for role-based access control (RBAC).
package bridge

import (
	"strings"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

// AccessSpec describes the access control configuration extracted from a model.
type AccessSpec struct {
	// Roles are the role definitions from the model.
	Roles []RoleSpec

	// Rules are the access rules from the model.
	Rules []AccessRuleSpec

	// RoleHierarchy maps each role to its effective roles (including inherited).
	// For example, if "admin" inherits "user", RoleHierarchy["admin"] = ["admin", "user"].
	RoleHierarchy map[string][]string
}

// RoleSpec describes a single role extracted from the model.
type RoleSpec struct {
	// ID is the role identifier.
	ID string

	// Name is the display name (defaults to ID if empty).
	Name string

	// Description describes the role.
	Description string

	// Inherits are the parent role IDs.
	Inherits []string

	// AllRoles includes this role and all inherited roles (flattened).
	AllRoles []string

	// DynamicGrant is an expression to dynamically grant this role based on state.
	// Example: "balances[user.login] > 0" grants the role if user has a balance.
	DynamicGrant string
}

// AccessRuleSpec describes a single access rule extracted from the model.
type AccessRuleSpec struct {
	// TransitionID is the transition this rule applies to ("*" for all).
	TransitionID string

	// Roles are the allowed roles (empty = any authenticated user).
	Roles []string

	// Guard is the optional guard expression.
	Guard string

	// HasGuard indicates if a guard expression is present.
	HasGuard bool
}

// ExtractAccessSpec extracts access control configuration from a model.
func ExtractAccessSpec(model *metamodel.Model) *AccessSpec {
	spec := &AccessSpec{
		Roles:         make([]RoleSpec, 0, len(model.Roles)),
		Rules:         make([]AccessRuleSpec, 0, len(model.Access)),
		RoleHierarchy: make(map[string][]string),
	}

	// Build role hierarchy first
	roleMap := make(map[string]*metamodel.Role)
	for i := range model.Roles {
		roleMap[model.Roles[i].ID] = &model.Roles[i]
	}

	// Resolve role hierarchy (flatten inheritance)
	spec.RoleHierarchy = ResolveRoleHierarchy(model.Roles)

	// Extract role specs
	for _, role := range model.Roles {
		name := role.Name
		if name == "" {
			name = toPascalCase(role.ID)
		}

		roleSpec := RoleSpec{
			ID:           role.ID,
			Name:         name,
			Description:  role.Description,
			Inherits:     role.Inherits,
			AllRoles:     spec.RoleHierarchy[role.ID],
			DynamicGrant: role.DynamicGrant,
		}
		spec.Roles = append(spec.Roles, roleSpec)
	}

	// Extract access rules
	for _, rule := range model.Access {
		ruleSpec := AccessRuleSpec{
			TransitionID: rule.Transition,
			Roles:        rule.Roles,
			Guard:        rule.Guard,
			HasGuard:     rule.Guard != "",
		}
		spec.Rules = append(spec.Rules, ruleSpec)
	}

	return spec
}

// ResolveRoleHierarchy flattens role inheritance into a map of role ID to all effective roles.
// For example, if admin inherits from user, RoleHierarchy["admin"] = ["admin", "user"].
func ResolveRoleHierarchy(roles []metamodel.Role) map[string][]string {
	hierarchy := make(map[string][]string)

	// Build role map for lookup
	roleMap := make(map[string]*metamodel.Role)
	for i := range roles {
		roleMap[roles[i].ID] = &roles[i]
	}

	// Recursively expand each role's inheritance
	var expand func(roleID string, seen map[string]bool) []string
	expand = func(roleID string, seen map[string]bool) []string {
		if seen[roleID] {
			return nil // Prevent cycles
		}
		seen[roleID] = true

		result := []string{roleID}

		role, exists := roleMap[roleID]
		if !exists {
			return result
		}

		for _, parentID := range role.Inherits {
			parentRoles := expand(parentID, seen)
			for _, pr := range parentRoles {
				if !containsString(result, pr) {
					result = append(result, pr)
				}
			}
		}

		return result
	}

	// Resolve hierarchy for each role
	for _, role := range roles {
		seen := make(map[string]bool)
		hierarchy[role.ID] = expand(role.ID, seen)
	}

	return hierarchy
}

// RulesForTransition returns all access rules that apply to a specific transition.
// This includes rules with matching transition ID and wildcard rules ("*").
func (s *AccessSpec) RulesForTransition(transitionID string) []AccessRuleSpec {
	var result []AccessRuleSpec
	for _, rule := range s.Rules {
		if rule.TransitionID == transitionID || rule.TransitionID == "*" {
			result = append(result, rule)
		}
	}
	return result
}

// HasAccessControl returns true if the spec has any roles or access rules defined.
func (s *AccessSpec) HasAccessControl() bool {
	return len(s.Roles) > 0 || len(s.Rules) > 0
}

// RoleByID returns a role by its ID, or nil if not found.
func (s *AccessSpec) RoleByID(roleID string) *RoleSpec {
	for i := range s.Roles {
		if s.Roles[i].ID == roleID {
			return &s.Roles[i]
		}
	}
	return nil
}

// EffectiveRoles returns all effective roles for a given role ID (including inherited).
func (s *AccessSpec) EffectiveRoles(roleID string) []string {
	if roles, ok := s.RoleHierarchy[roleID]; ok {
		return roles
	}
	return []string{roleID}
}

// containsString checks if a slice contains a string.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// ToConstName converts a role ID to a Go constant name (e.g., "admin" -> "RoleAdmin").
func ToRoleConstName(roleID string) string {
	return "Role" + toPascalCase(roleID)
}

// toPascalCase is defined in orm.go, but we include a copy here for self-contained access.go.
// This converts snake_case or kebab-case to PascalCase.
func toPascalCaseAccess(s string) string {
	if s == "" {
		return ""
	}

	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, "-", " ")

	words := strings.Fields(s)
	var result strings.Builder

	for _, word := range words {
		if word == "" {
			continue
		}
		runes := []rune(word)
		if len(runes) > 0 {
			if runes[0] >= 'a' && runes[0] <= 'z' {
				runes[0] = runes[0] - 32
			}
		}
		for i := 1; i < len(runes); i++ {
			if runes[i] >= 'A' && runes[i] <= 'Z' {
				runes[i] = runes[i] + 32
			}
		}
		result.WriteString(string(runes))
	}

	return result.String()
}

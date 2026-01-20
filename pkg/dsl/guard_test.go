package dsl

import "testing"

func TestHasRole(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		bindings map[string]any
		want     bool
		wantErr  bool
	}{
		{
			name: "user has admin role",
			expr: "hasRole('admin')",
			bindings: map[string]any{
				"user": map[string]any{
					"id":    "user-1",
					"roles": []string{"admin", "editor"},
				},
			},
			want: true,
		},
		{
			name: "user does not have admin role",
			expr: "hasRole('admin')",
			bindings: map[string]any{
				"user": map[string]any{
					"id":    "user-1",
					"roles": []string{"viewer"},
				},
			},
			want: false,
		},
		{
			name: "user has role with []any type",
			expr: "hasRole('editor')",
			bindings: map[string]any{
				"user": map[string]any{
					"id":    "user-1",
					"roles": []any{"admin", "editor"},
				},
			},
			want: true,
		},
		{
			name: "no user in bindings",
			expr: "hasRole('admin')",
			bindings: map[string]any{
				"other": "value",
			},
			want: false,
		},
		{
			name: "combined with other conditions",
			expr: "hasRole('admin') || user.id == 'owner-1'",
			bindings: map[string]any{
				"user": map[string]any{
					"id":    "owner-1",
					"roles": []string{"viewer"},
				},
			},
			want: true,
		},
		{
			name: "admin check passes",
			expr: "hasRole('admin') || user.id == owner_id",
			bindings: map[string]any{
				"user": map[string]any{
					"id":    "user-1",
					"roles": []string{"admin"},
				},
				"owner_id": "other-user",
			},
			want: true,
		},
		{
			name: "neither admin nor owner",
			expr: "hasRole('admin') || user.id == owner_id",
			bindings: map[string]any{
				"user": map[string]any{
					"id":    "user-1",
					"roles": []string{"viewer"},
				},
				"owner_id": "other-user",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Evaluate(tt.expr, tt.bindings, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("Evaluate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIncludes(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		bindings map[string]any
		want     bool
		wantErr  bool
	}{
		{
			name: "string array contains value",
			expr: "includes(tags, 'important')",
			bindings: map[string]any{
				"tags": []string{"urgent", "important", "review"},
			},
			want: true,
		},
		{
			name: "string array does not contain value",
			expr: "includes(tags, 'archived')",
			bindings: map[string]any{
				"tags": []string{"urgent", "important", "review"},
			},
			want: false,
		},
		{
			name: "any array contains value",
			expr: "includes(items, 'foo')",
			bindings: map[string]any{
				"items": []any{"foo", "bar", "baz"},
			},
			want: true,
		},
		{
			name: "empty array",
			expr: "includes(tags, 'anything')",
			bindings: map[string]any{
				"tags": []string{},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Evaluate(tt.expr, tt.bindings, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("Evaluate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

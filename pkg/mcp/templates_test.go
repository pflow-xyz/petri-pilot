package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestTemplate_List(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_template"
	req.Params.Arguments = map[string]any{}
	res, err := handleTemplate(context.Background(), req)
	if err != nil {
		t.Fatalf("handleTemplate: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	var wrapper struct {
		Total     int
		Templates []struct {
			Name        string
			Category    string
			Description string
		}
	}
	if err := json.Unmarshal([]byte(textBlock(t, res)), &wrapper); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if wrapper.Total != len(wrapper.Templates) || wrapper.Total < 5 {
		t.Errorf("expected at least 5 templates, got %d", wrapper.Total)
	}
}

func TestTemplate_AllParse(t *testing.T) {
	// Every template must parse cleanly via the standard pipeline. Catches
	// JSON typos at test time rather than when a user tries to load one.
	for name, tpl := range defiTemplates() {
		t.Run(name, func(t *testing.T) {
			parsed, err := parseModelV2(tpl.Model)
			if err != nil {
				t.Fatalf("parse %q: %v", name, err)
			}
			if len(parsed.Model.Places) == 0 {
				t.Errorf("template %q has no places", name)
			}
			if len(parsed.Model.Transitions) == 0 {
				t.Errorf("template %q has no transitions", name)
			}
			if len(parsed.Model.Arcs) == 0 {
				t.Errorf("template %q has no arcs", name)
			}
		})
	}
}

func TestTemplate_LoadByName(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_template"
	req.Params.Arguments = map[string]any{"name": "constant_product_amm"}
	res, err := handleTemplate(context.Background(), req)
	if err != nil {
		t.Fatalf("handleTemplate: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	var resp struct {
		Name        string
		Category    string
		Description string
		Model       json.RawMessage
	}
	if err := json.Unmarshal([]byte(textBlock(t, res)), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Name != "constant_product_amm" {
		t.Errorf("name = %q, want constant_product_amm", resp.Name)
	}
	if len(resp.Model) < 100 {
		t.Errorf("model JSON too small: %d bytes", len(resp.Model))
	}
	// The returned model should itself be loadable.
	if _, err := parseModelV2(string(resp.Model)); err != nil {
		t.Errorf("returned model doesn't parse: %v", err)
	}
}

func TestTemplate_UnknownName(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_template"
	req.Params.Arguments = map[string]any{"name": "bogus_template"}
	res, err := handleTemplate(context.Background(), req)
	if err != nil {
		t.Fatalf("handleTemplate: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected error for unknown template")
	}
}

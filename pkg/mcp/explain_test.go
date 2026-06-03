package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestExplain_ListAllConcepts(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_explain"
	req.Params.Arguments = map[string]any{}
	res, err := handleExplain(context.Background(), req)
	if err != nil {
		t.Fatalf("handleExplain: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	var wrapper struct {
		Total  int
		Topics []struct {
			Name     string
			Category string
			Summary  string
		}
	}
	if err := json.Unmarshal([]byte(textBlock(t, res)), &wrapper); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if wrapper.Total != len(wrapper.Topics) || wrapper.Total < 10 {
		t.Errorf("expected ≥10 concepts, got %d", wrapper.Total)
	}
	// Every entry must have a summary.
	for _, e := range wrapper.Topics {
		if e.Summary == "" {
			t.Errorf("concept %q has empty summary", e.Name)
		}
	}
}

func TestExplain_AllConceptsHaveContent(t *testing.T) {
	// Each concept must have non-empty intuition/formula/derivation/example.
	// Catches drift if someone adds a concept and forgets a field.
	for name, c := range defiConcepts() {
		t.Run(name, func(t *testing.T) {
			if c.Intuition == "" {
				t.Errorf("concept %q: empty Intuition", name)
			}
			if c.Formula == "" {
				t.Errorf("concept %q: empty Formula", name)
			}
			if c.Derivation == "" {
				t.Errorf("concept %q: empty Derivation", name)
			}
			if c.Example == "" {
				t.Errorf("concept %q: empty Example", name)
			}
			if len(c.SeeAlso) == 0 {
				t.Errorf("concept %q: no SeeAlso suggestions", name)
			}
		})
	}
}

func TestExplain_LoadByName(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_explain"
	req.Params.Arguments = map[string]any{"topic": "impermanent_loss"}
	res, err := handleExplain(context.Background(), req)
	if err != nil {
		t.Fatalf("handleExplain: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	var resp struct {
		Name      string
		Intuition string
		Formula   string
		Example   string
		SeeAlso   []string
	}
	if err := json.Unmarshal([]byte(textBlock(t, res)), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Name != "impermanent_loss" {
		t.Errorf("name = %q, want impermanent_loss", resp.Name)
	}
	// The formula text must contain the canonical IL expression.
	if !strings.Contains(resp.Formula, "2√r") && !strings.Contains(resp.Formula, "2 √r") {
		t.Errorf("formula doesn't mention 2√r: %s", resp.Formula)
	}
	if len(resp.SeeAlso) == 0 {
		t.Errorf("expected at least one SeeAlso entry")
	}
}

func TestExplain_UnknownTopic(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_explain"
	req.Params.Arguments = map[string]any{"topic": "definitely_not_a_topic"}
	res, err := handleExplain(context.Background(), req)
	if err != nil {
		t.Fatalf("handleExplain: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected error for unknown topic")
	}
}

func TestExplain_SoftMatchSubstring(t *testing.T) {
	// "amm" should match exactly one concept (constant_product_amm).
	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_explain"
	req.Params.Arguments = map[string]any{"topic": "constant_product"}
	res, err := handleExplain(context.Background(), req)
	if err != nil {
		t.Fatalf("handleExplain: %v", err)
	}
	if res.IsError {
		t.Fatalf("error: %s", textBlock(t, res))
	}
	var resp struct {
		Name string
	}
	if err := json.Unmarshal([]byte(textBlock(t, res)), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Name != "constant_product_amm" {
		t.Errorf("substring 'constant_product' should match constant_product_amm, got %q", resp.Name)
	}
}

package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// A minimal token-only model inside the core-mode subset.
const coreModelJSON = `{
  "name": "gate",
  "places": [
    {"id": "open", "initial": 1, "capacity": 1},
    {"id": "closed", "initial": 0, "capacity": 1}
  ],
  "transitions": [{"id": "shut"}],
  "arcs": [
    {"from": "open", "to": "shut"},
    {"from": "shut", "to": "closed"}
  ]
}`

func callCodegen(t *testing.T, language string) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_codegen"
	req.Params.Arguments = map[string]any{
		"model":    coreModelJSON,
		"language": language,
	}

	result, err := handleCodegen(context.Background(), req)
	if err != nil {
		t.Fatalf("handleCodegen error: %v", err)
	}
	return result
}

// The tool schema advertises rust/python/javascript; every advertised
// language must actually generate. This is the regression test for the
// description/handler mismatch where javascript and python were advertised
// but rejected.
func TestCodegenCoreLanguages(t *testing.T) {
	wantSnippet := map[string]string{
		"go-core":    "func FireShut(",
		"rust":       "pub fn fire_shut(",
		"python":     "def fire_shut(",
		"javascript": "function fireShut(",
		"js":         "function fireShut(",
	}
	for lang, want := range wantSnippet {
		result := callCodegen(t, lang)
		if result.IsError {
			t.Errorf("%s: tool returned error: %s", lang, resultText(result))
			continue
		}
		text := resultText(result)
		if !strings.Contains(text, "=== gate.") {
			t.Errorf("%s: missing generated file header:\n%s", lang, text)
		}
		if !strings.Contains(text, want) {
			t.Errorf("%s: generated code missing %q", lang, want)
		}
	}
}

// Every core form must generate through the MCP surface, and lean must
// emit theorems.
func TestCodegenFormsAndLean(t *testing.T) {
	for _, form := range []string{"generated", "interpreter", "lambda", "contract"} {
		req := mcp.CallToolRequest{}
		req.Params.Name = "petri_codegen"
		req.Params.Arguments = map[string]any{
			"model":    coreModelJSON,
			"language": "rust",
			"form":     form,
		}
		result, err := handleCodegen(context.Background(), req)
		if err != nil {
			t.Fatalf("%s: %v", form, err)
		}
		if result.IsError {
			t.Errorf("%s: tool returned error: %s", form, resultText(result))
			continue
		}
		if !strings.Contains(resultText(result), "[rust/"+form+"]") {
			t.Errorf("%s: output missing form header", form)
		}
	}

	result := callCodegen(t, "lean")
	if result.IsError {
		t.Fatalf("lean: tool returned error: %s", resultText(result))
	}
	text := resultText(result)
	for _, want := range []string{"=== gate.lean ===", "theorem reachable_closed", "theorem state_count"} {
		if !strings.Contains(text, want) {
			t.Errorf("lean output missing %q", want)
		}
	}
}

// petri_application composes entities into one bundle; a fused pair of
// actions must surface as a coordinator-backed composed app, and the
// reference must be reported, not dropped.
func TestApplicationComposesEntities(t *testing.T) {
	spec := `{
	  "name": "shopdemo",
	  "entities": [
	    {"id": "order",
	     "fields": [{"id": "item_id", "type": "reference", "reference": {"entity": "inventory", "on_delete": "restrict"}}],
	     "states": [{"id": "draft", "initial": true}, {"id": "placed"}],
	     "actions": [{"id": "place_order", "from_states": ["draft"], "to_state": "placed",
	                  "effects": [{"field": "item_id", "value": "item_id"}]}]},
	    {"id": "inventory",
	     "fields": [{"id": "stock", "type": "int64"}],
	     "states": [{"id": "available", "initial": true}, {"id": "reserved"}],
	     "actions": [{"id": "reserve_stock", "from_states": ["available"], "to_state": "reserved",
	                  "effects": [{"field": "stock", "value": "amount", "op": "subtract"}]}]}
	  ]
	}`
	fusions := `[{"id": "order_reserves_stock", "members": [
	  {"entity": "order", "action": "place_order"},
	  {"entity": "inventory", "action": "reserve_stock"}]}]`

	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_application"
	req.Params.Arguments = map[string]any{"spec": spec, "fusions": fusions}

	result, err := handleApplication(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %s", resultText(result))
	}
	text := resultText(result)
	for _, want := range []string{
		"2 entities composed into one bundle",
		"entity order:", "entity inventory:",
		"links: 1",
		"reference: order.item_id -> inventory (on_delete=restrict)",
		"app.go", "flatmodel.go", "order/aggregate.go", "inventory/aggregate.go",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("output missing %q:\n%s", want, text)
		}
	}
}

// petri_bundle takes a raw bundle document with inline models.
func TestBundleToolGeneratesComposedApp(t *testing.T) {
	doc := `{
	  "name": "duo",
	  "subnets": [
	    {"id": "a", "model": {"name": "a",
	      "places": [{"id": "p", "kind": "token", "initial": 1}, {"id": "q", "kind": "token"}],
	      "transitions": [{"id": "go"}],
	      "arcs": [{"from": "p", "to": "go"}, {"from": "go", "to": "q"}]}},
	    {"id": "b", "model": {"name": "b",
	      "places": [{"id": "x", "kind": "token", "initial": 1}, {"id": "y", "kind": "token"}],
	      "transitions": [{"id": "sync"}],
	      "arcs": [{"from": "x", "to": "sync"}, {"from": "sync", "to": "y"}]}}
	  ],
	  "links": [{"id": "handshake", "kind": "event",
	    "from": {"subnet": "a", "transition": "go"},
	    "to": {"subnet": "b", "transition": "sync"}}]
	}`

	req := mcp.CallToolRequest{}
	req.Params.Name = "petri_bundle"
	req.Params.Arguments = map[string]any{"bundle": doc}

	result, err := handleBundle(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %s", resultText(result))
	}
	text := resultText(result)
	for _, want := range []string{"2 subnets, 1 links", "FireHandshake", "=== app.go ==="} {
		if !strings.Contains(text, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestCodegenRejectsUnknownLanguage(t *testing.T) {
	result := callCodegen(t, "cobol")
	if !result.IsError {
		t.Fatal("expected error for unknown language")
	}
	if !strings.Contains(resultText(result), "rust") {
		t.Errorf("error should list supported languages: %s", resultText(result))
	}
}

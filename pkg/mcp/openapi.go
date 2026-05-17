package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/mark3labs/mcp-go/server"
)

// OpenAPISpec generates an OpenAPI 3.0.3 spec from the MCPServer's registered tools.
func OpenAPISpec(s *server.MCPServer) ([]byte, error) {
	tools := s.ListTools()
	if tools == nil {
		tools = map[string]*server.ServerTool{}
	}

	// Sort tool names for stable output
	names := make([]string, 0, len(tools))
	for name := range tools {
		names = append(names, name)
	}
	sort.Strings(names)

	// Build paths
	paths := make(map[string]interface{})
	for _, name := range names {
		st := tools[name]
		tool := st.Tool

		// Build request body schema from tool's InputSchema
		reqSchema := map[string]interface{}{
			"type": "object",
		}
		if tool.InputSchema.Properties != nil && len(tool.InputSchema.Properties) > 0 {
			reqSchema["properties"] = tool.InputSchema.Properties
		}
		if len(tool.InputSchema.Required) > 0 {
			reqSchema["required"] = tool.InputSchema.Required
		}

		path := fmt.Sprintf("/mcp/tools/%s", name)
		paths[path] = map[string]interface{}{
			"post": map[string]interface{}{
				"summary":     tool.Description,
				"operationId": name,
				"tags":        []string{toolTag(name)},
				"requestBody": map[string]interface{}{
					"required": len(tool.InputSchema.Required) > 0,
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": reqSchema,
						},
					},
				},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "Tool result",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/ToolResult",
								},
							},
						},
					},
					"400": map[string]interface{}{
						"description": "Invalid parameters",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/ToolError",
								},
							},
						},
					},
				},
			},
		}
	}

	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":       "petri-pilot MCP Tools API",
			"version":     "1.0.0",
			"description": "Auto-generated OpenAPI spec from MCP tool definitions. These tools are available via the MCP protocol (JSON-RPC over Streamable HTTP at /mcp) or as individual REST endpoints at /mcp/tools/{name}.",
		},
		"servers": []map[string]interface{}{
			{"url": "https://pilot.pflow.xyz", "description": "Production"},
			{"url": "http://localhost:8083", "description": "Development"},
		},
		"paths": paths,
		"components": map[string]interface{}{
			"schemas": map[string]interface{}{
				"ToolResult": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"content": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"type": map[string]interface{}{
										"type": "string",
										"enum": []string{"text"},
									},
									"text": map[string]interface{}{
										"type": "string",
									},
								},
							},
						},
						"isError": map[string]interface{}{
							"type": "boolean",
						},
					},
				},
				"ToolError": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"error": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
		},
	}

	return json.MarshalIndent(spec, "", "  ")
}

// toolTag categorizes a tool by prefix for OpenAPI tag grouping.
func toolTag(name string) string {
	if len(name) > 6 && name[:6] == "petri_" {
		return "petri"
	}
	return "tools"
}

// OpenAPIHandler returns an http.HandlerFunc serving the OpenAPI JSON spec.
func OpenAPIHandler(s *server.MCPServer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		spec, err := OpenAPISpec(s)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(spec)
	}
}

// LandingPageHandler returns an http.HandlerFunc that shows a tool listing page
// instead of hanging on a browser GET to /mcp.
func LandingPageHandler(s *server.MCPServer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only intercept GET requests — let POST/DELETE through to MCP transport
		if r.Method != http.MethodGet {
			return
		}
		// If Accept header wants event-stream (SSE client), don't intercept
		if r.Header.Get("Accept") == "text/event-stream" {
			return
		}

		tools := s.ListTools()
		names := make([]string, 0, len(tools))
		for name := range tools {
			names = append(names, name)
		}
		sort.Strings(names)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>petri-pilot MCP Server</title>
<style>
  body { font-family: system-ui, sans-serif; max-width: 900px; margin: 2em auto; padding: 0 1em; background: #0d1117; color: #c9d1d9; }
  h1 { color: #58a6ff; }
  a { color: #58a6ff; }
  .tool { border: 1px solid #30363d; border-radius: 6px; padding: 12px 16px; margin: 8px 0; background: #161b22; }
  .tool-name { font-family: monospace; font-weight: bold; color: #7ee787; font-size: 1.05em; }
  .tool-desc { color: #8b949e; margin-top: 4px; font-size: 0.9em; }
  .tool-params { margin-top: 6px; font-size: 0.85em; color: #c9d1d9; }
  .param { font-family: monospace; color: #d2a8ff; }
  .required { color: #f85149; font-size: 0.75em; }
  .tag { display: inline-block; padding: 2px 8px; border-radius: 12px; font-size: 0.75em; margin-left: 8px; }
  .tag-petri { background: #1f3d2a; color: #7ee787; }
  .tag-tools { background: #30363d; color: #8b949e; }
  .links { margin: 1em 0; }
  .links a { margin-right: 1.5em; }
  code { background: #30363d; padding: 2px 6px; border-radius: 3px; font-size: 0.9em; }
</style>
</head>
<body>
<h1>petri-pilot MCP Server</h1>
<p>This endpoint serves the <a href="https://modelcontextprotocol.io">Model Context Protocol</a> (MCP) over Streamable HTTP.</p>
<div class="links">
  <a href="/mcp/openapi.json">OpenAPI Spec (JSON)</a>
  <a href="https://github.com/pflow-xyz/petri-pilot">GitHub</a>
</div>
<h2>Available Tools</h2>
`)

		for _, name := range names {
			st := tools[name]
			tool := st.Tool
			tag := toolTag(name)

			fmt.Fprintf(w, `<div class="tool">
  <span class="tool-name">%s</span>
  <span class="tag tag-%s">%s</span>
`, name, tag, tag)

			if tool.Description != "" {
				fmt.Fprintf(w, `  <div class="tool-desc">%s</div>`, tool.Description)
			}

			if tool.InputSchema.Properties != nil && len(tool.InputSchema.Properties) > 0 {
				requiredSet := make(map[string]bool)
				for _, r := range tool.InputSchema.Required {
					requiredSet[r] = true
				}

				paramNames := make([]string, 0, len(tool.InputSchema.Properties))
				for pn := range tool.InputSchema.Properties {
					paramNames = append(paramNames, pn)
				}
				sort.Strings(paramNames)

				fmt.Fprint(w, `  <div class="tool-params">`)
				for _, pn := range paramNames {
					req := ""
					if requiredSet[pn] {
						req = ` <span class="required">required</span>`
					}

					desc := ""
					if prop, ok := tool.InputSchema.Properties[pn].(map[string]interface{}); ok {
						if d, ok := prop["description"].(string); ok {
							desc = " — " + d
						}
					}

					fmt.Fprintf(w, `<span class="param">%s</span>%s%s<br>`, pn, req, desc)
				}
				fmt.Fprint(w, `</div>`)
			}

			fmt.Fprint(w, `</div>`)
		}

		fmt.Fprintf(w, `
<h2>Usage</h2>
<p>Connect via MCP client:</p>
<pre><code>{
  "mcpServers": {
    "petri-pilot": {
      "url": "https://pilot.pflow.xyz/mcp"
    }
  }
}</code></pre>
<p>Or use the <a href="/mcp/openapi.json">OpenAPI spec</a> for REST-style tool invocation.</p>
<p style="color:#8b949e;margin-top:2em;font-size:0.85em">%d tools registered</p>
</body>
</html>`, len(names))
	}
}

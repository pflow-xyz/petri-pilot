package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestHandleDesignWorkflowPrompt(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		description string
		wantErr     bool
	}{
		{
			name:        "with description",
			description: "a simple task management system",
			wantErr:     false,
		},
		{
			name:        "without description",
			description: "",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := mcp.GetPromptRequest{
				Params: mcp.GetPromptParams{
					Name: "design-workflow",
					Arguments: map[string]string{
						"description": tt.description,
					},
				},
			}

			result, err := handleDesignWorkflowPrompt(ctx, request)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleDesignWorkflowPrompt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if result == nil {
					t.Error("handleDesignWorkflowPrompt() returned nil result")
					return
				}

				if len(result.Messages) == 0 {
					t.Error("handleDesignWorkflowPrompt() returned no messages")
				}

				// Verify messages contain key guidance
				hasUserMessage := false
				hasAssistantMessage := false
				for _, msg := range result.Messages {
					if msg.Role == mcp.RoleUser {
						hasUserMessage = true
					}
					if msg.Role == mcp.RoleAssistant {
						hasAssistantMessage = true
						// Check for key guidance sections
						content := getTextFromContent(msg.Content)
						if content == "" {
							t.Error("Assistant message has no text content")
						}
						// Check for presence of key guidance
						if !strings.Contains(content, "Step 1") {
							t.Error("Expected guidance to contain 'Step 1'")
						}
						if !strings.Contains(content, "Places") && !strings.Contains(content, "States") {
							t.Error("Expected guidance to mention Places/States")
						}
						if !strings.Contains(content, "Transitions") {
							t.Error("Expected guidance to mention Transitions")
						}
						if !strings.Contains(content, "Arcs") {
							t.Error("Expected guidance to mention Arcs")
						}
					}
				}

				if !hasUserMessage {
					t.Error("Expected at least one user message")
				}
				if !hasAssistantMessage {
					t.Error("Expected at least one assistant message")
				}
			}
		})
	}
}

func TestHandleAddAccessControlPrompt(t *testing.T) {
	ctx := context.Background()

	sampleModel := `{
		"name": "test-workflow",
		"places": [{"id": "pending"}, {"id": "approved"}],
		"transitions": [{"id": "submit"}, {"id": "approve"}],
		"arcs": []
	}`

	tests := []struct {
		name    string
		model   string
		wantErr bool
	}{
		{
			name:    "without model",
			model:   "",
			wantErr: false,
		},
		{
			name:    "with valid model",
			model:   sampleModel,
			wantErr: false,
		},
		{
			name:    "with invalid JSON",
			model:   "{invalid json}",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := mcp.GetPromptRequest{
				Params: mcp.GetPromptParams{
					Name: "add-access-control",
					Arguments: map[string]string{
						"model": tt.model,
					},
				},
			}

			result, err := handleAddAccessControlPrompt(ctx, request)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleAddAccessControlPrompt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if result == nil {
					t.Error("handleAddAccessControlPrompt() returned nil result")
					return
				}

				if len(result.Messages) == 0 {
					t.Error("handleAddAccessControlPrompt() returned no messages")
				}

				// Verify messages contain key guidance
				for _, msg := range result.Messages {
					if msg.Role == mcp.RoleAssistant {
						content := getTextFromContent(msg.Content)
						if content == "" {
							t.Error("Assistant message has no text content")
						}
						// Check for key guidance sections
						if !strings.Contains(content, "roles") {
							t.Error("Expected guidance to mention 'roles'")
						}
						if !strings.Contains(content, "access") {
							t.Error("Expected guidance to mention 'access'")
						}

						// If model was provided, check for transition-specific guidance
						if tt.model != "" && tt.model != "{invalid json}" {
							if !strings.Contains(content, "submit") && !strings.Contains(content, "approve") {
								t.Error("Expected guidance to mention transitions from model")
							}
						}
					}
				}
			}
		})
	}
}

func TestHandleAddViewsPrompt(t *testing.T) {
	ctx := context.Background()

	sampleModel := `{
		"name": "test-workflow",
		"places": [{"id": "pending"}, {"id": "approved"}],
		"transitions": [{"id": "submit"}, {"id": "approve"}],
		"arcs": []
	}`

	tests := []struct {
		name    string
		model   string
		wantErr bool
	}{
		{
			name:    "without model",
			model:   "",
			wantErr: false,
		},
		{
			name:    "with valid model",
			model:   sampleModel,
			wantErr: false,
		},
		{
			name:    "with invalid JSON",
			model:   "{invalid json}",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := mcp.GetPromptRequest{
				Params: mcp.GetPromptParams{
					Name: "add-views",
					Arguments: map[string]string{
						"model": tt.model,
					},
				},
			}

			result, err := handleAddViewsPrompt(ctx, request)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleAddViewsPrompt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if result == nil {
					t.Error("handleAddViewsPrompt() returned nil result")
					return
				}

				if len(result.Messages) == 0 {
					t.Error("handleAddViewsPrompt() returned no messages")
				}

				// Verify messages contain key guidance
				for _, msg := range result.Messages {
					if msg.Role == mcp.RoleAssistant {
						content := getTextFromContent(msg.Content)
						if content == "" {
							t.Error("Assistant message has no text content")
						}
						// Check for key guidance sections
						if !strings.Contains(content, "view") {
							t.Error("Expected guidance to mention 'view'")
						}
						if !strings.Contains(content, "fields") {
							t.Error("Expected guidance to mention 'fields'")
						}
						if !strings.Contains(content, "kind") {
							t.Error("Expected guidance to mention view 'kind'")
						}

						// If model was provided, check for model-specific guidance
						if tt.model != "" && tt.model != "{invalid json}" {
							if !strings.Contains(content, "pending") && !strings.Contains(content, "approved") {
								t.Error("Expected guidance to mention states from model")
							}
						}
					}
				}
			}
		})
	}
}

// Helper function to extract text content from mcp.Content
func getTextFromContent(content mcp.Content) string {
	// Try to get text content
	if textContent, ok := content.(mcp.TextContent); ok {
		return textContent.Text
	}

	// Try to unmarshal as JSON to get text field
	data, err := json.Marshal(content)
	if err != nil {
		return ""
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return ""
	}

	if text, ok := obj["text"].(string); ok {
		return text
	}

	return ""
}

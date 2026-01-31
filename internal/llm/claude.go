package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// ClaudeClient implements the Client interface using Anthropic's Claude API.
type ClaudeClient struct {
	client anthropic.Client
	model  anthropic.Model
}

// ClaudeOptions configures the Claude client.
type ClaudeOptions struct {
	// APIKey is the Anthropic API key. If empty, uses ANTHROPIC_API_KEY env var.
	APIKey string

	// Model specifies which Claude model to use.
	// Defaults to claude-sonnet-4-20250514.
	Model string
}

// DefaultClaudeOptions returns sensible defaults.
func DefaultClaudeOptions() ClaudeOptions {
	return ClaudeOptions{
		Model: "claude-sonnet-4-20250514",
	}
}

// NewClaudeClient creates a Claude client with the given options.
func NewClaudeClient(opts ClaudeOptions) *ClaudeClient {
	var clientOpts []option.RequestOption
	if opts.APIKey != "" {
		clientOpts = append(clientOpts, option.WithAPIKey(opts.APIKey))
	}

	model := anthropic.Model(opts.Model)
	if opts.Model == "" {
		model = anthropic.ModelClaudeSonnet4_20250514
	}

	return &ClaudeClient{
		client: anthropic.NewClient(clientOpts...),
		model:  model,
	}
}

// Complete generates a completion using Claude.
func (c *ClaudeClient) Complete(ctx context.Context, req Request) (*Response, error) {
	// Build message params
	params := anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: int64(req.MaxTokens),
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(req.Prompt)),
		},
	}

	// Add system prompt if provided
	if req.System != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: req.System},
		}
	}

	// Set temperature if specified
	if req.Temperature > 0 {
		params.Temperature = anthropic.Float(req.Temperature)
	}

	// Make the API call
	message, err := c.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("claude api error: %w", err)
	}

	// Extract text content from response
	var content strings.Builder
	for _, block := range message.Content {
		if block.Type == "text" {
			content.WriteString(block.Text)
		}
	}

	return &Response{
		Content: content.String(),
		Usage: Usage{
			InputTokens:  int(message.Usage.InputTokens),
			OutputTokens: int(message.Usage.OutputTokens),
		},
	}, nil
}

// Package llm provides an abstraction layer for LLM providers.
package llm

import "context"

// Client defines the interface for LLM providers.
type Client interface {
	// Complete generates a completion for the given request.
	Complete(ctx context.Context, req Request) (*Response, error)
}

// Request represents an LLM completion request.
type Request struct {
	// Prompt is the input text
	Prompt string

	// Temperature controls randomness (0-1)
	Temperature float64

	// MaxTokens limits response length
	MaxTokens int

	// System provides system-level instructions
	System string
}

// Response contains the LLM output.
type Response struct {
	// Content is the generated text
	Content string

	// Usage tracks token consumption
	Usage Usage
}

// Usage tracks API consumption.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

package mcp

import (
	"os"
	"testing"
)

func TestMarkdownPreview(t *testing.T) {
	// Skip in CI environment (no Chrome available)
	if os.Getenv("CI") != "" {
		t.Skip("Skipping markdown preview test in CI - no Chrome available")
	}

	// Skip if no Chrome available locally
	if os.Getenv("PUPPETEER_EXECUTABLE_PATH") == "" {
		// Try to find Chrome on macOS
		if _, err := os.Stat("/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"); err != nil {
			t.Skip("Chrome not found, skipping test")
		}
		os.Setenv("PUPPETEER_EXECUTABLE_PATH", "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome")
	}

	manager := &E2EManager{
		sessions: make(map[string]*BrowserSession),
	}

	markdown := `# Hello World

This is a test.

## Features
- Item 1
- Item 2

` + "```" + `bash
echo "hello"
` + "```" + `
`

	result, err := manager.PreviewMarkdown(markdown)
	if err != nil {
		t.Fatalf("PreviewMarkdown failed: %v", err)
	}

	if len(result.Screenshot) == 0 {
		t.Error("Screenshot is empty")
	}

	// Check that screenshot is not just white pixels
	// A non-blank PNG should have more than just the header and some white pixel data
	if len(result.Screenshot) < 5000 {
		t.Logf("Screenshot size: %d bytes", len(result.Screenshot))
		t.Error("Screenshot appears to be too small - likely blank")
	}

	if len(result.Errors) > 0 {
		t.Errorf("Render errors: %v", result.Errors)
	}

	t.Logf("Screenshot size: %d bytes, Errors: %d, Warnings: %d",
		len(result.Screenshot), len(result.Errors), len(result.Warnings))
}

func TestMarkdownPreviewWithMermaid(t *testing.T) {
	// Skip in CI environment (no Chrome available)
	if os.Getenv("CI") != "" {
		t.Skip("Skipping markdown preview test in CI - no Chrome available")
	}

	// Skip if no Chrome available locally
	if os.Getenv("PUPPETEER_EXECUTABLE_PATH") == "" {
		// Try to find Chrome on macOS
		if _, err := os.Stat("/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"); err != nil {
			t.Skip("Chrome not found, skipping test")
		}
		os.Setenv("PUPPETEER_EXECUTABLE_PATH", "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome")
	}

	manager := &E2EManager{
		sessions: make(map[string]*BrowserSession),
	}

	markdown := `# Test with Mermaid

` + "```" + `mermaid
flowchart TD
    A[Start] --> B[End]
` + "```" + `
`

	result, err := manager.PreviewMarkdown(markdown)
	if err != nil {
		t.Fatalf("PreviewMarkdown failed: %v", err)
	}

	if len(result.Screenshot) == 0 {
		t.Error("Screenshot is empty")
	}

	if len(result.Screenshot) < 5000 {
		t.Logf("Screenshot size: %d bytes", len(result.Screenshot))
		t.Error("Screenshot appears to be too small - likely blank")
	}

	t.Logf("Screenshot size: %d bytes, Errors: %d, Warnings: %d",
		len(result.Screenshot), len(result.Errors), len(result.Warnings))
}

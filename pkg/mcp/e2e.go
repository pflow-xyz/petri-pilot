// Package mcp provides e2e testing tools for MCP.
package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/mark3labs/mcp-go/mcp"
)

// E2EManager manages headless browser sessions for e2e testing.
type E2EManager struct {
	mu       sync.RWMutex
	sessions map[string]*BrowserSession
	counter  int
}

// CDPEvent represents a Chrome DevTools Protocol event captured from the browser.
type CDPEvent struct {
	Type      string    `json:"type"`                // console, request, response, exception
	Subtype   string    `json:"subtype,omitempty"`   // log, warn, error, info (for console)
	Message   string    `json:"message"`             // Main content
	Detail    string    `json:"detail,omitempty"`    // Additional detail (URL, status, etc.)
	Timestamp time.Time `json:"timestamp"`
}

// BrowserSession represents a headless browser session.
type BrowserSession struct {
	ID           string
	URL          string
	DebugSession string // Debug session ID from the app
	ctx          context.Context
	cancel       context.CancelFunc
	allocCancel  context.CancelFunc

	// CDP event capture
	events   []CDPEvent
	eventsMu sync.RWMutex
	maxEvents int
}

var e2eManager = &E2EManager{
	sessions: make(map[string]*BrowserSession),
}

// --- Tool Definitions ---

func e2eStartBrowserTool() mcp.Tool {
	return mcp.NewTool("e2e_start_browser",
		mcp.WithDescription("Start a headless browser session pointing to a URL. Returns the browser session ID and debug session ID."),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("The URL to navigate to (e.g., http://localhost:8080)"),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Timeout in seconds for browser operations (default: 30)"),
		),
	)
}

func e2eListSessionsTool() mcp.Tool {
	return mcp.NewTool("e2e_list_sessions",
		mcp.WithDescription("List all active headless browser sessions and their debug session IDs."),
	)
}

func e2eEvalTool() mcp.Tool {
	return mcp.NewTool("e2e_eval",
		mcp.WithDescription("Evaluate JavaScript code in a browser session via the app's debug endpoint. Returns the result. Optionally takes a screenshot after execution."),
		mcp.WithString("session_id",
			mcp.Required(),
			mcp.Description("The browser session ID (from e2e_start_browser)"),
		),
		mcp.WithString("code",
			mcp.Required(),
			mcp.Description("JavaScript code to evaluate in the browser context"),
		),
		mcp.WithBoolean("screenshot",
			mcp.Description("Take a screenshot after eval (default: false)"),
		),
		mcp.WithNumber("wait_ms",
			mcp.Description("Wait this many milliseconds before taking screenshot (default: 0)"),
		),
	)
}

func e2eStopBrowserTool() mcp.Tool {
	return mcp.NewTool("e2e_stop_browser",
		mcp.WithDescription("Stop a headless browser session and clean up resources."),
		mcp.WithString("session_id",
			mcp.Required(),
			mcp.Description("The browser session ID to stop"),
		),
	)
}

func e2eScreenshotTool() mcp.Tool {
	return mcp.NewTool("e2e_screenshot",
		mcp.WithDescription("Take a screenshot of the current browser state. Returns base64-encoded PNG."),
		mcp.WithString("session_id",
			mcp.Required(),
			mcp.Description("The browser session ID"),
		),
	)
}

func e2eEventsTool() mcp.Tool {
	return mcp.NewTool("e2e_events",
		mcp.WithDescription("Get captured CDP events (console logs, network requests, exceptions) from a browser session."),
		mcp.WithString("session_id",
			mcp.Required(),
			mcp.Description("The browser session ID"),
		),
		mcp.WithString("types",
			mcp.Description("Comma-separated event types to filter: console,request,response,exception (default: all)"),
		),
		mcp.WithBoolean("clear",
			mcp.Description("Clear events after retrieving (default: false)"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of events to return (default: 100, 0 for all)"),
		),
	)
}

func e2eNavigateTool() mcp.Tool {
	return mcp.NewTool("e2e_navigate",
		mcp.WithDescription("Navigate to a URL within an existing browser session."),
		mcp.WithString("session_id",
			mcp.Required(),
			mcp.Description("The browser session ID"),
		),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("The URL to navigate to"),
		),
		mcp.WithBoolean("wait_ready",
			mcp.Description("Wait for document ready state (default: true)"),
		),
	)
}

func e2eClickTool() mcp.Tool {
	return mcp.NewTool("e2e_click",
		mcp.WithDescription("Click an element by CSS selector."),
		mcp.WithString("session_id",
			mcp.Required(),
			mcp.Description("The browser session ID"),
		),
		mcp.WithString("selector",
			mcp.Required(),
			mcp.Description("CSS selector for the element to click"),
		),
	)
}

func e2eTypeTool() mcp.Tool {
	return mcp.NewTool("e2e_type",
		mcp.WithDescription("Type text into an input element."),
		mcp.WithString("session_id",
			mcp.Required(),
			mcp.Description("The browser session ID"),
		),
		mcp.WithString("selector",
			mcp.Required(),
			mcp.Description("CSS selector for the input element"),
		),
		mcp.WithString("text",
			mcp.Required(),
			mcp.Description("Text to type into the input"),
		),
		mcp.WithBoolean("clear",
			mcp.Description("Clear existing text before typing (default: true)"),
		),
	)
}

func e2eRunTool() mcp.Tool {
	return mcp.NewTool("e2e_run",
		mcp.WithDescription("Execute JavaScript directly in the browser context. Unlike e2e_eval, this runs directly via chromedp without needing a debug session. Use 'return' to get a result."),
		mcp.WithString("session_id",
			mcp.Required(),
			mcp.Description("The browser session ID"),
		),
		mcp.WithString("code",
			mcp.Required(),
			mcp.Description("JavaScript code to execute. Wrap in async IIFE for async code: (async () => { ... })()"),
		),
		mcp.WithBoolean("screenshot",
			mcp.Description("Take a screenshot after execution (default: false)"),
		),
		mcp.WithNumber("wait_ms",
			mcp.Description("Wait this many milliseconds after execution before returning/screenshot (default: 0)"),
		),
	)
}

func e2eWaitTool() mcp.Tool {
	return mcp.NewTool("e2e_wait",
		mcp.WithDescription("Wait for an element to be visible, or for a JavaScript condition to be true."),
		mcp.WithString("session_id",
			mcp.Required(),
			mcp.Description("The browser session ID"),
		),
		mcp.WithString("selector",
			mcp.Description("CSS selector to wait for (element must be visible)"),
		),
		mcp.WithString("condition",
			mcp.Description("JavaScript expression that should evaluate to true"),
		),
		mcp.WithNumber("timeout_ms",
			mcp.Description("Maximum time to wait in milliseconds (default: 10000)"),
		),
	)
}

func markdownPreviewTool() mcp.Tool {
	return mcp.NewTool("markdown_preview",
		mcp.WithDescription("Render markdown like GitHub and take a screenshot. Useful for validating generated README files. Returns base64-encoded PNG and any rendering errors."),
		mcp.WithString("markdown",
			mcp.Description("Raw markdown content to render"),
		),
		mcp.WithString("file_path",
			mcp.Description("Path to a markdown file to render (alternative to markdown parameter)"),
		),
	)
}

// --- Tool Handlers ---

func handleE2EStartBrowser(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	url, err := request.RequireString("url")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing url parameter: %v", err)), nil
	}

	timeout := request.GetInt("timeout", 30)

	// Create browser session
	session, err := e2eManager.StartBrowser(ctx, url, time.Duration(timeout)*time.Second)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to start browser: %v", err)), nil
	}

	result := map[string]any{
		"browser_session_id": session.ID,
		"debug_session_id":   session.DebugSession,
		"url":                session.URL,
		"status":             "connected",
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(output)), nil
}

func handleE2EListSessions(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessions := e2eManager.ListSessions()

	result := map[string]any{
		"sessions": sessions,
		"count":    len(sessions),
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(output)), nil
}

func handleE2EEval(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionID, err := request.RequireString("session_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing session_id parameter: %v", err)), nil
	}

	code, err := request.RequireString("code")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing code parameter: %v", err)), nil
	}

	takeScreenshot := request.GetBool("screenshot", false)
	waitMs := request.GetInt("wait_ms", 0)

	result, err := e2eManager.Eval(ctx, sessionID, code)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("eval failed: %v", err)), nil
	}

	// If screenshot requested, wait and take it
	if takeScreenshot {
		if waitMs > 0 {
			time.Sleep(time.Duration(waitMs) * time.Millisecond)
		}

		screenshot, err := e2eManager.Screenshot(sessionID)
		if err != nil {
			// Return result with screenshot error
			output, _ := json.MarshalIndent(map[string]any{
				"result":           result,
				"screenshot_error": err.Error(),
			}, "", "  ")
			return mcp.NewToolResultText(string(output)), nil
		}

		// Return both result and screenshot
		output, _ := json.MarshalIndent(result, "", "  ")
		b64 := base64.StdEncoding.EncodeToString(screenshot)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(string(output)),
				mcp.NewImageContent(b64, "image/png"),
			},
		}, nil
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(output)), nil
}

func handleE2EStopBrowser(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionID, err := request.RequireString("session_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing session_id parameter: %v", err)), nil
	}

	if err := e2eManager.StopBrowser(sessionID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to stop browser: %v", err)), nil
	}

	result := map[string]any{
		"session_id": sessionID,
		"status":     "stopped",
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(output)), nil
}

func handleE2EScreenshot(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionID, err := request.RequireString("session_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing session_id parameter: %v", err)), nil
	}

	screenshot, err := e2eManager.Screenshot(sessionID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("screenshot failed: %v", err)), nil
	}

	// Encode as base64 and return as image content
	b64 := base64.StdEncoding.EncodeToString(screenshot)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewImageContent(b64, "image/png"),
		},
	}, nil
}

func handleE2EEvents(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionID, err := request.RequireString("session_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing session_id parameter: %v", err)), nil
	}

	typesStr := request.GetString("types", "")
	clear := request.GetBool("clear", false)
	limit := request.GetInt("limit", 100)

	var typeFilter []string
	if typesStr != "" {
		typeFilter = strings.Split(typesStr, ",")
		for i := range typeFilter {
			typeFilter[i] = strings.TrimSpace(typeFilter[i])
		}
	}

	events, err := e2eManager.GetEvents(sessionID, typeFilter, clear, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get events: %v", err)), nil
	}

	result := map[string]any{
		"session_id": sessionID,
		"count":      len(events),
		"events":     events,
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(output)), nil
}

func handleE2ENavigate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionID, err := request.RequireString("session_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing session_id: %v", err)), nil
	}

	url, err := request.RequireString("url")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing url: %v", err)), nil
	}

	waitReady := request.GetBool("wait_ready", true)

	if err := e2eManager.Navigate(sessionID, url, waitReady); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("navigation failed: %v", err)), nil
	}

	result := map[string]any{
		"session_id": sessionID,
		"url":        url,
		"status":     "navigated",
	}
	output, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(output)), nil
}

func handleE2EClick(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionID, err := request.RequireString("session_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing session_id: %v", err)), nil
	}

	selector, err := request.RequireString("selector")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing selector: %v", err)), nil
	}

	if err := e2eManager.Click(sessionID, selector); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("click failed: %v", err)), nil
	}

	result := map[string]any{
		"session_id": sessionID,
		"selector":   selector,
		"status":     "clicked",
	}
	output, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(output)), nil
}

func handleE2EType(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionID, err := request.RequireString("session_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing session_id: %v", err)), nil
	}

	selector, err := request.RequireString("selector")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing selector: %v", err)), nil
	}

	text, err := request.RequireString("text")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing text: %v", err)), nil
	}

	clearFirst := request.GetBool("clear", true)

	if err := e2eManager.Type(sessionID, selector, text, clearFirst); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("type failed: %v", err)), nil
	}

	result := map[string]any{
		"session_id": sessionID,
		"selector":   selector,
		"status":     "typed",
	}
	output, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(output)), nil
}

func handleE2ERun(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionID, err := request.RequireString("session_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing session_id: %v", err)), nil
	}

	code, err := request.RequireString("code")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing code: %v", err)), nil
	}

	takeScreenshot := request.GetBool("screenshot", false)
	waitMs := request.GetInt("wait_ms", 0)

	result, err := e2eManager.Run(sessionID, code)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("run failed: %v", err)), nil
	}

	// Wait if requested
	if waitMs > 0 {
		time.Sleep(time.Duration(waitMs) * time.Millisecond)
	}

	// If screenshot requested, take it
	if takeScreenshot {
		screenshot, err := e2eManager.Screenshot(sessionID)
		if err != nil {
			output, _ := json.MarshalIndent(map[string]any{
				"result":           result,
				"screenshot_error": err.Error(),
			}, "", "  ")
			return mcp.NewToolResultText(string(output)), nil
		}

		output, _ := json.MarshalIndent(result, "", "  ")
		b64 := base64.StdEncoding.EncodeToString(screenshot)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(string(output)),
				mcp.NewImageContent(b64, "image/png"),
			},
		}, nil
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(output)), nil
}

func handleE2EWait(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionID, err := request.RequireString("session_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing session_id: %v", err)), nil
	}

	selector := request.GetString("selector", "")
	condition := request.GetString("condition", "")
	timeoutMs := request.GetInt("timeout_ms", 10000)

	if selector == "" && condition == "" {
		return mcp.NewToolResultError("either 'selector' or 'condition' is required"), nil
	}

	if err := e2eManager.Wait(sessionID, selector, condition, time.Duration(timeoutMs)*time.Millisecond); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("wait failed: %v", err)), nil
	}

	result := map[string]any{
		"session_id": sessionID,
		"status":     "ready",
	}
	if selector != "" {
		result["selector"] = selector
	}
	if condition != "" {
		result["condition"] = condition
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(output)), nil
}

func handleMarkdownPreview(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	markdown := request.GetString("markdown", "")
	filePath := request.GetString("file_path", "")

	// Either markdown content or file path is required
	if markdown == "" && filePath == "" {
		return mcp.NewToolResultError("either 'markdown' or 'file_path' parameter is required"), nil
	}

	// If file path is provided, read the file
	if filePath != "" {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to read file: %v", err)), nil
		}
		markdown = string(content)
	}

	// Preview the markdown
	result, err := e2eManager.PreviewMarkdown(markdown)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("preview failed: %v", err)), nil
	}

	// Build response
	var contents []mcp.Content

	// Add any errors as text
	if len(result.Errors) > 0 {
		errorText := "Rendering Errors:\n"
		for _, e := range result.Errors {
			errorText += "  - " + e + "\n"
		}
		contents = append(contents, mcp.NewTextContent(errorText))
	}

	// Add the screenshot
	b64 := base64.StdEncoding.EncodeToString(result.Screenshot)
	contents = append(contents, mcp.NewImageContent(b64, "image/png"))

	// Add summary
	summary := fmt.Sprintf("Markdown preview complete. Errors: %d, Warnings: %d",
		len(result.Errors), len(result.Warnings))
	contents = append(contents, mcp.NewTextContent(summary))

	return &mcp.CallToolResult{Content: contents}, nil
}

// --- E2EManager Methods ---

// StartBrowser launches a headless browser and navigates to the given URL.
func (m *E2EManager) StartBrowser(ctx context.Context, url string, timeout time.Duration) (*BrowserSession, error) {
	// Create browser context with headless options
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
	)

	// Support PUPPETEER_EXECUTABLE_PATH for custom Chrome location (same as e2e tests)
	if chromePath := os.Getenv("PUPPETEER_EXECUTABLE_PATH"); chromePath != "" {
		opts = append(opts, chromedp.ExecPath(chromePath))
	}

	// Create allocator without parent context timeout to keep browser alive
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)

	// Generate session ID
	m.mu.Lock()
	m.counter++
	sessionID := fmt.Sprintf("browser-%d", m.counter)
	m.mu.Unlock()

	// Create session early so we can attach event listeners
	session := &BrowserSession{
		ID:          sessionID,
		URL:         url,
		ctx:         browserCtx,
		cancel:      browserCancel,
		allocCancel: allocCancel,
		events:      make([]CDPEvent, 0, 100),
		maxEvents:   1000,
	}

	// Set up CDP event listeners for console, network, and exceptions
	chromedp.ListenTarget(browserCtx, func(ev interface{}) {
		switch e := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			// Capture console.log, console.warn, console.error, etc.
			var textParts []string
			for _, arg := range e.Args {
				if arg.Value != nil {
					// Remove quotes from JSON string values
					val := string(arg.Value)
					if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
						val = val[1 : len(val)-1]
					}
					textParts = append(textParts, val)
				} else if arg.Description != "" {
					textParts = append(textParts, arg.Description)
				}
			}
			session.addEvent("console", e.Type.String(), strings.Join(textParts, " "), "")

		case *runtime.EventExceptionThrown:
			// Capture uncaught exceptions
			msg := ""
			detail := ""
			if e.ExceptionDetails != nil {
				if e.ExceptionDetails.Exception != nil && e.ExceptionDetails.Exception.Description != "" {
					msg = e.ExceptionDetails.Exception.Description
				} else if e.ExceptionDetails.Text != "" {
					msg = e.ExceptionDetails.Text
				}
				if e.ExceptionDetails.URL != "" {
					detail = fmt.Sprintf("%s:%d:%d", e.ExceptionDetails.URL, e.ExceptionDetails.LineNumber, e.ExceptionDetails.ColumnNumber)
				}
			}
			session.addEvent("exception", "error", msg, detail)

		case *network.EventRequestWillBeSent:
			// Capture outgoing network requests
			session.addEvent("request", e.Request.Method, e.Request.URL, "")

		case *network.EventResponseReceived:
			// Capture network responses
			status := fmt.Sprintf("%d", e.Response.Status)
			session.addEvent("response", status, e.Response.URL, e.Response.MimeType)
		}
	})

	// Navigate to URL with timeout - use chromedp.WithTimeout action instead of context
	err := chromedp.Run(browserCtx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
		// Wait a bit for WebSocket connection to establish
		chromedp.Sleep(3*time.Second),
	)

	if err != nil {
		allocCancel()
		browserCancel()
		return nil, fmt.Errorf("browser navigation failed: %w", err)
	}

	// Try to get the debug session ID from the page (may not be set if frontend doesn't expose it)
	var debugSessionID string
	_ = chromedp.Run(browserCtx,
		chromedp.Evaluate(`(window.debugSessionId && typeof window.debugSessionId === 'string') ? window.debugSessionId : ""`, &debugSessionID),
	)

	// If we didn't get a debug session ID from the page, try the API
	if debugSessionID == "" {
		debugSessionID, _ = m.fetchDebugSession(url)
	}

	session.DebugSession = debugSessionID

	m.mu.Lock()
	m.sessions[sessionID] = session
	m.mu.Unlock()

	return session, nil
}

// addEvent adds a CDP event to the session's event buffer.
func (s *BrowserSession) addEvent(eventType, subtype, message, detail string) {
	s.eventsMu.Lock()
	defer s.eventsMu.Unlock()

	event := CDPEvent{
		Type:      eventType,
		Subtype:   subtype,
		Message:   message,
		Detail:    detail,
		Timestamp: time.Now(),
	}

	s.events = append(s.events, event)

	// Trim if over max capacity
	if len(s.events) > s.maxEvents {
		s.events = s.events[len(s.events)-s.maxEvents:]
	}
}

// fetchDebugSession tries to get the latest debug session from the app's API.
func (m *E2EManager) fetchDebugSession(baseURL string) (string, error) {
	resp, err := http.Get(baseURL + "/api/debug/sessions")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Sessions []struct {
			ID string `json:"id"`
		} `json:"sessions"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Sessions) > 0 {
		// Return the most recent session
		return result.Sessions[len(result.Sessions)-1].ID, nil
	}

	return "", nil
}

// ListSessions returns information about all active browser sessions.
func (m *E2EManager) ListSessions() []map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]map[string]any, 0, len(m.sessions))
	for _, session := range m.sessions {
		result = append(result, map[string]any{
			"browser_session_id": session.ID,
			"debug_session_id":   session.DebugSession,
			"url":                session.URL,
		})
	}
	return result
}

// Eval sends JavaScript code to the browser session for evaluation via the debug API.
func (m *E2EManager) Eval(ctx context.Context, sessionID string, code string) (any, error) {
	m.mu.RLock()
	session, ok := m.sessions[sessionID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Re-fetch the current debug session ID (sessions can change)
	debugSessionID, err := m.fetchDebugSession(session.URL)
	if err != nil || debugSessionID == "" {
		// Fall back to stored session ID
		debugSessionID = session.DebugSession
	}

	if debugSessionID == "" {
		return nil, fmt.Errorf("no debug session connected for browser session %s", sessionID)
	}

	// Update stored debug session
	m.mu.Lock()
	if s, ok := m.sessions[sessionID]; ok {
		s.DebugSession = debugSessionID
	}
	m.mu.Unlock()

	// Call the debug eval API
	evalURL := session.URL + "/api/debug/sessions/" + debugSessionID + "/eval"

	reqBody, _ := json.Marshal(map[string]string{"code": code})
	resp, err := http.Post(evalURL, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("eval request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("eval failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result any
	if err := json.Unmarshal(body, &result); err != nil {
		return string(body), nil
	}

	return result, nil
}

// StopBrowser closes a browser session and cleans up resources.
func (m *E2EManager) StopBrowser(sessionID string) error {
	m.mu.Lock()
	session, ok := m.sessions[sessionID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("session not found: %s", sessionID)
	}
	delete(m.sessions, sessionID)
	m.mu.Unlock()

	// Cancel browser context
	if session.cancel != nil {
		session.cancel()
	}
	if session.allocCancel != nil {
		session.allocCancel()
	}

	return nil
}

// GetEvents returns captured CDP events from a browser session.
func (m *E2EManager) GetEvents(sessionID string, typeFilter []string, clear bool, limit int) ([]CDPEvent, error) {
	m.mu.RLock()
	session, ok := m.sessions[sessionID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	session.eventsMu.Lock()
	defer session.eventsMu.Unlock()

	// Filter by type if specified
	var filtered []CDPEvent
	if len(typeFilter) == 0 {
		filtered = make([]CDPEvent, len(session.events))
		copy(filtered, session.events)
	} else {
		typeSet := make(map[string]bool)
		for _, t := range typeFilter {
			typeSet[t] = true
		}
		for _, ev := range session.events {
			if typeSet[ev.Type] {
				filtered = append(filtered, ev)
			}
		}
	}

	// Apply limit (0 means no limit)
	if limit > 0 && len(filtered) > limit {
		// Return most recent events
		filtered = filtered[len(filtered)-limit:]
	}

	// Clear events if requested
	if clear {
		session.events = make([]CDPEvent, 0, 100)
	}

	return filtered, nil
}

// Navigate navigates to a URL within an existing browser session.
func (m *E2EManager) Navigate(sessionID, url string, waitReady bool) error {
	m.mu.RLock()
	session, ok := m.sessions[sessionID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	actions := []chromedp.Action{chromedp.Navigate(url)}
	if waitReady {
		actions = append(actions, chromedp.WaitReady("body"))
	}

	if err := chromedp.Run(session.ctx, actions...); err != nil {
		return fmt.Errorf("navigate failed: %w", err)
	}

	// Update session URL
	m.mu.Lock()
	if s, ok := m.sessions[sessionID]; ok {
		s.URL = url
	}
	m.mu.Unlock()

	return nil
}

// Click clicks an element by CSS selector.
func (m *E2EManager) Click(sessionID, selector string) error {
	m.mu.RLock()
	session, ok := m.sessions[sessionID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	err := chromedp.Run(session.ctx,
		chromedp.WaitVisible(selector),
		chromedp.Click(selector),
	)
	if err != nil {
		return fmt.Errorf("click failed: %w", err)
	}

	return nil
}

// Type types text into an input element.
func (m *E2EManager) Type(sessionID, selector, text string, clearFirst bool) error {
	m.mu.RLock()
	session, ok := m.sessions[sessionID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	actions := []chromedp.Action{chromedp.WaitVisible(selector)}
	if clearFirst {
		actions = append(actions, chromedp.Clear(selector))
	}
	actions = append(actions, chromedp.SendKeys(selector, text))

	if err := chromedp.Run(session.ctx, actions...); err != nil {
		return fmt.Errorf("type failed: %w", err)
	}

	return nil
}

// Run executes JavaScript directly in the browser context.
func (m *E2EManager) Run(sessionID, code string) (any, error) {
	m.mu.RLock()
	session, ok := m.sessions[sessionID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	var result any
	err := chromedp.Run(session.ctx,
		chromedp.Evaluate(code, &result),
	)
	if err != nil {
		return nil, fmt.Errorf("run failed: %w", err)
	}

	return result, nil
}

// Wait waits for an element to be visible or a condition to be true.
func (m *E2EManager) Wait(sessionID, selector, condition string, timeout time.Duration) error {
	m.mu.RLock()
	session, ok := m.sessions[sessionID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Create a timeout context
	ctx, cancel := context.WithTimeout(session.ctx, timeout)
	defer cancel()

	if selector != "" {
		if err := chromedp.Run(ctx, chromedp.WaitVisible(selector)); err != nil {
			return fmt.Errorf("wait for selector failed: %w", err)
		}
	}

	if condition != "" {
		if err := chromedp.Run(ctx,
			chromedp.Poll(condition, nil, chromedp.WithPollingInterval(100*time.Millisecond)),
		); err != nil {
			return fmt.Errorf("wait for condition failed: %w", err)
		}
	}

	return nil
}

// Screenshot takes a screenshot of the current browser state.
func (m *E2EManager) Screenshot(sessionID string) ([]byte, error) {
	m.mu.RLock()
	session, ok := m.sessions[sessionID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	var screenshot []byte
	err := chromedp.Run(session.ctx,
		chromedp.FullScreenshot(&screenshot, 90),
	)
	if err != nil {
		return nil, fmt.Errorf("screenshot failed: %w", err)
	}

	return screenshot, nil
}

// StopAllBrowsers closes all browser sessions.
func (m *E2EManager) StopAllBrowsers() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, session := range m.sessions {
		if session.cancel != nil {
			session.cancel()
		}
		if session.allocCancel != nil {
			session.allocCancel()
		}
	}
	m.sessions = make(map[string]*BrowserSession)
}

// MarkdownPreviewResult contains the result of markdown preview.
type MarkdownPreviewResult struct {
	Screenshot []byte   `json:"screenshot"`
	Errors     []string `json:"errors,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
}

// PreviewMarkdown renders markdown with GitHub-like styling and takes a screenshot.
func (m *E2EManager) PreviewMarkdown(markdown string) (*MarkdownPreviewResult, error) {
	// Create a temporary browser context with file access enabled
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("allow-file-access-from-files", true),
		chromedp.Flag("disable-web-security", true),
		chromedp.WindowSize(1200, 800),
	)

	// Check for custom Chrome path
	if chromePath := os.Getenv("PUPPETEER_EXECUTABLE_PATH"); chromePath != "" {
		opts = append(opts, chromedp.ExecPath(chromePath))
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Set timeout
	ctx, cancelTimeout := context.WithTimeout(ctx, 30*time.Second)
	defer cancelTimeout()

	// Escape markdown for embedding in JavaScript - convert to JSON string for safety
	markdownJSON, err := json.Marshal(markdown)
	if err != nil {
		return nil, fmt.Errorf("failed to encode markdown: %w", err)
	}

	// HTML template with GitHub-like markdown styling
	// Uses JSON-encoded markdown to safely embed content
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
body {
	font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
	font-size: 16px;
	line-height: 1.5;
	color: #24292f;
	background-color: #ffffff;
	max-width: 980px;
	margin: 0 auto;
	padding: 45px;
}
h1, h2, h3, h4, h5, h6 {
	margin-top: 24px;
	margin-bottom: 16px;
	font-weight: 600;
	line-height: 1.25;
}
h1 { font-size: 2em; border-bottom: 1px solid #d0d7de; padding-bottom: .3em; }
h2 { font-size: 1.5em; border-bottom: 1px solid #d0d7de; padding-bottom: .3em; }
h3 { font-size: 1.25em; }
code {
	padding: .2em .4em;
	margin: 0;
	font-size: 85%%;
	background-color: rgba(175, 184, 193, 0.2);
	border-radius: 6px;
	font-family: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace;
}
pre {
	padding: 16px;
	overflow: auto;
	font-size: 85%%;
	line-height: 1.45;
	background-color: #f6f8fa;
	border-radius: 6px;
}
pre code {
	padding: 0;
	margin: 0;
	background-color: transparent;
	border: 0;
}
table {
	border-spacing: 0;
	border-collapse: collapse;
	margin-top: 0;
	margin-bottom: 16px;
}
table th, table td {
	padding: 6px 13px;
	border: 1px solid #d0d7de;
}
table tr {
	background-color: #ffffff;
	border-top: 1px solid #d0d7de;
}
table tr:nth-child(2n) {
	background-color: #f6f8fa;
}
blockquote {
	padding: 0 1em;
	color: #656d76;
	border-left: .25em solid #d0d7de;
	margin: 0 0 16px 0;
}
ul, ol {
	padding-left: 2em;
	margin-top: 0;
	margin-bottom: 16px;
}
a {
	color: #0969da;
	text-decoration: none;
}
a:hover {
	text-decoration: underline;
}
hr {
	height: .25em;
	padding: 0;
	margin: 24px 0;
	background-color: #d0d7de;
	border: 0;
}
/* Mermaid error styling */
.mermaid-error {
	background-color: #ffebe9;
	border: 1px solid #ff8182;
	border-radius: 6px;
	padding: 16px;
	margin: 16px 0;
	color: #cf222e;
	font-family: monospace;
}
</style>
<script src="https://cdn.jsdelivr.net/npm/marked/marked.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/mermaid/dist/mermaid.min.js"></script>
</head>
<body>
<div id="content"></div>
<script>
// Track errors and render state
window.renderErrors = [];
window.renderWarnings = [];
window.renderComplete = false;

async function renderMarkdown() {
	try {
		// Parse markdown from JSON-encoded string
		const markdown = %s;

		// Custom renderer for mermaid blocks (works with both old and new marked.js API)
		const renderer = new marked.Renderer();
		const originalCodeRenderer = renderer.code.bind(renderer);
		renderer.code = function(codeOrObj, language) {
			// Handle both old API (code, language) and new API ({text, lang})
			let code, lang;
			if (typeof codeOrObj === 'object') {
				code = codeOrObj.text;
				lang = codeOrObj.lang;
			} else {
				code = codeOrObj;
				lang = language;
			}
			if (lang === 'mermaid') {
				return '<pre class="mermaid">' + code + '</pre>';
			}
			return originalCodeRenderer(codeOrObj, language);
		};

		marked.setOptions({
			renderer: renderer,
			gfm: true,
			breaks: false,
		});

		// Initialize mermaid with error handling
		mermaid.initialize({
			startOnLoad: false,
			theme: 'default',
			securityLevel: 'loose',
		});

		// Render markdown
		document.getElementById('content').innerHTML = marked.parse(markdown);

		// Render mermaid diagrams with error handling
		const mermaidBlocks = document.querySelectorAll('.mermaid');
		for (let i = 0; i < mermaidBlocks.length; i++) {
			const el = mermaidBlocks[i];
			try {
				const { svg } = await mermaid.render('mermaid-' + i, el.textContent);
				el.innerHTML = svg;
			} catch (err) {
				window.renderErrors.push('Mermaid error in diagram ' + i + ': ' + err.message);
				el.className = 'mermaid-error';
				el.textContent = 'Mermaid rendering error: ' + err.message;
			}
		}
	} catch (err) {
		window.renderErrors.push('Markdown parse error: ' + err.message);
		document.getElementById('content').innerHTML = '<div class="mermaid-error">Render error: ' + err.message + '</div>';
	}
	window.renderComplete = true;
}

renderMarkdown();
</script>
</body>
</html>`, string(markdownJSON))

	// Write HTML to temp file - data URLs can't load external scripts
	tmpFile, err := os.CreateTemp("", "markdown-preview-*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(html); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	var screenshot []byte
	var errors, warnings []string

	err = chromedp.Run(ctx,
		chromedp.Navigate("file://"+tmpPath),
		// Wait for render to complete (polls for renderComplete flag)
		chromedp.Poll(`window.renderComplete === true`, nil, chromedp.WithPollingInterval(100*time.Millisecond)),
		// Small additional delay for mermaid SVG rendering
		chromedp.Sleep(500*time.Millisecond),
		// Get any render errors
		chromedp.Evaluate(`window.renderErrors || []`, &errors),
		chromedp.Evaluate(`window.renderWarnings || []`, &warnings),
		// Take screenshot
		chromedp.FullScreenshot(&screenshot, 90),
	)
	if err != nil {
		return nil, fmt.Errorf("preview failed: %w", err)
	}

	return &MarkdownPreviewResult{
		Screenshot: screenshot,
		Errors:     errors,
		Warnings:   warnings,
	}, nil
}

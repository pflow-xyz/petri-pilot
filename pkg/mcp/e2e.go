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
		mcp.WithDescription("Evaluate JavaScript code in a browser session via the app's debug endpoint. Returns the result."),
		mcp.WithString("session_id",
			mcp.Required(),
			mcp.Description("The browser session ID (from e2e_start_browser)"),
		),
		mcp.WithString("code",
			mcp.Required(),
			mcp.Description("JavaScript code to evaluate in the browser context"),
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

	result, err := e2eManager.Eval(ctx, sessionID, code)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("eval failed: %v", err)), nil
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

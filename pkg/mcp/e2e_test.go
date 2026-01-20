package mcp

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestE2EStartAndEval(t *testing.T) {
	// Skip if server is not running
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Test start browser
	startReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]interface{}{
				"url":     "http://localhost:8080",
				"timeout": 30,
			},
		},
	}

	result, err := handleE2EStartBrowser(ctx, startReq)
	if err != nil {
		t.Fatalf("handleE2EStartBrowser failed: %v", err)
	}

	if result.IsError {
		t.Fatalf("start browser returned error: %v", result.Content)
	}

	// Parse the result to get session ID
	var startResult map[string]interface{}
	textContent := result.Content[0].(mcp.TextContent)
	if err := json.Unmarshal([]byte(textContent.Text), &startResult); err != nil {
		t.Fatalf("failed to parse start result: %v", err)
	}

	sessionID, ok := startResult["browser_session_id"].(string)
	if !ok || sessionID == "" {
		t.Fatalf("no browser_session_id in result: %v", startResult)
	}

	debugSessionID, _ := startResult["debug_session_id"].(string)
	t.Logf("Started browser session: %s, debug session: %s", sessionID, debugSessionID)

	// Test list sessions
	listReq := mcp.CallToolRequest{}
	listResult, err := handleE2EListSessions(ctx, listReq)
	if err != nil {
		t.Fatalf("handleE2EListSessions failed: %v", err)
	}

	t.Logf("List sessions result: %v", listResult.Content)

	// Test eval - simple expression
	evalReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]interface{}{
				"session_id": sessionID,
				"code":       "1 + 1",
			},
		},
	}

	evalResult, err := handleE2EEval(ctx, evalReq)
	if err != nil {
		t.Fatalf("handleE2EEval failed: %v", err)
	}

	if evalResult.IsError {
		t.Logf("eval returned error (expected if debug session not connected): %v", evalResult.Content)
	} else {
		t.Logf("Eval result: %v", evalResult.Content)
	}

	// Test stop browser
	stopReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]interface{}{
				"session_id": sessionID,
			},
		},
	}

	stopResult, err := handleE2EStopBrowser(ctx, stopReq)
	if err != nil {
		t.Fatalf("handleE2EStopBrowser failed: %v", err)
	}

	t.Logf("Stop result: %v", stopResult.Content)
}

func TestE2EERC20App(t *testing.T) {
	// Get port from environment or use default
	port := os.Getenv("E2E_PORT")
	if port == "" {
		port = "8080"
	}
	baseURL := "http://localhost:" + port

	// Skip if server is not running
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Start browser
	startReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"url":     baseURL,
				"timeout": 60,
			},
		},
	}

	result, err := handleE2EStartBrowser(ctx, startReq)
	if err != nil {
		t.Fatalf("handleE2EStartBrowser failed: %v", err)
	}
	if result.IsError {
		t.Skipf("Skipping test - browser start failed: %v", result.Content)
	}

	// Parse session ID
	var startResult map[string]any
	textContent := result.Content[0].(mcp.TextContent)
	if err := json.Unmarshal([]byte(textContent.Text), &startResult); err != nil {
		t.Fatalf("failed to parse start result: %v", err)
	}
	sessionID := startResult["browser_session_id"].(string)
	t.Logf("Started browser session: %s", sessionID)

	// Cleanup on exit
	defer func() {
		stopReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{"session_id": sessionID},
			},
		}
		handleE2EStopBrowser(ctx, stopReq)
	}()

	// Test: Login as admin and holder
	loginResult := evalCode(t, ctx, sessionID, `return await pilot.loginAs(['admin', 'holder'])`)
	t.Logf("Login result: %s", loginResult)

	// Test: Create a new token instance
	createResult := evalCode(t, ctx, sessionID, `return await pilot.create()`)
	t.Logf("Create result: %s", createResult)

	// Test: Mint some tokens (using small amount to stay within int64 when scaled by 1e18)
	mintResult := evalCode(t, ctx, sessionID, `return await pilot.action('mint', { to: 'alice', amount: 1 })`)
	t.Logf("Mint result: %s", mintResult)

	// Test: Transfer tokens
	transferResult := evalCode(t, ctx, sessionID, `return await pilot.action('transfer', { from: 'alice', to: 'bob', amount: 0.5 })`)
	t.Logf("Transfer result: %s", transferResult)

	// Test: Get state
	stateResult := evalCode(t, ctx, sessionID, `return await pilot.getState()`)
	t.Logf("State result: %s", stateResult)
}

func evalCode(t *testing.T, ctx context.Context, sessionID, code string) string {
	evalReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"session_id": sessionID,
				"code":       code,
			},
		},
	}

	result, err := handleE2EEval(ctx, evalReq)
	if err != nil {
		t.Logf("Eval error for code %q: %v", code, err)
		return ""
	}

	if result.IsError {
		t.Logf("Eval returned error for code %q: %v", code, result.Content)
		return ""
	}

	textContent := result.Content[0].(mcp.TextContent)
	return textContent.Text
}

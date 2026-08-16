package mcp

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

// TestMain points the usage log at a temp file BEFORE any test triggers the
// lazy logger — the logger binds its path once per process.
var testLogPath string

func TestMain(m *testing.M) {
	dir, _ := os.MkdirTemp("", "observe")
	testLogPath = filepath.Join(dir, "usage.log")
	os.Setenv("PILOT_USAGE_LOG", testLogPath)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// A tool call must leave a usage line in the log FILE — stdout dies with
// the tmux pane, which is how the 2026-08-16 memory balloon escaped
// attribution.
func TestObserveMiddlewareWritesAUsageLine(t *testing.T) {
	handler := func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		return mcplib.NewToolResultText("ok"), nil
	}
	req := mcplib.CallToolRequest{}
	req.Params.Name = "petri_validate"
	req.Params.Arguments = map[string]any{"model": `{"name":"x"}`}

	if _, err := ObserveMiddleware(handler)(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(testLogPath)
	if err != nil {
		t.Fatalf("no usage log written: %v", err)
	}
	line := string(b)
	if !strings.Contains(line, "tool=petri_validate") || !strings.Contains(line, "deltaMB=") {
		t.Fatalf("usage line missing fields: %q", line)
	}
}

// While a call runs, the in-flight table must hold its name and arguments —
// that table is the whole record for a call that never returns.
func TestInflightTableNamesTheRunningCall(t *testing.T) {
	seen := make(chan string, 1)
	handler := func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		inflightMu.Lock()
		var got string
		for _, c := range inflight {
			if c.Tool == "petri_analyze" {
				got = c.Args
			}
		}
		inflightMu.Unlock()
		seen <- got
		return mcplib.NewToolResultText("ok"), nil
	}
	req := mcplib.CallToolRequest{}
	req.Params.Name = "petri_analyze"
	req.Params.Arguments = map[string]any{"model": `{"name":"boom"}`}
	if _, err := ObserveMiddleware(handler)(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if args := <-seen; !strings.Contains(args, "boom") {
		t.Fatalf("in-flight args did not carry the model: %q", args)
	}
	inflightMu.Lock()
	n := len(inflight)
	inflightMu.Unlock()
	if n != 0 {
		t.Fatalf("in-flight table not cleaned up: %d entries remain", n)
	}
}

// The caller's IP must travel from the HTTP layer into the usage line —
// it is the repeat-user signal an anonymous endpoint legitimately has.
func TestCallerRidesTheContextIntoTheUsageLine(t *testing.T) {
	r, _ := http.NewRequest("POST", "/mcp", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")
	r.Header.Set("User-Agent", "opencode/1.18.18")
	r.Header.Set("Mcp-Session-Id", "mcp-session-deadbeef-0000")
	ctx := WithCaller(context.Background(), r)

	handler := func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		return mcplib.NewToolResultText("ok"), nil
	}
	req := mcplib.CallToolRequest{}
	req.Params.Name = "petri_simulate"
	if _, err := ObserveMiddleware(handler)(ctx, req); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(testLogPath)
	log := string(b)
	if !strings.Contains(log, "ip=203.0.113.7") || !strings.Contains(log, `ua="opencode/1.18.18"`) || !strings.Contains(log, "sess=deadbeef") {
		t.Fatalf("caller signals missing from usage log: %q", log)
	}
}

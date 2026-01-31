package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestServiceToolDefinitions(t *testing.T) {
	tools := ServiceTools()
	if len(tools) != 6 {
		t.Errorf("expected 6 service tools, got %d", len(tools))
	}

	expectedTools := []string{
		"service_start",
		"service_stop",
		"service_list",
		"service_stats",
		"service_logs",
		"service_health",
	}

	for i, expected := range expectedTools {
		if tools[i].Tool.Name != expected {
			t.Errorf("tool %d: expected %s, got %s", i, expected, tools[i].Tool.Name)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d        time.Duration
		expected string
	}{
		{5 * time.Second, "5s"},
		{65 * time.Second, "1m5s"},
		{3665 * time.Second, "1h1m5s"},
		{0, "0s"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.d)
		if result != tt.expected {
			t.Errorf("formatDuration(%v) = %s, expected %s", tt.d, result, tt.expected)
		}
	}
}

func TestParseCPUTime(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"0:05.00", 5.0},
		{"1:30.50", 90.5},
		{"1:00:00", 3600.0},
	}

	for _, tt := range tests {
		result := parseCPUTime(tt.input)
		if result != tt.expected {
			t.Errorf("parseCPUTime(%s) = %f, expected %f", tt.input, result, tt.expected)
		}
	}
}

func TestCheckHealth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"status":   "ok",
			"database": "connected",
		})
	}))
	defer ts.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	health := checkHealth(client, ts.URL)

	if health == nil {
		t.Fatal("expected health status, got nil")
	}
	if health.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", health.Status)
	}
	if health.Database != "connected" {
		t.Errorf("expected database 'connected', got '%s'", health.Database)
	}
}

func TestIsRunning(t *testing.T) {
	// Current process should be running
	if !isRunning(os.Getpid()) {
		t.Error("expected current process to be running")
	}

	// Non-existent PID should not be running
	if isRunning(999999) {
		t.Error("expected non-existent PID to not be running")
	}
}

func TestServiceManagerWithTempState(t *testing.T) {
	// Create a temporary state directory
	tmpDir := t.TempDir()
	os.Setenv("PETRI_STATE_DIR", tmpDir)
	defer os.Unsetenv("PETRI_STATE_DIR")

	// Create a fresh manager
	mgr := newServiceManager()

	// List should be empty
	services, err := mgr.List()
	if err != nil {
		t.Fatalf("failed to list services: %v", err)
	}
	if len(services) != 0 {
		t.Errorf("expected empty list, got %d services", len(services))
	}

	// Get non-existent should return false
	_, ok, err := mgr.Get("nonexistent")
	if err != nil {
		t.Fatalf("failed to get service: %v", err)
	}
	if ok {
		t.Error("expected service not found")
	}

	// Stats for non-existent should error
	_, err = mgr.Stats("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent service stats")
	}

	// Logs for non-existent should error
	_, err = mgr.Logs("nonexistent", 50, "both")
	if err == nil {
		t.Error("expected error for non-existent service logs")
	}

	// Stop non-existent should error
	err = mgr.Stop("nonexistent")
	if err == nil {
		t.Error("expected error for stopping non-existent service")
	}
}

func TestServiceStartValidation(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("PETRI_STATE_DIR", tmpDir)
	defer os.Unsetenv("PETRI_STATE_DIR")

	mgr := newServiceManager()
	ctx := context.Background()

	// Test with non-existent directory
	_, err := mgr.Start(ctx, "/nonexistent/path", 8080, nil)
	if err == nil {
		t.Error("expected error for non-existent directory")
	}
}

func TestStateFilePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("PETRI_STATE_DIR", tmpDir)
	defer os.Unsetenv("PETRI_STATE_DIR")

	stateFile := filepath.Join(tmpDir, "services.json")

	// Write some state manually
	state := map[string]*ServiceState{
		"svc-1": {
			ID:        "svc-1",
			Name:      "test",
			Directory: "/tmp/test",
			URL:       "http://localhost:8080",
			PID:       999999, // Non-existent PID
			Port:      8080,
			StartedAt: time.Now(),
		},
	}
	data, _ := json.Marshal(state)
	os.WriteFile(stateFile, data, 0644)

	// Create manager and list - should clean up dead service
	mgr := newServiceManager()
	services, err := mgr.List()
	if err != nil {
		t.Fatalf("failed to list services: %v", err)
	}

	// Service should be cleaned up since PID doesn't exist
	if len(services) != 0 {
		t.Errorf("expected dead service to be cleaned up, got %d services", len(services))
	}
}

// Integration test that requires a real Go executable
func TestServiceManagerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temp directories
	tmpDir := t.TempDir()
	os.Setenv("PETRI_STATE_DIR", tmpDir)
	defer os.Unsetenv("PETRI_STATE_DIR")

	serviceDir := filepath.Join(tmpDir, "testservice")
	os.MkdirAll(serviceDir, 0755)

	// Write test service
	mainGo := filepath.Join(serviceDir, "main.go")
	err := os.WriteFile(mainGo, []byte(`
package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "{\"status\":\"ok\"}")
	})
	http.ListenAndServe(":"+port, nil)
}
`), 0644)
	if err != nil {
		t.Fatalf("failed to write test main.go: %v", err)
	}

	// Create go.mod
	goMod := filepath.Join(serviceDir, "go.mod")
	err = os.WriteFile(goMod, []byte("module testservice\ngo 1.21\n"), 0644)
	if err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	mgr := newServiceManager()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Start the service
	svc, err := mgr.Start(ctx, serviceDir, 18080, nil)
	if err != nil {
		t.Fatalf("failed to start service: %v", err)
	}

	defer mgr.Stop(svc.ID)

	// Verify it's tracked
	services, err := mgr.List()
	if err != nil {
		t.Fatalf("failed to list services: %v", err)
	}
	if len(services) != 1 {
		t.Errorf("expected 1 service, got %d", len(services))
	}

	// Get by ID
	got, ok, err := mgr.Get(svc.ID)
	if err != nil {
		t.Fatalf("failed to get service: %v", err)
	}
	if !ok {
		t.Error("expected service to be found")
	}
	if got.PID != svc.PID {
		t.Errorf("expected PID %d, got %d", svc.PID, got.PID)
	}

	// Get stats
	stats, err := mgr.Stats(svc.ID)
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}
	if stats.Status != "running" {
		t.Errorf("expected status 'running', got '%s'", stats.Status)
	}
	if stats.Health == nil {
		t.Error("expected health status")
	}

	// Test logs
	_, err = mgr.Logs(svc.ID, 10, "both")
	if err != nil {
		t.Fatalf("failed to get logs: %v", err)
	}

	// Stop the service
	err = mgr.Stop(svc.ID)
	if err != nil {
		t.Fatalf("failed to stop service: %v", err)
	}

	// Verify it's removed from tracking
	services, _ = mgr.List()
	if len(services) != 0 {
		t.Errorf("expected 0 services after stop, got %d", len(services))
	}
}

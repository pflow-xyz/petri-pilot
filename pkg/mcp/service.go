// Service management tools for MCP server.
// Provides control and monitoring of generated Petri-pilot services.
// Uses file-based state persistence to work across MCP invocations.

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// ServiceState represents persisted service information.
type ServiceState struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Directory string            `json:"directory"`
	URL       string            `json:"url"`
	PID       int               `json:"pid"`
	Port      int               `json:"port"`
	StartedAt time.Time         `json:"started_at"`
	Env       map[string]string `json:"env,omitempty"`
}

// ServiceStats contains runtime statistics for a service.
type ServiceStats struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Status        string        `json:"status"`
	PID           int           `json:"pid"`
	URL           string        `json:"url"`
	Uptime        string        `json:"uptime"`
	UptimeSeconds float64       `json:"uptime_seconds"`
	MemoryRSS     int64         `json:"memory_rss_bytes,omitempty"`
	MemoryVMS     int64         `json:"memory_vms_bytes,omitempty"`
	CPUUser       float64       `json:"cpu_user_seconds,omitempty"`
	CPUSystem     float64       `json:"cpu_system_seconds,omitempty"`
	Health        *HealthStatus `json:"health,omitempty"`
}

// HealthStatus from the service's /health endpoint.
type HealthStatus struct {
	Status    string `json:"status"`
	Database  string `json:"database,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

// ServiceManager manages services with file-based state persistence.
type ServiceManager struct {
	mu        sync.RWMutex
	stateFile string
}

// stateDir returns the directory for storing service state.
func stateDir() string {
	dir := os.Getenv("PETRI_STATE_DIR")
	if dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".petri-pilot")
}

// newServiceManager creates a service manager with the state file.
func newServiceManager() *ServiceManager {
	dir := stateDir()
	os.MkdirAll(dir, 0755)
	return &ServiceManager{
		stateFile: filepath.Join(dir, "services.json"),
	}
}

// Global service manager singleton.
var svcManager = newServiceManager()

// loadState reads the current service state from disk.
func (m *ServiceManager) loadState() (map[string]*ServiceState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := os.ReadFile(m.stateFile)
	if os.IsNotExist(err) {
		return make(map[string]*ServiceState), nil
	}
	if err != nil {
		return nil, err
	}

	var services map[string]*ServiceState
	if err := json.Unmarshal(data, &services); err != nil {
		return make(map[string]*ServiceState), nil
	}
	return services, nil
}

// saveState writes the current service state to disk.
func (m *ServiceManager) saveState(services map[string]*ServiceState) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := json.MarshalIndent(services, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.stateFile, data, 0644)
}

// nextID generates the next service ID.
func (m *ServiceManager) nextID(services map[string]*ServiceState) string {
	maxID := 0
	for id := range services {
		if strings.HasPrefix(id, "svc-") {
			if n, err := strconv.Atoi(strings.TrimPrefix(id, "svc-")); err == nil && n > maxID {
				maxID = n
			}
		}
	}
	return fmt.Sprintf("svc-%d", maxID+1)
}

// isRunning checks if a process is still running.
func isRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds, so we send signal 0 to check
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// Start launches a generated service.
func (m *ServiceManager) Start(ctx context.Context, directory string, port int, env map[string]string) (*ServiceState, error) {
	services, err := m.loadState()
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	// Check if service is already running from this directory
	for _, svc := range services {
		if svc.Directory == directory && isRunning(svc.PID) {
			return nil, fmt.Errorf("service already running from %s (id: %s, pid: %d)", directory, svc.ID, svc.PID)
		}
	}

	// Check if port is in use
	for _, svc := range services {
		if svc.Port == port && isRunning(svc.PID) {
			return nil, fmt.Errorf("port %d already in use by service %s", port, svc.ID)
		}
	}

	// Determine service name from directory
	name := filepath.Base(directory)

	// Create log directory
	logDir := filepath.Join(stateDir(), "logs", name)
	os.MkdirAll(logDir, 0755)

	// Open log files
	stdoutFile, err := os.Create(filepath.Join(logDir, "stdout.log"))
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout log: %w", err)
	}
	stderrFile, err := os.Create(filepath.Join(logDir, "stderr.log"))
	if err != nil {
		stdoutFile.Close()
		return nil, fmt.Errorf("failed to create stderr log: %w", err)
	}

	// Build the binary first
	binaryPath := filepath.Join(directory, name)
	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = directory
	// Disable Go workspace mode so it uses the local go.mod instead of parent
	buildCmd.Env = append(os.Environ(), "GOWORK=off")
	buildOutput, buildErr := buildCmd.CombinedOutput()
	if buildErr != nil {
		stdoutFile.Close()
		stderrFile.Close()
		return nil, fmt.Errorf("failed to build service: %v\n%s", buildErr, string(buildOutput))
	}

	// Run the built binary - start in new process group so it survives parent exit
	cmd := exec.Command(binaryPath)
	cmd.Dir = directory
	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Start in new process group
	}

	// Set environment
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("PORT=%d", port))
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		stdoutFile.Close()
		stderrFile.Close()
		os.Remove(binaryPath) // Clean up binary on failure
		return nil, fmt.Errorf("failed to start service: %v", err)
	}

	// Close log files (process has them open now)
	stdoutFile.Close()
	stderrFile.Close()

	// Create service state
	id := m.nextID(services)
	svc := &ServiceState{
		ID:        id,
		Name:      name,
		Directory: directory,
		URL:       fmt.Sprintf("http://localhost:%d", port),
		PID:       cmd.Process.Pid,
		Port:      port,
		StartedAt: time.Now(),
		Env:       env,
	}

	// Save state immediately so we track the PID
	services[id] = svc
	if err := m.saveState(services); err != nil {
		// Try to kill the process since we can't track it
		cmd.Process.Kill()
		return nil, fmt.Errorf("failed to save state: %w", err)
	}

	// Wait for service to become healthy
	healthURL := fmt.Sprintf("%s/health", svc.URL)
	client := &http.Client{Timeout: 2 * time.Second}

	healthCtx, healthCancel := context.WithTimeout(ctx, 30*time.Second)
	defer healthCancel()

	for {
		select {
		case <-healthCtx.Done():
			// Timeout - check if process is still running
			if !isRunning(svc.PID) {
				// Process died, clean up state
				delete(services, id)
				m.saveState(services)
				return nil, fmt.Errorf("service failed to start: process exited")
			}
			// Still running but not healthy yet - return anyway
			return svc, nil
		case <-time.After(200 * time.Millisecond):
			if !isRunning(svc.PID) {
				delete(services, id)
				m.saveState(services)
				return nil, fmt.Errorf("service failed to start: process exited")
			}
			resp, err := client.Get(healthURL)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return svc, nil
				}
			}
		}
	}
}

// Stop terminates a running service.
func (m *ServiceManager) Stop(id string) error {
	services, err := m.loadState()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	svc, ok := services[id]
	if !ok {
		return fmt.Errorf("service not found: %s", id)
	}

	if !isRunning(svc.PID) {
		// Already stopped, just clean up state
		delete(services, id)
		m.saveState(services)
		return nil
	}

	// Send SIGTERM first
	process, err := os.FindProcess(svc.PID)
	if err != nil {
		delete(services, id)
		m.saveState(services)
		return nil
	}

	process.Signal(syscall.SIGTERM)

	// Wait up to 5 seconds for graceful shutdown
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		if !isRunning(svc.PID) {
			delete(services, id)
			m.saveState(services)
			return nil
		}
	}

	// Force kill
	process.Kill()
	delete(services, id)
	m.saveState(services)
	return nil
}

// List returns all tracked services with their current status.
func (m *ServiceManager) List() ([]*ServiceState, error) {
	services, err := m.loadState()
	if err != nil {
		return nil, err
	}

	// Clean up dead services
	changed := false
	result := make([]*ServiceState, 0, len(services))
	for id, svc := range services {
		if isRunning(svc.PID) {
			result = append(result, svc)
		} else {
			delete(services, id)
			changed = true
		}
	}

	if changed {
		m.saveState(services)
	}

	return result, nil
}

// Get returns a specific service.
func (m *ServiceManager) Get(id string) (*ServiceState, bool, error) {
	services, err := m.loadState()
	if err != nil {
		return nil, false, err
	}

	svc, ok := services[id]
	if !ok {
		return nil, false, nil
	}

	// Check if still running
	if !isRunning(svc.PID) {
		delete(services, id)
		m.saveState(services)
		return nil, false, nil
	}

	return svc, true, nil
}

// Stats returns runtime statistics for a service.
func (m *ServiceManager) Stats(id string) (*ServiceStats, error) {
	svc, ok, err := m.Get(id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("service not found: %s", id)
	}

	status := "stopped"
	if isRunning(svc.PID) {
		status = "running"
	}

	stats := &ServiceStats{
		ID:            svc.ID,
		Name:          svc.Name,
		Status:        status,
		PID:           svc.PID,
		URL:           svc.URL,
		UptimeSeconds: time.Since(svc.StartedAt).Seconds(),
		Uptime:        formatDuration(time.Since(svc.StartedAt)),
	}

	// Get process stats if running
	if status == "running" {
		if memStats := getProcessMemory(svc.PID); memStats != nil {
			stats.MemoryRSS = memStats.RSS
			stats.MemoryVMS = memStats.VMS
		}
		if cpuStats := getProcessCPU(svc.PID); cpuStats != nil {
			stats.CPUUser = cpuStats.User
			stats.CPUSystem = cpuStats.System
		}

		// Check health endpoint
		client := &http.Client{Timeout: 2 * time.Second}
		if health := checkHealth(client, svc.URL+"/health"); health != nil {
			stats.Health = health
		}
	}

	return stats, nil
}

// Logs returns recent log output for a service.
func (m *ServiceManager) Logs(id string, lines int, stream string) ([]string, error) {
	svc, ok, err := m.Get(id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("service not found: %s", id)
	}

	logDir := filepath.Join(stateDir(), "logs", svc.Name)

	var logFiles []string
	switch stream {
	case "stdout":
		logFiles = []string{filepath.Join(logDir, "stdout.log")}
	case "stderr":
		logFiles = []string{filepath.Join(logDir, "stderr.log")}
	default:
		logFiles = []string{
			filepath.Join(logDir, "stdout.log"),
			filepath.Join(logDir, "stderr.log"),
		}
	}

	var allLines []string
	for _, logFile := range logFiles {
		content, err := os.ReadFile(logFile)
		if err != nil {
			continue
		}
		fileLines := strings.Split(string(content), "\n")
		allLines = append(allLines, fileLines...)
	}

	// Return last N lines
	if len(allLines) > lines {
		allLines = allLines[len(allLines)-lines:]
	}

	return allLines, nil
}

// Helper functions

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	if h > 0 {
		return fmt.Sprintf("%dh%dm%ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

type ProcessMemory struct {
	RSS int64
	VMS int64
}

func getProcessMemory(pid int) *ProcessMemory {
	data, err := exec.Command("ps", "-o", "rss=,vsz=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return nil
	}
	fields := strings.Fields(string(data))
	if len(fields) < 2 {
		return nil
	}
	rss, _ := strconv.ParseInt(fields[0], 10, 64)
	vms, _ := strconv.ParseInt(fields[1], 10, 64)
	return &ProcessMemory{
		RSS: rss * 1024,
		VMS: vms * 1024,
	}
}

type ProcessCPU struct {
	User   float64
	System float64
}

func getProcessCPU(pid int) *ProcessCPU {
	data, err := exec.Command("ps", "-o", "utime=,stime=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return nil
	}
	fields := strings.Fields(string(data))
	if len(fields) < 2 {
		return nil
	}
	return &ProcessCPU{
		User:   parseCPUTime(fields[0]),
		System: parseCPUTime(fields[1]),
	}
}

func parseCPUTime(s string) float64 {
	parts := strings.Split(s, ":")
	var total float64
	for i, p := range parts {
		v, _ := strconv.ParseFloat(p, 64)
		switch len(parts) - i {
		case 3:
			total += v * 3600
		case 2:
			total += v * 60
		case 1:
			total += v
		}
	}
	return total
}

func checkHealth(client *http.Client, url string) *HealthStatus {
	resp, err := client.Get(url)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var health HealthStatus
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return &HealthStatus{Status: "ok"}
	}
	return &health
}

// MCP Tool Definitions

func serviceStartTool() mcp.Tool {
	return mcp.NewTool("service_start",
		mcp.WithDescription("Start a generated Petri-pilot service. Builds and launches the service, waiting for it to become healthy. The service runs detached and persists after the MCP call ends."),
		mcp.WithString("directory",
			mcp.Required(),
			mcp.Description("Absolute path to the generated service directory (e.g., /path/to/generated/erc20-token)"),
		),
		mcp.WithNumber("port",
			mcp.Description("Port to run the service on (default: 8080)"),
		),
	)
}

func handleServiceStart(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	directory, err := request.RequireString("directory")
	if err != nil {
		return mcp.NewToolResultError("missing directory parameter"), nil
	}

	port := getIntParam(request, "port", 8080)

	svc, err := svcManager.Start(ctx, directory, port, nil)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to start service: %v", err)), nil
	}

	result := map[string]any{
		"id":         svc.ID,
		"name":       svc.Name,
		"url":        svc.URL,
		"pid":        svc.PID,
		"status":     "running",
		"started_at": svc.StartedAt.Format(time.RFC3339),
	}
	output, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(output)), nil
}

func serviceStopTool() mcp.Tool {
	return mcp.NewTool("service_stop",
		mcp.WithDescription("Stop a running Petri-pilot service by its ID."),
		mcp.WithString("service_id",
			mcp.Required(),
			mcp.Description("The service ID (e.g., svc-1)"),
		),
	)
}

func handleServiceStop(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	serviceID, err := request.RequireString("service_id")
	if err != nil {
		return mcp.NewToolResultError("missing service_id parameter"), nil
	}

	if err := svcManager.Stop(serviceID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to stop service: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Service %s stopped successfully", serviceID)), nil
}

func serviceListTool() mcp.Tool {
	return mcp.NewTool("service_list",
		mcp.WithDescription("List all tracked Petri-pilot services and their status."),
	)
}

func handleServiceList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	services, err := svcManager.List()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list services: %v", err)), nil
	}

	result := make([]map[string]any, len(services))
	for i, svc := range services {
		status := "stopped"
		if isRunning(svc.PID) {
			status = "running"
		}
		result[i] = map[string]any{
			"id":         svc.ID,
			"name":       svc.Name,
			"directory":  svc.Directory,
			"url":        svc.URL,
			"pid":        svc.PID,
			"status":     status,
			"started_at": svc.StartedAt.Format(time.RFC3339),
		}
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(output)), nil
}

func serviceStatsTool() mcp.Tool {
	return mcp.NewTool("service_stats",
		mcp.WithDescription("Get runtime statistics for a service including memory, CPU, uptime, and health status."),
		mcp.WithString("service_id",
			mcp.Required(),
			mcp.Description("The service ID (e.g., svc-1)"),
		),
	)
}

func handleServiceStats(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	serviceID, err := request.RequireString("service_id")
	if err != nil {
		return mcp.NewToolResultError("missing service_id parameter"), nil
	}

	stats, err := svcManager.Stats(serviceID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get stats: %v", err)), nil
	}

	output, _ := json.MarshalIndent(stats, "", "  ")
	return mcp.NewToolResultText(string(output)), nil
}

func serviceLogsTool() mcp.Tool {
	return mcp.NewTool("service_logs",
		mcp.WithDescription("Get recent log output from a service."),
		mcp.WithString("service_id",
			mcp.Required(),
			mcp.Description("The service ID (e.g., svc-1)"),
		),
		mcp.WithNumber("lines",
			mcp.Description("Number of lines to return (default: 50)"),
		),
		mcp.WithString("stream",
			mcp.Description("Log stream: stdout, stderr, or both (default: both)"),
		),
	)
}

func handleServiceLogs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	serviceID, err := request.RequireString("service_id")
	if err != nil {
		return mcp.NewToolResultError("missing service_id parameter"), nil
	}

	lines := getIntParam(request, "lines", 50)
	stream := request.GetString("stream", "both")

	logLines, err := svcManager.Logs(serviceID, lines, stream)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get logs: %v", err)), nil
	}

	result := map[string]any{
		"service_id": serviceID,
		"stream":     stream,
		"lines":      logLines,
	}
	output, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(output)), nil
}

func serviceHealthTool() mcp.Tool {
	return mcp.NewTool("service_health",
		mcp.WithDescription("Check the health endpoint of a running service."),
		mcp.WithString("service_id",
			mcp.Required(),
			mcp.Description("The service ID (e.g., svc-1)"),
		),
	)
}

func handleServiceHealth(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	serviceID, err := request.RequireString("service_id")
	if err != nil {
		return mcp.NewToolResultError("missing service_id parameter"), nil
	}

	svc, ok, err := svcManager.Get(serviceID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get service: %v", err)), nil
	}
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("service not found: %s", serviceID)), nil
	}

	if !isRunning(svc.PID) {
		return mcp.NewToolResultError(fmt.Sprintf("service %s is not running", serviceID)), nil
	}

	client := &http.Client{Timeout: 5 * time.Second}
	healthURL := svc.URL + "/health"
	resp, err := client.Get(healthURL)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("health check failed: %v", err)), nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	result := map[string]any{
		"service_id":  serviceID,
		"url":         healthURL,
		"status_code": resp.StatusCode,
		"body":        json.RawMessage(body),
	}
	output, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(output)), nil
}

// ServiceTools returns all service management tool definitions and handlers.
func ServiceTools() []struct {
	Tool    mcp.Tool
	Handler func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)
} {
	return []struct {
		Tool    mcp.Tool
		Handler func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)
	}{
		{serviceStartTool(), handleServiceStart},
		{serviceStopTool(), handleServiceStop},
		{serviceListTool(), handleServiceList},
		{serviceStatsTool(), handleServiceStats},
		{serviceLogsTool(), handleServiceLogs},
		{serviceHealthTool(), handleServiceHealth},
	}
}

// getIntParam extracts an integer parameter from the request, with a default value.
func getIntParam(request mcp.CallToolRequest, name string, defaultVal int) int {
	if args, ok := request.Params.Arguments.(map[string]any); ok {
		if v, exists := args[name]; exists {
			if f, ok := v.(float64); ok {
				return int(f)
			}
		}
	}
	return defaultVal
}

// Package pflowxyz provides a service adapter for pflow-xyz.
// It runs the pflow-xyz webserver as a subprocess.
package pflowxyz

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pflow-xyz/petri-pilot/pkg/serve"
)

const ServiceName = "pflow"

func init() {
	serve.Register(ServiceName, NewService)
}

// Service implements serve.ProcessService for pflow-xyz
type Service struct {
	dataDir string
}

// NewService creates a new pflow-xyz service instance
func NewService() (serve.Service, error) {
	// Get data directory from environment or use default
	dataDir := os.Getenv("PFLOW_DATA_DIR")
	if dataDir == "" {
		dataDir = "./pflow-data"
	}

	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("creating data directory: %w", err)
	}

	return &Service{
		dataDir: dataDir,
	}, nil
}

func (s *Service) Name() string {
	return ServiceName
}

func (s *Service) BuildHandler() http.Handler {
	// Not used for process services
	return nil
}

func (s *Service) Close() error {
	return nil
}

// RunProcess implements serve.ProcessService
func (s *Service) RunProcess(ctx context.Context, port int) error {
	// Find the pflow-xyz binary
	binaryPath, err := findPflowBinary()
	if err != nil {
		return err
	}

	// Build command arguments
	args := []string{
		"-port", fmt.Sprintf("%d", port),
		"-data", s.dataDir,
	}

	// Create command with context for cancellation
	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Pass through environment variables for GitHub OAuth
	cmd.Env = os.Environ()

	log.Printf("[pflow] Starting: %s %v", binaryPath, args)
	log.Printf("[pflow] Data directory: %s", s.dataDir)

	if err := cmd.Run(); err != nil {
		// Check if this was a context cancellation
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("pflow subprocess exited: %w", err)
	}

	return nil
}

// findPflowBinary locates the pflow-xyz webserver binary
func findPflowBinary() (string, error) {
	// Check common locations
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		homeDir, _ := os.UserHomeDir()
		gopath = filepath.Join(homeDir, "go")
	}

	// 1. Check if "pflow-webserver" is in PATH
	if path, err := exec.LookPath("pflow-webserver"); err == nil {
		return path, nil
	}

	// 2. Check GOPATH/bin for pflow-webserver
	pflowWebserverBin := filepath.Join(gopath, "bin", "pflow-webserver")
	if _, err := os.Stat(pflowWebserverBin); err == nil {
		return pflowWebserverBin, nil
	}

	// 3. Check if generic "webserver" is in PATH (legacy name)
	if path, err := exec.LookPath("webserver"); err == nil {
		return path, nil
	}

	// 4. Check GOPATH/bin for webserver
	webserverBin := filepath.Join(gopath, "bin", "webserver")
	if _, err := os.Stat(webserverBin); err == nil {
		return webserverBin, nil
	}

	// 5. Check current directory
	cwd, _ := os.Getwd()
	cwdBin := filepath.Join(cwd, "pflow-webserver")
	if _, err := os.Stat(cwdBin); err == nil {
		return cwdBin, nil
	}

	// 6. Try to build it by cloning the repo (go:embed files require the full repo)
	log.Printf("[pflow] Binary not found, attempting to build from source...")
	if err := buildFromSource(gopath); err != nil {
		return "", fmt.Errorf("pflow webserver binary not found and failed to build: %w\n\n"+
			"To install manually:\n"+
			"  git clone https://github.com/pflow-xyz/pflow-xyz.git\n"+
			"  cd pflow-xyz\n"+
			"  go build -o $GOPATH/bin/pflow-webserver ./cmd/webserver", err)
	}

	// Check again after build
	if _, err := os.Stat(pflowWebserverBin); err == nil {
		return pflowWebserverBin, nil
	}

	return "", fmt.Errorf("pflow webserver binary not found after build attempt")
}

// buildFromSource clones and builds the pflow-xyz webserver
func buildFromSource(gopath string) error {
	// Create temp directory for clone
	tmpDir, err := os.MkdirTemp("", "pflow-xyz-*")
	if err != nil {
		return fmt.Errorf("creating temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	repoDir := filepath.Join(tmpDir, "pflow-xyz")

	// Clone the repository
	log.Printf("[pflow] Cloning https://github.com/pflow-xyz/pflow-xyz.git...")
	cloneCmd := exec.Command("git", "clone", "--depth=1", "https://github.com/pflow-xyz/pflow-xyz.git", repoDir)
	cloneCmd.Stdout = os.Stdout
	cloneCmd.Stderr = os.Stderr
	if err := cloneCmd.Run(); err != nil {
		return fmt.Errorf("cloning repository: %w", err)
	}

	// The pflow-xyz build requires copying public/ to internal/static/public first
	// (see Makefile in the repo). We replicate that step here.
	log.Printf("[pflow] Preparing build (copying public assets)...")
	srcDir := filepath.Join(repoDir, "public")
	dstDir := filepath.Join(repoDir, "internal", "static", "public")

	// Copy public directory to internal/static/public
	copyCmd := exec.Command("cp", "-r", srcDir, dstDir)
	copyCmd.Stdout = os.Stdout
	copyCmd.Stderr = os.Stderr
	if err := copyCmd.Run(); err != nil {
		return fmt.Errorf("copying public assets: %w", err)
	}

	// Build the binary
	outputBin := filepath.Join(gopath, "bin", "pflow-webserver")
	log.Printf("[pflow] Building webserver binary...")
	buildCmd := exec.Command("go", "build", "-o", outputBin, "./cmd/webserver")
	buildCmd.Dir = repoDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("building binary: %w", err)
	}

	log.Printf("[pflow] Installed binary to %s", outputBin)
	return nil
}

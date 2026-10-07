package appverify

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// FreePort asks the OS for an ephemeral port and immediately releases it.
// There is a window in which another process could grab it before Process
// starts, but that race exists in every "find a free port" helper without a
// privileged reservation API, and is acceptable for a short-lived
// build-then-verify run.
func FreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// Process is a running generated-app binary, started for verification and
// meant to be stopped by the caller when done.
type Process struct {
	cmd    *exec.Cmd
	URL    string
	Port   int
	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

// Tidy runs `go mod tidy` in dir. A freshly generated go.mod names its
// requirements but has no go.sum and hasn't been resolved against the
// (possibly replaced) dependency graph yet, so `go build` alone refuses with
// "updates to go.mod needed" until this has run once.
func Tidy(dir string, extraEnv ...string) (output []byte, err error) {
	env := append(os.Environ(), "GOWORK=off")
	env = append(env, extraEnv...)
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("go mod tidy: %w", err)
	}
	return out, nil
}

// BuildAll runs `go build ./...` in dir, to validate that everything
// compiles without producing (or needing) a specific runnable binary.
func BuildAll(dir string, extraEnv ...string) (output []byte, err error) {
	env := append(os.Environ(), "GOWORK=off")
	env = append(env, extraEnv...)
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = dir
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("go build ./...: %w", err)
	}
	return out, nil
}

// BuildBinary runs `go build -o <output> <target>` in dir (target defaults
// to "." when empty). GOWORK is disabled so the build uses dir's own go.mod
// (or the parent module's, for a submodule) rather than a workspace file
// that happens to be in scope.
func BuildBinary(dir, outputPath, target string, extraEnv ...string) (output []byte, err error) {
	if target == "" {
		target = "."
	}
	env := append(os.Environ(), "GOWORK=off")
	env = append(env, extraEnv...)

	build := exec.Command("go", "build", "-o", outputPath, target)
	build.Dir = dir
	build.Env = env
	out, err := build.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("go build: %w", err)
	}
	return out, nil
}

// StartProcess launches binPath (working directory dir) with PORT set to a
// free port, and polls /health until it answers 200, the process exits, or
// timeout elapses.
func StartProcess(dir, binPath string, timeout time.Duration, extraEnv ...string) (*Process, error) {
	port, err := FreePort()
	if err != nil {
		return nil, fmt.Errorf("allocating a port: %w", err)
	}

	p := &Process{
		URL:    fmt.Sprintf("http://127.0.0.1:%d", port),
		Port:   port,
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
	}

	abs := binPath
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(dir, binPath)
	}
	cmd := exec.Command(abs)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", port))
	cmd.Env = append(cmd.Env, extraEnv...)
	cmd.Stdout = p.stdout
	cmd.Stderr = p.stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting %s: %w", abs, err)
	}
	p.cmd = cmd

	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	for {
		if p.cmd.ProcessState != nil {
			// Already reaped somehow (shouldn't happen without Wait, but be
			// defensive) — treat as a crash.
			return p, fmt.Errorf("process exited before becoming healthy:\nstdout:\n%s\nstderr:\n%s", p.stdout.String(), p.stderr.String())
		}
		if !processAlive(cmd.Process.Pid) {
			return p, fmt.Errorf("process exited before becoming healthy:\nstdout:\n%s\nstderr:\n%s", p.stdout.String(), p.stderr.String())
		}
		resp, err := client.Get(p.URL + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return p, nil
			}
		}
		if time.Now().After(deadline) {
			return p, fmt.Errorf("timed out waiting for %s/health:\nstdout:\n%s\nstderr:\n%s", p.URL, p.stdout.String(), p.stderr.String())
		}
		time.Sleep(150 * time.Millisecond)
	}
}

// Stop sends SIGTERM, waits briefly, then SIGKILLs if the process is still
// alive.
func (p *Process) Stop() {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return
	}
	_ = p.cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		p.cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = p.cmd.Process.Kill()
		<-done
	}
}

// Output returns the captured stdout/stderr, for callers that want to
// surface it in an error report.
func (p *Process) Output() (stdout, stderr string) {
	if p == nil {
		return "", ""
	}
	return p.stdout.String(), p.stderr.String()
}

func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

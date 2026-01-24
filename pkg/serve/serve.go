// Package serve provides a service registration framework for generated Petri-pilot services.
// It allows generated services to register themselves at init() time and be started via the CLI.
package serve

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
	"time"
)

// Service represents a generated Petri-pilot service that can be started.
type Service interface {
	// Name returns the service name.
	Name() string

	// BuildHandler returns the HTTP handler for this service.
	// This should be called after Initialize.
	BuildHandler() http.Handler

	// Close cleans up any resources used by the service.
	Close() error
}

// ProcessService is an optional interface for services that manage their own process.
// If a service implements this interface, Run will call RunProcess instead of
// creating an HTTP server with BuildHandler.
type ProcessService interface {
	Service

	// RunProcess starts the service process and blocks until it exits or ctx is cancelled.
	// The port parameter specifies the port the service should listen on.
	RunProcess(ctx context.Context, port int) error
}

// ServiceFactory is a function that creates a new Service instance.
type ServiceFactory func() (Service, error)

// registry holds all registered services.
var (
	registry   = make(map[string]ServiceFactory)
	registryMu sync.RWMutex
)

// Register adds a service factory to the global registry.
// This is typically called from an init() function in the generated service package.
func Register(name string, factory ServiceFactory) {
	registryMu.Lock()
	defer registryMu.Unlock()

	if _, exists := registry[name]; exists {
		log.Printf("warning: service %q already registered, overwriting", name)
	}
	registry[name] = factory
}

// Get retrieves a service factory by name.
func Get(name string) (ServiceFactory, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	factory, ok := registry[name]
	return factory, ok
}

// List returns the names of all registered services, sorted alphabetically.
func List() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()

	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Options configures service startup.
type Options struct {
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// DefaultOptions returns sensible default options.
func DefaultOptions() Options {
	return Options{
		Port:         8080,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// RunMultiple starts multiple services on a single port, each mounted at /{service-name}/.
// It blocks until interrupted.
func RunMultiple(names []string, opts Options) error {
	if len(names) == 0 {
		return fmt.Errorf("no services specified")
	}
	if len(names) == 1 {
		return Run(names[0], opts)
	}

	// Create all services
	services := make([]Service, 0, len(names))
	for _, name := range names {
		factory, ok := Get(name)
		if !ok {
			return fmt.Errorf("service %q not found", name)
		}
		svc, err := factory()
		if err != nil {
			// Clean up already-created services
			for _, s := range services {
				s.Close()
			}
			return fmt.Errorf("creating service %q: %w", name, err)
		}
		services = append(services, svc)
	}

	// Ensure cleanup
	defer func() {
		for _, svc := range services {
			svc.Close()
		}
	}()

	// Get port
	port := opts.Port
	if envPort := os.Getenv("PORT"); envPort != "" && port == 0 {
		fmt.Sscanf(envPort, "%d", &port)
	}
	if port == 0 {
		port = 8080
	}

	// Build combined mux
	mux := http.NewServeMux()

	// Shared health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Mount each service at /{name}/
	for i, svc := range services {
		name := names[i]
		handler := svc.BuildHandler()
		prefix := "/" + name
		mux.Handle(prefix+"/", http.StripPrefix(prefix, handler))
		log.Printf("  Mounted %s at %s/", name, prefix)
	}

	// Root handler lists available services
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><head><title>Petri Pilot Services</title></head><body>"))
		w.Write([]byte("<h1>Available Services</h1><ul>"))
		for _, name := range names {
			fmt.Fprintf(w, `<li><a href="/%s/">%s</a></li>`, name, name)
		}
		w.Write([]byte("</ul></body></html>"))
	})

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  opts.ReadTimeout,
		WriteTimeout: opts.WriteTimeout,
		IdleTimeout:  opts.IdleTimeout,
	}

	// Start server
	errCh := make(chan error, 1)
	go func() {
		log.Printf("Starting multi-service server on http://localhost:%d", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for interrupt
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		log.Println("Shutting down server...")
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	log.Println("Server stopped")
	return nil
}

// Run starts a service and blocks until interrupted.
func Run(name string, opts Options) error {
	factory, ok := Get(name)
	if !ok {
		return fmt.Errorf("service %q not found", name)
	}

	svc, err := factory()
	if err != nil {
		return fmt.Errorf("creating service %q: %w", name, err)
	}
	defer svc.Close()

	// Get port from environment if not specified
	port := opts.Port
	if envPort := os.Getenv("PORT"); envPort != "" && port == 0 {
		fmt.Sscanf(envPort, "%d", &port)
	}
	if port == 0 {
		port = 8080
	}

	// Check if this is a process-based service
	if procSvc, ok := svc.(ProcessService); ok {
		return runProcessService(procSvc, port)
	}

	// Standard HTTP handler service
	return runHTTPService(svc, port, opts)
}

// runProcessService runs a service that manages its own process.
func runProcessService(svc ProcessService, port int) error {
	// Create context that cancels on interrupt signal
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Run service in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- svc.RunProcess(ctx, port)
	}()

	// Wait for interrupt or error
	select {
	case <-quit:
		log.Println("Shutting down service...")
		cancel()
		// Wait for service to stop (with timeout)
		select {
		case <-errCh:
		case <-time.After(30 * time.Second):
			log.Println("Service shutdown timed out")
		}
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			return fmt.Errorf("service error: %w", err)
		}
	}

	log.Println("Service stopped")
	return nil
}

// runHTTPService runs a standard HTTP handler service.
func runHTTPService(svc Service, port int, opts Options) error {
	handler := svc.BuildHandler()

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      handler,
		ReadTimeout:  opts.ReadTimeout,
		WriteTimeout: opts.WriteTimeout,
		IdleTimeout:  opts.IdleTimeout,
	}

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		log.Printf("Starting %s server on http://localhost:%d", svc.Name(), port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		log.Println("Shutting down server...")
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	}

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	log.Println("Server stopped")
	return nil
}

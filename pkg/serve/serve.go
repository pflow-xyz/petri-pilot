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

	handler := svc.BuildHandler()

	// Get port from environment if not specified
	port := opts.Port
	if envPort := os.Getenv("PORT"); envPort != "" && port == 0 {
		fmt.Sscanf(envPort, "%d", &port)
	}
	if port == 0 {
		port = 8080
	}

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
		log.Printf("Starting %s server on http://localhost:%d", name, port)
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

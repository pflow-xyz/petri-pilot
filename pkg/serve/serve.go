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
	"path/filepath"
	"sort"
	"strings"
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

// GraphQLService is an optional interface for services that support the unified GraphQL endpoint.
// Services implementing this interface will have their schemas combined into a single GraphQL API.
type GraphQLService interface {
	Service

	// GraphQLSchema returns the GraphQL schema definition string for this service.
	GraphQLSchema() string

	// GraphQLResolvers returns a map of resolver functions for this service.
	// Keys are operation names (e.g., "erc20token", "erc20tokenList", "erc20token_transfer").
	GraphQLResolvers() map[string]GraphQLResolver
}

// GraphQLResolver is a function that handles a GraphQL operation.
type GraphQLResolver func(ctx context.Context, variables map[string]any) (any, error)

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

	// Determine base URL for OAuth callbacks
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://localhost:%d", port)
	}

	// Initialize auth handler
	authHandler := NewAuthHandler(baseURL)
	if authHandler.Enabled() {
		log.Printf("  GitHub OAuth enabled")
	}

	// Build combined mux
	mux := http.NewServeMux()

	// Register auth routes
	authHandler.RegisterRoutes(mux)

	// Shared health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Collect GraphQL-enabled services and create unified endpoint
	var graphqlServices []GraphQLService
	for _, svc := range services {
		if gqlSvc, ok := svc.(GraphQLService); ok {
			graphqlServices = append(graphqlServices, gqlSvc)
		}
	}
	if len(graphqlServices) > 0 {
		unifiedGraphQL := NewUnifiedGraphQL(graphqlServices)
		mux.Handle("/graphql", unifiedGraphQL.Handler())
		mux.HandleFunc("/graphql/i", PlaygroundHandler("/graphql"))
		mux.HandleFunc("/schema", unifiedGraphQL.SchemaHandler())
		log.Printf("  Unified GraphQL at /graphql (%d services)", len(graphqlServices))
		log.Printf("  GraphQL Playground at /graphql/i")
	}

	// Mount each service at /{name}/ and /~{name}/
	for i, svc := range services {
		name := names[i]
		handler := svc.BuildHandler()
		prefix := "/" + name
		mux.Handle(prefix+"/", http.StripPrefix(prefix, handler))
		log.Printf("  Mounted %s at %s/", name, prefix)

		// Always mount generated frontend at /~{name}/ for dashboard access
		// Dashboard requires authentication
		packageName := strings.ReplaceAll(name, "-", "")
		generatedPath := filepath.Join("generated", packageName, "frontend")
		if _, err := os.Stat(generatedPath); err == nil {
			genPrefix := "/~" + name
			spaHandler := createSPAHandler(generatedPath)
			// Create combined handler that proxies API calls to main service
			genHandler := createGeneratedFrontendHandler(spaHandler, handler)
			// Wrap with auth middleware - dashboard requires authentication
			protectedHandler := authHandler.RequireAuth(http.StripPrefix(genPrefix, genHandler))
			mux.Handle(genPrefix+"/", protectedHandler)
			log.Printf("  Mounted %s dash at %s/ (auth required)", name, genPrefix)
		}
	}

	// Root handler - serve landing page if it exists, otherwise list services
	if _, err := os.Stat("landing"); err == nil {
		// Serve landing page directory
		landingHandler := createSPAHandler("landing")
		mux.Handle("/", landingHandler)
		log.Printf("  Serving landing page from landing/")
	} else {
		// Fallback: list available services
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)

			// Build service cards
			var cards strings.Builder
			for _, name := range names {
				cards.WriteString(fmt.Sprintf(`
					<div class="service-card">
						<h2>%s</h2>
						<div class="links">
							<a href="/%s/" class="btn btn-primary">Open App</a>
							<a href="/~%s/" class="btn btn-secondary">Dashboard</a>
						</div>
					</div>`, name, name, name))
			}

			html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Petri Pilot</title>
	<style>
		* { box-sizing: border-box; margin: 0; padding: 0; }
		body {
			font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
			background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
			min-height: 100vh;
			padding: 2rem;
		}
		.container {
			max-width: 900px;
			margin: 0 auto;
		}
		header {
			text-align: center;
			color: white;
			margin-bottom: 3rem;
		}
		header h1 {
			font-size: 2.5rem;
			margin-bottom: 0.5rem;
		}
		header p {
			opacity: 0.9;
			font-size: 1.1rem;
		}
		.services {
			display: grid;
			grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
			gap: 1.5rem;
		}
		.service-card {
			background: white;
			border-radius: 12px;
			padding: 1.5rem;
			box-shadow: 0 4px 20px rgba(0,0,0,0.15);
			transition: transform 0.2s, box-shadow 0.2s;
		}
		.service-card:hover {
			transform: translateY(-4px);
			box-shadow: 0 8px 30px rgba(0,0,0,0.2);
		}
		.service-card h2 {
			color: #333;
			margin-bottom: 1rem;
			font-size: 1.25rem;
		}
		.links {
			display: flex;
			gap: 0.75rem;
		}
		.btn {
			flex: 1;
			padding: 0.6rem 1rem;
			border-radius: 6px;
			text-decoration: none;
			font-weight: 500;
			text-align: center;
			transition: opacity 0.2s;
		}
		.btn:hover { opacity: 0.9; }
		.btn-primary {
			background: #667eea;
			color: white;
		}
		.btn-secondary {
			background: #f0f0f0;
			color: #333;
		}
	</style>
</head>
<body>
	<div class="container">
		<header>
			<h1>Petri Pilot</h1>
			<p>Event-sourced applications from Petri net models</p>
		</header>
		<div class="services">%s</div>
	</div>
</body>
</html>`, cards.String())
			w.Write([]byte(html))
		})
	}

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
	name := svc.Name()

	// Determine base URL for OAuth callbacks
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://localhost:%d", port)
	}

	// Initialize auth handler for single-service mode
	authHandler := NewAuthHandler(baseURL)
	if authHandler.Enabled() {
		log.Printf("  GitHub OAuth enabled")
	}

	// Always mount generated frontend at /~{name}/ for dashboard access
	var finalHandler http.Handler = handler
	packageName := strings.ReplaceAll(name, "-", "")
	generatedPath := filepath.Join("generated", packageName, "frontend")
	if _, err := os.Stat(generatedPath); err == nil {
		// Create mux to handle both main handler and generated frontend
		mux := http.NewServeMux()

		// Register auth routes
		authHandler.RegisterRoutes(mux)

		genPrefix := "/~" + name
		spaHandler := createSPAHandler(generatedPath)
		genHandler := createGeneratedFrontendHandler(spaHandler, handler)
		// Wrap with auth middleware - dashboard requires authentication
		protectedHandler := authHandler.RequireAuth(http.StripPrefix(genPrefix, genHandler))
		mux.Handle(genPrefix+"/", protectedHandler)
		// Default handler for everything else
		mux.Handle("/", handler)
		finalHandler = mux
		log.Printf("  Dash available at %s/ (auth required)", genPrefix)
	}

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      finalHandler,
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

// createGeneratedFrontendHandler creates a handler that serves only the generated frontend.
// API calls should go to the main service URL (without ~), not through the dash.
func createGeneratedFrontendHandler(spaHandler, _ http.Handler) http.Handler {
	// Dash only serves the frontend - no API proxying
	return spaHandler
}

// createSPAHandler creates an HTTP handler for serving a single-page application.
// It serves static files and falls back to index.html for SPA routing.
func createSPAHandler(frontendPath string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clean the path
		path := filepath.Clean(r.URL.Path)
		if path == "/" {
			path = "/index.html"
		}

		// Try to serve the file
		fullPath := filepath.Join(frontendPath, path)
		if _, err := os.Stat(fullPath); err == nil {
			http.ServeFile(w, r, fullPath)
			return
		}

		// For SPA routing, serve index.html for non-existent paths
		// (but not for paths that look like static assets)
		ext := filepath.Ext(path)
		if ext == "" || ext == ".html" {
			http.ServeFile(w, r, filepath.Join(frontendPath, "index.html"))
			return
		}

		// 404 for missing static assets
		http.NotFound(w, r)
	})
}

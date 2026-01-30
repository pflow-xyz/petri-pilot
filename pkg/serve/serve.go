// Package serve provides a service registration framework for generated Petri-pilot services.
// It allows generated services to register themselves at init() time and be started via the CLI.
package serve

import (
	"context"
	"encoding/json"
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

		// Add virtual models (analysis tools) to the model list
		allModels := append([]string{}, names...)
		allModels = append(allModels, "hand-strength", "deck-tracker", "poker-hand")

		mux.HandleFunc("/models", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			json.NewEncoder(w).Encode(allModels)
		})

		// Hand strength Petri net model endpoint
		mux.HandleFunc("/hand-strength/api/schema", HandStrengthModelHandler())
		mux.HandleFunc("/hand-strength/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/pflow?model=hand-strength", http.StatusTemporaryRedirect)
		})

		// Deck tracker Petri net model endpoint
		mux.HandleFunc("/deck-tracker/api/schema", DeckTrackerModelHandler())
		mux.HandleFunc("/deck-tracker/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/pflow?model=deck-tracker", http.StatusTemporaryRedirect)
		})

		// Poker hand evaluator Petri net model endpoint
		mux.HandleFunc("/poker-hand/api/schema", PokerHandModelHandler())
		mux.HandleFunc("/poker-hand/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/pflow?model=poker-hand", http.StatusTemporaryRedirect)
		})

		mux.HandleFunc("/pflow", PflowHandler())
		log.Printf("  Unified GraphQL at /graphql (%d services)", len(graphqlServices))
		log.Printf("  GraphQL Playground at /graphql/i")
		log.Printf("  Petri Net Viewer at /pflow")
		log.Printf("  Hand Strength Model at /hand-strength/")
	}

	// Serve shared frontend assets (used by custom frontends via ../shared/ imports)
	sharedPath := filepath.Join("frontends", "shared")
	if _, err := os.Stat(sharedPath); err == nil {
		mux.Handle("/shared/", http.StripPrefix("/shared/", http.FileServer(http.Dir(sharedPath))))
	}

	// Mount each service at /{name}/ and /app/{name}/
	// Check for custom frontends first and mount them at /{name}/
	for i, svc := range services {
		name := names[i]
		handler := svc.BuildHandler()
		prefix := "/" + name
		
		// Check if there's a custom frontend for this service
		customFrontendPath := filepath.Join("frontends", name)
		if _, err := os.Stat(customFrontendPath); err == nil {
			// Mount custom frontend at /{name}/
			customHandler := createSPAHandler(customFrontendPath)
			// Combine custom frontend with API handler (API calls go to service)
			combinedHandler := createGeneratedFrontendHandler(customHandler, handler)
			mux.Handle(prefix+"/", http.StripPrefix(prefix, combinedHandler))
			log.Printf("  Mounted %s custom frontend at %s/", name, prefix)
		} else {
			// No custom frontend, mount service handler directly
			mux.Handle(prefix+"/", http.StripPrefix(prefix, handler))
			log.Printf("  Mounted %s at %s/", name, prefix)
		}

		// Always mount generated frontend at /app/{name}/ for dashboard access
		// Dashboard requires authentication
		packageName := strings.ReplaceAll(name, "-", "")
		generatedPath := filepath.Join("generated", packageName, "frontend")
		if _, err := os.Stat(generatedPath); err == nil {
			genPrefix := "/app/" + name
			spaHandler := createSPAHandler(generatedPath)
			// Create combined handler that proxies API calls to main service
			genHandler := createGeneratedFrontendHandler(spaHandler, handler)
			// Wrap with auth middleware - dashboard requires authentication
			protectedHandler := authHandler.RequireAuth(http.StripPrefix(genPrefix, genHandler))
			mux.Handle(genPrefix+"/", protectedHandler)
			log.Printf("  Mounted %s dash at %s/ (auth required)", name, genPrefix)
		}
	}

	// Explicitly return 404 for /frontends/ routes (legacy path)
	mux.HandleFunc("/frontends/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

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
							<a href="/app/%s/" class="btn btn-secondary">Dashboard</a>
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

	// Always mount generated frontend at /app/{name}/ for dashboard access
	var finalHandler http.Handler = handler
	packageName := strings.ReplaceAll(name, "-", "")
	generatedPath := filepath.Join("generated", packageName, "frontend")
	if _, err := os.Stat(generatedPath); err == nil {
		// Create mux to handle both main handler and generated frontend
		mux := http.NewServeMux()

		// Register auth routes
		authHandler.RegisterRoutes(mux)

		// Serve shared frontend assets (used by custom frontends via ../shared/ imports)
		sharedPath := filepath.Join("frontends", "shared")
		if _, err := os.Stat(sharedPath); err == nil {
			mux.Handle("/shared/", http.StripPrefix("/shared/", http.FileServer(http.Dir(sharedPath))))
		}

		genPrefix := "/app/" + name
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

// createGeneratedFrontendHandler creates a handler that combines frontend and API routing.
// API calls (paths starting with /api/) are routed to the service handler.
// All other requests are served by the SPA handler.
func createGeneratedFrontendHandler(spaHandler, serviceHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Route API calls to the service handler
		if strings.HasPrefix(r.URL.Path, "/api/") {
			serviceHandler.ServeHTTP(w, r)
			return
		}
		// Everything else goes to the SPA handler
		spaHandler.ServeHTTP(w, r)
	})
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

		// Serve favicon.svg as fallback for favicon.ico requests
		if path == "/favicon.ico" {
			svgPath := filepath.Join(frontendPath, "favicon.svg")
			if _, err := os.Stat(svgPath); err == nil {
				w.Header().Set("Content-Type", "image/svg+xml")
				http.ServeFile(w, r, svgPath)
				return
			}
		}

		// 404 for missing static assets
		http.NotFound(w, r)
	})
}

// HandStrengthModelHandler returns a handler that serves the hand strength Petri net model.
// Query params: hole (e.g., "Ah,Kd"), community (e.g., "Qs,Jh,Tc")
func HandStrengthModelHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// Parse query parameters for cards
		holeStr := r.URL.Query().Get("hole")
		communityStr := r.URL.Query().Get("community")

		// Default example hand: AK suited vs QJs flop
		if holeStr == "" {
			holeStr = "Ah,Kh"
		}
		if communityStr == "" {
			communityStr = "Qs,Jh,Tc"
		}

		// Build the hand strength model
		model := buildHandStrengthModel(holeStr, communityStr)
		json.NewEncoder(w).Encode(model)
	}
}

// buildHandStrengthModel creates a Petri net model for hand strength computation.
func buildHandStrengthModel(holeStr, communityStr string) map[string]interface{} {
	// Parse cards to compute example values
	holeCards := strings.Split(holeStr, ",")
	communityCards := strings.Split(communityStr, ",")

	// Calculate preflop quality (simplified)
	preflopQuality := 0.0
	if len(holeCards) >= 2 {
		// Check for high cards, pairs, suited, connected
		c1, c2 := strings.TrimSpace(holeCards[0]), strings.TrimSpace(holeCards[1])
		r1 := strings.Index("23456789TJQKA", string(c1[0]))
		r2 := strings.Index("23456789TJQKA", string(c2[0]))
		if r1 < 0 {
			r1 = 0
		}
		if r2 < 0 {
			r2 = 0
		}
		highRank := r1
		if r2 > r1 {
			highRank = r2
		}
		lowRank := r1
		if r2 < r1 {
			lowRank = r2
		}

		// Pocket pair
		if r1 == r2 {
			preflopQuality = 0.4 + float64(highRank)/12*0.5
		} else {
			// High cards + connectivity
			preflopQuality = float64(highRank)/12*0.25 + float64(lowRank)/12*0.1
			// Suited bonus
			if len(c1) > 1 && len(c2) > 1 && c1[len(c1)-1] == c2[len(c2)-1] {
				preflopQuality += 0.1
			}
			// Connectivity
			gap := highRank - lowRank
			if gap == 1 {
				preflopQuality += 0.12
			} else if gap == 2 {
				preflopQuality += 0.08
			}
			// Broadway
			if lowRank >= 8 {
				preflopQuality += 0.15
			}
		}
		if preflopQuality > 1 {
			preflopQuality = 1
		}
	}

	// Count community cards for decay factor
	numCommunity := len(communityCards)
	if communityStr == "" {
		numCommunity = 0
	}
	preflopWeight := 1.0 - float64(numCommunity)*0.15
	if preflopWeight < 0.2 {
		preflopWeight = 0.2
	}

	// Calculate draws (simplified)
	flushDraw := 0.0
	straightDraw := 0.0
	if numCommunity >= 3 {
		// Check for flush draw (simplified - count suits)
		suits := make(map[byte]int)
		for _, c := range append(holeCards, communityCards...) {
			c = strings.TrimSpace(c)
			if len(c) > 1 {
				suits[c[len(c)-1]]++
			}
		}
		for _, count := range suits {
			if count >= 4 {
				flushDraw = 0.36 // 9 outs * 4%
			} else if count == 3 {
				flushDraw = 0.18 // Backdoor flush draw
			}
		}
		// Simplified straight draw detection
		straightDraw = 0.16 // Assume OESD (8 outs * 2%)
	}

	// Build the Petri net model
	return map[string]interface{}{
		"name":        "hand-strength",
		"description": fmt.Sprintf("Hand Strength ODE Model - Hole: %s, Community: %s", holeStr, communityStr),
		"places": []map[string]interface{}{
			{"id": "preflop_quality", "initial": int(preflopQuality * preflopWeight * 100), "x": 100, "y": 50},
			{"id": "current_hand", "initial": 0, "x": 100, "y": 150},
			{"id": "kicker_value", "initial": 0, "x": 100, "y": 250},
			{"id": "flush_draw", "initial": int(flushDraw * 100), "x": 300, "y": 50},
			{"id": "straight_draw", "initial": int(straightDraw * 100), "x": 300, "y": 150},
			{"id": "pair_draw", "initial": 0, "x": 300, "y": 250},
			{"id": "improvement_potential", "initial": 0, "x": 200, "y": 350},
			{"id": "hand_strength", "initial": 0, "x": 500, "y": 150},
		},
		"transitions": []map[string]interface{}{
			{"id": "compute_strength", "x": 350, "y": 150},
			{"id": "add_flush_value", "x": 420, "y": 50},
			{"id": "add_straight_value", "x": 420, "y": 150},
			{"id": "add_pair_value", "x": 420, "y": 250},
		},
		"arcs": []map[string]interface{}{
			{"from": "preflop_quality", "to": "compute_strength"},
			{"from": "current_hand", "to": "compute_strength"},
			{"from": "kicker_value", "to": "compute_strength"},
			{"from": "improvement_potential", "to": "compute_strength"},
			{"from": "compute_strength", "to": "hand_strength"},
			{"from": "flush_draw", "to": "add_flush_value"},
			{"from": "add_flush_value", "to": "hand_strength"},
			{"from": "straight_draw", "to": "add_straight_value"},
			{"from": "add_straight_value", "to": "hand_strength"},
			{"from": "pair_draw", "to": "add_pair_value"},
			{"from": "add_pair_value", "to": "hand_strength"},
		},
	}
}

// DeckTrackerModelHandler returns a handler that serves the deck tracker Petri net model.
// Query params: hole (e.g., "Ah,Kd"), community (e.g., "Qs,Jh,Tc"), folded (e.g., "2c,3d")
func DeckTrackerModelHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// Parse query parameters
		holeStr := r.URL.Query().Get("hole")
		communityStr := r.URL.Query().Get("community")

		// Build the deck tracker model
		model := buildDeckTrackerModel(holeStr, communityStr)
		json.NewEncoder(w).Encode(model)
	}
}

// buildDeckTrackerModel creates a Petri net showing all 52 cards and their states.
func buildDeckTrackerModel(holeStr, communityStr string) map[string]interface{} {
	ranks := []string{"A", "K", "Q", "J", "T", "9", "8", "7", "6", "5", "4", "3", "2"}
	suits := []string{"h", "d", "c", "s"} // hearts, diamonds, clubs, spades
	suitNames := map[string]string{"h": "hearts", "d": "diamonds", "c": "clubs", "s": "spades"}
	suitSymbols := map[string]string{"h": "♥", "d": "♦", "c": "♣", "s": "♠"}

	// Parse dealt cards
	dealtCards := make(map[string]string) // card -> location (hole, community)
	if holeStr != "" {
		for _, c := range strings.Split(holeStr, ",") {
			c = strings.TrimSpace(c)
			if c != "" {
				dealtCards[normalizeCard(c)] = "hole"
			}
		}
	}
	if communityStr != "" {
		for _, c := range strings.Split(communityStr, ",") {
			c = strings.TrimSpace(c)
			if c != "" {
				dealtCards[normalizeCard(c)] = "community"
			}
		}
	}

	// Build places for each card, organized by suit
	places := []map[string]interface{}{}

	// Summary places at the top
	deckCount := 52 - len(dealtCards)
	holeCount := 0
	communityCount := 0
	for _, loc := range dealtCards {
		if loc == "hole" {
			holeCount++
		} else {
			communityCount++
		}
	}

	places = append(places,
		map[string]interface{}{"id": "deck_remaining", "initial": deckCount, "x": 100, "y": 30},
		map[string]interface{}{"id": "hole_cards", "initial": holeCount, "x": 250, "y": 30},
		map[string]interface{}{"id": "community_cards", "initial": communityCount, "x": 400, "y": 30},
	)

	// Card places organized in a grid by suit and rank
	for suitIdx, suit := range suits {
		for rankIdx, rank := range ranks {
			cardID := rank + suit
			x := 50 + rankIdx*45
			y := 100 + suitIdx*80

			// Check if card is dealt
			initial := 1
			location, dealt := dealtCards[cardID]
			if dealt {
				initial = 0
				_ = location // Could use this for coloring
			}

			places = append(places, map[string]interface{}{
				"id":      fmt.Sprintf("%s%s", rank, suitSymbols[suit]),
				"initial": initial,
				"x":       x,
				"y":       y,
			})
		}
	}

	// Add suit labels as annotation places
	for suitIdx, suit := range suits {
		places = append(places, map[string]interface{}{
			"id":      suitNames[suit],
			"initial": 0,
			"x":       620,
			"y":       100 + suitIdx*80,
		})
	}

	// Transitions for dealing
	transitions := []map[string]interface{}{
		{"id": "deal_hole", "x": 175, "y": 30},
		{"id": "deal_community", "x": 325, "y": 30},
	}

	// Arcs from deck to dealing transitions
	arcs := []map[string]interface{}{
		{"from": "deck_remaining", "to": "deal_hole"},
		{"from": "deal_hole", "to": "hole_cards"},
		{"from": "deck_remaining", "to": "deal_community"},
		{"from": "deal_community", "to": "community_cards"},
	}

	description := "Deck Tracker - 52 cards"
	if holeStr != "" || communityStr != "" {
		description = fmt.Sprintf("Deck Tracker - Hole: %s, Community: %s (%d remaining)",
			holeStr, communityStr, deckCount)
	}

	return map[string]interface{}{
		"name":        "deck-tracker",
		"description": description,
		"places":      places,
		"transitions": transitions,
		"arcs":        arcs,
	}
}

// normalizeCard normalizes card notation (e.g., "10h" -> "Th", "ah" -> "Ah")
func normalizeCard(card string) string {
	card = strings.TrimSpace(card)
	if len(card) < 2 {
		return card
	}
	// Handle "10" as "T"
	if strings.HasPrefix(card, "10") {
		card = "T" + card[2:]
	}
	// Uppercase rank
	if len(card) >= 1 {
		card = strings.ToUpper(card[:1]) + strings.ToLower(card[1:])
	}
	return card
}

// PokerHandModelHandler returns a handler that serves the poker hand evaluator Petri net.
// This model detects hand patterns like pairs, flushes, straights using Petri net structure.
func PokerHandModelHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		holeStr := r.URL.Query().Get("hole")
		communityStr := r.URL.Query().Get("community")

		// Default example: Full house (Aces full of Kings)
		if holeStr == "" {
			holeStr = "Ah,Ad"
		}
		if communityStr == "" {
			communityStr = "As,Kd,Kc"
		}

		model := buildPokerHandModel(holeStr, communityStr)
		json.NewEncoder(w).Encode(model)
	}
}

// buildPokerHandModel creates a Petri net for poker hand evaluation with pattern detection.
func buildPokerHandModel(holeStr, communityStr string) map[string]interface{} {
	ranks := []string{"A", "K", "Q", "J", "T", "9", "8", "7", "6", "5", "4", "3", "2"}
	suits := []string{"h", "d", "c", "s"}
	suitSymbols := map[string]string{"h": "♥", "d": "♦", "c": "♣", "s": "♠"}

	// Parse cards in hand
	inHand := make(map[string]bool)
	if holeStr != "" {
		for _, c := range strings.Split(holeStr, ",") {
			inHand[normalizeCard(strings.TrimSpace(c))] = true
		}
	}
	if communityStr != "" {
		for _, c := range strings.Split(communityStr, ",") {
			inHand[normalizeCard(strings.TrimSpace(c))] = true
		}
	}

	// Count ranks and suits in hand
	rankCounts := make(map[string]int)
	suitCounts := make(map[string]int)
	for card := range inHand {
		if len(card) >= 2 {
			rank := string(card[0])
			suit := string(card[len(card)-1])
			rankCounts[rank]++
			suitCounts[suit]++
		}
	}

	places := []map[string]interface{}{}
	transitions := []map[string]interface{}{}
	arcs := []map[string]interface{}{}

	// === CARD PLACES (organized by rank for pattern detection) ===
	// Each card has token=1 if in hand, 0 if not
	// Layout: 4 columns (suits) x 13 rows (ranks) - with generous spacing
	for rankIdx, rank := range ranks {
		for suitIdx, suit := range suits {
			cardID := rank + suit
			x := 50 + suitIdx*80
			y := 50 + rankIdx*65

			initial := 0
			if inHand[cardID] {
				initial = 1
			}

			places = append(places, map[string]interface{}{
				"id":      fmt.Sprintf("%s%s", rank, suitSymbols[suit]),
				"initial": initial,
				"x":       x,
				"y":       y,
			})
		}
	}

	// === HAND TYPE OUTPUT PLACES ===
	// These collect tokens when patterns are detected
	handPlaces := []struct {
		id       string
		strength int // Poker hand ranking value
	}{
		{"pair", 2},
		{"two_pair", 3},
		{"three_kind", 4},
		{"straight", 5},
		{"flush", 6},
		{"full_house", 7},
		{"four_kind", 8},
		{"straight_flush", 9},
	}

	for i, hp := range handPlaces {
		places = append(places, map[string]interface{}{
			"id":      hp.id,
			"initial": 0,
			"x":       1100,
			"y":       50 + i*100,
		})
	}

	// === SCORING TRANSITIONS (convert hand types to numeric strength) ===
	for i, hp := range handPlaces {
		transID := fmt.Sprintf("score_%s", hp.id)
		transitions = append(transitions, map[string]interface{}{
			"id": transID,
			"x":  1250,
			"y":  50 + i*100,
		})
		// Arc from hand type place to scoring transition
		arcs = append(arcs, map[string]interface{}{
			"from": hp.id,
			"to":   transID,
		})
		// Arc from scoring transition to hand_strength with weight = hand value
		arcs = append(arcs, map[string]interface{}{
			"from":   transID,
			"to":     "hand_strength",
			"weight": hp.strength,
		})
	}

	// === FINAL HAND STRENGTH PLACE ===
	places = append(places, map[string]interface{}{
		"id":      "hand_strength",
		"initial": 0,
		"x":       1400,
		"y":       400,
	})

	// === PAIR DETECTION TRANSITIONS (78 total: 6 combinations × 13 ranks) ===
	// C(4,2) = 6 ways to choose 2 suits from 4: hd, hc, hs, dc, ds, cs
	pairCombos := [][2]string{{"h", "d"}, {"h", "c"}, {"h", "s"}, {"d", "c"}, {"d", "s"}, {"c", "s"}}
	for rankIdx, rank := range ranks {
		for comboIdx, combo := range pairCombos {
			s1, s2 := combo[0], combo[1]
			transID := fmt.Sprintf("pair_%s_%s%s", rank, s1, s2)
			transitions = append(transitions, map[string]interface{}{
				"id": transID,
				"x":  420 + comboIdx*40,
				"y":  50 + rankIdx*65,
			})
			// Input arcs from the two specific cards
			arcs = append(arcs, map[string]interface{}{
				"from": fmt.Sprintf("%s%s", rank, suitSymbols[s1]),
				"to":   transID,
			})
			arcs = append(arcs, map[string]interface{}{
				"from": fmt.Sprintf("%s%s", rank, suitSymbols[s2]),
				"to":   transID,
			})
			// Output arc to pair place
			arcs = append(arcs, map[string]interface{}{
				"from": transID,
				"to":   "pair",
			})
		}
	}

	// === THREE OF A KIND DETECTION (52 total: 4 combinations × 13 ranks) ===
	// C(4,3) = 4 ways to choose 3 suits from 4: hdc, hds, hcs, dcs
	tripsCombos := [][3]string{{"h", "d", "c"}, {"h", "d", "s"}, {"h", "c", "s"}, {"d", "c", "s"}}
	for rankIdx, rank := range ranks {
		for comboIdx, combo := range tripsCombos {
			s1, s2, s3 := combo[0], combo[1], combo[2]
			transID := fmt.Sprintf("trips_%s_%s%s%s", rank, s1, s2, s3)
			transitions = append(transitions, map[string]interface{}{
				"id": transID,
				"x":  700 + comboIdx*40,
				"y":  50 + rankIdx*65,
			})
			// Input arcs from the three specific cards
			arcs = append(arcs, map[string]interface{}{
				"from": fmt.Sprintf("%s%s", rank, suitSymbols[s1]),
				"to":   transID,
			})
			arcs = append(arcs, map[string]interface{}{
				"from": fmt.Sprintf("%s%s", rank, suitSymbols[s2]),
				"to":   transID,
			})
			arcs = append(arcs, map[string]interface{}{
				"from": fmt.Sprintf("%s%s", rank, suitSymbols[s3]),
				"to":   transID,
			})
			// Output arc to three_kind place
			arcs = append(arcs, map[string]interface{}{
				"from": transID,
				"to":   "three_kind",
			})
		}
	}

	// === FOUR OF A KIND DETECTION (13 total: 1 combination × 13 ranks) ===
	// Only 1 way to have all 4 suits
	for rankIdx, rank := range ranks {
		transID := fmt.Sprintf("quads_%s", rank)
		transitions = append(transitions, map[string]interface{}{
			"id": transID,
			"x":  900,
			"y":  50 + rankIdx*65,
		})
		// Input arcs from all 4 cards
		for _, suit := range suits {
			arcs = append(arcs, map[string]interface{}{
				"from": fmt.Sprintf("%s%s", rank, suitSymbols[suit]),
				"to":   transID,
			})
		}
		// Output arc to four_kind place
		arcs = append(arcs, map[string]interface{}{
			"from": transID,
			"to":   "four_kind",
		})
	}

	// Note: Flush and straight detection omitted - would require thousands of transitions
	// (C(13,5) × 4 = 5148 flush transitions, straights even more complex)
	_ = suitCounts // Used for description

	// Count pairs for description
	pairCount := 0
	for _, count := range rankCounts {
		if count >= 2 {
			pairCount++
		}
	}

	// Full house detection (trips + pair)
	hasTrips := false
	for _, count := range rankCounts {
		if count >= 3 {
			hasTrips = true
			break
		}
	}

	description := "Poker Hand Evaluator (Working Model)"
	if holeStr != "" || communityStr != "" {
		// Determine best hand
		bestHand := "High Card"
		if pairCount >= 1 {
			bestHand = "Pair"
		}
		if pairCount >= 2 {
			bestHand = "Two Pair"
		}
		if hasTrips {
			bestHand = "Three of a Kind"
		}
		// Check for straight
		hasStraight := false
		rankOrder := "AKQJT98765432A"
		for i := 0; i <= len(rankOrder)-5; i++ {
			straightRanks := rankOrder[i : i+5]
			hasAll := true
			for _, r := range straightRanks {
				if rankCounts[string(r)] == 0 {
					hasAll = false
					break
				}
			}
			if hasAll {
				hasStraight = true
				break
			}
		}
		if hasStraight {
			bestHand = "Straight"
		}
		// Check for flush
		hasFlush := false
		for _, count := range suitCounts {
			if count >= 5 {
				hasFlush = true
				bestHand = "Flush"
			}
		}
		if hasTrips && pairCount >= 2 {
			bestHand = "Full House"
		}
		for _, count := range rankCounts {
			if count >= 4 {
				bestHand = "Four of a Kind"
			}
		}
		// Straight flush
		if hasStraight && hasFlush {
			bestHand = "Straight Flush"
		}

		description = fmt.Sprintf("Poker Hand - Hole: %s, Board: %s → %s", holeStr, communityStr, bestHand)
	}

	return map[string]interface{}{
		"name":        "poker-hand",
		"description": description,
		"places":      places,
		"transitions": transitions,
		"arcs":        arcs,
	}
}

// Package serve provides a service registration framework for generated Petri-pilot services.
// It allows generated services to register themselves at init() time and be started via the CLI.
package serve

import (
	"bytes"
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

// googleAnalyticsID is read from GOOGLE_ANALYTICS_ID env var at startup
var googleAnalyticsID = os.Getenv("GOOGLE_ANALYTICS_ID")

// getAnalyticsScript returns the Google Analytics script if configured
func getAnalyticsScript() string {
	if googleAnalyticsID == "" {
		return ""
	}
	return fmt.Sprintf(`<!-- Google Analytics -->
<script async src="https://www.googletagmanager.com/gtag/js?id=%s"></script>
<script>
  window.dataLayer = window.dataLayer || [];
  function gtag(){dataLayer.push(arguments);}
  gtag("js", new Date());
  gtag("config", "%s");
</script>
`, googleAnalyticsID, googleAnalyticsID)
}

// injectAnalytics injects Google Analytics script into HTML
func injectAnalytics(html []byte) []byte {
	script := getAnalyticsScript()
	if script == "" {
		return html
	}
	return bytes.Replace(html, []byte("</head>"), []byte(script+"</head>"), 1)
}

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

// RouteRegistrar is a function that registers custom routes on a mux.
type RouteRegistrar func(mux *http.ServeMux)

// Options configures service startup.
type Options struct {
	Port           int
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
	CustomRoutes   RouteRegistrar // Optional function to register custom routes
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

	// Check for ProcessService (only supported when running a single service)
	if len(services) == 1 {
		if procSvc, ok := services[0].(ProcessService); ok {
			return runProcessService(procSvc, port)
		}
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
	if googleAnalyticsID != "" {
		log.Printf("  Google Analytics: %s", googleAnalyticsID)
	}
	}

	// Build combined mux
	mux := http.NewServeMux()

	// Register auth routes
	authHandler.RegisterRoutes(mux)

	// Register deploy webhook/admin routes
	RegisterDeployRoutes(mux)

	// Register code-to-flow API (requires ANTHROPIC_API_KEY)
	RegisterCodeToFlowAPI(mux)

	// Register custom routes if provided
	if opts.CustomRoutes != nil {
		opts.CustomRoutes(mux)
	}

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
		// Use go-pflow's GraphQL server for query execution
		unifiedGraphQL := NewUnifiedGraphQLFromGoPflow(graphqlServices)
		mux.Handle("/graphql", unifiedGraphQL.Handler())
		mux.HandleFunc("/graphql/i", PlaygroundHandler("/graphql"))
		mux.HandleFunc("/schema", unifiedGraphQL.SchemaHandler())

		// Add virtual models (analysis tools) to the model list
		// Filter out zk-tic-tac-toe from the displayed models (frontend still accessible)
		var filteredNames []string
		for _, name := range names {
			if name != "zk-tic-tac-toe" {
				filteredNames = append(filteredNames, name)
			}
		}
		allModels := append([]string{}, filteredNames...)
		allModels = append(allModels, "poker-hand", "vet-clinic", "stoplight")

		mux.HandleFunc("/models", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			json.NewEncoder(w).Encode(allModels)
		})

		// Poker hand evaluator Petri net model endpoint
		mux.HandleFunc("/poker-hand/api/schema", PokerHandModelHandler())

		// Vet clinic model (standalone frontend with embedded model)
		mux.HandleFunc("/vet-clinic/api/schema", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			http.ServeFile(w, r, filepath.Join("frontends", "vet-clinic", "model.json"))
		})

		mux.HandleFunc("/pflow", PflowHandler())
		log.Printf("  Unified GraphQL at /graphql (%d services)", len(graphqlServices))
		log.Printf("  GraphQL Playground at /graphql/i")
		log.Printf("  Petri Net Viewer at /pflow")
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

	// Mount standalone frontends (directories in frontends/ without a backend service)
	mountedNames := make(map[string]bool)
	for _, n := range names {
		mountedNames[n] = true
	}
	mountedNames["shared"] = true // never mount shared as a standalone frontend
	if entries, err := os.ReadDir("frontends"); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() || mountedNames[entry.Name()] {
				continue
			}
			name := entry.Name()
			frontendPath := filepath.Join("frontends", name)
			indexPath := filepath.Join(frontendPath, "index.html")
			if _, err := os.Stat(indexPath); err != nil {
				continue // skip directories without index.html
			}
			prefix := "/" + name
			spaHandler := createSPAHandler(frontendPath)
			mux.Handle(prefix+"/", http.StripPrefix(prefix, spaHandler))
			log.Printf("  Mounted %s standalone frontend at %s/", name, prefix)
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
				packageName := strings.ReplaceAll(name, "-", "")
				generatedPath := filepath.Join("generated", packageName, "frontend")
				dashLink := ""
				if _, err := os.Stat(generatedPath); err == nil {
					dashLink = fmt.Sprintf(`<a href="/app/%s/" class="btn btn-secondary">Dashboard</a>`, name)
				}
				cards.WriteString(fmt.Sprintf(`
					<div class="service-card">
						<h2>%s</h2>
						<div class="links">
							<a href="/%s/" class="btn btn-primary">Open App</a>
							%s
						</div>
					</div>`, name, name, dashLink))
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
// Deprecated: Use RunMultiple instead, which handles single services consistently.
func Run(name string, opts Options) error {
	return RunMultiple([]string{name}, opts)
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

// createGeneratedFrontendHandler creates a handler that combines frontend and API routing.
// API calls (paths starting with /api/, /zk/, or /admin/) are routed to the service handler.
// All other requests are served by the SPA handler.
func createGeneratedFrontendHandler(spaHandler, serviceHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Route API, ZK, and admin calls to the service handler
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/zk/") || strings.HasPrefix(r.URL.Path, "/admin/") {
			serviceHandler.ServeHTTP(w, r)
			return
		}
		// Everything else goes to the SPA handler
		spaHandler.ServeHTTP(w, r)
	})
}

// createSPAHandler creates an HTTP handler for serving a single-page application.
// It serves static files and falls back to index.html for SPA routing.
// serveHTMLWithGA reads an HTML file, injects GA + per-page SEO, and serves it.
// frontendName is the frontend directory name (e.g. "tic-tac-toe") used to
// look up curated SEO metadata; pass "" to skip SEO injection.
func serveHTMLWithGA(w http.ResponseWriter, filePath, frontendName string) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	data = injectSEO(data, frontendName)
	data = injectAnalytics(data)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func createSPAHandler(frontendPath string) http.Handler {
	frontendName := frontendNameFromPath(frontendPath)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clean the path
		path := filepath.Clean(r.URL.Path)
		if path == "/" {
			path = "/index.html"
		}

		// Try to serve the file
		fullPath := filepath.Join(frontendPath, path)
		if _, err := os.Stat(fullPath); err == nil {
			// For HTML files, inject GA + SEO
			if strings.HasSuffix(path, ".html") {
				serveHTMLWithGA(w, fullPath, frontendName)
				return
			}
			http.ServeFile(w, r, fullPath)
			return
		}

		// Return JSON 404 for API routes that don't match a backend service
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/zk/") || strings.HasPrefix(r.URL.Path, "/admin/") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"no backend service available for this route"}`))
			return
		}

		// For SPA routing, serve index.html for non-existent paths
		// (but not for paths that look like static assets)
		ext := filepath.Ext(path)
		if ext == "" || ext == ".html" {
			serveHTMLWithGA(w, filepath.Join(frontendPath, "index.html"), frontendName)
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
	// Each card has two places:
	//   - Hand place (e.g., "A♥") - token=1 if card is in hand
	//   - Deck place (e.g., "deck_A♥") - token=1 if card is still in deck
	// Layout: 4 columns (suits) x 13 rows (ranks) - with generous spacing
	// Deck places are positioned to the left of hand places
	for rankIdx, rank := range ranks {
		for suitIdx, suit := range suits {
			cardID := rank + suit
			cardSymbol := fmt.Sprintf("%s%s", rank, suitSymbols[suit])
			handX := 150 + suitIdx*120
			deckX := -350 + suitIdx*120
			y := 50 + rankIdx*100

			inHandInitial := 0
			inDeckInitial := 1
			if inHand[cardID] {
				inHandInitial = 1
				inDeckInitial = 0
			}

			// Hand place (where cards go when dealt)
			places = append(places, map[string]interface{}{
				"id":      cardSymbol,
				"initial": inHandInitial,
				"x":       handX,
				"y":       y,
			})

			// Deck place (where cards start before being dealt)
			deckPlaceID := fmt.Sprintf("deck_%s", cardSymbol)
			places = append(places, map[string]interface{}{
				"id":      deckPlaceID,
				"initial": inDeckInitial,
				"x":       deckX,
				"y":       y,
			})

			// === DEAL TRANSITIONS (move card from deck to hand) ===
			// Only enabled if card is in deck (has token in deck place)
			dealTransID := fmt.Sprintf("deal_%s", cardSymbol)
			transitions = append(transitions, map[string]interface{}{
				"id": dealTransID,
				"x":  (deckX + handX) / 2,
				"y":  y,
			})
			// Input arc from deck place (consume card from deck)
			arcs = append(arcs, map[string]interface{}{
				"from": deckPlaceID,
				"to":   dealTransID,
			})
			// Output arc to hand place (card appears in hand)
			arcs = append(arcs, map[string]interface{}{
				"from": dealTransID,
				"to":   cardSymbol,
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
			"x":       1600,
			"y":       50 + i*150,
		})
	}

	// === SCORING TRANSITIONS (convert hand types to numeric strength) ===
	for i, hp := range handPlaces {
		transID := fmt.Sprintf("score_%s", hp.id)
		transitions = append(transitions, map[string]interface{}{
			"id": transID,
			"x":  1800,
			"y":  50 + i*150,
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
		"x":       2000,
		"y":       600,
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
				"x":  600 + comboIdx*60,
				"y":  50 + rankIdx*100,
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
				"x":  1020 + comboIdx*60,
				"y":  50 + rankIdx*100,
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
			"x":  1320,
			"y":  50 + rankIdx*100,
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

	// === STRAIGHT FLUSH DETECTION (40 total: 10 straights × 4 suits) ===
	// Specific 5-card sequences in the same suit
	straightStarts := []string{"A", "K", "Q", "J", "T", "9", "8", "7", "6", "5"} // A-high through 5-high (wheel)
	straightRanks := map[string][]string{
		"A": {"A", "K", "Q", "J", "T"}, // Royal/Broadway
		"K": {"K", "Q", "J", "T", "9"},
		"Q": {"Q", "J", "T", "9", "8"},
		"J": {"J", "T", "9", "8", "7"},
		"T": {"T", "9", "8", "7", "6"},
		"9": {"9", "8", "7", "6", "5"},
		"8": {"8", "7", "6", "5", "4"},
		"7": {"7", "6", "5", "4", "3"},
		"6": {"6", "5", "4", "3", "2"},
		"5": {"5", "4", "3", "2", "A"}, // Wheel (A plays low)
	}
	for startIdx, start := range straightStarts {
		for suitIdx, suit := range suits {
			transID := fmt.Sprintf("sf_%s_%s", start, suit)
			transitions = append(transitions, map[string]interface{}{
				"id": transID,
				"x":  600 + suitIdx*80,
				"y":  1400 + startIdx*60,
			})
			// Input arcs from all 5 cards in the straight flush
			for _, rank := range straightRanks[start] {
				arcs = append(arcs, map[string]interface{}{
					"from": fmt.Sprintf("%s%s", rank, suitSymbols[suit]),
					"to":   transID,
				})
			}
			// Output arc to straight_flush place
			arcs = append(arcs, map[string]interface{}{
				"from": transID,
				"to":   "straight_flush",
			})
		}
	}

	// === FLUSH DETECTION (simplified - 4 suits × representative combinations) ===
	// Full detection would need C(13,5)=1287 per suit, so we detect common flush patterns
	// For demonstration, detect when the 5 highest cards of a suit are present
	flushPatterns := [][]string{
		{"A", "K", "Q", "J", "T"}, // Ace-high flush
		{"A", "K", "Q", "J", "9"}, // Ace-high flush variant
		{"K", "Q", "J", "T", "9"}, // King-high flush
		{"Q", "J", "T", "9", "8"}, // Queen-high flush
		{"J", "T", "9", "8", "7"}, // Jack-high flush
		{"T", "9", "8", "7", "6"}, // Ten-high flush
		{"9", "8", "7", "6", "5"}, // Nine-high flush
		{"8", "7", "6", "5", "4"}, // Eight-high flush
		{"7", "6", "5", "4", "3"}, // Seven-high flush
		{"6", "5", "4", "3", "2"}, // Six-high flush
	}
	for patIdx, pattern := range flushPatterns {
		for suitIdx, suit := range suits {
			transID := fmt.Sprintf("flush_%d_%s", patIdx, suit)
			transitions = append(transitions, map[string]interface{}{
				"id": transID,
				"x":  1000 + suitIdx*80,
				"y":  1400 + patIdx*60,
			})
			// Input arcs from the 5 cards
			for _, rank := range pattern {
				arcs = append(arcs, map[string]interface{}{
					"from": fmt.Sprintf("%s%s", rank, suitSymbols[suit]),
					"to":   transID,
				})
			}
			// Output arc to flush place
			arcs = append(arcs, map[string]interface{}{
				"from": transID,
				"to":   "flush",
			})
		}
	}

	// === STRAIGHT DETECTION (10 straights × 4^5 suit combinations is too many) ===
	// Simplified: detect straights using a single suit combination per straight
	// This demonstrates the pattern - full detection would need 10,240 transitions
	for startIdx, start := range straightStarts {
		// Use hearts for demonstration (one transition per straight pattern)
		transID := fmt.Sprintf("straight_%s", start)
		transitions = append(transitions, map[string]interface{}{
			"id": transID,
			"x":  1400,
			"y":  1400 + startIdx*60,
		})
		// Input arcs - one card per rank, cycling through suits
		for i, rank := range straightRanks[start] {
			suit := suits[i%4]
			arcs = append(arcs, map[string]interface{}{
				"from": fmt.Sprintf("%s%s", rank, suitSymbols[suit]),
				"to":   transID,
			})
		}
		// Output arc to straight place
		arcs = append(arcs, map[string]interface{}{
			"from": transID,
			"to":   "straight",
		})
	}

	// === TWO PAIR DETECTION (using pair output) ===
	// Add an intermediate place that collects pair detections
	places = append(places, map[string]interface{}{
		"id":      "pair_count",
		"initial": 0,
		"x":       1450,
		"y":       50,
	})
	// Modify pair transitions to also output to pair_count (already done via pair place)
	// Two pair fires when pair_count has 2 tokens
	transitions = append(transitions, map[string]interface{}{
		"id": "detect_two_pair",
		"x":  1500,
		"y":  200,
	})
	arcs = append(arcs, map[string]interface{}{
		"from":   "pair",
		"to":     "detect_two_pair",
		"weight": 2,
	})
	arcs = append(arcs, map[string]interface{}{
		"from": "detect_two_pair",
		"to":   "two_pair",
	})

	// === FULL HOUSE DETECTION (trips + pair) ===
	transitions = append(transitions, map[string]interface{}{
		"id": "detect_full_house",
		"x":  1500,
		"y":  600,
	})
	arcs = append(arcs, map[string]interface{}{
		"from": "three_kind",
		"to":   "detect_full_house",
	})
	arcs = append(arcs, map[string]interface{}{
		"from": "pair",
		"to":   "detect_full_house",
	})
	arcs = append(arcs, map[string]interface{}{
		"from": "detect_full_house",
		"to":   "full_house",
	})

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

package api

import (
	"net/http"
	"strings"

	"github.com/pflow-xyz/petri-pilot/pkg/schema"
)

// Route defines an API endpoint.
type Route struct {
	// Method is the HTTP method (GET, POST, etc.).
	Method string

	// Path is the URL path pattern.
	Path string

	// Handler processes requests.
	Handler http.HandlerFunc

	// Description for documentation.
	Description string

	// TransitionID links to a Petri net transition (if applicable).
	TransitionID string
}

// Router manages API routes.
type Router struct {
	routes     []Route
	mux        *http.ServeMux
	middleware []Middleware
	prefix     string
}

// NewRouter creates a new router.
func NewRouter() *Router {
	return &Router{
		mux: http.NewServeMux(),
	}
}

// WithPrefix sets a path prefix for all routes.
func (r *Router) WithPrefix(prefix string) *Router {
	r.prefix = strings.TrimSuffix(prefix, "/")
	return r
}

// Use adds middleware to the router.
func (r *Router) Use(mw ...Middleware) *Router {
	r.middleware = append(r.middleware, mw...)
	return r
}

// Handle registers a route.
func (r *Router) Handle(method, path, description string, handler http.HandlerFunc) *Router {
	r.routes = append(r.routes, Route{
		Method:      method,
		Path:        path,
		Handler:     handler,
		Description: description,
	})
	return r
}

// GET registers a GET route.
func (r *Router) GET(path, description string, handler http.HandlerFunc) *Router {
	return r.Handle("GET", path, description, handler)
}

// POST registers a POST route.
func (r *Router) POST(path, description string, handler http.HandlerFunc) *Router {
	return r.Handle("POST", path, description, handler)
}

// PUT registers a PUT route.
func (r *Router) PUT(path, description string, handler http.HandlerFunc) *Router {
	return r.Handle("PUT", path, description, handler)
}

// DELETE registers a DELETE route.
func (r *Router) DELETE(path, description string, handler http.HandlerFunc) *Router {
	return r.Handle("DELETE", path, description, handler)
}

// Transition registers a route for a Petri net transition.
// Accepts either http.HandlerFunc or http.Handler (for middleware-wrapped handlers).
func (r *Router) Transition(transitionID, path, description string, handler http.Handler) *Router {
	// Convert http.Handler to http.HandlerFunc if needed
	var hf http.HandlerFunc
	if fn, ok := handler.(http.HandlerFunc); ok {
		hf = fn
	} else {
		hf = func(w http.ResponseWriter, req *http.Request) {
			handler.ServeHTTP(w, req)
		}
	}
	r.routes = append(r.routes, Route{
		Method:       "POST",
		Path:         path,
		Handler:      hf,
		Description:  description,
		TransitionID: transitionID,
	})
	return r
}

// Build constructs the final HTTP handler.
func (r *Router) Build() http.Handler {
	// Group routes by path
	pathHandlers := make(map[string]map[string]http.HandlerFunc)

	for _, route := range r.routes {
		path := r.prefix + route.Path
		if pathHandlers[path] == nil {
			pathHandlers[path] = make(map[string]http.HandlerFunc)
		}
		pathHandlers[path][route.Method] = route.Handler
	}

	// Register with mux
	for path, methods := range pathHandlers {
		methods := methods // capture for closure
		r.mux.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
			if handler, ok := methods[req.Method]; ok {
				handler(w, req)
				return
			}
			if req.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		})
	}

	// Apply middleware
	var handler http.Handler = r.mux
	for i := len(r.middleware) - 1; i >= 0; i-- {
		handler = r.middleware[i](handler)
	}

	return handler
}

// Routes returns all registered routes.
func (r *Router) Routes() []Route {
	return r.routes
}

// RouterFromModel generates routes from a Petri net model.
func RouterFromModel(model *schema.Model, transitionHandler func(transitionID string) http.HandlerFunc, stateHandler http.HandlerFunc) *Router {
	router := NewRouter()

	// Add state endpoint
	router.GET("/state/{id}", "Get aggregate state", stateHandler)

	// Add transition endpoints
	for _, t := range model.Transitions {
		path := "/transitions/" + t.ID
		description := t.Description
		if description == "" {
			description = "Fire " + t.ID + " transition"
		}
		router.Transition(t.ID, path, description, transitionHandler(t.ID))
	}

	return router
}

// OpenAPISpec generates an OpenAPI specification from routes.
func (r *Router) OpenAPISpec(title, version, description string) map[string]any {
	paths := make(map[string]any)

	for _, route := range r.routes {
		path := r.prefix + route.Path
		if paths[path] == nil {
			paths[path] = make(map[string]any)
		}

		operation := map[string]any{
			"summary":     route.Description,
			"operationId": strings.ToLower(route.Method) + "_" + strings.ReplaceAll(strings.Trim(path, "/"), "/", "_"),
			"responses": map[string]any{
				"200": map[string]any{
					"description": "Successful response",
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{"type": "object"},
						},
					},
				},
				"400": map[string]any{
					"description": "Bad request",
				},
				"500": map[string]any{
					"description": "Internal server error",
				},
			},
		}

		if route.Method == "POST" || route.Method == "PUT" {
			operation["requestBody"] = map[string]any{
				"required": true,
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": map[string]any{"type": "object"},
					},
				},
			}
		}

		if route.TransitionID != "" {
			operation["tags"] = []string{"transitions"}
			operation["x-transition-id"] = route.TransitionID
		}

		paths[path].(map[string]any)[strings.ToLower(route.Method)] = operation
	}

	return map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       title,
			"version":     version,
			"description": description,
		},
		"paths": paths,
	}
}

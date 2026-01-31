package serve

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

// User represents an authenticated GitHub user.
type User struct {
	ID        int64    `json:"id"`
	Login     string   `json:"login"`
	Name      string   `json:"name"`
	Email     string   `json:"email"`
	AvatarURL string   `json:"avatar_url"`
	Roles     []string `json:"roles,omitempty"`
}

// Session represents an authenticated session.
type Session struct {
	User      *User     `json:"user"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// AuthHandler handles GitHub OAuth authentication for the serve layer.
type AuthHandler struct {
	config      *oauth2.Config
	sessions    map[string]*Session
	states      map[string]time.Time
	mu          sync.RWMutex
	frontendURL string
	enabled     bool
}

// NewAuthHandler creates a new auth handler from environment variables.
func NewAuthHandler(baseURL string) *AuthHandler {
	clientID := os.Getenv("GITHUB_CLIENT_ID")
	clientSecret := os.Getenv("GITHUB_CLIENT_SECRET")

	enabled := clientID != "" && clientSecret != ""

	return &AuthHandler{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Scopes:       []string{"user:email"},
			Endpoint:     github.Endpoint,
			RedirectURL:  baseURL + "/auth/callback",
		},
		sessions:    make(map[string]*Session),
		states:      make(map[string]time.Time),
		frontendURL: baseURL,
		enabled:     enabled,
	}
}

// Enabled returns whether GitHub OAuth is configured.
func (h *AuthHandler) Enabled() bool {
	return h.enabled
}

// HandleStatus returns the authentication configuration status.
func (h *AuthHandler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"github_enabled": h.enabled,
	})
}

// HandleLogin redirects to GitHub OAuth.
func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if !h.enabled {
		http.Error(w, "GitHub OAuth not configured", http.StatusServiceUnavailable)
		return
	}

	state := generateToken(16)
	h.mu.Lock()
	h.states[state] = time.Now().Add(10 * time.Minute)
	// Clean up old states
	for s, exp := range h.states {
		if time.Now().After(exp) {
			delete(h.states, s)
		}
	}
	h.mu.Unlock()

	url := h.config.AuthCodeURL(state)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// HandleCallback handles the GitHub OAuth callback.
func (h *AuthHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")

	h.mu.Lock()
	exp, ok := h.states[state]
	if ok {
		delete(h.states, state)
	}
	h.mu.Unlock()

	if !ok || time.Now().After(exp) {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	token, err := h.config.Exchange(r.Context(), code)
	if err != nil {
		http.Error(w, "failed to exchange token", http.StatusInternalServerError)
		return
	}

	// Fetch user info from GitHub
	client := h.config.Client(r.Context(), token)
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		http.Error(w, "failed to fetch user", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var user User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		http.Error(w, "failed to decode user", http.StatusInternalServerError)
		return
	}

	// Create session
	sessionToken := generateToken(32)
	session := &Session{
		User:      &user,
		Token:     sessionToken,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	h.mu.Lock()
	h.sessions[sessionToken] = session
	h.mu.Unlock()

	// Set session cookie for server-side auth checks
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    sessionToken,
		Path:     "/",
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   strings.HasPrefix(h.frontendURL, "https"),
	})

	// Redirect to frontend with token in URL params (for localStorage)
	redirectURL := h.frontendURL + "/?token=" + sessionToken + "&expires_at=" + session.ExpiresAt.Format(time.RFC3339)
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

// HandleLogout invalidates the session.
func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	token := extractToken(r)
	if token != "" {
		h.mu.Lock()
		delete(h.sessions, token)
		h.mu.Unlock()
	}

	// Clear the session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	w.WriteHeader(http.StatusNoContent)
}

// HandleMe returns the current user.
func (h *AuthHandler) HandleMe(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromRequest(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// HandleDebugLogin creates a debug session with optional roles.
func (h *AuthHandler) HandleDebugLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Login string   `json:"login"`
		Roles []string `json:"roles"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.Login == "" {
		req.Login = "guest"
	}

	user := &User{
		ID:    0,
		Login: req.Login,
		Name:  req.Login,
		Roles: req.Roles,
	}

	sessionToken := generateToken(32)
	session := &Session{
		User:      user,
		Token:     sessionToken,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	h.mu.Lock()
	h.sessions[sessionToken] = session
	h.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

// UserFromRequest extracts the user from the request.
func (h *AuthHandler) UserFromRequest(r *http.Request) *User {
	token := extractToken(r)
	if token == "" {
		return nil
	}

	h.mu.RLock()
	session, ok := h.sessions[token]
	h.mu.RUnlock()

	if !ok || time.Now().After(session.ExpiresAt) {
		return nil
	}
	return session.User
}

// Middleware adds user to request context if authenticated.
func (h *AuthHandler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := h.UserFromRequest(r)
		if user != nil {
			ctx := context.WithValue(r.Context(), userContextKey, user)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

// RegisterRoutes registers authentication routes on the mux.
func (h *AuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /auth/status", h.HandleStatus)
	mux.HandleFunc("GET /auth/login", h.HandleLogin)
	mux.HandleFunc("GET /auth/callback", h.HandleCallback)
	mux.HandleFunc("POST /auth/logout", h.HandleLogout)
	mux.HandleFunc("GET /auth/me", h.HandleMe)
	mux.HandleFunc("POST /auth/debug/login", h.HandleDebugLogin)
}

// RequireAuth returns middleware that requires authentication.
// Browser requests get an HTML login page, API requests get JSON 401.
func (h *AuthHandler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := h.UserFromRequest(r)
		if user == nil {
			// Check if this is a browser request (accepts HTML)
			accept := r.Header.Get("Accept")
			if strings.Contains(accept, "text/html") {
				h.serveLoginPage(w, r)
				return
			}
			// Return 401 with JSON response for API requests
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]any{
				"error":          "unauthorized",
				"message":        "Authentication required",
				"login_url":      "/auth/login",
				"github_enabled": h.enabled,
			})
			return
		}
		// Add user to context and continue
		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// serveLoginPage serves an HTML page prompting the user to log in.
func (h *AuthHandler) serveLoginPage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)

	loginButton := ""
	if h.enabled {
		loginButton = `<a href="/auth/login" class="btn btn-primary">
			<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="currentColor">
				<path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
			</svg>
			Login with GitHub
		</a>`
	} else {
		loginButton = `<p class="note">GitHub OAuth is not configured. Please contact the administrator.</p>`
	}

	html := `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Login Required</title>
	<style>
		* { box-sizing: border-box; margin: 0; padding: 0; }
		body {
			font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
			background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
			min-height: 100vh;
			display: flex;
			align-items: center;
			justify-content: center;
			padding: 2rem;
		}
		.card {
			background: white;
			border-radius: 16px;
			padding: 3rem;
			max-width: 400px;
			width: 100%;
			text-align: center;
			box-shadow: 0 20px 60px rgba(0,0,0,0.3);
		}
		.icon {
			width: 80px;
			height: 80px;
			background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
			border-radius: 50%;
			display: flex;
			align-items: center;
			justify-content: center;
			margin: 0 auto 1.5rem;
		}
		.icon svg {
			width: 40px;
			height: 40px;
			fill: white;
		}
		h1 {
			color: #333;
			font-size: 1.5rem;
			margin-bottom: 0.5rem;
		}
		p {
			color: #666;
			margin-bottom: 2rem;
		}
		.btn {
			display: inline-flex;
			align-items: center;
			gap: 0.5rem;
			padding: 0.875rem 1.5rem;
			border-radius: 8px;
			text-decoration: none;
			font-weight: 600;
			font-size: 1rem;
			transition: transform 0.2s, box-shadow 0.2s;
		}
		.btn:hover {
			transform: translateY(-2px);
			box-shadow: 0 4px 12px rgba(102, 126, 234, 0.4);
		}
		.btn-primary {
			background: #24292e;
			color: white;
		}
		.note {
			color: #999;
			font-size: 0.9rem;
		}
		.back-link {
			margin-top: 1.5rem;
		}
		.back-link a {
			color: #667eea;
			text-decoration: none;
		}
		.back-link a:hover {
			text-decoration: underline;
		}
	</style>
</head>
<body>
	<div class="card">
		<div class="icon">
			<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
				<path d="M18 8h-1V6c0-2.76-2.24-5-5-5S7 3.24 7 6v2H6c-1.1 0-2 .9-2 2v10c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V10c0-1.1-.9-2-2-2zm-6 9c-1.1 0-2-.9-2-2s.9-2 2-2 2 .9 2 2-.9 2-2 2zm3.1-9H8.9V6c0-1.71 1.39-3.1 3.1-3.1 1.71 0 3.1 1.39 3.1 3.1v2z"/>
			</svg>
		</div>
		<h1>Authentication Required</h1>
		<p>You need to log in to access the dashboard.</p>
		` + loginButton + `
		<div class="back-link">
			<a href="/">← Back to Home</a>
		</div>
	</div>
</body>
</html>`

	w.Write([]byte(html))
}

// Context key for user
type contextKey string

const userContextKey contextKey = "user"

// Helper functions

func generateToken(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func extractToken(r *http.Request) string {
	// Check Authorization header
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	// Check cookie
	if cookie, err := r.Cookie("session"); err == nil {
		return cookie.Value
	}
	return ""
}

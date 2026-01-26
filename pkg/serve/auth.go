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

	// Redirect to frontend with token in URL params
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
	mux.HandleFunc("POST /api/debug/login", h.HandleDebugLogin)
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

package serve

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// POST /auth/debug/login mints a session with caller-chosen roles, so it exists only with DEV_MODE=true (audit M12).
func TestDebugLoginOnlyInDevMode(t *testing.T) {
	for _, tc := range []struct {
		dev  string
		want bool
	}{{"", false}, {"false", false}, {"1", false}, {"true", true}} {
		t.Setenv("DEV_MODE", tc.dev)
		mux := http.NewServeMux()
		NewAuthHandler("http://localhost").RegisterRoutes(mux)
		req := httptest.NewRequest(http.MethodPost, "/auth/debug/login", strings.NewReader(`{"login":"x","roles":["admin"]}`))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		registered := rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed
		if registered != tc.want {
			t.Errorf("DEV_MODE=%q: debug login registered=%v (status %d), want %v", tc.dev, registered, rec.Code, tc.want)
		}
		if !tc.want && strings.Contains(rec.Header().Get("Set-Cookie"), "session") {
			t.Errorf("DEV_MODE=%q: a session cookie was set", tc.dev)
		}
	}
}

package sim_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	_ "github.com/pflow-xyz/petri-pilot/generated"
	"github.com/pflow-xyz/petri-pilot/pkg/serve"
)

// TestCafeIsServable: the composed app registers with the server and its
// scenario endpoints answer through the real service handler.
//
// Until service.go was generated for bundles, the composition pipeline produced
// a coordinator, a flattened model and a full test suite — and no way to run any
// of it. Every single-net app registered itself with the server; a bundle
// registered nothing, so the one kind of app whose whole point is answering
// cross-entity questions was the one kind nobody could reach.
func TestCafeIsServable(t *testing.T) {
	factory, ok := serve.Get("cafe")
	if !ok {
		t.Fatalf("cafe is not registered; registered services: %v", serve.List())
	}
	svc, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	for _, tc := range []struct {
		method, path, body string
	}{
		{http.MethodGet, "/api/rates", ""},
		{http.MethodPost, "/api/scenario", `{"marking":{"staff/available":3},"hours":2}`},
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		svc.BuildHandler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s %s through the service handler: %d %s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}

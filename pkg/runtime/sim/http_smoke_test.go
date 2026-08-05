package sim_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pflow-xyz/go-pflow/eventsource"
	"github.com/pflow-xyz/petri-pilot/generated/beancounter"
)

// The bean counter is examples/coffeeshop's model generated with -simulation.
// These drive the generated handlers over real HTTP, which is the only way to
// know the wiring is right rather than merely that it compiles.

func newShop(t *testing.T) (http.Handler, string) {
	t.Helper()
	app := beancounter.NewApplication(eventsource.NewMemoryStore())
	h := beancounter.BuildRouter(app, nil)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/api/coffeeshop", nil))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		AggregateID string `json:"aggregate_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	return h, created.AggregateID
}

func get(t *testing.T, h http.Handler, path string) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec.Code, body
}

// TestRatesEndpointServesTheModel: the dashboard keeps its own rate table today;
// this is the endpoint that makes that unnecessary.
func TestRatesEndpoint(t *testing.T) {
	h, _ := newShop(t)
	code, body := get(t, h, "/api/rates")
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	rates, _ := body["rates"].(map[string]any)
	if rates["make_espresso"] != 20.0 {
		t.Errorf("make_espresso rate = %v, want 20 (as declared in the model)", rates["make_espresso"])
	}
}

// TestPredictReportsDivergence: this model's weight-20 arcs make the continuous
// solution run away. The endpoint must say so instead of serving the numbers —
// this is the case the old frontend's dead /predict call would have hit.
func TestPredictReportsDivergence(t *testing.T) {
	h, id := newShop(t)
	code, body := get(t, h, "/api/predict/"+id+"?hours=8")
	if code != http.StatusOK {
		t.Fatalf("status %d: %v", code, body)
	}
	if body["diverged"] != true {
		t.Errorf("expected diverged=true for a model with weight-20 arcs, got %v", body["diverged"])
	}
	if body["reason"] == nil {
		t.Error("divergence must carry a reason the caller can act on")
	}
}

// TestSimulateReturnsATrajectory: the discrete engine handles this model fine.
func TestSimulateReturnsATrajectory(t *testing.T) {
	h, id := newShop(t)
	code, body := get(t, h, "/api/simulate/"+id+"?hours=4&realizations=5")
	if code != http.StatusOK {
		t.Fatalf("status %d: %v", code, body)
	}
	if body["method"] != "ssa" {
		t.Errorf("method = %v, want ssa", body["method"])
	}
	series, _ := body["series"].([]any)
	if len(series) == 0 {
		t.Fatal("no series returned")
	}
}

// TestSimulationDoesNotMutate is the whole point: asking what happens next must
// leave the aggregate exactly as it was. The dashboard's approach — POSTing real
// transitions — cannot satisfy this, which is why it needs a reset button.
func TestSimulationDoesNotMutate(t *testing.T) {
	h, id := newShop(t)

	_, before := get(t, h, "/api/coffeeshop/"+id)
	for _, path := range []string{
		"/api/predict/" + id + "?hours=8",
		"/api/simulate/" + id + "?hours=8&realizations=5",
	} {
		if code, body := get(t, h, path); code != http.StatusOK {
			t.Fatalf("%s: %d %v", path, code, body)
		}
	}
	_, after := get(t, h, "/api/coffeeshop/"+id)

	b, _ := json.Marshal(before["places"])
	a, _ := json.Marshal(after["places"])
	if string(a) != string(b) {
		t.Errorf("simulation changed the aggregate\n before %s\n after  %s", b, a)
	}
	if before["version"] != after["version"] {
		t.Errorf("simulation appended events: version %v -> %v", before["version"], after["version"])
	}
}

func TestRejectsBadParameters(t *testing.T) {
	h, id := newShop(t)
	for _, q := range []string{"?hours=-1", "?samples=1", "?rate.no_such_transition=5"} {
		if code, _ := get(t, h, "/api/predict/"+id+q); code != http.StatusBadRequest {
			t.Errorf("%s should be rejected, got %d", q, code)
		}
	}
}

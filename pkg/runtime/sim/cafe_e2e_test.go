package sim_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pflow-xyz/go-pflow/eventsource"

	"github.com/pflow-xyz/petri-pilot/generated/cafe"
	"github.com/pflow-xyz/petri-pilot/pkg/runtime/sim"
)

// The composed café, end to end over HTTP.
//
// This lives here rather than in generated/cafe because that tree is
// regenerated wholesale: a hand-written test sitting inside it would be
// deleted by the next `rm -rf && codegen`. The claim under test is about the
// scenario endpoints this package provides, so this is where it belongs.

func testApp(t *testing.T) http.Handler {
	t.Helper()
	app, err := cafe.NewApp(eventsource.NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}
	return app.Handler()
}

func post(t *testing.T, h http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestScenarioAnswersTheStaffingQuestion is the end-to-end claim: an operator
// with no aggregate, no state and no setup can ask what a third barista buys
// them, over HTTP, and get an answer that respects every link in the bundle.
func TestScenarioAnswersTheStaffingQuestion(t *testing.T) {
	h := testApp(t)

	rec := post(t, h, "/api/scenario/compare", `{"scenarios":[
		{"name":"today","marking":{"staff/available":1},"hours":8,"realizations":10,"seed":11},
		{"name":"one more","marking":{"staff/available":2},"hours":8,"realizations":10,"seed":11},
		{"name":"two more","marking":{"staff/available":3},"hours":8,"realizations":10,"seed":11}
	]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}

	var cmp sim.Comparison
	if err := json.Unmarshal(rec.Body.Bytes(), &cmp); err != nil {
		t.Fatal(err)
	}
	if len(cmp.Scenarios) != 3 {
		t.Fatalf("got %d scenarios, want 3", len(cmp.Scenarios))
	}

	var served, walked []float64
	for _, s := range cmp.Scenarios {
		served = append(served, s.Result.Final["counter/orders_complete"])
		walked = append(walked, s.Result.Final["counter/walked_out"])
		t.Logf("%-9s served=%.0f walked out=%.0f utilization=%.2f",
			s.Name, s.Result.Final["counter/orders_complete"],
			s.Result.Final["counter/walked_out"], s.Result.Metrics.Utilization["staff"])
	}

	// More hands, more drinks, fewer people giving up. If the composition had
	// dropped the staff subnet — or the engine had ignored the pool — these
	// would all be the same number.
	for i := 1; i < len(served); i++ {
		if served[i] <= served[i-1] {
			t.Errorf("scenario %q served %.0f, no more than %q's %.0f",
				cmp.Scenarios[i].Name, served[i], cmp.Scenarios[i-1].Name, served[i-1])
		}
		if walked[i] >= walked[i-1] {
			t.Errorf("scenario %q lost %.0f customers, no fewer than %q's %.0f",
				cmp.Scenarios[i].Name, walked[i], cmp.Scenarios[i-1].Name, walked[i-1])
		}
	}
}

// TestScenarioRespectsTheStockLink: the counter cannot make what the pantry
// cannot pay for. This is the fusion doing real work — the counter's net has
// never heard of coffee beans.
func TestScenarioRespectsTheStockLink(t *testing.T) {
	h := testApp(t)

	rec := post(t, h, "/api/scenario",
		`{"marking":{"pantry/coffee_beans":0,"pantry/milk":0,"staff/available":5},
		  "rates":{"pantry/restock_coffee_beans":0,"pantry/restock_milk":0,"pantry/restock_cups":0},
		  "hours":4,"realizations":5,"seed":3}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var res sim.Result
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if got := res.Final["counter/orders_complete"]; got != 0 {
		t.Errorf("%v drinks were served with no beans and no milk", got)
	}
	if res.Final["counter/walked_out"] == 0 {
		t.Error("nobody left an empty shop that could not serve them")
	}
}

// TestScheduleOverHTTP: a morning rush is the one question a constant rate
// cannot express, so it has to survive the wire.
func TestScheduleOverHTTP(t *testing.T) {
	h := testApp(t)

	rec := post(t, h, "/api/scenario", `{
		"schedule":{"counter/order_latte":[{"until":1,"value":200},{"until":8,"value":1}]},
		"hours":8,"samples":80,"realizations":6,"seed":5}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var res sim.Result
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if n := len(res.Times); n == 0 || res.Times[n-1] < 7.5 {
		t.Fatalf("the run covers %v, not the full horizon", res.Times)
	}
	var peak float64
	for _, s := range res.Series {
		if s.Place != "counter/orders_pending" {
			continue
		}
		for _, v := range s.Values {
			if v > peak {
				peak = v
			}
		}
	}
	if peak < 5 {
		t.Errorf("peak queue was %.0f under a 200/h rush; the schedule did not reach the engine", peak)
	}
	t.Logf("peak queue under the rush: %.0f", peak)
}

// TestScenarioRejectsATypo: the failure this prevents is a scenario that runs,
// ignores the knob it did not recognise, and reports "no difference" to a
// question it never asked.
func TestScenarioRejectsATypo(t *testing.T) {
	h := testApp(t)
	rec := post(t, h, "/api/scenario", `{"marking":{"staff/baristas":3},"hours":1}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "staff/baristas") {
		t.Errorf("the error does not name the offending place: %s", rec.Body.String())
	}
}

// TestScenarioIsPureOverHTTP: the endpoint must not write. If it did, asking
// "what if" would leave a trace in the log forever — the exact defect the old
// dashboard had.
func TestScenarioIsPureOverHTTP(t *testing.T) {
	store := eventsource.NewMemoryStore()
	app, err := cafe.NewApp(store)
	if err != nil {
		t.Fatal(err)
	}
	h := app.Handler()

	rec := post(t, h, "/api/scenario", `{"marking":{"staff/available":4},"hours":4,"realizations":4,"seed":2}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}

	for _, entity := range []string{"counter", "pantry", "staff"} {
		events, err := store.Read(t.Context(), cafe.StreamID(entity, "demo"), 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 0 {
			t.Errorf("asking a hypothetical appended %d event(s) to %s", len(events), entity)
		}
	}
}

// TestRatesEndpointReportsTheModel: a client that renders controls should seed
// them from the net, not from a table of its own that will drift from it.
func TestRatesEndpointReportsTheModel(t *testing.T) {
	h := testApp(t)
	req := httptest.NewRequest(http.MethodGet, "/api/rates", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Rates   map[string]float64 `json:"rates"`
		Initial map[string]int     `json:"initial"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Rates["counter/order_latte"] != 15 {
		t.Errorf("order_latte rate = %v, want the model's 15", body.Rates["counter/order_latte"])
	}
	if body.Initial["staff/available"] != 2 {
		t.Errorf("staff/available = %v, want the model's 2", body.Initial["staff/available"])
	}
}

package eventgen

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/pflow-xyz/go-pflow/eventlog"
	"github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/go-pflow/mining"
	"github.com/pflow-xyz/go-pflow/petri"
)

// A three-stage service line with a resource pool: arrivals start cases,
// the pool is held across service, cases end in a sink.
func serviceLine() *metamodel.Model {
	kfalse := false
	return &metamodel.Model{
		Name: "service-line",
		Places: []metamodel.Place{
			{ID: "waiting", Initial: 0},
			{ID: "in_service", Initial: 0},
			{ID: "done", Initial: 0},
			{ID: "staff", Initial: 2},
		},
		Transitions: []metamodel.Transition{
			{ID: "arrive", Rate: 6},
			{ID: "start", Rate: 720},
			{ID: "finish", Rate: 4},
		},
		Arcs: []metamodel.Arc{
			{From: "arrive", To: "waiting"},
			{From: "waiting", To: "start", Kinetic: &kfalse},
			{From: "staff", To: "start"},
			{From: "start", To: "in_service"},
			{From: "in_service", To: "finish"},
			{From: "finish", To: "staff"},
			{From: "finish", To: "done"},
		},
	}
}

func TestPlayoutDeterministic(t *testing.T) {
	m := serviceLine()
	var a, b bytes.Buffer
	for _, buf := range []*bytes.Buffer{&a, &b} {
		log, err := Playout(m, Options{Cases: 50, Seed: 42})
		if err != nil {
			t.Fatal(err)
		}
		if err := WriteCSV(buf, log); err != nil {
			t.Fatal(err)
		}
	}
	if a.String() != b.String() {
		t.Fatal("same seed produced different bytes")
	}
	if a.Len() == 0 {
		t.Fatal("empty log")
	}
}

// Every case must be complete: arrive, start, finish, in order. FIFO
// attribution through a conserved staff pool is the thing being proven.
func TestPlayoutCaseIntegrity(t *testing.T) {
	m := serviceLine()
	log, err := Playout(m, Options{Cases: 200, Seed: 7})
	if err != nil {
		t.Fatal(err)
	}
	if got := log.NumCases(); got != 200 {
		t.Fatalf("expected 200 cases, got %d", got)
	}
	for id, trace := range log.Cases {
		if len(trace.Events) != 3 {
			t.Fatalf("case %s has %d events, want 3: %+v", id, len(trace.Events), trace.Events)
		}
		want := []string{"arrive", "start", "finish"}
		for i, ev := range trace.Events {
			if ev.Activity != want[i] {
				t.Fatalf("case %s event %d is %q, want %q", id, i, ev.Activity, want[i])
			}
			if i > 0 && ev.Timestamp.Before(trace.Events[i-1].Timestamp) {
				t.Fatalf("case %s timestamps out of order", id)
			}
		}
	}
}

// The scientific gate from the plan: mining the synthetic log approximately
// recovers the rates the model declares, and token replay of the log against
// the generating net is perfectly fitting. If either fails, the dataset is
// decoration, not data.
func TestRateRecoveryAndConformance(t *testing.T) {
	m := serviceLine()
	log, err := Playout(m, Options{Cases: 1000, Seed: 11})
	if err != nil {
		t.Fatal(err)
	}

	net := buildPetriNet(m)

	// go-pflow's timing semantics: an activity's "duration" is the dwell
	// until the NEXT event in the case, in seconds. So the service rate
	// (finish fires at 4/h) is recovered from the dwell after `start`, and
	// a trace's last activity never gets a sample at all. The gate converts
	// units and aims at the dwell that actually encodes the rate.
	learned := mining.LearnRatesFromLog(log, net)
	serviceRate := learned["start"] * 3600
	if serviceRate < 4*0.8 || serviceRate > 4*1.2 {
		t.Errorf("service rate recovered as %.2f/h from the post-start dwell, want 4±20%%", serviceRate)
	}
	stats := mining.ExtractTiming(log)
	meanInter := 0.0
	for _, v := range stats.InterArrivalTimes {
		meanInter += v
	}
	meanInter /= float64(len(stats.InterArrivalTimes))
	arrivalRate := 3600 / meanInter
	if arrivalRate < 6*0.8 || arrivalRate > 6*1.2 {
		t.Errorf("arrival rate recovered as %.2f/h from inter-arrivals, want 6±20%%", arrivalRate)
	}

	// Fitness cannot reach 1.0 for a net with resource pools — the formula
	// counts final-state tokens (done, returned staff) as "remaining". The
	// replayability claim is zero MISSING tokens: every trace's every event
	// was fireable when replayed.
	conf := mining.CheckConformance(log, net)
	if conf.MissingTokens != 0 {
		t.Errorf("%d missing tokens; the generating net cannot replay its own log", conf.MissingTokens)
	}
}

// The vet clinic — read arcs, inhibitors, non-kinetic pickups, weight-2
// holds — must play out and drain: every started case ends in a sink.
func TestPlayoutVetClinic(t *testing.T) {
	data, err := os.ReadFile("../../../services/vet-clinic.json")
	if err != nil {
		t.Fatal(err)
	}
	var m metamodel.Model
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	log, err := Playout(&m, Options{Cases: 100, Seed: 3})
	if err != nil {
		t.Fatal(err)
	}
	if got := log.NumCases(); got != 100 {
		t.Fatalf("expected 100 cases, got %d", got)
	}
	terminal := map[string]bool{"process_checkout": true, "abandon_exam": true,
		"abandon_tech": true, "abandon_diag": true, "abandon_surgery": true,
		"abandon_dental": true, "divert_emergency": true, "finish_checkout": true}
	ended := 0
	for _, trace := range log.Cases {
		last := trace.Events[len(trace.Events)-1]
		if terminal[last.Activity] {
			ended++
		}
	}
	if ended < 95 {
		t.Errorf("only %d/100 cases reached a terminal activity", ended)
	}
}

func TestWritersRoundTripThroughGoPflowParsers(t *testing.T) {
	m := serviceLine()
	log, err := Playout(m, Options{Cases: 20, Seed: 5})
	if err != nil {
		t.Fatal(err)
	}
	var csvBuf bytes.Buffer
	if err := WriteCSV(&csvBuf, log); err != nil {
		t.Fatal(err)
	}
	parsed, err := eventlog.ParseCSVReader(&csvBuf, eventlog.DefaultCSVConfig())
	if err != nil {
		t.Fatalf("go-pflow's own parser rejected our CSV: %v", err)
	}
	if parsed.NumCases() != 20 {
		t.Errorf("CSV round trip lost cases: %d", parsed.NumCases())
	}

	var jsonlBuf bytes.Buffer
	if err := WriteJSONL(&jsonlBuf, log); err != nil {
		t.Fatal(err)
	}
	parsed2, err := eventlog.ParseJSONLReader(&jsonlBuf, eventlog.DefaultJSONLConfig())
	if err != nil {
		t.Fatalf("go-pflow's own parser rejected our JSONL: %v", err)
	}
	if parsed2.NumCases() != 20 {
		t.Errorf("JSONL round trip lost cases: %d", parsed2.NumCases())
	}
}

// buildPetriNet converts for the mining API (consuming arcs only, mirroring
// pkg/mcp's buildOdeNet).
func buildPetriNet(m *metamodel.Model) *petri.PetriNet {
	b := petri.Build()
	for _, p := range m.Places {
		b = b.Place(p.ID, float64(p.Initial))
	}
	for _, t := range m.Transitions {
		b = b.Transition(t.ID)
	}
	for i := range m.Arcs {
		a := &m.Arcs[i]
		if a.IsRead() || a.IsInhibitor() {
			continue
		}
		w := a.Weight
		if w == 0 {
			w = 1
		}
		b = b.Arc(a.From, a.To, float64(w))
	}
	return b.Done()
}

// A closed-loop net (no source transition) structurally cannot produce a
// case, and a silent empty log is indistinguishable from success — it must
// refuse and say why. Predator-prey is the standing example.
func TestPlayoutRefusesClosedLoopModel(t *testing.T) {
	m := &metamodel.Model{
		Name: "predator-prey-shaped",
		Places: []metamodel.Place{
			{ID: "rabbits", Initial: 100},
			{ID: "foxes", Initial: 10},
		},
		Transitions: []metamodel.Transition{
			{ID: "breed", Rate: 1},
			{ID: "hunt", Rate: 0.01},
		},
		Arcs: []metamodel.Arc{
			{From: "rabbits", To: "breed"},
			{From: "breed", To: "rabbits", Weight: 2},
			{From: "rabbits", To: "hunt"},
			{From: "foxes", To: "hunt"},
			{From: "hunt", To: "foxes", Weight: 2},
		},
	}
	_, err := Playout(m, Options{Cases: 5, Seed: 1})
	if err == nil {
		t.Fatal("closed-loop model produced a log instead of refusing")
	}
	if !strings.Contains(err.Error(), "no source transition") {
		t.Fatalf("refusal does not name the cause: %v", err)
	}
}

// Sources that exist but never fire (rate 0 via the solver map, the
// emergency_arrives pattern) also mean zero cases — same rule, second cause.
func TestPlayoutRefusesDormantSources(t *testing.T) {
	m := serviceLine()
	m.Simulation = &metamodel.Simulation{Solver: &metamodel.SolverConfig{
		Rates: map[string]float64{"arrive": 0},
	}}
	_, err := Playout(m, Options{Cases: 5, Seed: 1})
	if err == nil {
		t.Fatal("dormant-source model produced a log instead of refusing")
	}
	if !strings.Contains(err.Error(), "never fired") {
		t.Fatalf("refusal does not name the cause: %v", err)
	}
}

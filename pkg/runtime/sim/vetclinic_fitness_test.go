package sim

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

// The vet-clinic fitness gates run the shipped model with nothing overridden,
// mirroring cafe_fitness_test.go: size the model so the intended constraint
// binds, then assert it. Every expectation is qualitative (a direction, a knee,
// an exact zero) rather than a magic number, so a rate tweak moves what the
// gates expect instead of breaking them.

func vetClinicModel(t *testing.T) *metamodel.Model {
	t.Helper()
	data, err := os.ReadFile("../../../services/vet-clinic.json")
	if err != nil {
		t.Fatalf("reading vet-clinic model: %v", err)
	}
	var m metamodel.Model
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parsing vet-clinic model: %v", err)
	}
	return &m
}

func vetRun(t *testing.T, m *metamodel.Model, s Scenario) *Result {
	t.Helper()
	if s.Horizon == 0 {
		s.Horizon = 8
	}
	if s.Realizations == 0 {
		s.Realizations = 30
	}
	if s.Seed == 0 {
		s.Seed = 42
	}
	res, err := Run(m, s)
	if err != nil {
		t.Fatalf("scenario %q: %v", s.Name, err)
	}
	return res
}

// GATE 1: the quiet clinic diverts nobody. emergency_arrives is pinned to zero
// through simulation.solver.rates — a transition-level 0 would read as unset
// and default to 1/h, which is exactly the silent leak this gate exists to
// catch.
func TestVetClinicNoEmergenciesMeansNoDiversions(t *testing.T) {
	m := vetClinicModel(t)
	if r := Rates(m)["emergency_arrives"]; r != 0 {
		t.Fatalf("emergency_arrives rate = %v, want 0 (solver-map zero lost?)", r)
	}
	res := vetRun(t, m, Scenario{Name: "baseline"})
	if res.Final["diverted"] != 0 {
		t.Errorf("baseline day diverted %.2f emergencies that never arrived", res.Final["diverted"])
	}
	if res.Final["wait_emergency"] != 0 || res.Final["in_emergency"] != 0 {
		t.Errorf("emergency pipeline occupied with no arrivals: wait=%.2f in=%.2f",
			res.Final["wait_emergency"], res.Final["in_emergency"])
	}
}

// GATE 2: patients are conserved. Everyone the source produced is either out
// one of the three doors or still somewhere inside at close; a mismatch means
// an arc is minting or destroying patients.
func TestVetClinicPatientsConserved(t *testing.T) {
	m := vetClinicModel(t)
	res := vetRun(t, m, Scenario{Name: "conservation"})
	arrived := res.Metrics.Throughput["patient_arrives"] + res.Metrics.Throughput["emergency_arrives"]
	out := res.Final["discharged"] + res.Final["walked_out"] + res.Final["diverted"]
	inFlight := 0.0
	for _, p := range []string{
		"arrival", "wait_exam", "wait_tech", "wait_surgery", "wait_dental", "wait_diag",
		"wait_emergency", "wait_checkout", "in_wellness", "in_sick", "in_vaccine",
		"in_nail_trim", "in_weight", "in_suture", "in_spay", "in_neuter", "in_dental",
		"in_xray", "in_emergency", "in_recovery", "checking_out",
	} {
		inFlight += res.Final[p]
	}
	// wait_lab/in_lab hold samples, not patients: the patient rides to
	// checkout only after the lab result, so the sample pipeline is counted.
	inFlight += res.Final["wait_lab"] + res.Final["in_lab"]
	if diff := arrived - out - inFlight; diff > 0.5 || diff < -0.5 {
		t.Errorf("patients not conserved: arrived %.1f, accounted %.1f (diff %.2f)",
			arrived, out+inFlight, diff)
	}
}

// GATE 3: staffing has a knee. Cutting to one DVM must hurt, and the fourth
// DVM must buy far less than the second did — otherwise the model is answering
// "how many vets" with a straight line, which no queue does.
func TestVetClinicStaffingKnee(t *testing.T) {
	m := vetClinicModel(t)
	walked := map[int]float64{}
	for _, dvm := range []int{1, 2, 4} {
		res := vetRun(t, m, Scenario{Name: "staffing", Marking: map[string]int{"dvm_avail": dvm}})
		walked[dvm] = res.Final["walked_out"]
	}
	t.Logf("walkouts by DVM count: 1=%.1f 2=%.1f 4=%.1f", walked[1], walked[2], walked[4])
	gain12 := walked[1] - walked[2]
	gain24 := walked[2] - walked[4]
	if gain12 <= 0 {
		t.Errorf("second DVM did not reduce walkouts (1 DVM: %.1f, 2 DVM: %.1f)", walked[1], walked[2])
	}
	if gain24 > gain12/2 {
		t.Errorf("no knee: doubling 2->4 DVMs bought %.1f, second DVM bought %.1f", gain24, gain12)
	}
}

// GATE 4: an emergency wave disrupts the routine day, and staffing buys some
// of it back. Same seed throughout — without it, this gate would be measuring
// dice, not disruptions.
func TestVetClinicEmergencyDisruption(t *testing.T) {
	m := vetClinicModel(t)
	wave := map[string][]Segment{
		"emergency_arrives": {{Until: 2, Value: 0}, {Until: 4, Value: 2}, {Until: 8, Value: 0}},
	}
	baseline := vetRun(t, m, Scenario{Name: "baseline"})
	crisis := vetRun(t, m, Scenario{Name: "crisis", Marking: map[string]int{"wait_emergency": 3}, Schedule: wave})
	staffed := vetRun(t, m, Scenario{Name: "crisis+staff", Marking: map[string]int{"wait_emergency": 3, "dvm_avail": 3, "rvt_avail": 4}, Schedule: wave})

	t.Logf("walked: base=%.1f crisis=%.1f staffed=%.1f | diverted: crisis=%.1f staffed=%.1f",
		baseline.Final["walked_out"], crisis.Final["walked_out"], staffed.Final["walked_out"],
		crisis.Final["diverted"], staffed.Final["diverted"])

	if crisis.Final["walked_out"] <= baseline.Final["walked_out"] {
		t.Errorf("emergency wave did not raise routine walkouts (base %.1f, crisis %.1f)",
			baseline.Final["walked_out"], crisis.Final["walked_out"])
	}
	if crisis.Final["diverted"] == 0 {
		t.Error("an emergency wave against 2 DVMs diverted nobody; divert path dead?")
	}
	if staffed.Final["walked_out"] >= crisis.Final["walked_out"] {
		t.Errorf("extra staff bought nothing in the crisis (crisis %.1f, staffed %.1f)",
			crisis.Final["walked_out"], staffed.Final["walked_out"])
	}
}

// GATE 5: closing the surgery gate stops procedures without touching the rest
// of the day. surgery_day is a read arc precisely so a scenario can flip it;
// this gate fails if it ever silently becomes a consumed token again.
func TestVetClinicSurgeryGate(t *testing.T) {
	m := vetClinicModel(t)
	open := vetRun(t, m, Scenario{Name: "surgery-day"})
	closed := vetRun(t, m, Scenario{Name: "no-surgery", Marking: map[string]int{"surgery_day": 0}})

	surgOpen := open.Metrics.Throughput["finish_spay"] + open.Metrics.Throughput["finish_neuter"] + open.Metrics.Throughput["finish_dental"]
	surgClosed := closed.Metrics.Throughput["finish_spay"] + closed.Metrics.Throughput["finish_neuter"] + closed.Metrics.Throughput["finish_dental"]
	if surgOpen == 0 {
		t.Error("no procedures completed on a surgery day")
	}
	if surgClosed != 0 {
		t.Errorf("surgery_day=0 still completed %.1f procedures", surgClosed)
	}
	if closed.Metrics.Throughput["finish_wellness"] == 0 {
		t.Error("closing the surgery gate stopped wellness exams too")
	}
}

// GATE 6: what the run waits on is staff, not rooms. The clinic is sized so
// people bind before space — four exam rooms against two DVMs. If a room pool
// outranks every staff pool in Contended, the sizing has drifted.
func TestVetClinicStaffLimitedNotRoomLimited(t *testing.T) {
	m := vetClinicModel(t)
	res := vetRun(t, m, Scenario{Name: "bottleneck", Marking: map[string]int{"wait_emergency": 2}})
	staffRank, roomRank := -1, -1
	for i, c := range res.Contended {
		if c.Kind != SupplyConserved {
			continue
		}
		switch c.Place {
		case "dvm_avail", "rvt_avail", "receptionist_avail":
			if staffRank == -1 {
				staffRank = i
			}
		case "exam_room_free", "surgery_free", "dental_free", "radiology_free", "treatment_free", "recovery_free":
			if roomRank == -1 {
				roomRank = i
			}
		}
	}
	if staffRank == -1 {
		t.Fatal("no staff pool appears in Contended at all")
	}
	if roomRank != -1 && roomRank < staffRank {
		t.Errorf("a room pool (rank %d) outranks every staff pool (rank %d): clinic is space-limited, not staff-limited",
			roomRank, staffRank)
	}
}

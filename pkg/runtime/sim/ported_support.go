package sim

import (
	"fmt"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

// Support types the structural readings ported from sim.pflow.xyz share with
// its diagnose and classify machinery, which is engine-bound and stays there.
// Kept verbatim so a future port of Diagnose/Classes drops in without a
// rename.

type Class struct {
	ID      string   `json:"id"`
	Members []string `json:"members"`
	// Kind is a term from the classification vocabulary, not a loose adjective:
	// FungibleSet, Role, AssertedClass or IdenticalDynamics. It doubles as the
	// JSON-LD @type of the class, so a new kind extends a vocabulary instead of
	// a set of magic strings.
	Kind string `json:"kind"`
	// Fungible stays for callers written against the first shape of this.
	Fungible bool `json:"fungible"`
	// Method names how the class was found, and Evidence how the claim was
	// settled. A candidate that failed its experiment says so rather than
	// disappearing: "these look alike but are not interchangeable" is a
	// finding about the model, not a dead end.
	Method string `json:"method"`
	// Evidence is the human sentence; EstablishedBy is the same thing as data.
	// A class routinely carries more than one: colour refinement proves the net
	// cannot distinguish its members, and a separate experiment decides whether
	// they are interchangeable — different claims with different failure modes,
	// and the reason this is a list rather than a field.
	Evidence      string     `json:"evidence,omitempty"`
	EstablishedBy []Evidence `json:"establishedBy,omitempty"`
	Verified      bool       `json:"verified"`
	// Asserted marks a class the modeller declared rather than one discovery
	// found. It is still re-checked and still reports what it costs: a claim
	// that contradicts a measurement is an assumption a consumer was handed,
	// not a fact about the net, and hiding that would make the two
	// indistinguishable.
	Asserted bool `json:"asserted,omitempty"`
}

type DiagnoseOptions struct {
	Hours        float64
	Realizations int
	Seed         int64
	// Probes are the multiples of the baseline pool level to measure influence
	// at. Two points is the minimum that can show a constraint moving. The
	// top multiple doubles as the relief step for greedy relief (relieveKnob)
	// — a pool's marking and a source/patience knob's rate both move by this
	// same multiple when they are fixed as the operating point for the next
	// round, so measurement and relief agree on what "one point further out"
	// means instead of the two needing a second, independent constant.
	Probes []float64
}

func (o DiagnoseOptions) withDefaults() DiagnoseOptions {
	if o.Hours == 0 {
		o.Hours = 8
	}
	if o.Realizations == 0 {
		o.Realizations = 24
	}
	if o.Seed == 0 {
		o.Seed = 7
	}
	if len(o.Probes) == 0 {
		// Baseline, and again with the top knob relieved — enough to catch a
		// constraint that only appears once the first one is fixed.
		o.Probes = []float64{1, 2}
	}
	return o
}

func tokenPlaces(m *metamodel.Model) ([]string, map[string]int, error) {
	var places []string
	index := map[string]int{}
	for i := range m.Places {
		p := &m.Places[i]
		if !p.IsToken() {
			continue // data places hold values, not counts; they have no trajectory
		}
		index[p.ID] = len(places)
		places = append(places, p.ID)
	}
	if len(places) == 0 {
		return nil, nil, fmt.Errorf("model %q has no token places to simulate", m.Name)
	}
	return places, index, nil
}

// Classification vocabulary terms. These are the JSON-LD @type of a class and
// the value of Kind, so consumers match on a term rather than on prose.
const (
	// KindFungible: the net cannot distinguish the members, and an experiment
	// found them interchangeable.
	KindFungible = "FungibleSet"
	// KindRole: the same position in the net, different levels or rates.
	KindRole = "Role"
	// KindAsserted: declared by the modeller, re-checked here.
	KindAsserted = "AssertedClass"
	// KindIdenticalDynamics: members whose trajectories coincide.
	KindIdenticalDynamics = "IdenticalDynamics"
	// KindObservableLumped: members whose OWN trajectories may genuinely
	// differ, but whose SUM is exactly reconstructible for one declared
	// observable (clue.go's ConstrainedLumping) — weaker, and so more
	// permissive, than KindIdenticalDynamics: it proves nothing about any
	// member individually, only about their combined contribution to the
	// stated question.
	KindObservableLumped = "ObservableEquivalentSum"
)

func nameParts(id string) []string {
	var parts []string
	cur := ""
	prevDigit := false
	for _, r := range id {
		isAlnum := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		isDigit := r >= '0' && r <= '9'
		if !isAlnum {
			if cur != "" {
				parts = append(parts, cur)
				cur = ""
			}
			prevDigit = false
			continue
		}
		if cur != "" && isDigit != prevDigit {
			parts = append(parts, cur)
			cur = ""
		}
		cur += string(r)
		prevDigit = isDigit
	}
	if cur != "" {
		parts = append(parts, cur)
	}
	return parts
}

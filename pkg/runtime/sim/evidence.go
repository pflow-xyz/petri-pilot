package sim

import "fmt"

// Evidence is how a claim was established, as data rather than as a sentence.
//
// The roadmap's organising idea is that structure, measurement and assumption
// must never be reported in the same voice. Until this type existed they were:
// a class carried a free-text `method` and an English `evidence` line like
// "trajectories agree to within 0.42, inside the 0.50 this run's own spread
// carries", and no consumer could tell a proof from a sample without parsing
// prose. For a phase where an asserted class is meant to carry a proof
// obligation that is discharged or admitted, a proof and a measurement having
// the same shape is fatal.
//
// The three kinds are deliberately not a scale from good to bad. A measurement
// at a large sample can be far more useful than a structural fact about a
// property nobody cares about. They differ in what would overturn them: a
// structural claim falls only if the net changes, a measurement falls if you
// sample differently, an assumption falls when someone disagrees.
const (
	// StructuralProof is a fact about the net, holding for every run.
	StructuralProof = "StructuralProof"
	// Measurement is a fact about runs at a stated sample size and seed.
	Measurement = "Measurement"
	// Assumption is a claim the engine was handed rather than derived.
	Assumption = "Assumption"
)

// Evidence carries one justification. Fields are per-kind and omitempty, so a
// document shows only what applies; the shared Summary is the human sentence
// that used to be the whole of it.
type Evidence struct {
	Type string `json:"@type"`

	// StructuralProof. DoesNotEstablish is not decoration: colour refinement
	// proves that a net does not distinguish two places and specifically does
	// NOT prove they are interchangeable, because 1-WL over-approximates
	// isomorphism. A structural claim that omits its own limit is the way this
	// system would start lying.
	Algorithm        string `json:"algorithm,omitempty"`
	Establishes      string `json:"establishes,omitempty"`
	DoesNotEstablish string `json:"doesNotEstablish,omitempty"`

	// Measurement. Observed and Tolerance are in the units of Statistic, and
	// WithinTolerance is the verdict rather than something a reader recomputes
	// from a rounded pair.
	Statistic       string   `json:"statistic,omitempty"`
	Seed            int64    `json:"seed,omitempty"`
	Realizations    int      `json:"realizations,omitempty"`
	Horizon         float64  `json:"horizon,omitempty"`
	Observed        *float64 `json:"observed,omitempty"`
	Tolerance       *float64 `json:"tolerance,omitempty"`
	WithinTolerance *bool    `json:"withinTolerance,omitempty"`

	// Assumption.
	StatedBy string `json:"statedBy,omitempty"`
	Reason   string `json:"reason,omitempty"`

	Summary string `json:"summary,omitempty"`
}

func structural(algorithm, establishes, doesNot string) Evidence {
	return Evidence{
		Type: StructuralProof, Algorithm: algorithm,
		Establishes: establishes, DoesNotEstablish: doesNot,
		Summary: establishes,
	}
}

func measured(statistic string, opts DiagnoseOptions, observed, tolerance float64, summary string) Evidence {
	within := observed <= tolerance && observed >= -tolerance
	return Evidence{
		Type: Measurement, Statistic: statistic,
		Seed: opts.Seed, Realizations: opts.Realizations, Horizon: opts.Hours,
		Observed: &observed, Tolerance: &tolerance, WithinTolerance: &within,
		Summary: summary,
	}
}

// unmeasurable records that a run could not decide the question, which is a
// different report from deciding it negatively. Conflating the two is how a
// harness ends up saying "not equivalent" where the honest answer is "this
// sample cannot tell".
func unmeasurable(statistic string, opts DiagnoseOptions, why string) Evidence {
	return Evidence{
		Type: Measurement, Statistic: statistic,
		Seed: opts.Seed, Realizations: opts.Realizations, Horizon: opts.Hours,
		Summary: why,
	}
}

func assumed(statedBy, reason string) Evidence {
	e := Evidence{Type: Assumption, StatedBy: statedBy, Reason: reason}
	e.Summary = fmt.Sprintf("declared by %s", statedBy)
	if reason != "" {
		e.Summary += ": " + reason
	}
	return e
}

// Summarise joins the human-readable halves, so the single-sentence field the
// console and the CLI already render keeps working.
func Summarise(ev []Evidence) string {
	out := ""
	for _, e := range ev {
		if e.Summary == "" {
			continue
		}
		if out != "" {
			out += "; "
		}
		out += e.Summary
	}
	return out
}

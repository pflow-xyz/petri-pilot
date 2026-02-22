package codegen

// Template names used as keys in the generator.
const (
	TemplateTopology       = "topology.go"
	TemplateCircuit        = "circuit.go"
	TemplateWitness        = "witness.go"
	TemplateState          = "state.go"
	TemplateScoringCircuit = "scoring_circuit.go"
	TemplateScoringWitness = "scoring_witness.go"
)

// topologyTmpl generates the Petri net topology: place/transition constants,
// stoichiometry matrix, rate constants, transition inputs, and initial marking.
var topologyTmpl = `package {{.PackageName}}

import "math/big"

import zkode "github.com/pflow-xyz/petri-pilot/zk-ode"

const NumPlaces = {{.NumPlaces}}
const NumTransitions = {{.NumTransitions}}
const MaxInputsPerTransition = {{.MaxInputsPerTransition}}

// Place indices.
const (
{{- range .Places}}
	{{.ConstName}} = {{.Index}}
{{- end}}
)

// PlaceNames maps indices to human-readable names.
var PlaceNames = [NumPlaces]string{
{{- range .Places}}
	"{{.ID}}",
{{- end}}
}

// Transition indices.
const (
{{- range .Transitions}}
	{{.ConstName}} = {{.Index}}
{{- end}}
)

// TransitionNames maps transition indices to human-readable names.
var TransitionNames = [NumTransitions]string{
{{- range .Transitions}}
	"{{.ID}}",
{{- end}}
}

// Stoichiometry is the net-change matrix S[place][transition].
var Stoichiometry = [NumPlaces][NumTransitions]int{
{{- range $p := .Stoichiometry}}
	{ {{- range $i, $v := $p}}{{if $i}}, {{end}}{{$v}}{{end -}} },
{{- end}}
}

// TransitionInputs lists the input place indices for each transition.
var TransitionInputs [NumTransitions][]int

// NumInputs is the number of input places per transition.
var NumInputs [NumTransitions]int

// RateConstants holds the rate constant k[t] for each transition as fixed-point.
var RateConstants [NumTransitions]*big.Int

func init() {
	initRateConstants()
	initTransitionInputs()
}

func initRateConstants() {
{{- range .Transitions}}
	RateConstants[{{.ConstName}}] = zkode.FixFromFloat({{printf "%g" .Rate}})
{{- end}}
}

func initTransitionInputs() {
{{- range .Transitions}}
	TransitionInputs[{{.ConstName}}] = []int{ {{- range $i, $inp := .Inputs}}{{if $i}}, {{end}}{{$inp}}{{end -}} }
	NumInputs[{{.ConstName}}] = {{.NumInputs}}
{{- end}}
}

// DefaultInitialMarking returns the initial marking as fixed-point field elements.
func DefaultInitialMarking() [NumPlaces]*big.Int {
	var m [NumPlaces]*big.Int
{{- range .Places}}
	m[{{.ConstName}}] = zkode.FixFromFloat({{printf "%g" .Initial}})
{{- end}}
	return m
}
`

// circuitTmpl generates the gnark ODE circuit with Tsit5 integration.
var circuitTmpl = `package {{.PackageName}}

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash/mimc"
	zkode "github.com/pflow-xyz/petri-pilot/zk-ode"
)

// ODECircuit proves that one fixed-step Tsit5 ODE integration was computed
// correctly over the Petri net with mass-action kinetics.
type ODECircuit struct {
	// Public inputs
	PreStateRoot  frontend.Variable ` + "`" + `gnark:",public"` + "`" + `
	PostStateRoot frontend.Variable ` + "`" + `gnark:",public"` + "`" + `
	StepSize      frontend.Variable ` + "`" + `gnark:",public"` + "`" + `

	// Private witness
	PreMarking  [NumPlaces]frontend.Variable
	PostMarking [NumPlaces]frontend.Variable
}

func (c *ODECircuit) Define(api frontend.API) error {
	// 1. Verify pre-state root matches private marking
	preH, _ := mimc.NewMiMC(api)
	for _, v := range c.PreMarking {
		preH.Write(v)
	}
	api.AssertIsEqual(preH.Sum(), c.PreStateRoot)

	// 2. Compute 7 Tsit5 stages
	var k [7][NumPlaces]frontend.Variable

	for stage := 0; stage < 7; stage++ {
		var yStage [NumPlaces]frontend.Variable
		for p := 0; p < NumPlaces; p++ {
			yStage[p] = c.PreMarking[p]
		}

		for j := 0; j < len(zkode.Tsit5A[stage]); j++ {
			hA := zkode.FixMul(api, c.StepSize, zkode.Tsit5A[stage][j])
			for p := 0; p < NumPlaces; p++ {
				contrib := zkode.FixMul(api, hA, k[j][p])
				yStage[p] = api.Add(yStage[p], contrib)
			}
		}

		// Evaluate mass-action rates at stage state
		var rates [NumTransitions]frontend.Variable
		for t := 0; t < NumTransitions; t++ {
			rates[t] = computeRate(api, yStage[:], t)
		}

		// Compute derivatives: k[stage][p] = sum(S[p][t] * rate[t])
		for p := 0; p < NumPlaces; p++ {
			k[stage][p] = frontend.Variable(0)
			for t := 0; t < NumTransitions; t++ {
				s := Stoichiometry[p][t]
				if s == 0 {
					continue
				}
				switch {
				case s == 1:
					k[stage][p] = api.Add(k[stage][p], rates[t])
				case s == -1:
					k[stage][p] = api.Sub(k[stage][p], rates[t])
				case s > 1:
					k[stage][p] = api.Add(k[stage][p], api.Mul(rates[t], s))
				case s < -1:
					k[stage][p] = api.Sub(k[stage][p], api.Mul(rates[t], -s))
				}
			}
		}
	}

	// 3. Compute expected post state
	var postExpected [NumPlaces]frontend.Variable
	for p := 0; p < NumPlaces; p++ {
		postExpected[p] = c.PreMarking[p]
	}
	for j := 0; j < 7; j++ {
		if zkode.Tsit5B[j].Sign() == 0 {
			continue
		}
		hB := zkode.FixMul(api, c.StepSize, zkode.Tsit5B[j])
		for p := 0; p < NumPlaces; p++ {
			contrib := zkode.FixMul(api, hB, k[j][p])
			postExpected[p] = api.Add(postExpected[p], contrib)
		}
	}

	// 4. Assert post marking matches expected
	for p := 0; p < NumPlaces; p++ {
		api.AssertIsEqual(c.PostMarking[p], postExpected[p])
	}

	// 5. Verify post-state root
	postH, _ := mimc.NewMiMC(api)
	for _, v := range c.PostMarking {
		postH.Write(v)
	}
	api.AssertIsEqual(postH.Sum(), c.PostStateRoot)

	return nil
}

// computeRate computes the mass-action rate for transition t:
// rate = k[t] * product(marking[input]) for all input places.
func computeRate(api frontend.API, marking []frontend.Variable, t int) frontend.Variable {
	inputs := TransitionInputs[t]
	if len(inputs) == 0 {
		return RateConstants[t]
	}
	rate := marking[inputs[0]]
	for i := 1; i < len(inputs); i++ {
		rate = zkode.FixMul(api, rate, marking[inputs[i]])
	}
	rate = zkode.FixMul(api, rate, RateConstants[t])
	return rate
}
`

// witnessTmpl generates the native big.Int Tsit5 step matching the circuit.
var witnessTmpl = `package {{.PackageName}}

import "math/big"

import zkode "github.com/pflow-xyz/petri-pilot/zk-ode"

// NativeStep performs one Tsit5 ODE integration step using native big.Int arithmetic.
func NativeStep(marking [NumPlaces]*big.Int, h *big.Int) [NumPlaces]*big.Int {
	var k [7][NumPlaces]*big.Int

	zero := big.NewInt(0)
	for s := 0; s < 7; s++ {
		for p := 0; p < NumPlaces; p++ {
			k[s][p] = new(big.Int).Set(zero)
		}
	}

	for stage := 0; stage < 7; stage++ {
		var yStage [NumPlaces]*big.Int
		for p := 0; p < NumPlaces; p++ {
			yStage[p] = new(big.Int).Set(marking[p])
		}

		for j := 0; j < len(zkode.Tsit5A[stage]); j++ {
			hA := zkode.NativeFixMul(h, zkode.Tsit5A[stage][j])
			for p := 0; p < NumPlaces; p++ {
				contrib := zkode.NativeFixMul(hA, k[j][p])
				yStage[p] = zkode.NativeFixAdd(yStage[p], contrib)
			}
		}

		// Mass-action rates
		var rates [NumTransitions]*big.Int
		for t := 0; t < NumTransitions; t++ {
			rates[t] = nativeRate(yStage, t)
		}

		// Derivatives
		for p := 0; p < NumPlaces; p++ {
			k[stage][p] = new(big.Int).Set(zero)
			for t := 0; t < NumTransitions; t++ {
				s := Stoichiometry[p][t]
				if s == 0 {
					continue
				}
				switch {
				case s == 1:
					k[stage][p] = zkode.NativeFixAdd(k[stage][p], rates[t])
				case s == -1:
					k[stage][p] = zkode.NativeFixSub(k[stage][p], rates[t])
				case s > 1:
					for i := 0; i < s; i++ {
						k[stage][p] = zkode.NativeFixAdd(k[stage][p], rates[t])
					}
				case s < -1:
					for i := 0; i < -s; i++ {
						k[stage][p] = zkode.NativeFixSub(k[stage][p], rates[t])
					}
				}
			}
		}
	}

	// Final weighted sum
	var post [NumPlaces]*big.Int
	for p := 0; p < NumPlaces; p++ {
		post[p] = new(big.Int).Set(marking[p])
	}
	for j := 0; j < 7; j++ {
		if zkode.Tsit5B[j].Sign() == 0 {
			continue
		}
		hB := zkode.NativeFixMul(h, zkode.Tsit5B[j])
		for p := 0; p < NumPlaces; p++ {
			contrib := zkode.NativeFixMul(hB, k[j][p])
			post[p] = zkode.NativeFixAdd(post[p], contrib)
		}
	}

	return post
}

// nativeRate computes rate = k[t] * product(marking[inputs[t]]) in native big.Int.
func nativeRate(marking [NumPlaces]*big.Int, t int) *big.Int {
	inputs := TransitionInputs[t]
	if len(inputs) == 0 {
		return new(big.Int).Set(RateConstants[t])
	}
	rate := new(big.Int).Set(marking[inputs[0]])
	for i := 1; i < len(inputs); i++ {
		rate = zkode.NativeFixMul(rate, marking[inputs[i]])
	}
	rate = zkode.NativeFixMul(rate, RateConstants[t])
	return rate
}

// StepWitness contains all data needed for one circuit assignment.
type StepWitness struct {
	PreState  *ODEState
	PostState *ODEState
	StepSize  *big.Int
}

// ComputeStep runs one Tsit5 step and generates a full witness.
func ComputeStep(state *ODEState, h *big.Int) *StepWitness {
	postMarking := NativeStep(state.Marking, h)
	postState := NewODEState(postMarking)
	postState.Step = state.Step + 1

	return &StepWitness{
		PreState:  state,
		PostState: postState,
		StepSize:  h,
	}
}

// ToCircuitAssignment converts a StepWitness into a gnark circuit assignment.
func (w *StepWitness) ToCircuitAssignment() *ODECircuit {
	c := &ODECircuit{
		PreStateRoot:  w.PreState.Root,
		PostStateRoot: w.PostState.Root,
		StepSize:      w.StepSize,
	}
	for p := 0; p < NumPlaces; p++ {
		c.PreMarking[p] = w.PreState.Marking[p]
		c.PostMarking[p] = w.PostState.Marking[p]
	}
	return c
}
`

// stateTmpl generates the ODEState struct with MiMC root computation.
var stateTmpl = `package {{.PackageName}}

import (
	"math/big"

	zkode "github.com/pflow-xyz/petri-pilot/zk-ode"
)

// ODEState tracks the current marking and MiMC state root.
type ODEState struct {
	Marking [NumPlaces]*big.Int
	Root    *big.Int
	Step    int
}

// NewODEState creates a state from a marking, computing the MiMC root.
func NewODEState(marking [NumPlaces]*big.Int) *ODEState {
	s := &ODEState{
		Marking: marking,
		Step:    0,
	}
	s.Root = zkode.ComputeRoot(marking[:])
	return s
}

// ApplyDiscreteMove applies a transition to a discrete marking using the
// stoichiometry matrix. Returns a new marking with clean integer values.
func ApplyDiscreteMove(marking [NumPlaces]*big.Int, transition int) [NumPlaces]*big.Int {
	one := zkode.FixFromFloat(1.0)
	var result [NumPlaces]*big.Int
	for p := 0; p < NumPlaces; p++ {
		result[p] = new(big.Int).Set(marking[p])
		s := Stoichiometry[p][transition]
		if s > 0 {
			for i := 0; i < s; i++ {
				result[p] = zkode.NativeFixAdd(result[p], one)
			}
		} else if s < 0 {
			for i := 0; i < -s; i++ {
				result[p] = zkode.NativeFixSub(result[p], one)
			}
		}
	}
	return result
}
`

// scoringCircuitTmpl generates the scoring extension for the ODE circuit.
// Only generated when HasScoring is true.
var scoringCircuitTmpl = `package {{.PackageName}}

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash/mimc"
	zkode "github.com/pflow-xyz/petri-pilot/zk-ode"
)

var (
	ScoringBonus   = zkode.FixFromFloat({{printf "%g" .Scoring.Bonus}})
	ScoringPenalty = zkode.FixFromFloat({{printf "%g" .Scoring.Penalty}})
)

// ScoringCircuit proves one ODE step plus tactical scoring for candidate transitions.
type ScoringCircuit struct {
	// Public inputs
	PreStateRoot  frontend.Variable ` + "`gnark:\",public\"`" + `
	PostStateRoot frontend.Variable ` + "`gnark:\",public\"`" + `
	StepSize      frontend.Variable ` + "`gnark:\",public\"`" + `
	Scores        [{{.Scoring.NumCandidates}}]frontend.Variable ` + "`gnark:\",public\"`" + `

	// Private witness
	PreMarking  [NumPlaces]frontend.Variable
	PostMarking [NumPlaces]frontend.Variable
}

func (c *ScoringCircuit) Define(api frontend.API) error {
	// 1. Verify pre-state root
	preH, _ := mimc.NewMiMC(api)
	for _, v := range c.PreMarking {
		preH.Write(v)
	}
	api.AssertIsEqual(preH.Sum(), c.PreStateRoot)

	// 2. Compute initial rates
	var initialRates [NumTransitions]frontend.Variable
	for t := 0; t < NumTransitions; t++ {
		initialRates[t] = computeRate(api, c.PreMarking[:], t)
	}

	// 3. Tsit5 ODE step
	var k [7][NumPlaces]frontend.Variable

	for stage := 0; stage < 7; stage++ {
		var yStage [NumPlaces]frontend.Variable
		for p := 0; p < NumPlaces; p++ {
			yStage[p] = c.PreMarking[p]
		}

		for j := 0; j < len(zkode.Tsit5A[stage]); j++ {
			hA := zkode.FixMul(api, c.StepSize, zkode.Tsit5A[stage][j])
			for p := 0; p < NumPlaces; p++ {
				contrib := zkode.FixMul(api, hA, k[j][p])
				yStage[p] = api.Add(yStage[p], contrib)
			}
		}

		var rates [NumTransitions]frontend.Variable
		for t := 0; t < NumTransitions; t++ {
			rates[t] = computeRate(api, yStage[:], t)
		}

		for p := 0; p < NumPlaces; p++ {
			k[stage][p] = frontend.Variable(0)
			for t := 0; t < NumTransitions; t++ {
				s := Stoichiometry[p][t]
				if s == 0 {
					continue
				}
				switch {
				case s == 1:
					k[stage][p] = api.Add(k[stage][p], rates[t])
				case s == -1:
					k[stage][p] = api.Sub(k[stage][p], rates[t])
				case s > 1:
					k[stage][p] = api.Add(k[stage][p], api.Mul(rates[t], s))
				case s < -1:
					k[stage][p] = api.Sub(k[stage][p], api.Mul(rates[t], -s))
				}
			}
		}
	}

	var postExpected [NumPlaces]frontend.Variable
	for p := 0; p < NumPlaces; p++ {
		postExpected[p] = c.PreMarking[p]
	}
	for j := 0; j < 7; j++ {
		if zkode.Tsit5B[j].Sign() == 0 {
			continue
		}
		hB := zkode.FixMul(api, c.StepSize, zkode.Tsit5B[j])
		for p := 0; p < NumPlaces; p++ {
			contrib := zkode.FixMul(api, hB, k[j][p])
			postExpected[p] = api.Add(postExpected[p], contrib)
		}
	}

	for p := 0; p < NumPlaces; p++ {
		api.AssertIsEqual(c.PostMarking[p], postExpected[p])
	}

	// 4. Verify post-state root
	postH, _ := mimc.NewMiMC(api)
	for _, v := range c.PostMarking {
		postH.Write(v)
	}
	api.AssertIsEqual(postH.Sum(), c.PostStateRoot)

	// 5. Scoring: evaluate each candidate transition
	candidateIndices := []int{ {{- range $i, $c := .Scoring.Candidates}}{{if $i}}, {{end}}{{$c}}{{end -}} }
	targetIndices := []int{ {{- range $i, $t := .Scoring.Targets}}{{if $i}}, {{end}}{{$t}}{{end -}} }
	one := zkode.FixFromFloat(1.0)

	for ci, cIdx := range candidateIndices {
		baseRate := initialRates[cIdx]

		// Win flag: does firing candidate enable any target?
		winSum := frontend.Variable(0)
		for _, tIdx := range targetIndices {
			// Check if all inputs of target would be satisfied after candidate fires
			lineWin := frontend.Variable(one)
			for _, inp := range TransitionInputs[tIdx] {
				// Hypothetical marking: current + stoichiometry effect of candidate
				hyp := c.PreMarking[inp]
				sEffect := Stoichiometry[inp][cIdx]
				if sEffect > 0 {
					for i := 0; i < sEffect; i++ {
						hyp = api.Add(hyp, one)
					}
				} else if sEffect < 0 {
					for i := 0; i < -sEffect; i++ {
						hyp = api.Sub(hyp, one)
					}
				}
				lineWin = zkode.FixMul(api, lineWin, hyp)
			}
			winSum = api.Add(winSum, lineWin)
		}
		winFlag := zkode.IsNonZeroFP(api, winSum)

		// Block flag: does opponent have an unblocked threat?
		threatSum := frontend.Variable(0)
		for _, tIdx := range targetIndices {
			inputs := TransitionInputs[tIdx]
			for missingIdx := 0; missingIdx < len(inputs); missingIdx++ {
				missing := inputs[missingIdx]
				// Skip if our move fills the missing input
				if Stoichiometry[missing][cIdx] > 0 {
					continue
				}
				// Check if missing place is "empty" (has zero marking)
				// and the other inputs are satisfied (non-zero)
				otherProduct := frontend.Variable(one)
				for checkIdx := 0; checkIdx < len(inputs); checkIdx++ {
					if checkIdx == missingIdx {
						continue
					}
					otherProduct = zkode.FixMul(api, otherProduct, c.PreMarking[inputs[checkIdx]])
				}
				// Missing must be empty (zero) for it to be a threat
				// Actually for threat detection: we want the missing one to be fillable
				// Simplified: threat = product of other inputs being non-zero
				threatSum = api.Add(threatSum, otherProduct)
			}
		}
		blockFlag := zkode.IsNonZeroFP(api, threatSum)

		// score = baseRate + bonus * winFlag - penalty * blockFlag * (1 - winFlag)
		bonusTerm := zkode.FixMul(api, ScoringBonus, winFlag)
		oneMinusWin := api.Sub(one, winFlag)
		penaltyTerm := zkode.FixMul(api, ScoringPenalty, zkode.FixMul(api, blockFlag, oneMinusWin))
		score := api.Add(baseRate, bonusTerm)
		score = api.Sub(score, penaltyTerm)

		// Mask: score = 0 if candidate not fireable (rate == 0)
		fireable := zkode.IsNonZeroFP(api, initialRates[cIdx])
		score = zkode.FixMul(api, score, fireable)

		api.AssertIsEqual(score, c.Scores[ci])
	}

	return nil
}
`

// scoringWitnessTmpl generates the native scoring computation matching the circuit.
var scoringWitnessTmpl = `package {{.PackageName}}

import "math/big"

import zkode "github.com/pflow-xyz/petri-pilot/zk-ode"

// ScoringWitness contains all data for one scoring circuit assignment.
type ScoringWitness struct {
	PreState  *ODEState
	PostState *ODEState
	StepSize  *big.Int
	Scores    [{{.Scoring.NumCandidates}}]*big.Int
}

// ComputeScoringStep runs one Tsit5 ODE step and computes tactical scores.
func ComputeScoringStep(state *ODEState, h *big.Int) *ScoringWitness {
	one := zkode.FixFromFloat(1.0)
	zero := zkode.FixFromFloat(0.0)

	// Compute all initial rates
	var initialRates [NumTransitions]*big.Int
	for t := 0; t < NumTransitions; t++ {
		initialRates[t] = nativeRate(state.Marking, t)
	}

	// Run ODE step
	postMarking := NativeStep(state.Marking, h)
	postState := NewODEState(postMarking)
	postState.Step = state.Step + 1

	candidateIndices := []int{ {{- range $i, $c := .Scoring.Candidates}}{{if $i}}, {{end}}{{$c}}{{end -}} }
	targetIndices := []int{ {{- range $i, $t := .Scoring.Targets}}{{if $i}}, {{end}}{{$t}}{{end -}} }

	var scores [{{.Scoring.NumCandidates}}]*big.Int

	for ci, cIdx := range candidateIndices {
		baseRate := new(big.Int).Set(initialRates[cIdx])

		// Check if candidate is fireable
		if initialRates[cIdx].Cmp(zero) == 0 {
			scores[ci] = new(big.Int).Set(zero)
			continue
		}

		// Win detection
		winFlag := false
		for _, tIdx := range targetIndices {
			allSatisfied := true
			for _, inp := range TransitionInputs[tIdx] {
				hyp := new(big.Int).Set(state.Marking[inp])
				sEffect := Stoichiometry[inp][cIdx]
				if sEffect > 0 {
					for i := 0; i < sEffect; i++ {
						hyp = zkode.NativeFixAdd(hyp, one)
					}
				} else if sEffect < 0 {
					for i := 0; i < -sEffect; i++ {
						hyp = zkode.NativeFixSub(hyp, one)
					}
				}
				if hyp.Cmp(zero) == 0 {
					allSatisfied = false
					break
				}
			}
			if allSatisfied {
				winFlag = true
				break
			}
		}

		// Block detection
		blockFlag := false
		for _, tIdx := range targetIndices {
			inputs := TransitionInputs[tIdx]
			for missingIdx := 0; missingIdx < len(inputs); missingIdx++ {
				missing := inputs[missingIdx]
				if Stoichiometry[missing][cIdx] > 0 {
					continue
				}
				allOthers := true
				for checkIdx := 0; checkIdx < len(inputs); checkIdx++ {
					if checkIdx == missingIdx {
						continue
					}
					if state.Marking[inputs[checkIdx]].Cmp(zero) == 0 {
						allOthers = false
						break
					}
				}
				if allOthers {
					blockFlag = true
					break
				}
			}
			if blockFlag {
				break
			}
		}

		score := new(big.Int).Set(baseRate)
		if winFlag {
			score = zkode.NativeFixAdd(score, ScoringBonus)
		} else if blockFlag {
			score = zkode.NativeFixSub(score, ScoringPenalty)
		}
		scores[ci] = score
	}

	return &ScoringWitness{
		PreState:  state,
		PostState: postState,
		StepSize:  h,
		Scores:    scores,
	}
}

// ToScoringCircuitAssignment converts a ScoringWitness into a gnark circuit assignment.
func (w *ScoringWitness) ToScoringCircuitAssignment() *ScoringCircuit {
	c := &ScoringCircuit{
		PreStateRoot:  w.PreState.Root,
		PostStateRoot: w.PostState.Root,
		StepSize:      w.StepSize,
	}
	for i := 0; i < {{.Scoring.NumCandidates}}; i++ {
		c.Scores[i] = w.Scores[i]
	}
	for p := 0; p < NumPlaces; p++ {
		c.PreMarking[p] = w.PreState.Marking[p]
		c.PostMarking[p] = w.PostState.Marking[p]
	}
	return c
}
`

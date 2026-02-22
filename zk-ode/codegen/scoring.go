package codegen

import (
	"encoding/json"
	"os"
	"path"
)

// ScoringConfig defines hypothesis-based scoring for generated ZK ODE circuits.
// Candidates are the transitions being evaluated (e.g., player moves).
// Targets are transitions whose enablement indicates a "win" condition.
// The generator checks if firing a candidate enables a target (win flag)
// or if the opponent can reach a target without the candidate (block flag).
type ScoringConfig struct {
	// Candidates are transition ID globs for moves to score (e.g., "x_play_*").
	Candidates []string `json:"candidates"`

	// Targets are transition ID globs for win conditions (e.g., "x_win_*").
	Targets []string `json:"targets"`

	// Bonus is the score bonus for enabling a target (default 10.0).
	Bonus float64 `json:"bonus"`

	// Penalty is the score penalty for an opponent threat (default 1.5).
	Penalty float64 `json:"penalty"`

	// UseRateWeights uses rate constants as base position weights in scoring.
	UseRateWeights bool `json:"useRateWeights"`
}

// LoadScoringConfig reads a ScoringConfig from a JSON file.
func LoadScoringConfig(filepath string) (*ScoringConfig, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	var cfg ScoringConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Bonus == 0 {
		cfg.Bonus = 10.0
	}
	if cfg.Penalty == 0 {
		cfg.Penalty = 1.5
	}
	return &cfg, nil
}

// MatchGlobs returns the indices of IDs matching any of the given globs.
func MatchGlobs(ids []string, globs []string) []int {
	var matched []int
	for i, id := range ids {
		for _, g := range globs {
			if ok, _ := path.Match(g, id); ok {
				matched = append(matched, i)
				break
			}
		}
	}
	return matched
}

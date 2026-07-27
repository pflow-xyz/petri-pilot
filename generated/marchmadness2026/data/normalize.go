package data

import (
	"math"
	"sort"
)

// TeamStats holds the normalized multi-faceted strength metrics (0-100 scale).
type TeamStats struct {
	Name     string
	Seed     int
	Offense  float64 // Scoring efficiency (percentile of AdjOE)
	Defense  float64 // Defensive efficiency (percentile of AdjDE, inverted)
	Record   float64 // Win% * 50 + SOS percentile * 50
	Momentum float64 // Last-10 win%, weighted by recency + conf tourney bonus
	Depth    float64 // Bench minutes share + roster concentration (Herfindahl)
}

// NormalizeTeams converts raw data into 0-100 normalized TeamStats.
func NormalizeTeams(raw []RawTeamData) []TeamStats {
	n := len(raw)
	if n == 0 {
		return nil
	}

	// Collect values for percentile ranking
	adjOEs := make([]float64, n)
	adjDEs := make([]float64, n)
	recordScores := make([]float64, n)
	momentumScores := make([]float64, n)
	depthScores := make([]float64, n)

	for i, t := range raw {
		adjOEs[i] = t.AdjOE
		adjDEs[i] = t.AdjDE
		recordScores[i] = computeRecordScore(t)
		momentumScores[i] = computeMomentum(t)
		depthScores[i] = computeDepth(t)
	}

	// Convert to percentile ranks (0-100)
	offPct := percentileRanks(adjOEs, false) // Higher AdjOE = better
	defPct := percentileRanks(adjDEs, true)  // Lower AdjDE = better (invert)
	recPct := percentileRanks(recordScores, false)
	momPct := percentileRanks(momentumScores, false)
	depPct := percentileRanks(depthScores, false)

	result := make([]TeamStats, n)
	for i, t := range raw {
		result[i] = TeamStats{
			Name:     t.Name,
			Offense:  offPct[i],
			Defense:  defPct[i],
			Record:   recPct[i],
			Momentum: momPct[i],
			Depth:    depPct[i],
		}
	}

	return result
}

// NormalizeTournamentTeams returns only tournament teams (68) with seeds assigned.
func NormalizeTournamentTeams(raw []RawTeamData, seeds map[string]int) []TeamStats {
	all := NormalizeTeams(raw)
	if len(all) == 0 {
		return nil
	}

	// Build name→index map
	nameIdx := make(map[string]int)
	for i, t := range all {
		nameIdx[t.Name] = i
	}

	var tournament []TeamStats
	for name, seed := range seeds {
		if idx, ok := nameIdx[name]; ok {
			ts := all[idx]
			ts.Seed = seed
			tournament = append(tournament, ts)
		}
	}

	// Sort by seed then by composite score
	sort.Slice(tournament, func(i, j int) bool {
		if tournament[i].Seed != tournament[j].Seed {
			return tournament[i].Seed < tournament[j].Seed
		}
		ci := composite(tournament[i])
		cj := composite(tournament[j])
		return ci > cj
	})

	return tournament
}

func composite(t TeamStats) float64 {
	return t.Offense*0.20 + t.Defense*0.25 + t.Record*0.20 + t.Momentum*0.20 + t.Depth*0.15
}

// computeRecordScore calculates record quality: win_pct * 50 + SOS * 50
func computeRecordScore(t RawTeamData) float64 {
	total := t.Wins + t.Losses
	if total == 0 {
		return 50 // Default for missing data
	}
	winPct := float64(t.Wins) / float64(total)

	// SOS is typically on a scale where 0 = average.
	// Normalize SOS: use Barthag as a proxy if SOS not available
	sosScore := t.Barthag * 100
	if t.SOS != 0 {
		// Barttorvik SOS is typically in the range -10 to +15
		// Normalize to 0-100
		sosScore = math.Min(100, math.Max(0, (t.SOS+10)*100/25))
	}

	return winPct*50 + sosScore/100*50
}

// computeMomentum calculates momentum from recent games.
func computeMomentum(t RawTeamData) float64 {
	if len(t.RecentGames) == 0 {
		// Fall back to overall record quality + Barthag
		total := t.Wins + t.Losses
		if total == 0 {
			return 50
		}
		return float64(t.Wins)/float64(total)*80 + t.Barthag*20
	}

	// Exponential decay weighting: most recent = weight 1.0, 10th = weight 0.5
	totalWeight := 0.0
	weightedWins := 0.0

	for i, g := range t.RecentGames {
		weight := math.Pow(0.93, float64(i)) // ~0.5 at i=10
		totalWeight += weight
		if g.Win {
			weightedWins += weight
		}
	}

	momentum := 0.0
	if totalWeight > 0 {
		momentum = (weightedWins / totalWeight) * 80 // 0-80 from recent games
	}

	// Conference tournament bonus
	if t.ConfTourneyWin {
		momentum += 15
	} else if t.ConfTourneyRU {
		momentum += 8
	}

	// Barthag bonus (quality indicator)
	momentum += t.Barthag * 5

	return math.Min(100, momentum)
}

// computeDepth calculates roster depth from minutes distribution.
func computeDepth(t RawTeamData) float64 {
	if len(t.MinutesDistribution) == 0 {
		// Fall back to a mid-range estimate based on available data
		// Teams with more wins tend to have better depth management
		total := t.Wins + t.Losses
		if total == 0 {
			return 50
		}
		return math.Min(100, float64(t.Wins)/float64(total)*60+t.Barthag*40)
	}

	// Herfindahl index of minutes shares
	// HHI = sum(share_i^2), range [1/n, 1]
	// Lower HHI = more evenly distributed minutes = deeper roster
	hhi := 0.0
	for _, share := range t.MinutesDistribution {
		hhi += share * share
	}

	n := float64(len(t.MinutesDistribution))
	if n <= 1 {
		return 50
	}

	// Normalize: HHI ranges from 1/n (perfectly equal) to 1 (one player plays all)
	// depth_score = 100 * (1 - normalized_hhi)
	minHHI := 1.0 / n
	normalizedHHI := (hhi - minHHI) / (1.0 - minHHI)
	depthScore := 100 * (1 - normalizedHHI)

	// Add bench PPG bonus (0-20 points)
	if t.BenchPPG > 0 {
		benchBonus := math.Min(20, t.BenchPPG/2)
		depthScore = depthScore*0.8 + benchBonus
	}

	return math.Min(100, math.Max(0, depthScore))
}

// percentileRanks converts raw values to percentile ranks (0-100).
// If invert is true, lower values get higher percentiles.
func percentileRanks(values []float64, invert bool) []float64 {
	n := len(values)
	result := make([]float64, n)

	// Create sorted index
	type indexedValue struct {
		idx int
		val float64
	}
	sorted := make([]indexedValue, n)
	for i, v := range values {
		sorted[i] = indexedValue{i, v}
	}

	if invert {
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].val > sorted[j].val })
	} else {
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].val < sorted[j].val })
	}

	// Assign percentile ranks
	for rank, sv := range sorted {
		result[sv.idx] = float64(rank) / float64(n-1) * 100
	}

	return result
}

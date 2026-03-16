package backtest

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pflow-xyz/petri-pilot/generated/marchmadness2026/data"
)

// Result holds the accuracy metrics for a single season backtest.
type Result struct {
	Season          int
	Champion        string
	PredictedTop4   []string
	ActualFinalFour []string
	ChampInTop4     bool
	FF4InTop8       int // How many actual F4 teams were in our predicted top 8
	Top8Accuracy    float64
}

// Weights holds the facet weights for the model.
type Weights struct {
	Offense  float64
	Defense  float64
	Record   float64
	Momentum float64
	Depth    float64
}

// DefaultWeights returns the default facet weights.
func DefaultWeights() Weights {
	return Weights{
		Offense:  0.20,
		Defense:  0.25,
		Record:   0.20,
		Momentum: 0.20,
		Depth:    0.15,
	}
}

// RunBacktest runs the model against historical data for a range of seasons.
func RunBacktest(
	historicalTeams map[int][]data.RawTeamData,
	tournamentResults map[int][]data.TournamentResult,
	seeds map[int]map[string]int,
	weights Weights,
) []Result {
	var results []Result

	for season, rawTeams := range historicalTeams {
		tourney, ok := tournamentResults[season]
		if !ok {
			continue
		}

		// Normalize teams
		allNorm := data.NormalizeTeams(rawTeams)
		if len(allNorm) == 0 {
			continue
		}

		// Compute composite scores and rank
		type ranked struct {
			Name  string
			Score float64
		}
		var rankings []ranked
		for _, t := range allNorm {
			score := t.Offense*weights.Offense + t.Defense*weights.Defense +
				t.Record*weights.Record + t.Momentum*weights.Momentum +
				t.Depth*weights.Depth
			rankings = append(rankings, ranked{t.Name, score})
		}
		sort.Slice(rankings, func(i, j int) bool { return rankings[i].Score > rankings[j].Score })

		// Get actual results
		champion := data.GetChampion(tourney)
		finalFour := data.GetFinalFour(tourney)

		// Predicted top 4 and top 8
		top4 := make([]string, 0, 4)
		top8 := make([]string, 0, 8)
		for i := 0; i < len(rankings) && i < 8; i++ {
			if i < 4 {
				top4 = append(top4, rankings[i].Name)
			}
			top8 = append(top8, rankings[i].Name)
		}

		// Score accuracy
		champInTop4 := false
		for _, t := range top4 {
			if strings.EqualFold(t, champion) {
				champInTop4 = true
				break
			}
		}

		ff4InTop8 := 0
		for _, ff := range finalFour {
			for _, t := range top8 {
				if strings.EqualFold(t, ff) {
					ff4InTop8++
					break
				}
			}
		}

		results = append(results, Result{
			Season:          season,
			Champion:        champion,
			PredictedTop4:   top4,
			ActualFinalFour: finalFour,
			ChampInTop4:     champInTop4,
			FF4InTop8:       ff4InTop8,
			Top8Accuracy:    float64(ff4InTop8) / float64(len(finalFour)) * 100,
		})
	}

	// Sort by season
	sort.Slice(results, func(i, j int) bool { return results[i].Season < results[j].Season })

	return results
}

// PrintResults displays backtest results in a formatted table.
func PrintResults(results []Result) {
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("  Backtest Results: Model vs Actual Tournament Outcomes")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Printf("%-8s %-16s %-6s %-6s  %s\n", "Season", "Champion", "InTop4", "FF/Top8", "Predicted Top 4")
	fmt.Println("──────── ──────────────── ────── ──────  ──────────────────────────────────────")

	totalChampHits := 0
	totalFF4Hits := 0
	totalFF4 := 0

	for _, r := range results {
		champMark := "  ✗"
		if r.ChampInTop4 {
			champMark = "  ✓"
			totalChampHits++
		}
		totalFF4Hits += r.FF4InTop8
		totalFF4 += len(r.ActualFinalFour)

		top4Str := strings.Join(r.PredictedTop4, ", ")
		fmt.Printf("%-8d %-16s %s    %d/%-2d    %s\n",
			r.Season, r.Champion, champMark, r.FF4InTop8, len(r.ActualFinalFour), top4Str)
	}

	fmt.Println()
	fmt.Println("Summary:")
	n := len(results)
	if n > 0 {
		fmt.Printf("  Champion in Top 4: %d/%d (%.1f%%)\n", totalChampHits, n, float64(totalChampHits)/float64(n)*100)
		fmt.Printf("  F4 teams in Top 8: %d/%d (%.1f%%)\n", totalFF4Hits, totalFF4, float64(totalFF4Hits)/float64(totalFF4)*100)
	}
}

// SweepWeights runs backtests across different weight configurations to find optimal weights.
func SweepWeights(
	historicalTeams map[int][]data.RawTeamData,
	tournamentResults map[int][]data.TournamentResult,
	seeds map[int]map[string]int,
) Weights {
	bestScore := 0.0
	bestWeights := DefaultWeights()

	// Sweep each facet in increments of 0.05
	steps := []float64{0.10, 0.15, 0.20, 0.25, 0.30, 0.35}

	for _, off := range steps {
		for _, def := range steps {
			for _, rec := range steps {
				for _, mom := range steps {
					dep := 1.0 - off - def - rec - mom
					if dep < 0.05 || dep > 0.40 {
						continue
					}

					w := Weights{off, def, rec, mom, dep}
					results := RunBacktest(historicalTeams, tournamentResults, seeds, w)

					// Score: champion accuracy + F4 accuracy
					score := 0.0
					for _, r := range results {
						if r.ChampInTop4 {
							score += 2.0
						}
						score += float64(r.FF4InTop8) * 0.5
					}

					if score > bestScore {
						bestScore = score
						bestWeights = w
					}
				}
			}
		}
	}

	fmt.Printf("Best weights found (score=%.1f):\n", bestScore)
	fmt.Printf("  Offense=%.2f Defense=%.2f Record=%.2f Momentum=%.2f Depth=%.2f\n",
		bestWeights.Offense, bestWeights.Defense, bestWeights.Record, bestWeights.Momentum, bestWeights.Depth)

	return bestWeights
}

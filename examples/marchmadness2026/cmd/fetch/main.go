package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/pflow-xyz/petri-pilot/examples/marchmadness2026/data"
)

func main() {
	season := flag.Int("season", time.Now().Year(), "Season year to fetch (e.g., 2027)")
	cacheDir := flag.String("cache", "cache", "Cache directory")
	noCache := flag.Bool("no-cache", false, "Skip cache, fetch fresh data")
	topN := flag.Int("top", 25, "Number of top teams to display")
	flag.Parse()

	fmt.Printf("NCAA Data Pipeline — Season %d\n", *season)
	fmt.Println("═══════════════════════════════════")

	// Set up cache
	var cache *data.Cache
	if !*noCache {
		var err error
		cache, err = data.NewCache(*cacheDir + "/ncaa.db")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: cache init failed: %v\n", err)
		} else {
			defer cache.Close()
			if staleness, err := cache.CacheStaleness(*season); err == nil {
				fmt.Printf("Cache age: %s\n", staleness.Truncate(time.Minute))
			}
		}
	}

	// Fetch all team data
	start := time.Now()
	raw, err := data.FetchAllTeams(*season, cache)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Fetched %d teams in %s\n\n", len(raw), time.Since(start).Truncate(time.Millisecond))

	// Normalize to 0-100 scale
	normalized := data.NormalizeTeams(raw)

	// Compute composite scores and sort
	type rankedTeam struct {
		data.TeamStats
		Composite float64
	}

	var ranked []rankedTeam
	for _, t := range normalized {
		comp := t.Offense*0.20 + t.Defense*0.25 + t.Record*0.20 + t.Momentum*0.20 + t.Depth*0.15
		ranked = append(ranked, rankedTeam{t, comp})
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].Composite > ranked[j].Composite })

	// Display top N
	fmt.Printf("Top %d Teams (Composite Score):\n", *topN)
	fmt.Println("Rank  Team                     Off   Def   Rec   Mom   Dep   Composite")
	fmt.Println("────  ────────────────────────  ────  ────  ────  ────  ────  ─────────")
	for i := 0; i < *topN && i < len(ranked); i++ {
		t := ranked[i]
		fmt.Printf(" %2d.  %-24s %5.1f %5.1f %5.1f %5.1f %5.1f    %5.1f\n",
			i+1, t.Name, t.Offense, t.Defense, t.Record, t.Momentum, t.Depth, t.Composite)
	}

	// Summary statistics
	fmt.Printf("\nDataset: %d D-I teams\n", len(raw))
	if len(raw) > 0 {
		var totalAdjOE, totalAdjDE float64
		for _, t := range raw {
			totalAdjOE += t.AdjOE
			totalAdjDE += t.AdjDE
		}
		n := float64(len(raw))
		fmt.Printf("Avg AdjOE: %.1f  Avg AdjDE: %.1f\n", totalAdjOE/n, totalAdjDE/n)
	}
}

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/pflow-xyz/petri-pilot/generated/marchmadness2026/backtest"
	"github.com/pflow-xyz/petri-pilot/generated/marchmadness2026/data"
)

func main() {
	bartPath := flag.String("barttorvik", "data/historical/barttorvik.csv", "Path to historical Barttorvik CSV")
	resultsPath := flag.String("results", "data/historical/tournament_results.csv", "Path to tournament results CSV")
	sweep := flag.Bool("sweep", false, "Sweep weight configurations to find optimal")
	flag.Parse()

	fmt.Println("NCAA Bracket Model — Historical Backtest")
	fmt.Println("═══════════════════════════════════════════")
	fmt.Println()

	// Load historical Barttorvik data
	fmt.Printf("Loading historical team data from %s...\n", *bartPath)
	historicalTeams, err := data.LoadHistoricalBarttorvik(*bartPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading Barttorvik data: %v\n", err)
		fmt.Println("\nTo run backtesting, you need historical CSV data files.")
		fmt.Println("Place them at:")
		fmt.Printf("  %s  — Barttorvik team data by season\n", *bartPath)
		fmt.Printf("  %s  — Tournament results by season\n", *resultsPath)
		fmt.Println("\nExpected CSV columns:")
		fmt.Println("  barttorvik.csv: Season,Team,Conf,Rec,AdjOE,AdjDE,Barthag,SOS,WAB")
		fmt.Println("  results.csv:    Season,Round,WTeam,WScore,WSeed,LTeam,LScore,LSeed")
		os.Exit(1)
	}
	fmt.Printf("  Loaded %d seasons\n", len(historicalTeams))

	// Load tournament results
	fmt.Printf("Loading tournament results from %s...\n", *resultsPath)
	tournamentResults, err := data.LoadKaggleResults(*resultsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading tournament results: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  Loaded %d seasons of results\n", len(tournamentResults))

	// Count overlapping seasons
	overlapSeasons := 0
	for season := range historicalTeams {
		if _, ok := tournamentResults[season]; ok {
			overlapSeasons++
		}
	}
	fmt.Printf("  %d seasons with both team data and results\n\n", overlapSeasons)

	if overlapSeasons == 0 {
		fmt.Println("No overlapping seasons found. Check your data files.")
		os.Exit(1)
	}

	// Seeds are not always in the data — pass nil (backtest handles it)
	var seeds map[int]map[string]int

	if *sweep {
		fmt.Println("Sweeping weight configurations...")
		fmt.Println()
		bestWeights := backtest.SweepWeights(historicalTeams, tournamentResults, seeds)
		fmt.Println()
		fmt.Println("Running backtest with optimal weights...")
		results := backtest.RunBacktest(historicalTeams, tournamentResults, seeds, bestWeights)
		backtest.PrintResults(results)
	} else {
		weights := backtest.DefaultWeights()
		fmt.Printf("Using default weights: OFF=%.0f%% DEF=%.0f%% REC=%.0f%% MOM=%.0f%% DEP=%.0f%%\n\n",
			weights.Offense*100, weights.Defense*100, weights.Record*100, weights.Momentum*100, weights.Depth*100)

		results := backtest.RunBacktest(historicalTeams, tournamentResults, seeds, weights)
		backtest.PrintResults(results)
	}
}

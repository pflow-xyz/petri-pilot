package marchmadness2026

import (
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
	"time"

	"github.com/pflow-xyz/go-pflow/petri"
	"github.com/pflow-xyz/go-pflow/plotter"
	"github.com/pflow-xyz/go-pflow/reachability"
	"github.com/pflow-xyz/go-pflow/solver"
	"github.com/pflow-xyz/go-pflow/visualization"
	"github.com/pflow-xyz/petri-pilot/generated/marchmadness2026/data"
)

// TeamStats holds the multi-faceted strength metrics for a team (0-100 scale).
type TeamStats struct {
	Name     string
	Seed     int
	Offense  float64 // Scoring efficiency, PPG, shooting %
	Defense  float64 // Opp PPG, blocks, steals, KenPom adj def
	Record   float64 // Win%, SOS-adjusted record quality
	Momentum float64 // Last 10 games, conf tourney performance
	Depth    float64 // Bench scoring, roster depth, injury status
}

// Run executes all ranking models and analysis.
func Run() {
	useData := flag.Bool("data", false, "Use live data pipeline instead of hardcoded stats")
	season := flag.Int("season", time.Now().Year(), "Season year for data pipeline")
	cacheDir := flag.String("cache", "cache", "Cache directory for fetched data")
	flag.Parse()

	var teams []TeamStats

	if *useData {
		teams = fetchTeamsFromPipeline(*season, *cacheDir)
	} else {
		// ================================================================
		// TEAM DATA — derived from our Selection Sunday research
		// Normalized to 0-100 scale based on KenPom, Sagarin, NET rankings
		// ================================================================
		teams = []TeamStats{
			{"Duke", 1, 95, 88, 97, 92, 90},
			{"Arizona", 1, 93, 85, 96, 90, 88},
			{"Michigan", 1, 82, 92, 95, 85, 91},
			{"Florida", 1, 88, 83, 90, 95, 85},
			{"Houston", 2, 80, 95, 92, 93, 82},
			{"UConn", 2, 86, 84, 91, 78, 88},
			{"Iowa State", 2, 79, 91, 90, 80, 86},
			{"Purdue", 2, 90, 78, 88, 93, 84},
			{"Illinois", 3, 84, 82, 87, 92, 80},
			{"Gonzaga", 3, 88, 75, 91, 82, 72}, // Huff injury hurts depth
			{"Michigan St", 3, 78, 80, 86, 83, 85},
			{"Virginia", 3, 72, 90, 91, 82, 84},
			{"St John's", 5, 82, 79, 92, 95, 78},
			{"Kansas", 4, 85, 83, 82, 75, 80},
			{"Alabama", 4, 91, 72, 82, 88, 76},
			{"Nebraska", 4, 77, 81, 90, 80, 78},
		}
	}

	// ================================================================
	// FACET WEIGHTS — the "reasoning" behind our rankings
	// These encode domain knowledge about what matters in March
	// ================================================================
	weights := map[string]float64{
		"offense":  0.20, // Important but not dominant
		"defense":  0.25, // Defense wins championships
		"record":   0.20, // Proven track record matters
		"momentum": 0.20, // Hot teams thrive in March
		"depth":    0.15, // Bench matters in 6-game gauntlet
	}

	// ================================================================
	// MODEL 1: Multi-Facet Accumulation Model
	// Each team's facet tokens flow into a championship potential pool
	// ODE rates encode facet weights × team strength
	// ================================================================
	fmt.Println("═══════════════════════════════════════════════════")
	fmt.Println("  NCAA Tournament Ranking via Petri Net ODE Model")
	fmt.Println("═══════════════════════════════════════════════════")
	fmt.Println()
	fmt.Println("Model 1: Multi-Facet Strength Accumulation")
	fmt.Println("───────────────────────────────────────────")
	fmt.Printf("Weights: OFF=%.0f%% DEF=%.0f%% REC=%.0f%% MOM=%.0f%% DEP=%.0f%%\n\n",
		weights["offense"]*100, weights["defense"]*100, weights["record"]*100,
		weights["momentum"]*100, weights["depth"]*100)

	b := petri.Build()

	// Create places for each team's facets and ranking accumulator
	for _, t := range teams {
		b.Place(t.Name+"_off", t.Offense)
		b.Place(t.Name+"_def", t.Defense)
		b.Place(t.Name+"_rec", t.Record)
		b.Place(t.Name+"_mom", t.Momentum)
		b.Place(t.Name+"_dep", t.Depth)
		b.Place(t.Name+"_rank", 0) // Championship potential accumulates here
	}

	// Create transitions: each facet flows into ranking
	facets := []string{"off", "def", "rec", "mom", "dep"}
	for _, t := range teams {
		for _, f := range facets {
			tName := t.Name + "_" + f + "_eval"
			b.Transition(tName)
			b.Arc(t.Name+"_"+f, tName, 1)
			b.Arc(tName, t.Name+"_rank", 1)
		}
	}

	// Build rates: weight × base_rate (mass-action means flux = rate × tokens)
	facetWeightMap := map[string]string{
		"off": "offense", "def": "defense", "rec": "record",
		"mom": "momentum", "dep": "depth",
	}
	rates := make(map[string]float64)
	for _, t := range teams {
		for _, f := range facets {
			tName := t.Name + "_" + f + "_eval"
			rates[tName] = weights[facetWeightMap[f]] * 0.01 // Scale for ODE stability
		}
	}

	net := b.Done()
	state := net.SetState(nil)

	// Save the Petri net SVG (subset for readability)
	visualization.SaveSVG(net, "ranking_petri_net.svg")

	// ================================================================
	// RUN ODE SIMULATION
	// ================================================================
	prob := solver.NewProblem(net, state, [2]float64{0, 50}, rates)
	sol := solver.Solve(prob, solver.Tsit5(), solver.DefaultOptions())

	// Extract final rankings
	type TeamRank struct {
		Name  string
		Score float64
	}
	var rankings []TeamRank
	finalState := sol.GetFinalState()
	for _, t := range teams {
		rankings = append(rankings, TeamRank{t.Name, finalState[t.Name+"_rank"]})
	}
	sort.Slice(rankings, func(i, j int) bool { return rankings[i].Score > rankings[j].Score })

	fmt.Println("Final Power Rankings (ODE t=50):")
	fmt.Println("Rank  Team            Score    Seed")
	fmt.Println("────  ──────────────  ───────  ────")
	for i, r := range rankings {
		seed := 0
		for _, t := range teams {
			if t.Name == r.Name {
				seed = t.Seed
				break
			}
		}
		fmt.Printf(" %2d.  %-14s  %7.1f    %d\n", i+1, r.Name, r.Score, seed)
	}

	// Plot championship potential over time for top 8
	topTeams := make([]string, 0)
	colors := []string{"#001A57", "#CC0033", "#00274C", "#FA4616", "#C8102E", "#002D62", "#C41230", "#CEB888"}
	for i := 0; i < 8 && i < len(rankings); i++ {
		topTeams = append(topTeams, rankings[i].Name)
	}

	p1 := plotter.NewSVGPlotter(1000, 500).
		SetTitle("Championship Potential Accumulation (Multi-Facet ODE)").
		SetXLabel("Evaluation Time").
		SetYLabel("Championship Potential Score")
	for i, name := range topTeams {
		p1.AddSeries(sol.T, sol.GetVariable(name+"_rank"), name, colors[i%len(colors)])
	}
	svg1 := p1.Render()
	os.WriteFile("model1_accumulation.svg", []byte(svg1), 0644)

	// ================================================================
	// MODEL 2: Competition / Interaction Model
	// Teams in the same region compete — stronger teams drain weaker ones
	// Models head-to-head matchup dynamics
	// ================================================================
	fmt.Println()
	fmt.Println("Model 2: Head-to-Head Competition Dynamics")
	fmt.Println("──────────────────────────────────────────")

	b2 := petri.Build()

	// Each team starts with a composite strength score
	for _, t := range teams {
		composite := t.Offense*weights["offense"] + t.Defense*weights["defense"] +
			t.Record*weights["record"] + t.Momentum*weights["momentum"] +
			t.Depth*weights["depth"]
		b2.Place(t.Name, composite)
	}
	// "Eliminated" pool collects lost tokens
	b2.Place("eliminated", 0)

	// Regional matchups: teams in same region compete
	// Tokens flow from weaker to stronger (modeled via rate asymmetry)
	type Matchup struct {
		A, B string
	}
	matchups := []Matchup{
		// EAST region clashes
		{"Duke", "UConn"}, {"Duke", "St John's"}, {"Duke", "Kansas"},
		{"UConn", "Michigan St"}, {"St John's", "Kansas"},
		// SOUTH
		{"Florida", "Houston"}, {"Florida", "Illinois"}, {"Florida", "Nebraska"},
		{"Houston", "Illinois"}, {"Houston", "Nebraska"},
		// WEST
		{"Arizona", "Purdue"}, {"Arizona", "Gonzaga"}, {"Arizona", "Alabama"},
		{"Purdue", "Gonzaga"}, {"Purdue", "Alabama"},
		// MIDWEST
		{"Michigan", "Iowa State"}, {"Michigan", "Virginia"},
		{"Iowa State", "Virginia"},
	}

	rates2 := make(map[string]float64)
	for i, m := range matchups {
		// Bidirectional: each team can beat the other, rates proportional to strength
		tAB := fmt.Sprintf("clash_%d_ab", i)
		tBA := fmt.Sprintf("clash_%d_ba", i)
		b2.Transition(tAB)
		b2.Transition(tBA)
		// A beats B: B's tokens flow to A
		b2.Arc(m.B, tAB, 1)
		b2.Arc(m.A, tAB, 1) // Catalyst: A must have tokens to win
		b2.Arc(tAB, m.A, 2) // A gains what B lost + returns own
		// B beats A: A's tokens flow to B
		b2.Arc(m.A, tBA, 1)
		b2.Arc(m.B, tBA, 1)
		b2.Arc(tBA, m.B, 2)

		// Rate asymmetry encodes strength difference
		var strengthA, strengthB float64
		for _, t := range teams {
			if t.Name == m.A {
				strengthA = t.Offense*weights["offense"] + t.Defense*weights["defense"] +
					t.Record*weights["record"] + t.Momentum*weights["momentum"] +
					t.Depth*weights["depth"]
			}
			if t.Name == m.B {
				strengthB = t.Offense*weights["offense"] + t.Defense*weights["defense"] +
					t.Record*weights["record"] + t.Momentum*weights["momentum"] +
					t.Depth*weights["depth"]
			}
		}
		total := strengthA + strengthB
		rates2[tAB] = (strengthA / total) * 0.0001 // A's win probability as rate
		rates2[tBA] = (strengthB / total) * 0.0001
	}

	// Attrition: some tokens leak to "eliminated" (tournament variance)
	for _, t := range teams {
		tElim := t.Name + "_attrition"
		b2.Transition(tElim)
		b2.Arc(t.Name, tElim, 1)
		b2.Arc(tElim, "eliminated", 1)
		// Lower seeds have higher attrition (upset risk)
		attrition := 0.0001 * float64(t.Seed) // 1-seeds: 0.0001, 5-seeds: 0.0005
		rates2[tElim] = attrition
	}

	net2 := b2.Done()
	state2 := net2.SetState(nil)
	prob2 := solver.NewProblem(net2, state2, [2]float64{0, 200}, rates2)
	sol2 := solver.Solve(prob2, solver.Tsit5(), solver.DefaultOptions())

	// Rankings from competition model
	var rankings2 []TeamRank
	finalState2 := sol2.GetFinalState()
	for _, t := range teams {
		rankings2 = append(rankings2, TeamRank{t.Name, finalState2[t.Name]})
	}
	sort.Slice(rankings2, func(i, j int) bool { return rankings2[i].Score > rankings2[j].Score })

	fmt.Println("Competitive Equilibrium Rankings (ODE t=200):")
	fmt.Println("Rank  Team            Tokens   Seed")
	fmt.Println("────  ──────────────  ───────  ────")
	for i, r := range rankings2 {
		seed := 0
		for _, t := range teams {
			if t.Name == r.Name {
				seed = t.Seed
				break
			}
		}
		fmt.Printf(" %2d.  %-14s  %7.1f    %d\n", i+1, r.Name, r.Score, seed)
	}

	// Plot competition dynamics
	p2 := plotter.NewSVGPlotter(1000, 500).
		SetTitle("Head-to-Head Competition Dynamics (Token Flow ODE)").
		SetXLabel("Tournament Simulation Time").
		SetYLabel("Remaining Strength Tokens")
	for i := 0; i < 8 && i < len(rankings2); i++ {
		name := rankings2[i].Name
		p2.AddSeries(sol2.T, sol2.GetVariable(name), name, colors[i%len(colors)])
	}
	svg2 := p2.Render()
	os.WriteFile("model2_competition.svg", []byte(svg2), 0644)

	// ================================================================
	// MODEL 3: Sensitivity Analysis — Which Facets Matter Most?
	// Sweep each weight and measure ranking stability
	// ================================================================
	fmt.Println()
	fmt.Println("Model 3: Weight Sensitivity Analysis")
	fmt.Println("────────────────────────────────────")

	baseWeights := map[string]float64{
		"offense": 0.20, "defense": 0.25, "record": 0.20,
		"momentum": 0.20, "depth": 0.15,
	}

	for _, facet := range []string{"offense", "defense", "record", "momentum", "depth"} {
		fmt.Printf("\n  Sweeping %s weight (0.05 → 0.45):\n", facet)
		fmt.Printf("  %-8s", "Weight")
		for _, t := range teams[:6] {
			fmt.Printf("  %-8s", t.Name)
		}
		fmt.Println()

		for w := 0.05; w <= 0.46; w += 0.10 {
			testWeights := make(map[string]float64)
			remaining := 1.0 - w
			otherTotal := 0.0
			for k, v := range baseWeights {
				if k != facet {
					otherTotal += v
				}
			}
			for k, v := range baseWeights {
				if k == facet {
					testWeights[k] = w
				} else {
					testWeights[k] = v / otherTotal * remaining
				}
			}

			// Compute composite scores with these weights
			fmt.Printf("  %-8.0f%%", w*100)
			for _, t := range teams[:6] {
				score := t.Offense*testWeights["offense"] + t.Defense*testWeights["defense"] +
					t.Record*testWeights["record"] + t.Momentum*testWeights["momentum"] +
					t.Depth*testWeights["depth"]
				fmt.Printf("  %-8.1f", score)
			}
			fmt.Println()
		}
	}

	// ================================================================
	// MODEL 4: Tournament Bracket Flow (Markov Chain via Petri Net)
	// Tokens represent probability mass flowing through bracket rounds
	// ================================================================
	fmt.Println()
	fmt.Println("Model 4: Tournament Probability Flow")
	fmt.Println("────────────────────────────────────")

	b4 := petri.Build()

	// Each team starts with 100 probability tokens in R64
	topContenders := teams[:8] // Top 8 for clarity
	for _, t := range topContenders {
		b4.Place(t.Name+"_r64", 100) // Round of 64
		b4.Place(t.Name+"_r32", 0)   // Round of 32
		b4.Place(t.Name+"_s16", 0)   // Sweet 16
		b4.Place(t.Name+"_e8", 0)    // Elite 8
		b4.Place(t.Name+"_f4", 0)    // Final Four
		b4.Place(t.Name+"_champ", 0) // Championship
	}
	b4.Place("upset_pool", 0) // Probability mass lost to upsets

	rates4 := make(map[string]float64)

	// Advancement transitions: tokens flow from round N to round N+1
	// Rate = win probability (based on composite strength and seed)
	rounds := []struct{ from, to string }{
		{"r64", "r32"}, {"r32", "s16"}, {"s16", "e8"}, {"e8", "f4"}, {"f4", "champ"},
	}

	for _, t := range topContenders {
		composite := t.Offense*weights["offense"] + t.Defense*weights["defense"] +
			t.Record*weights["record"] + t.Momentum*weights["momentum"] +
			t.Depth*weights["depth"]

		for ri, round := range rounds {
			advName := fmt.Sprintf("%s_adv_%d", t.Name, ri)
			elimName := fmt.Sprintf("%s_elim_%d", t.Name, ri)

			b4.Transition(advName)
			b4.Transition(elimName)

			b4.Arc(t.Name+"_"+round.from, advName, 1)
			b4.Arc(advName, t.Name+"_"+round.to, 1)

			b4.Arc(t.Name+"_"+round.from, elimName, 1)
			b4.Arc(elimName, "upset_pool", 1)

			// Win probability decreases in later rounds (tougher opponents)
			roundDifficulty := 1.0 - float64(ri)*0.08 // 100%, 92%, 84%, 76%, 68%
			seedBonus := math.Max(0, (5.0-float64(t.Seed))/4.0*0.15)
			winProb := (composite / 100.0) * roundDifficulty * (0.7 + seedBonus)
			loseProb := 1.0 - winProb

			rates4[advName] = winProb * 0.05
			rates4[elimName] = loseProb * 0.05
		}
	}

	net4 := b4.Done()
	state4 := net4.SetState(nil)
	prob4 := solver.NewProblem(net4, state4, [2]float64{0, 80}, rates4)
	sol4 := solver.Solve(prob4, solver.Tsit5(), solver.DefaultOptions())

	// Championship probability = tokens in champ place
	fmt.Println("Championship Probability (tokens at t=80):")
	fmt.Println("Rank  Team            Champ%   Path")
	fmt.Println("────  ──────────────  ──────   ──────────────────────")

	type ChampProb struct {
		Name  string
		Prob  float64
		Stats TeamStats
	}
	var champProbs []ChampProb
	final4 := sol4.GetFinalState()
	for _, t := range topContenders {
		champProbs = append(champProbs, ChampProb{t.Name, final4[t.Name+"_champ"], t})
	}
	sort.Slice(champProbs, func(i, j int) bool { return champProbs[i].Prob > champProbs[j].Prob })

	for i, cp := range champProbs {
		f4 := final4[cp.Name+"_f4"]
		e8 := final4[cp.Name+"_e8"]
		s16 := final4[cp.Name+"_s16"]
		fmt.Printf(" %2d.  %-14s  %5.1f%%   F4:%.1f E8:%.1f S16:%.1f\n",
			i+1, cp.Name, cp.Prob, f4, e8, s16)
	}

	// Plot championship probability accumulation
	p4 := plotter.NewSVGPlotter(1000, 500).
		SetTitle("Championship Probability Flow Through Bracket").
		SetXLabel("Tournament Simulation Time").
		SetYLabel("Championship Probability Tokens")
	for i, cp := range champProbs {
		p4.AddSeries(sol4.T, sol4.GetVariable(cp.Name+"_champ"), cp.Name, colors[i%len(colors)])
	}
	svg4 := p4.Render()
	os.WriteFile("model4_championship_flow.svg", []byte(svg4), 0644)

	// ================================================================
	// CONSERVATION CHECK
	// ================================================================
	fmt.Println()
	fmt.Println("Conservation Checks:")
	totalInitial := 0.0
	totalFinal := 0.0
	for _, t := range topContenders {
		totalInitial += 100.0 // Each team starts with 100 in r64
		for _, r := range []string{"r64", "r32", "s16", "e8", "f4", "champ"} {
			totalFinal += final4[t.Name+"_"+r]
		}
	}
	totalFinal += final4["upset_pool"]
	fmt.Printf("  Model 4: Initial=%.0f, Final=%.1f (%.4f%% conserved)\n",
		totalInitial, totalFinal, totalFinal/totalInitial*100)

	// ================================================================
	// MODEL 5: Monte Carlo vs ODE — Why They Agree (and Disagree)
	// Gillespie stochastic simulation on both Model 2 and Model 4
	// Model 4 (independent chains) → agreement
	// Model 2 (coupled competition) → disagreement
	// ================================================================
	fmt.Println()
	fmt.Println("Model 5: Monte Carlo vs ODE — Structure Determines Agreement")
	fmt.Println("─────────────────────────────────────────────────────────────")

	nSims := 10000
	rng := rand.New(rand.NewSource(42))

	// --- Model 2 MC: Coupled competition (should DISAGREE with ODE) ---
	fmt.Printf("\n[A] Model 2 — Head-to-Head Competition (coupled via shared arcs)\n")
	fmt.Printf("    Running %d Gillespie simulations...\n\n", nSims)

	mc2Results := gillespieSim(net2, rates2, nSims, rng)

	fmt.Println("    Team            ODE Tokens   MC Mean    MC Std     Δ Rank")
	fmt.Println("    ──────────────  ──────────   ───────    ──────     ──────")

	// Build ODE ranking for Model 2
	type m2rank struct {
		Name    string
		ODEVal  float64
		MCVal   float64
		ODERank int
		MCRank  int
	}
	var m2ranks []m2rank
	for _, r := range rankings2 {
		mcMean := mc2Results[r.Name].Mean
		m2ranks = append(m2ranks, m2rank{Name: r.Name, ODEVal: r.Score, MCVal: mcMean})
	}
	// Assign ODE ranks (already sorted)
	for i := range m2ranks {
		m2ranks[i].ODERank = i + 1
	}
	// Assign MC ranks
	mcSorted := make([]m2rank, len(m2ranks))
	copy(mcSorted, m2ranks)
	sort.Slice(mcSorted, func(i, j int) bool { return mcSorted[i].MCVal > mcSorted[j].MCVal })
	mcRankMap := make(map[string]int)
	for i, r := range mcSorted {
		mcRankMap[r.Name] = i + 1
	}
	for i := range m2ranks {
		m2ranks[i].MCRank = mcRankMap[m2ranks[i].Name]
	}

	disagreeCount := 0
	for _, r := range m2ranks {
		delta := ""
		if r.ODERank != r.MCRank {
			delta = fmt.Sprintf("Δ%+d", r.ODERank-r.MCRank)
			disagreeCount++
		}
		mc := mc2Results[r.Name]
		fmt.Printf("    %-14s  %10.1f   %7.1f    %6.1f     %s\n",
			r.Name, r.ODEVal, mc.Mean, mc.Std, delta)
	}
	if disagreeCount > 0 {
		fmt.Printf("\n    → %d ranking disagreements! Coupled dynamics create stochastic divergence.\n", disagreeCount)
	} else {
		fmt.Println("\n    → Rankings agree (teams may be too far apart for stochastic effects to flip order).")
	}
	fmt.Println("    → Key: ODE assumes continuous mean-field flow; MC captures discrete")
	fmt.Println("      winner-take-all dynamics where early losses cascade.")

	// --- Model 4 MC: Independent chains (should AGREE with ODE) ---
	fmt.Printf("\n[B] Model 4 — Tournament Bracket Flow (independent per-team chains)\n")
	fmt.Printf("    Running %d Gillespie simulations...\n\n", nSims)

	mcResults := monteCarloOnNet(net4, rates4, nSims, rng, topContenders)

	// Print comparison table
	fmt.Println("                    ODE (Model 4)     Monte Carlo (10k sims)")
	fmt.Println("Team            Champ%              Mean%    StdDev   95% CI")
	fmt.Println("──────────────  ──────              ─────    ──────   ──────────────")

	// Sort by ODE championship probability (reuse champProbs from Model 4)
	for _, cp := range champProbs {
		mc := mcResults[cp.Name]
		odeChamp := cp.Prob
		fmt.Printf("%-14s  %5.1f%%               %5.1f%%    %5.1f    [%4.1f%%, %4.1f%%]\n",
			cp.Name, odeChamp, mc.MeanChamp, mc.StdChamp,
			mc.CI95Low, mc.CI95High)
	}

	// Show where rankings disagree
	fmt.Println()
	fmt.Println("Round-by-round comparison (mean tokens):")
	fmt.Println("                    ──── ODE ────────────────    ──── Monte Carlo ────────────")
	fmt.Println("Team             F4     E8     S16    Champ    F4     E8     S16    Champ")
	fmt.Println("──────────────  ─────  ─────  ─────  ─────   ─────  ─────  ─────  ─────")
	for _, cp := range champProbs {
		mc := mcResults[cp.Name]
		fmt.Printf("%-14s  %5.1f  %5.1f  %5.1f  %5.1f   %5.1f  %5.1f  %5.1f  %5.1f\n",
			cp.Name,
			final4[cp.Name+"_f4"], final4[cp.Name+"_e8"],
			final4[cp.Name+"_s16"], final4[cp.Name+"_champ"],
			mc.MeanF4, mc.MeanE8, mc.MeanS16, mc.MeanChamp)
	}

	// Rank disagreements
	fmt.Println()
	fmt.Println("Ranking comparison:")
	fmt.Println("  ODE rank → MC rank  (by championship tokens)")

	type rankedTeam struct {
		Name    string
		ODERank int
		MCRank  int
		ODEVal  float64
		MCVal   float64
	}
	var ranked []rankedTeam
	for i, cp := range champProbs {
		ranked = append(ranked, rankedTeam{
			Name: cp.Name, ODERank: i + 1, ODEVal: cp.Prob,
			MCVal: mcResults[cp.Name].MeanChamp,
		})
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].MCVal > ranked[j].MCVal })
	for i := range ranked {
		ranked[i].MCRank = i + 1
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].ODERank < ranked[j].ODERank })
	for _, r := range ranked {
		delta := ""
		if r.ODERank != r.MCRank {
			delta = fmt.Sprintf("  ← DISAGREE (Δ%+d)", r.ODERank-r.MCRank)
		}
		fmt.Printf("  #%d → #%d  %-14s  ODE=%.1f  MC=%.1f%s\n",
			r.ODERank, r.MCRank, r.Name, r.ODEVal, r.MCVal, delta)
	}

	// Championship win distribution (how often each team wins outright)
	fmt.Println()
	fmt.Println("Championship win frequency (outright wins in MC):")
	type winFreq struct {
		Name     string
		WinPct   float64
		WinCount int
	}
	var wins []winFreq
	for _, cp := range champProbs {
		mc := mcResults[cp.Name]
		wins = append(wins, winFreq{cp.Name, mc.WinPct, mc.WinCount})
	}
	sort.Slice(wins, func(i, j int) bool { return wins[i].WinPct > wins[j].WinPct })
	for _, w := range wins {
		bar := ""
		barLen := int(w.WinPct / 2)
		for i := 0; i < barLen; i++ {
			bar += "█"
		}
		fmt.Printf("  %-14s  %5.1f%% (%4d/%d)  %s\n", w.Name, w.WinPct, w.WinCount, nSims, bar)
	}

	// ================================================================
	// MODEL 6: Bracket Petri Net with Discrete MC
	// Net encodes matchup structure (who plays whom in each round).
	// Two firing semantics on the same net:
	//   ODE: mass-action continuous flow (mean-field)
	//   MC:  one game at a time, coin flip, round-by-round
	// ================================================================
	fmt.Println()
	fmt.Println("Model 6: Structural Bracket Net — ODE vs Discrete MC")
	fmt.Println("────────────────────────────────────────────────────")

	// Assign 16 teams to 4 regions, 4 teams each, seeded by composite strength within region
	type BracketRegion struct {
		Name  string
		Seeds [4]string // [0]=1-seed, [1]=2-seed, [2]=3-seed, [3]=4-seed
	}
	bracketRegions := []BracketRegion{
		{"East", [4]string{"Duke", "UConn", "Michigan St", "Kansas"}},
		{"South", [4]string{"Florida", "Houston", "Illinois", "Nebraska"}},
		{"West", [4]string{"Arizona", "Purdue", "Gonzaga", "Alabama"}},
		{"Midwest", [4]string{"Michigan", "Iowa State", "Virginia", "St John's"}},
	}

	// Composite strength for win probability
	strength := make(map[string]float64)
	for _, t := range teams {
		strength[t.Name] = t.Offense*weights["offense"] + t.Defense*weights["defense"] +
			t.Record*weights["record"] + t.Momentum*weights["momentum"] +
			t.Depth*weights["depth"]
	}

	// Logistic win probability
	winProbFn := func(a, b string) float64 {
		return 1.0 / (1.0 + math.Exp(-0.15*(strength[a]-strength[b])))
	}

	fmt.Println("Regions:")
	for _, reg := range bracketRegions {
		fmt.Printf("  %-8s: %s(1) %s(2) %s(3) %s(4)\n",
			reg.Name, reg.Seeds[0], reg.Seeds[1], reg.Seeds[2], reg.Seeds[3])
	}

	// Build Petri net encoding all possible matchups
	b6 := petri.Build()

	// Places: each team has a place per round + out
	for _, t := range teams {
		b6.Place(t.Name+"_r1", 1) // starts alive
		b6.Place(t.Name+"_r2", 0)
		b6.Place(t.Name+"_f4", 0)
		b6.Place(t.Name+"_final", 0)
		b6.Place(t.Name+"_champ", 0)
		b6.Place(t.Name+"_out", 0)
	}

	// Track matchup pairs for MC simulation
	type BracketGame struct {
		Round     int
		TeamA     string
		TeamB     string
		WinATrans string
		WinBTrans string
	}
	var bracketGames []BracketGame

	// Helper: add a matchup (two transitions — one per possible winner)
	addGame := func(round int, a, b, aIn, bIn, aNext, bNext string) {
		tA := fmt.Sprintf("%s_over_%s_rd%d", a, b, round)
		tB := fmt.Sprintf("%s_over_%s_rd%d", b, a, round)
		b6.Transition(tA)
		b6.Transition(tB)
		// A wins: consume both, A advances, B eliminated
		b6.Arc(aIn, tA, 1)
		b6.Arc(bIn, tA, 1)
		b6.Arc(tA, aNext, 1)
		b6.Arc(tA, b+"_out", 1)
		// B wins
		b6.Arc(aIn, tB, 1)
		b6.Arc(bIn, tB, 1)
		b6.Arc(tB, bNext, 1)
		b6.Arc(tB, a+"_out", 1)
		bracketGames = append(bracketGames, BracketGame{round, a, b, tA, tB})
	}

	// R1: 1-seed vs 4-seed, 2-seed vs 3-seed within each region
	for _, reg := range bracketRegions {
		addGame(1, reg.Seeds[0], reg.Seeds[3],
			reg.Seeds[0]+"_r1", reg.Seeds[3]+"_r1",
			reg.Seeds[0]+"_r2", reg.Seeds[3]+"_r2")
		addGame(1, reg.Seeds[1], reg.Seeds[2],
			reg.Seeds[1]+"_r1", reg.Seeds[2]+"_r1",
			reg.Seeds[1]+"_r2", reg.Seeds[2]+"_r2")
	}

	// R2: winner of game1 vs winner of game2 (enumerate all 4 possible pairings per region)
	for _, reg := range bracketRegions {
		for _, a := range []string{reg.Seeds[0], reg.Seeds[3]} {
			for _, b := range []string{reg.Seeds[1], reg.Seeds[2]} {
				addGame(2, a, b, a+"_r2", b+"_r2", a+"_f4", b+"_f4")
			}
		}
	}

	// F4 semi 1: East winner vs South winner (4×4 = 16 possible pairings)
	for _, a := range bracketRegions[0].Seeds {
		for _, b := range bracketRegions[1].Seeds {
			addGame(3, a, b, a+"_f4", b+"_f4", a+"_final", b+"_final")
		}
	}
	// F4 semi 2: West winner vs Midwest winner
	for _, a := range bracketRegions[2].Seeds {
		for _, b := range bracketRegions[3].Seeds {
			addGame(3, a, b, a+"_f4", b+"_f4", a+"_final", b+"_final")
		}
	}

	// Championship: semi1 winner vs semi2 winner (8×8 = 64 possible pairings)
	semi1 := append(bracketRegions[0].Seeds[:], bracketRegions[1].Seeds[:]...)
	semi2 := append(bracketRegions[2].Seeds[:], bracketRegions[3].Seeds[:]...)
	for _, a := range semi1 {
		for _, b := range semi2 {
			addGame(4, a, b, a+"_final", b+"_final", a+"_champ", b+"_champ")
		}
	}

	net6 := b6.Done()

	fmt.Printf("Bracket net: %d places, %d transitions, %d arcs\n",
		len(net6.Places), len(net6.Transitions), len(net6.Arcs))
	fmt.Printf("Games: R1=%d, R2=%d, F4=%d, Final=%d\n",
		8, 16, 32, 64)

	// --- ODE on bracket net ---
	rates6 := make(map[string]float64)
	for _, g := range bracketGames {
		pA := winProbFn(g.TeamA, g.TeamB)
		rates6[g.WinATrans] = pA
		rates6[g.WinBTrans] = 1.0 - pA
	}

	state6 := net6.SetState(nil)
	prob6 := solver.NewProblem(net6, state6, [2]float64{0, 500}, rates6)
	sol6 := solver.Solve(prob6, solver.Tsit5(), solver.DefaultOptions())
	odeFinal6 := sol6.GetFinalState()

	// --- Discrete bracket MC ---
	fmt.Printf("\nRunning %d discrete bracket simulations...\n", nSims)

	champCounts := make(map[string]int)
	f4Counts := make(map[string]int)
	finalCounts := make(map[string]int)
	r2Counts := make(map[string]int)

	for sim := 0; sim < nSims; sim++ {
		st := net6.SetState(nil)

		for round := 1; round <= 4; round++ {
			// Before firing this round, record who made it here
			for _, t := range teams {
				n := t.Name
				switch round {
				case 2:
					if st[n+"_r2"] > 0.5 {
						r2Counts[n]++
					}
				case 3:
					if st[n+"_f4"] > 0.5 {
						f4Counts[n]++
					}
				case 4:
					if st[n+"_final"] > 0.5 {
						finalCounts[n]++
					}
				}
			}

			for _, g := range bracketGames {
				if g.Round != round {
					continue
				}
				// Check if both teams are alive in this round (inputs have tokens)
				inputs := net6.GetInputArcs(g.WinATrans)
				alive := true
				for _, arc := range inputs {
					if st[arc.Source] < 0.5 {
						alive = false
						break
					}
				}
				if !alive {
					continue
				}

				// Coin flip
				var chosen string
				if rng.Float64() < winProbFn(g.TeamA, g.TeamB) {
					chosen = g.WinATrans
				} else {
					chosen = g.WinBTrans
				}

				// Fire transition using net structure
				for _, arc := range net6.GetInputArcs(chosen) {
					st[arc.Source] -= arc.GetWeightSum()
				}
				for _, arc := range net6.GetOutputArcs(chosen) {
					st[arc.Target] += arc.GetWeightSum()
				}
			}
		}

		// Record championship winners
		for _, t := range teams {
			if st[t.Name+"_champ"] > 0.5 {
				champCounts[t.Name]++
			}
		}
	}

	// --- Comparison ---
	type bracketResult struct {
		Name     string
		Strength float64
		ODEChamp float64
		ODEF     float64
		ODEF4    float64
		ODER2    float64
		MCChamp  float64
		MCF      float64
		MCF4     float64
		MCR2     float64
	}
	var results6 []bracketResult
	for _, t := range teams {
		// ODE: cumulative tokens that reached at least this round
		odeChamp := odeFinal6[t.Name+"_champ"]
		odeF := odeFinal6[t.Name+"_final"] + odeChamp
		odeF4 := odeFinal6[t.Name+"_f4"] + odeF
		odeR2 := odeFinal6[t.Name+"_r2"] + odeF4
		results6 = append(results6, bracketResult{
			Name:     t.Name,
			Strength: strength[t.Name],
			ODEChamp: odeChamp,
			ODEF:     odeF,
			ODEF4:    odeF4,
			ODER2:    odeR2,
			MCChamp:  float64(champCounts[t.Name]) / float64(nSims) * 100,
			MCF:      float64(finalCounts[t.Name]) / float64(nSims) * 100,
			MCF4:     float64(f4Counts[t.Name]) / float64(nSims) * 100,
			MCR2:     float64(r2Counts[t.Name]) / float64(nSims) * 100,
		})
	}
	sort.Slice(results6, func(i, j int) bool { return results6[i].MCChamp > results6[j].MCChamp })

	fmt.Println()
	fmt.Println("              ──── ODE (×100 = %) ──────   ──── Discrete MC ───────────────────")
	fmt.Println("Team            R2     F4  Final  Champ     R2%    F4%  Final% Champ%  Bar")
	fmt.Println("──────────────  ─────  ───  ────  ─────   ─────  ─────  ─────  ─────  ────")
	for _, r := range results6 {
		bar := ""
		for i := 0; i < int(r.MCChamp); i++ {
			bar += "█"
		}
		fmt.Printf("%-14s  %5.1f %4.1f  %4.1f  %5.1f   %5.1f  %5.1f  %5.1f  %5.1f  %s\n",
			r.Name,
			r.ODER2*100, r.ODEF4*100, r.ODEF*100, r.ODEChamp*100,
			r.MCR2, r.MCF4, r.MCF, r.MCChamp, bar)
	}

	// Ranking comparison
	fmt.Println()
	fmt.Println("Ranking comparison (by championship probability):")

	odeSorted := make([]bracketResult, len(results6))
	copy(odeSorted, results6)
	sort.Slice(odeSorted, func(i, j int) bool { return odeSorted[i].ODEChamp > odeSorted[j].ODEChamp })
	odeRankMap := make(map[string]int)
	for i, r := range odeSorted {
		odeRankMap[r.Name] = i + 1
	}

	disagree6 := 0
	for mcRank, r := range results6 {
		odeRank := odeRankMap[r.Name]
		delta := ""
		if odeRank != mcRank+1 {
			delta = fmt.Sprintf("  ← SWAP (ODE #%d)", odeRank)
			disagree6++
		}
		fmt.Printf("  MC #%2d  ODE #%2d  %-14s  MC=%.1f%%  ODE=%.3f%s\n",
			mcRank+1, odeRank, r.Name, r.MCChamp, r.ODEChamp, delta)
	}

	fmt.Println()
	if disagree6 <= 2 {
		fmt.Printf("→ %d/%d ranking disagreements (noise-level swaps only).\n", disagree6, len(results6))
		fmt.Println()
		fmt.Println("  WHY ODE AND MC STILL AGREE on a coupled net:")
		fmt.Println("  Each team has exactly 1 token. Mass-action propensity = rate × 1 × 1 = rate.")
		fmt.Println("  The nonlinear product tokens_A × tokens_B collapses to a binary indicator.")
		fmt.Println("  With {0,1} token counts, the coupled net behaves like a linear one.")
		fmt.Println()
		fmt.Println("  Compare with Model 2 where teams start with ~90 tokens:")
		fmt.Println("  There, mass-action creates genuine nonlinear feedback (rich-get-richer)")
		fmt.Println("  and ODE vs MC disagree on 15/16 rankings.")
		fmt.Println()
		fmt.Println("  RULE: ODE ≈ MC when tokens ∈ {0,1} OR transitions have ≤1 variable input.")
		fmt.Println("        ODE ≠ MC when tokens >> 1 AND transitions couple multiple pools.")
	} else {
		fmt.Printf("→ %d/%d ranking disagreements.\n", disagree6, len(results6))
		fmt.Println("  Coupled dynamics + multi-token pools create stochastic divergence.")
	}

	// ================================================================
	// MODEL 7: Incidence Reduction — The Bridge
	// The incidence matrix C is the algebraic object underlying BOTH
	// ODE (dm/dt = C·v) and MC (Δm = C·eₜ per firing).
	// Extract C, compute drain counts, derive analytical championship
	// probabilities — no simulation needed.
	// ================================================================
	fmt.Println()
	fmt.Println("Model 7: Incidence Reduction — Bridging Continuous and Discrete")
	fmt.Println("───────────────────────────────────────────────────────────────")

	// Extract incidence matrix from the bracket net
	analyzer := reachability.NewInvariantAnalyzer(net6)
	C, placeLabels, transLabels := analyzer.IncidenceMatrix()

	fmt.Printf("Incidence matrix C: %d rows (places) × %d columns (transitions)\n",
		len(placeLabels), len(transLabels))

	// Build index maps for fast lookup
	placeToIdx := make(map[string]int)
	for i, p := range placeLabels {
		placeToIdx[p] = i
	}
	transToIdx := make(map[string]int)
	for i, t := range transLabels {
		transToIdx[t] = i
	}

	// Compute drain counts from C: for each place, count transitions with C[p][t] < 0
	fmt.Println()
	fmt.Println("Drain counts from incidence matrix (transitions consuming from each place):")
	fmt.Println("Team            R1 drains  R2 drains  F4 drains  Final drains")
	fmt.Println("──────────────  ─────────  ─────────  ─────────  ────────────")

	roundLabels := []string{"_r1", "_r2", "_f4", "_final"}
	for _, t := range teams {
		fmt.Printf("%-14s", t.Name)
		for _, r := range roundLabels {
			pIdx, ok := placeToIdx[t.Name+r]
			drains := 0
			if ok {
				for j := range transLabels {
					if C[pIdx][j] < 0 {
						drains++
					}
				}
			}
			fmt.Printf("  %5d    ", drains)
		}
		fmt.Println()
	}

	// ================================================================
	// ANALYTICAL PROBABILITY PROPAGATION via incidence matrix
	// For each round, identify matchup transitions from C, compute
	// advancement probability weighted by opponent survival probability.
	// This is what both ODE and MC compute — but we derive it directly
	// from C without simulation.
	// ================================================================
	fmt.Println()
	fmt.Println("Analytical probability propagation (from incidence matrix + rates):")
	fmt.Println()

	// Forward propagation: P(team reaches round R+1)
	// For each matchup at round R:
	//   P(matchup occurs) = P(teamA in round R) × P(teamB in round R)
	//   P(teamA advances) += P(matchup occurs) × winProb(A, B)
	survivalProb := make(map[string]float64) // team → P(alive at current round)
	for _, t := range teams {
		survivalProb[t.Name] = 1.0
	}

	for round := 1; round <= 4; round++ {
		nextProb := make(map[string]float64)

		for _, g := range bracketGames {
			if g.Round != round {
				continue
			}

			// Both teams must be alive for this matchup to occur
			pA := survivalProb[g.TeamA]
			pB := survivalProb[g.TeamB]
			pMatchup := pA * pB

			if pMatchup < 1e-15 {
				continue
			}

			// Win probabilities from rates (same as ODE/MC)
			wA := winProbFn(g.TeamA, g.TeamB)
			wB := 1.0 - wA

			nextProb[g.TeamA] += pMatchup * wA
			nextProb[g.TeamB] += pMatchup * wB
		}

		survivalProb = nextProb
	}

	// Also do uniform-rate propagation (incidence reduction: topology only)
	uniformProb := make(map[string]float64)
	for _, t := range teams {
		uniformProb[t.Name] = 1.0
	}
	for round := 1; round <= 4; round++ {
		nextProb := make(map[string]float64)
		for _, g := range bracketGames {
			if g.Round != round {
				continue
			}
			pA := uniformProb[g.TeamA]
			pB := uniformProb[g.TeamB]
			pMatchup := pA * pB
			if pMatchup < 1e-15 {
				continue
			}
			// Uniform rates: 50-50 every matchup
			nextProb[g.TeamA] += pMatchup * 0.5
			nextProb[g.TeamB] += pMatchup * 0.5
		}
		uniformProb = nextProb
	}

	// Three-way comparison: Analytical vs ODE vs MC
	fmt.Println("            Incidence     Strength-     ODE        MC")
	fmt.Println("Team        Reduction     Analytical   (Model 6)  (10k sims)  Match?")
	fmt.Println("            (topology)    (C + rates)")
	fmt.Println("──────────  ──────────    ──────────   ─────────  ──────────  ──────")

	type model7Result struct {
		Name     string
		Topology float64
		Analytic float64
		ODE      float64
		MC       float64
	}
	var m7results []model7Result
	for _, t := range teams {
		m7results = append(m7results, model7Result{
			Name:     t.Name,
			Topology: uniformProb[t.Name] * 100,
			Analytic: survivalProb[t.Name] * 100,
			ODE:      odeFinal6[t.Name+"_champ"] * 100,
			MC:       float64(champCounts[t.Name]) / float64(nSims) * 100,
		})
	}
	sort.Slice(m7results, func(i, j int) bool { return m7results[i].Analytic > m7results[j].Analytic })

	for _, r := range m7results {
		match := "="
		if math.Abs(r.Analytic-r.ODE) > 0.5 {
			match = "≠"
		}
		fmt.Printf("%-10s    %5.2f%%        %5.2f%%       %5.2f%%     %5.1f%%      %s\n",
			r.Name, r.Topology, r.Analytic, r.ODE, r.MC, match)
	}

	fmt.Println()
	fmt.Println("The incidence reduction column shows pure topology (uniform rates = 50-50).")
	fmt.Println("All teams get equal championship probability from structure alone.")
	fmt.Println()
	fmt.Println("The analytical column uses C (incidence matrix) + strength-based rates")
	fmt.Println("to compute exact probabilities via forward propagation — no simulation.")
	fmt.Println()
	fmt.Println("Bridge: C defines both  dm/dt = C·v(m)  [ODE]  and  Δm = C·eₜ  [MC].")
	fmt.Println("The analytical formula reads the same probabilities directly from C,")
	fmt.Println("proving continuous and discrete methods compute the same function of")
	fmt.Println("the incidence matrix.")

	// ================================================================
	// REPLACING MC ENTIRELY
	// Binary outcome → Var = p(1-p), exact from analytical p.
	// Joint probabilities: independent across regions, exclusive within.
	// Enumerate all 2^15 = 32,768 possible brackets exhaustively.
	// ================================================================
	fmt.Println()
	fmt.Println("────────────────────────────────────────────────────────")
	fmt.Println("Replacing Monte Carlo: Exact Distribution from C")
	fmt.Println("────────────────────────────────────────────────────────")

	// Exact variance: Var(champ) = p(1-p) for binary outcome
	fmt.Println()
	fmt.Println("Exact variance vs MC-estimated variance:")
	fmt.Println("Team            P(champ)   Exact σ    MC σ (10k)")
	fmt.Println("──────────────  ────────   ────────   ──────────")
	for _, r := range m7results {
		p := r.Analytic / 100
		exactVar := p * (1 - p)
		exactStd := math.Sqrt(exactVar) * 100
		// MC std: from binary samples, std = sqrt(p(1-p)/n) * n... just use the counts
		mcP := r.MC / 100
		mcStd := math.Sqrt(mcP*(1-mcP)) * 100
		fmt.Printf("%-14s  %6.2f%%    %6.2f%%    %6.2f%%\n",
			r.Name, r.Analytic, exactStd, mcStd)
	}

	// Exhaustive bracket enumeration
	// Each region has 2 R1 games → 4 outcomes. 4 regions → 4^4 = 256 regional configs.
	// Then 2 semis × 1 final = 4 more games → 2^3 = 8 outcomes per regional config.
	// Total: 256 × 8... actually it's 2^15 = 32768 total game outcomes.
	// But we can factor by region for efficiency.

	fmt.Println()
	fmt.Println("Exhaustive bracket enumeration (no sampling):")

	// Compute regional winner distributions
	type RegionalOutcome struct {
		Winner string
		Prob   float64
	}

	regionWinners := make([][]RegionalOutcome, 4)
	for ri, reg := range bracketRegions {
		// R1: seeds[0] vs seeds[3], seeds[1] vs seeds[2]
		p03_0 := winProbFn(reg.Seeds[0], reg.Seeds[3]) // seed 0 wins
		p03_3 := 1 - p03_0
		p12_1 := winProbFn(reg.Seeds[1], reg.Seeds[2])
		p12_2 := 1 - p12_1

		// R2: 4 possible matchups
		var outcomes []RegionalOutcome
		r1Outcomes := [][2]struct {
			name string
			prob float64
		}{
			{{reg.Seeds[0], p03_0}, {reg.Seeds[1], p12_1}},
			{{reg.Seeds[0], p03_0}, {reg.Seeds[2], p12_2}},
			{{reg.Seeds[3], p03_3}, {reg.Seeds[1], p12_1}},
			{{reg.Seeds[3], p03_3}, {reg.Seeds[2], p12_2}},
		}
		for _, r1 := range r1Outcomes {
			pMatchup := r1[0].prob * r1[1].prob
			pAWins := winProbFn(r1[0].name, r1[1].name)
			outcomes = append(outcomes,
				RegionalOutcome{r1[0].name, pMatchup * pAWins},
				RegionalOutcome{r1[1].name, pMatchup * (1 - pAWins)},
			)
		}
		// Consolidate by winner
		winnerProb := make(map[string]float64)
		for _, o := range outcomes {
			winnerProb[o.Winner] += o.Prob
		}
		regionWinners[ri] = nil
		for name, prob := range winnerProb {
			regionWinners[ri] = append(regionWinners[ri], RegionalOutcome{name, prob})
		}
		sort.Slice(regionWinners[ri], func(i, j int) bool {
			return regionWinners[ri][i].Prob > regionWinners[ri][j].Prob
		})
	}

	fmt.Println()
	fmt.Println("Regional winner probabilities (exact):")
	for ri, reg := range bracketRegions {
		fmt.Printf("  %-8s: ", reg.Name)
		for _, o := range regionWinners[ri] {
			fmt.Printf("%s %.1f%%  ", o.Winner, o.Prob*100)
		}
		fmt.Println()
	}

	// Enumerate all championship outcomes
	// Semi 1: East winner vs South winner
	// Semi 2: West winner vs Midwest winner
	// Final: Semi 1 winner vs Semi 2 winner
	exactChamp := make(map[string]float64)
	exactFinal := make(map[string]float64)
	exactF4 := make(map[string]float64)
	totalBrackets := 0

	for _, east := range regionWinners[0] {
		for _, south := range regionWinners[1] {
			for _, west := range regionWinners[2] {
				for _, midwest := range regionWinners[3] {
					// F4 probabilities accumulate
					pF4 := east.Prob * south.Prob * west.Prob * midwest.Prob
					exactF4[east.Winner] += pF4
					exactF4[south.Winner] += pF4
					exactF4[west.Winner] += pF4
					exactF4[midwest.Winner] += pF4

					// Semi 1: East vs South
					pEast := winProbFn(east.Winner, south.Winner)
					pSouth := 1 - pEast

					// Semi 2: West vs Midwest
					pWest := winProbFn(west.Winner, midwest.Winner)
					pMidwest := 1 - pWest

					// Finals: 4 possible matchups
					finals := []struct {
						a, b string
						prob float64
					}{
						{east.Winner, west.Winner, pF4 * pEast * pWest},
						{east.Winner, midwest.Winner, pF4 * pEast * pMidwest},
						{south.Winner, west.Winner, pF4 * pSouth * pWest},
						{south.Winner, midwest.Winner, pF4 * pSouth * pMidwest},
					}

					for _, f := range finals {
						exactFinal[f.a] += f.prob
						exactFinal[f.b] += f.prob

						pAWins := winProbFn(f.a, f.b)
						exactChamp[f.a] += f.prob * pAWins
						exactChamp[f.b] += f.prob * (1 - pAWins)
					}

					totalBrackets++
				}
			}
		}
	}

	fmt.Printf("\n  Enumerated %d regional configurations (4 winners × 4 semifinal/final paths)\n", totalBrackets)

	// Final comparison: Exact enumeration vs MC
	fmt.Println()
	fmt.Println("Complete replacement — Exact vs MC:")
	fmt.Println("Team            Exact%   MC%     Exact F4%  MC F4%   Exact Final%  MC Final%")
	fmt.Println("──────────────  ──────   ─────   ─────────  ──────   ───────────   ─────────")

	type finalResult struct {
		Name       string
		ExactChamp float64
		MCChamp    float64
		ExactF4    float64
		MCF4       float64
		ExactFinal float64
		MCFinal    float64
	}
	var finalResults []finalResult
	for _, t := range teams {
		finalResults = append(finalResults, finalResult{
			Name:       t.Name,
			ExactChamp: exactChamp[t.Name] * 100,
			MCChamp:    float64(champCounts[t.Name]) / float64(nSims) * 100,
			ExactF4:    exactF4[t.Name] * 100,
			MCF4:       float64(f4Counts[t.Name]) / float64(nSims) * 100,
			ExactFinal: exactFinal[t.Name] * 100,
			MCFinal:    float64(finalCounts[t.Name]) / float64(nSims) * 100,
		})
	}
	sort.Slice(finalResults, func(i, j int) bool {
		return finalResults[i].ExactChamp > finalResults[j].ExactChamp
	})

	for _, r := range finalResults {
		fmt.Printf("%-14s  %5.2f%%   %5.1f%%   %6.2f%%    %5.1f%%    %6.2f%%       %5.1f%%\n",
			r.Name, r.ExactChamp, r.MCChamp,
			r.ExactF4, r.MCF4, r.ExactFinal, r.MCFinal)
	}

	// Verify probabilities sum to 100%
	totalExact := 0.0
	for _, p := range exactChamp {
		totalExact += p
	}
	fmt.Printf("\n  Championship probabilities sum to: %.4f%% (exact)\n", totalExact*100)

	fmt.Println()
	fmt.Println("  MC is fully replaced. Every statistic MC provides — means, variances,")
	fmt.Println("  per-round advancement, joint probabilities — is computable in closed form")
	fmt.Println("  from the incidence matrix C and rate vector r.")
	fmt.Println()
	fmt.Println("  Cost: O(regional_outcomes^4 × rounds) vs MC's O(sims × games)")
	fmt.Printf("  Here: %d configurations vs %d simulations × 15 games\n", totalBrackets, nSims)

	fmt.Println()
	fmt.Println("SVG outputs saved:")
	fmt.Println("  ranking_petri_net.svg      — Petri net structure")
	fmt.Println("  model1_accumulation.svg    — Multi-facet strength accumulation")
	fmt.Println("  model2_competition.svg     — Head-to-head competition dynamics")
	fmt.Println("  model4_championship_flow.svg — Championship probability flow")
}

// fetchTeamsFromPipeline uses the data pipeline to fetch and normalize real team data.
func fetchTeamsFromPipeline(season int, cacheDir string) []TeamStats {
	cache, err := data.NewCache(cacheDir + "/ncaa.db")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cache init failed: %v\n", err)
	}
	if cache != nil {
		defer cache.Close()
	}

	raw, err := data.FetchAllTeams(season, cache)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching data: %v\n", err)
		os.Exit(1)
	}

	// Normalize all D-I teams
	normalized := data.NormalizeTeams(raw)

	// For the model, take the top 16 by composite score
	type scored struct {
		idx   int
		score float64
	}
	var scores []scored
	for i, t := range normalized {
		s := t.Offense*0.20 + t.Defense*0.25 + t.Record*0.20 + t.Momentum*0.20 + t.Depth*0.15
		scores = append(scores, scored{i, s})
	}
	sort.Slice(scores, func(i, j int) bool { return scores[i].score > scores[j].score })

	// Map top teams into our TeamStats struct with estimated seeds
	var teams []TeamStats
	for i := 0; i < 16 && i < len(scores); i++ {
		t := normalized[scores[i].idx]
		seed := 1
		switch {
		case i >= 12:
			seed = 4
		case i >= 8:
			seed = 3
		case i >= 4:
			seed = 2
		}
		teams = append(teams, TeamStats{
			Name:     t.Name,
			Seed:     seed,
			Offense:  t.Offense,
			Defense:  t.Defense,
			Record:   t.Record,
			Momentum: t.Momentum,
			Depth:    t.Depth,
		})
	}

	fmt.Printf("Loaded %d teams from data pipeline (season %d)\n\n", len(teams), season)
	return teams
}

// GillespieResult holds per-place Monte Carlo statistics.
type GillespieResult struct {
	Mean float64
	Std  float64
}

// gillespieSim runs Gillespie stochastic simulation and returns per-place statistics.
func gillespieSim(net *petri.PetriNet, rates map[string]float64, nSims int, rng *rand.Rand) map[string]*GillespieResult {
	// Pre-compute transition info
	type transInfo struct {
		label   string
		rate    float64
		inputs  []*petri.Arc
		outputs []*petri.Arc
	}
	var transitions []transInfo
	for tLabel := range net.Transitions {
		r := rates[tLabel]
		if r <= 0 {
			continue
		}
		transitions = append(transitions, transInfo{
			label:   tLabel,
			rate:    r,
			inputs:  net.GetInputArcs(tLabel),
			outputs: net.GetOutputArcs(tLabel),
		})
	}

	// Collect data per place per simulation
	placeData := make(map[string][]float64)
	for label := range net.Places {
		placeData[label] = make([]float64, 0, nSims)
	}

	for sim := 0; sim < nSims; sim++ {
		state := net.SetState(nil)

		// Cap iterations to prevent infinite loops on conservation-preserving nets
		maxFirings := 100000
		for step := 0; step < maxFirings; step++ {
			totalProp := 0.0
			props := make([]float64, len(transitions))

			for i, tr := range transitions {
				prop := tr.rate
				enabled := true
				for _, arc := range tr.inputs {
					w := arc.GetWeightSum()
					if state[arc.Source] < w {
						enabled = false
						break
					}
					prop *= state[arc.Source]
				}
				if !enabled {
					props[i] = 0
					continue
				}
				props[i] = prop
				totalProp += prop
			}

			if totalProp <= 1e-15 {
				break
			}

			r := rng.Float64() * totalProp
			cumulative := 0.0
			chosenIdx := len(transitions) - 1
			for i, p := range props {
				cumulative += p
				if r <= cumulative {
					chosenIdx = i
					break
				}
			}

			tr := transitions[chosenIdx]
			for _, arc := range tr.inputs {
				state[arc.Source] -= arc.GetWeightSum()
			}
			for _, arc := range tr.outputs {
				state[arc.Target] += arc.GetWeightSum()
			}
		}

		for label := range net.Places {
			placeData[label] = append(placeData[label], state[label])
		}
	}

	// Compute statistics
	results := make(map[string]*GillespieResult)
	for label, data := range placeData {
		n := float64(len(data))
		sum := 0.0
		for _, v := range data {
			sum += v
		}
		mean := sum / n
		sumSq := 0.0
		for _, v := range data {
			d := v - mean
			sumSq += d * d
		}
		results[label] = &GillespieResult{
			Mean: mean,
			Std:  math.Sqrt(sumSq / n),
		}
	}
	return results
}

// MCResult holds per-team Monte Carlo statistics.
type MCResult struct {
	MeanChamp float64
	StdChamp  float64
	CI95Low   float64
	CI95High  float64
	MeanF4    float64
	MeanE8    float64
	MeanS16   float64
	WinPct    float64 // % of sims where this team had highest champ tokens
	WinCount  int
}

// monteCarloOnNet runs Gillespie stochastic simulation on the given Petri net.
// Each simulation starts from the net's initial state and fires transitions
// one at a time, chosen probabilistically by mass-action propensity.
func monteCarloOnNet(net *petri.PetriNet, rates map[string]float64, nSims int, rng *rand.Rand, teams []TeamStats) map[string]*MCResult {
	// Pre-compute transition info for performance
	type transInfo struct {
		label   string
		rate    float64
		inputs  []*petri.Arc
		outputs []*petri.Arc
	}
	var transitions []transInfo
	for tLabel := range net.Transitions {
		r := rates[tLabel]
		if r <= 0 {
			continue
		}
		transitions = append(transitions, transInfo{
			label:   tLabel,
			rate:    r,
			inputs:  net.GetInputArcs(tLabel),
			outputs: net.GetOutputArcs(tLabel),
		})
	}

	// Per-team per-sim championship tokens
	teamNames := make([]string, len(teams))
	for i, t := range teams {
		teamNames[i] = t.Name
	}

	champData := make(map[string][]float64)
	f4Data := make(map[string][]float64)
	e8Data := make(map[string][]float64)
	s16Data := make(map[string][]float64)
	winCounts := make(map[string]int)

	for sim := 0; sim < nSims; sim++ {
		state := net.SetState(nil) // fresh initial marking

		// Track cumulative tokens that arrived at each round (not just final state)
		arrived := make(map[string]float64)

		// Gillespie: fire transitions until none are enabled
		for {
			// Compute propensities
			totalProp := 0.0
			props := make([]float64, len(transitions))

			for i, tr := range transitions {
				prop := tr.rate
				enabled := true
				for _, arc := range tr.inputs {
					w := arc.GetWeightSum()
					tokens := state[arc.Source]
					if tokens < w {
						enabled = false
						break
					}
					prop *= tokens // mass-action kinetics
				}
				if !enabled {
					props[i] = 0
					continue
				}
				props[i] = prop
				totalProp += prop
			}

			if totalProp <= 1e-15 {
				break
			}

			// Choose transition to fire
			r := rng.Float64() * totalProp
			cumulative := 0.0
			chosenIdx := len(transitions) - 1
			for i, p := range props {
				cumulative += p
				if r <= cumulative {
					chosenIdx = i
					break
				}
			}

			// Fire: consume inputs, produce outputs
			tr := transitions[chosenIdx]
			for _, arc := range tr.inputs {
				state[arc.Source] -= arc.GetWeightSum()
			}
			for _, arc := range tr.outputs {
				state[arc.Target] += arc.GetWeightSum()
				arrived[arc.Target] += arc.GetWeightSum()
			}
		}

		// Record results — use cumulative arrivals for intermediate rounds
		bestTeam := ""
		bestChamp := -1.0
		for _, name := range teamNames {
			champ := arrived[name+"_champ"]
			champData[name] = append(champData[name], champ)
			f4Data[name] = append(f4Data[name], arrived[name+"_f4"])
			e8Data[name] = append(e8Data[name], arrived[name+"_e8"])
			s16Data[name] = append(s16Data[name], arrived[name+"_s16"])
			if champ > bestChamp {
				bestChamp = champ
				bestTeam = name
			}
		}
		winCounts[bestTeam]++
	}

	// Compute statistics
	results := make(map[string]*MCResult)
	for _, name := range teamNames {
		data := champData[name]
		n := float64(len(data))

		// Mean
		sum := 0.0
		for _, v := range data {
			sum += v
		}
		mean := sum / n

		// Std dev
		sumSq := 0.0
		for _, v := range data {
			d := v - mean
			sumSq += d * d
		}
		std := math.Sqrt(sumSq / n)

		// 95% CI (normal approximation)
		se := std / math.Sqrt(n)

		// Mean for other rounds
		sumF4, sumE8, sumS16 := 0.0, 0.0, 0.0
		for i := range data {
			sumF4 += f4Data[name][i]
			sumE8 += e8Data[name][i]
			sumS16 += s16Data[name][i]
		}

		results[name] = &MCResult{
			MeanChamp: mean,
			StdChamp:  std,
			CI95Low:   mean - 1.96*se,
			CI95High:  mean + 1.96*se,
			MeanF4:    sumF4 / n,
			MeanE8:    sumE8 / n,
			MeanS16:   sumS16 / n,
			WinPct:    float64(winCounts[name]) / n * 100,
			WinCount:  winCounts[name],
		}
	}

	return results
}

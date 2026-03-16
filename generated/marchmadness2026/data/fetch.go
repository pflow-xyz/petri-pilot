package data

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	ncaaAPIBase    = "https://ncaa-api.henrygd.me"
	barttovikURL   = "https://barttorvik.com/trank.php?year=%d&css=1&conlimit=#"
	sportsRefBase  = "https://www.sports-reference.com/cbb/schools"
	maxReqPerSec   = 5
)

var (
	rateLimiter = time.NewTicker(time.Second / maxReqPerSec)
	httpClient  = &http.Client{Timeout: 30 * time.Second}
)

// RawTeamData holds data collected from all sources before normalization.
type RawTeamData struct {
	Name       string
	Conference string

	// From NCAA API
	Wins       int
	Losses     int
	PPG        float64 // Points per game
	OppPPG     float64 // Opponent points per game
	FGPct      float64 // Field goal percentage
	ConfWins   int
	ConfLosses int

	// From Barttorvik
	AdjOE    float64 // Adjusted offensive efficiency
	AdjDE    float64 // Adjusted defensive efficiency
	Barthag  float64 // Win probability vs average D-I team
	SOS      float64 // Strength of schedule
	WAB      float64 // Wins above bubble
	Tempo    float64

	// From game logs (momentum)
	Last10Wins     int
	Last10Losses   int
	ConfTourneyWin bool // Won conference tournament
	ConfTourneyRU  bool // Conference tournament runner-up
	RecentGames    []GameResult

	// From Sports-Reference (depth)
	MinutesDistribution []float64 // Minutes share per player
	BenchPPG            float64
}

// GameResult holds a single game outcome for momentum calculation.
type GameResult struct {
	Date     time.Time
	Opponent string
	Win      bool
	Score    int
	OppScore int
}

// FetchAllTeams fetches data from all sources and returns raw team data.
// If cache is non-nil, it will check/store cached data.
func FetchAllTeams(season int, cache *Cache) ([]RawTeamData, error) {
	// Try cache first
	if cache != nil {
		cached, err := cache.LoadTeams(season)
		if err == nil && len(cached) > 0 {
			fmt.Printf("Loaded %d teams from cache\n", len(cached))
			return cached, nil
		}
	}

	// Fetch Barttorvik first — it has the most data in one page
	fmt.Println("Fetching Barttorvik advanced metrics...")
	teams, err := fetchBarttorvik(season)
	if err != nil {
		return nil, fmt.Errorf("barttorvik fetch: %w", err)
	}
	fmt.Printf("  Got %d teams from Barttorvik\n", len(teams))

	// Fetch NCAA API standings to fill in W-L records
	fmt.Println("Fetching NCAA API standings...")
	if err := fetchNCAAStandings(teams); err != nil {
		fmt.Printf("  Warning: NCAA standings fetch failed: %v\n", err)
		// Non-fatal — Barttorvik has W-L data too
	}

	// Fetch game logs for momentum (last 10 games)
	fmt.Println("Fetching game logs for momentum...")
	if err := fetchGameLogs(teams, season); err != nil {
		fmt.Printf("  Warning: game logs fetch failed: %v\n", err)
	}

	// Cache results
	if cache != nil {
		if err := cache.SaveTeams(season, teams); err != nil {
			fmt.Printf("  Warning: cache save failed: %v\n", err)
		}
	}

	return teams, nil
}

// fetchBarttorvik scrapes the team rankings page for advanced metrics.
func fetchBarttorvik(season int) ([]RawTeamData, error) {
	<-rateLimiter.C
	url := fmt.Sprintf(barttovikURL, season)
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("barttorvik returned status %d", resp.StatusCode)
	}

	return parseBarttovikHTML(resp.Body)
}

// parseBarttovikHTML parses the Barttorvik team rankings HTML table.
func parseBarttovikHTML(r io.Reader) ([]RawTeamData, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}

	var teams []RawTeamData
	var rows [][]string

	// Find the main data table and extract rows
	var walkTable func(*html.Node)
	walkTable = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tr" {
			var cells []string
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
					cells = append(cells, extractText(c))
				}
			}
			if len(cells) > 0 {
				rows = append(rows, cells)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walkTable(c)
		}
	}
	walkTable(doc)

	// Barttorvik table columns (typical order):
	// Rank, Team, Conf, Record, AdjOE, AdjDE, Barthag, ...
	// Column indices may vary — find them by header
	var colTeam, colConf, colRec, colAdjOE, colAdjDE, colBarthag, colSOS, colWAB int
	colTeam, colConf, colRec = 1, 2, 3
	colAdjOE, colAdjDE, colBarthag = 4, 5, 6
	colSOS = -1
	colWAB = -1

	// Try to find header row to get exact column positions
	if len(rows) > 0 {
		for i, cell := range rows[0] {
			cell = strings.TrimSpace(strings.ToLower(cell))
			switch {
			case cell == "team":
				colTeam = i
			case cell == "conf":
				colConf = i
			case cell == "rec" || cell == "record":
				colRec = i
			case cell == "adjoe" || cell == "adj oe":
				colAdjOE = i
			case cell == "adjde" || cell == "adj de":
				colAdjDE = i
			case cell == "barthag":
				colBarthag = i
			case cell == "sos" || cell == "adj sos":
				colSOS = i
			case cell == "wab":
				colWAB = i
			}
		}
	}

	// Parse data rows (skip header)
	for _, row := range rows[1:] {
		if len(row) <= colBarthag {
			continue
		}

		name := strings.TrimSpace(row[colTeam])
		if name == "" || name == "Team" {
			continue
		}

		t := RawTeamData{
			Name:       normalizeTeamName(name),
			Conference: strings.TrimSpace(row[colConf]),
		}

		// Parse record "W-L"
		if colRec < len(row) {
			fmt.Sscanf(row[colRec], "%d-%d", &t.Wins, &t.Losses)
		}

		t.AdjOE = parseFloat(row[colAdjOE])
		t.AdjDE = parseFloat(row[colAdjDE])
		t.Barthag = parseFloat(row[colBarthag])

		if colSOS >= 0 && colSOS < len(row) {
			t.SOS = parseFloat(row[colSOS])
		}
		if colWAB >= 0 && colWAB < len(row) {
			t.WAB = parseFloat(row[colWAB])
		}

		teams = append(teams, t)
	}

	return teams, nil
}

// fetchNCAAStandings fetches W-L records from the NCAA API.
func fetchNCAAStandings(teams []RawTeamData) error {
	<-rateLimiter.C
	resp, err := httpClient.Get(ncaaAPIBase + "/standings/basketball-men/d1")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("NCAA API returned status %d", resp.StatusCode)
	}

	var result json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	// The NCAA API response structure varies — we'll parse what we can.
	// Build a name→index map for quick lookup.
	nameIdx := make(map[string]int)
	for i := range teams {
		nameIdx[strings.ToLower(teams[i].Name)] = i
	}

	// Try to extract standings data
	var standings struct {
		Divisions []struct {
			Standings []struct {
				Team struct {
					ShortName string `json:"shortName"`
					Name      string `json:"name"`
				} `json:"team"`
				OverallRecord struct {
					Wins   int `json:"wins"`
					Losses int `json:"losses"`
				} `json:"overallRecord"`
				ConferenceRecord struct {
					Wins   int `json:"wins"`
					Losses int `json:"losses"`
				} `json:"conferenceRecord"`
			} `json:"standings"`
		} `json:"divisions"`
	}

	if err := json.Unmarshal(result, &standings); err != nil {
		// Try alternative format
		return fmt.Errorf("could not parse NCAA standings: %w", err)
	}

	for _, div := range standings.Divisions {
		for _, s := range div.Standings {
			name := normalizeTeamName(s.Team.ShortName)
			if name == "" {
				name = normalizeTeamName(s.Team.Name)
			}
			if idx, ok := nameIdx[strings.ToLower(name)]; ok {
				teams[idx].Wins = s.OverallRecord.Wins
				teams[idx].Losses = s.OverallRecord.Losses
				teams[idx].ConfWins = s.ConferenceRecord.Wins
				teams[idx].ConfLosses = s.ConferenceRecord.Losses
			}
		}
	}

	return nil
}

// fetchGameLogs fetches recent game results for momentum calculation.
func fetchGameLogs(teams []RawTeamData, season int) error {
	// For now, use Barttorvik data to estimate momentum
	// since game-by-game NCAA API scraping is slow for 363 teams.
	// TODO: enhance with targeted fetches for tournament teams.
	_ = season
	return nil
}

// FetchDepthData fetches minute distribution from Sports-Reference for specific teams.
func FetchDepthData(teams []RawTeamData, teamNames []string, season int) error {
	nameIdx := make(map[string]int)
	for i := range teams {
		nameIdx[strings.ToLower(teams[i].Name)] = i
	}

	for _, name := range teamNames {
		idx, ok := nameIdx[strings.ToLower(name)]
		if !ok {
			continue
		}

		slug := teamNameToSportsRefSlug(name)
		url := fmt.Sprintf("%s/%s/%d.html", sportsRefBase, slug, season)

		<-rateLimiter.C
		resp, err := httpClient.Get(url)
		if err != nil {
			fmt.Printf("  Warning: could not fetch depth for %s: %v\n", name, err)
			continue
		}

		if resp.StatusCode != 200 {
			resp.Body.Close()
			continue
		}

		minutes, benchPPG, err := parseSportsRefMinutes(resp.Body)
		resp.Body.Close()
		if err != nil {
			fmt.Printf("  Warning: could not parse depth for %s: %v\n", name, err)
			continue
		}

		teams[idx].MinutesDistribution = minutes
		teams[idx].BenchPPG = benchPPG
	}

	return nil
}

// parseSportsRefMinutes extracts minutes distribution from Sports-Reference HTML.
func parseSportsRefMinutes(r io.Reader) ([]float64, float64, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, 0, err
	}

	// Find the per-game stats table and extract MP (minutes played) column
	var minutes []float64
	var totalPoints, benchPoints float64
	var totalMinutes float64

	var walkNode func(*html.Node)
	walkNode = func(n *html.Node) {
		// Look for the totals table with per-game averages
		if n.Type == html.ElementNode && n.Data == "table" {
			for _, attr := range n.Attr {
				if attr.Key == "id" && attr.Val == "per_game" {
					parsePerGameTable(n, &minutes, &totalMinutes, &totalPoints, &benchPoints)
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walkNode(c)
		}
	}
	walkNode(doc)

	if len(minutes) == 0 {
		return nil, 0, fmt.Errorf("no minutes data found")
	}

	// Convert to shares
	shares := make([]float64, len(minutes))
	for i, m := range minutes {
		if totalMinutes > 0 {
			shares[i] = m / totalMinutes
		}
	}

	return shares, benchPoints, nil
}

func parsePerGameTable(table *html.Node, minutes *[]float64, totalMin, totalPts, benchPts *float64) {
	var rows [][]string
	var walkRows func(*html.Node)
	walkRows = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tr" {
			var cells []string
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
					cells = append(cells, extractText(c))
				}
			}
			if len(cells) > 0 {
				rows = append(rows, cells)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walkRows(c)
		}
	}
	walkRows(table)

	// Find MP and PTS column indices from header
	if len(rows) == 0 {
		return
	}

	mpCol, ptsCol := -1, -1
	for i, cell := range rows[0] {
		switch strings.TrimSpace(strings.ToLower(cell)) {
		case "mp":
			mpCol = i
		case "pts":
			ptsCol = i
		}
	}

	if mpCol < 0 {
		return
	}

	// Players with >0 minutes, sorted by MP descending (top 5 = starters, rest = bench)
	type playerMin struct {
		mp  float64
		pts float64
	}
	var players []playerMin

	for _, row := range rows[1:] {
		if mpCol >= len(row) {
			continue
		}
		mp := parseFloat(row[mpCol])
		if mp <= 0 {
			continue
		}
		var pts float64
		if ptsCol >= 0 && ptsCol < len(row) {
			pts = parseFloat(row[ptsCol])
		}
		players = append(players, playerMin{mp, pts})
		*totalMin += mp
		*totalPts += pts
	}

	for i, p := range players {
		*minutes = append(*minutes, p.mp)
		if i >= 5 { // Bench players (after top 5 by minutes)
			*benchPts += p.pts
		}
	}
}

// extractText recursively extracts text content from an HTML node.
func extractText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(extractText(c))
	}
	return sb.String()
}

// normalizeTeamName cleans up team names for consistent matching.
func normalizeTeamName(name string) string {
	name = strings.TrimSpace(name)
	// Remove seed numbers that sometimes appear
	for _, suffix := range []string{" (1)", " (2)", " (3)", " (4)", " (5)", " (6)", " (7)", " (8)",
		" (9)", " (10)", " (11)", " (12)", " (13)", " (14)", " (15)", " (16)"} {
		name = strings.TrimSuffix(name, suffix)
	}
	return name
}

// teamNameToSportsRefSlug converts a team name to Sports-Reference URL slug.
func teamNameToSportsRefSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, ".", "")
	slug = strings.ReplaceAll(slug, "'", "")
	slug = strings.ReplaceAll(slug, "&", "")

	// Common overrides
	overrides := map[string]string{
		"uconn":       "connecticut",
		"michigan-st": "michigan-state",
		"st-johns":    "st-johns-ny",
		"iowa-st":     "iowa-state",
	}
	if override, ok := overrides[slug]; ok {
		return override
	}
	return slug
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

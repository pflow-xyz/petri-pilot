package data

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// TournamentResult holds a single tournament game outcome.
type TournamentResult struct {
	Season   int
	Round    int // 1=R64, 2=R32, 3=S16, 4=E8, 5=F4, 6=Championship
	WTeam    string
	WScore   int
	WSeed    int
	LTeam    string
	LScore   int
	LSeed    int
}

// HistoricalSeason holds all tournament results and team stats for a season.
type HistoricalSeason struct {
	Season  int
	Results []TournamentResult
	Teams   []TeamStats
}

// LoadKaggleResults loads tournament results from a Kaggle CSV file.
// Expected columns: Season, DayNum, WTeamID, WScore, LTeamID, LScore
// Or: Season, Round, WTeam, WScore, WSeed, LTeam, LScore, LSeed
func LoadKaggleResults(path string) (map[int][]TournamentResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.TrimLeadingSpace = true

	// Read header
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}

	// Map column names to indices
	cols := make(map[string]int)
	for i, h := range header {
		cols[strings.TrimSpace(strings.ToLower(h))] = i
	}

	results := make(map[int][]TournamentResult)

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		r := TournamentResult{}

		if idx, ok := cols["season"]; ok {
			r.Season, _ = strconv.Atoi(row[idx])
		}
		if idx, ok := cols["round"]; ok {
			r.Round, _ = strconv.Atoi(row[idx])
		}

		// Team names — try different column name conventions
		for _, key := range []string{"wteam", "wteamname", "w_team", "winner"} {
			if idx, ok := cols[key]; ok {
				r.WTeam = strings.TrimSpace(row[idx])
				break
			}
		}
		for _, key := range []string{"lteam", "lteamname", "l_team", "loser"} {
			if idx, ok := cols[key]; ok {
				r.LTeam = strings.TrimSpace(row[idx])
				break
			}
		}

		if idx, ok := cols["wscore"]; ok {
			r.WScore, _ = strconv.Atoi(row[idx])
		}
		if idx, ok := cols["lscore"]; ok {
			r.LScore, _ = strconv.Atoi(row[idx])
		}
		if idx, ok := cols["wseed"]; ok {
			r.WSeed, _ = strconv.Atoi(row[idx])
		}
		if idx, ok := cols["lseed"]; ok {
			r.LSeed, _ = strconv.Atoi(row[idx])
		}

		if r.Season > 0 && r.WTeam != "" {
			results[r.Season] = append(results[r.Season], r)
		}
	}

	return results, nil
}

// LoadHistoricalBarttorvik loads historical Barttorvik data from a CSV file.
// This can be exported from barttorvik.com or assembled from yearly snapshots.
func LoadHistoricalBarttorvik(path string) (map[int][]RawTeamData, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}

	cols := make(map[string]int)
	for i, h := range header {
		cols[strings.TrimSpace(strings.ToLower(h))] = i
	}

	seasons := make(map[int][]RawTeamData)

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		var t RawTeamData
		var season int

		if idx, ok := cols["season"]; ok && idx < len(row) {
			season, _ = strconv.Atoi(row[idx])
		}
		if idx, ok := cols["team"]; ok && idx < len(row) {
			t.Name = normalizeTeamName(strings.TrimSpace(row[idx]))
		}
		if idx, ok := cols["conf"]; ok && idx < len(row) {
			t.Conference = strings.TrimSpace(row[idx])
		}
		if idx, ok := cols["adjoe"]; ok && idx < len(row) {
			t.AdjOE = parseFloat(row[idx])
		}
		if idx, ok := cols["adjde"]; ok && idx < len(row) {
			t.AdjDE = parseFloat(row[idx])
		}
		if idx, ok := cols["barthag"]; ok && idx < len(row) {
			t.Barthag = parseFloat(row[idx])
		}
		if idx, ok := cols["sos"]; ok && idx < len(row) {
			t.SOS = parseFloat(row[idx])
		}
		if idx, ok := cols["wab"]; ok && idx < len(row) {
			t.WAB = parseFloat(row[idx])
		}

		// Parse record
		if idx, ok := cols["rec"]; ok && idx < len(row) {
			fmt.Sscanf(row[idx], "%d-%d", &t.Wins, &t.Losses)
		} else {
			if idx, ok := cols["wins"]; ok && idx < len(row) {
				t.Wins, _ = strconv.Atoi(row[idx])
			}
			if idx, ok := cols["losses"]; ok && idx < len(row) {
				t.Losses, _ = strconv.Atoi(row[idx])
			}
		}

		if season > 0 && t.Name != "" {
			seasons[season] = append(seasons[season], t)
		}
	}

	return seasons, nil
}

// GetFinalFour extracts Final Four teams from tournament results.
func GetFinalFour(results []TournamentResult) []string {
	ff := make(map[string]bool)
	for _, r := range results {
		if r.Round >= 5 { // F4 or Championship
			ff[r.WTeam] = true
			ff[r.LTeam] = true
		}
	}
	var teams []string
	for t := range ff {
		teams = append(teams, t)
	}
	return teams
}

// GetChampion extracts the tournament champion.
func GetChampion(results []TournamentResult) string {
	for _, r := range results {
		if r.Round == 6 { // Championship game
			return r.WTeam
		}
	}
	// Try max round if round numbering differs
	maxRound := 0
	var champ string
	for _, r := range results {
		if r.Round > maxRound {
			maxRound = r.Round
			champ = r.WTeam
		}
	}
	return champ
}

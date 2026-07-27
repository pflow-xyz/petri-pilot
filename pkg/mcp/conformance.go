package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/pflow-xyz/go-pflow/eventlog"
	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/go-pflow/mining"
)

// conformanceTool answers the question the structural tools cannot: does this
// model describe what actually happened?
//
// petri_validate and petri_verify establish that a model is internally
// coherent and satisfies its stated properties. A model can pass both and still
// be the wrong model — deadlock-free, bounded, live, and simply not what the
// process does. Replaying a real event log against the net closes that gap.
func conformanceTool() mcp.Tool {
	return mcp.NewTool("petri_conformance",
		mcp.WithDescription(
			"Check how well a Petri net model matches real observed behavior, by replaying an event log "+
				"against it. Returns fitness (can the model reproduce the observed traces?), precision "+
				"(does the model allow behavior never observed?), and per-trace diagnostics naming the "+
				"activities that could not be replayed. Use after petri_validate/petri_verify to confirm "+
				"the model describes reality, not just a self-consistent fiction."),
		mcp.WithString("model",
			mcp.Required(),
			mcp.Description("The Petri net model as JSON or tokenmodel DSL (S-expression format starting with '(')"),
		),
		mcp.WithString("log",
			mcp.Required(),
			mcp.Description(`Event log as a JSON array of events. Each event needs at least a case id and an activity:
  [{"case":"order-1","activity":"receive","timestamp":"2026-01-01T10:00:00Z"},
   {"case":"order-1","activity":"ship"},
   {"case":"order-2","activity":"receive"}]

Accepted key aliases: case/caseId/case_id/trace, activity/action/task/event/transition,
timestamp/time/ts (RFC3339; optional — events keep their given order when absent),
resource (optional). Activity names must match the model's transition names.`),
		),
		mcp.WithBoolean("include_traces",
			mcp.Description("Include per-trace fitness detail in the output (default true; set false for a large log)"),
		),
	)
}

// logEvent is the wire form of a single event, tolerant of common key spellings
// so a caller does not have to reshape a log before checking it.
type logEvent struct {
	Case      string `json:"case"`
	CaseID    string `json:"caseId"`
	CaseSnake string `json:"case_id"`
	Trace     string `json:"trace"`

	Activity   string `json:"activity"`
	Action     string `json:"action"`
	Task       string `json:"task"`
	Event      string `json:"event"`
	Transition string `json:"transition"`

	Timestamp string `json:"timestamp"`
	Time      string `json:"time"`
	TS        string `json:"ts"`

	Resource string `json:"resource"`
}

func (e logEvent) caseID() string {
	for _, v := range []string{e.Case, e.CaseID, e.CaseSnake, e.Trace} {
		if v != "" {
			return v
		}
	}
	return ""
}

func (e logEvent) activity() string {
	for _, v := range []string{e.Activity, e.Action, e.Task, e.Event, e.Transition} {
		if v != "" {
			return v
		}
	}
	return ""
}

func (e logEvent) timestamp() (time.Time, bool) {
	for _, v := range []string{e.Timestamp, e.Time, e.TS} {
		if v == "" {
			continue
		}
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
			if t, err := time.Parse(layout, v); err == nil {
				return t, true
			}
		}
	}
	return time.Time{}, false
}

func handleConformance(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model parameter: %v", err)), nil
	}

	logJSON, err := request.RequireString("log")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing log parameter: %v", err)), nil
	}

	parsed, err := parseModelV2(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model: %v", err)), nil
	}
	net := buildVerifyNet(parsed.Model)

	log, warnings, err := parseEventLog(logJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid log: %v", err)), nil
	}
	if len(log.Cases) == 0 {
		return mcp.NewToolResultError("event log contains no cases"), nil
	}

	// Flag activities the model has no transition for. Without this, an
	// activity name typo reads as a low fitness score with no obvious cause.
	if unknown := unknownActivities(log, parsed.Model.Transitions); len(unknown) > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"log activities with no matching transition in the model: %v — these can never replay, so fitness is capped below 1.0",
			unknown))
	}

	result := mining.CheckFullConformance(log, net)

	type traceDetail struct {
		CaseID  string   `json:"case_id"`
		Fitness float64  `json:"fitness"`
		Fits    bool     `json:"fits"`
		Missing []string `json:"missing_activities,omitempty"`
	}

	output := struct {
		Fitness         float64       `json:"fitness"`
		Precision       float64       `json:"precision"`
		FScore          float64       `json:"f_score"`
		Cases           int           `json:"cases"`
		FittingTraces   int           `json:"fitting_traces"`
		AvgTraceFitness float64       `json:"avg_trace_fitness"`
		Traces          []traceDetail `json:"traces,omitempty"`
		Warnings        []string      `json:"warnings,omitempty"`
		Interpretation  string        `json:"interpretation"`
	}{
		Cases:    len(log.Cases),
		Warnings: warnings,
	}

	if result == nil {
		return mcp.NewToolResultError("conformance check returned no result"), nil
	}

	output.FScore = result.FScore
	if result.Precision != nil {
		output.Precision = result.Precision.Precision
	}

	if conf := result.Fitness; conf != nil {
		output.Fitness = conf.Fitness
		output.FittingTraces = conf.FittingTraces
		output.AvgTraceFitness = conf.AvgTraceFitness

		if request.GetBool("include_traces", true) {
			for _, tr := range conf.TraceResults {
				output.Traces = append(output.Traces, traceDetail{
					CaseID:  tr.CaseID,
					Fitness: tr.Fitness,
					Fits:    tr.Fitting,
					Missing: tr.MissingActivities,
				})
			}
			sort.Slice(output.Traces, func(i, j int) bool {
				// Worst-fitting traces first: those are the ones worth reading.
				if output.Traces[i].Fitness != output.Traces[j].Fitness {
					return output.Traces[i].Fitness < output.Traces[j].Fitness
				}
				return output.Traces[i].CaseID < output.Traces[j].CaseID
			})
		}
	}

	output.Interpretation = interpretConformance(output.Fitness, output.Precision)

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(data)), nil
}

// parseEventLog converts the JSON array form into an eventlog.EventLog.
func parseEventLog(src string) (*eventlog.EventLog, []string, error) {
	var events []logEvent
	if err := json.Unmarshal([]byte(src), &events); err != nil {
		return nil, nil, fmt.Errorf("log must be a JSON array of event objects: %w", err)
	}
	if len(events) == 0 {
		return nil, nil, fmt.Errorf("log is empty")
	}

	log := eventlog.NewEventLog()
	var warnings []string
	skipped := 0
	anyTimestamp := false

	// Events without a timestamp keep their input order; SortTraces is only
	// safe to call when every event actually carries one.
	base := time.Unix(0, 0).UTC()

	for i, e := range events {
		caseID, activity := e.caseID(), e.activity()
		if caseID == "" || activity == "" {
			skipped++
			continue
		}

		ts, ok := e.timestamp()
		if ok {
			anyTimestamp = true
		} else {
			ts = base.Add(time.Duration(i) * time.Second)
		}

		log.AddEvent(eventlog.Event{
			CaseID:    caseID,
			Activity:  activity,
			Timestamp: ts,
			Resource:  e.Resource,
		})
	}

	if skipped > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"%d event(s) skipped for missing a case id or activity name", skipped))
	}
	if !anyTimestamp {
		warnings = append(warnings, "no timestamps found — events were replayed in the order given")
	} else {
		log.SortTraces()
	}

	return log, warnings, nil
}

// unknownActivities returns log activity names with no matching transition.
func unknownActivities(log *eventlog.EventLog, transitions []goflowmetamodel.Transition) []string {
	known := make(map[string]bool, len(transitions))
	for _, t := range transitions {
		known[t.ID] = true
	}

	seen := make(map[string]bool)
	var unknown []string
	for _, trace := range log.Cases {
		for _, ev := range trace.Events {
			if !known[ev.Activity] && !seen[ev.Activity] {
				seen[ev.Activity] = true
				unknown = append(unknown, ev.Activity)
			}
		}
	}
	sort.Strings(unknown)
	return unknown
}

// interpretConformance turns two numbers into the sentence an agent needs to
// decide what to do next.
func interpretConformance(fitness, precision float64) string {
	switch {
	case fitness >= 0.95 && precision >= 0.75:
		return "The model both reproduces the observed behavior and stays close to it — a good fit."
	case fitness >= 0.95:
		return "The model reproduces every observed trace but permits much behavior never seen (low precision): it is underfitted, likely too permissive. Consider tightening guards or removing transitions."
	case fitness >= 0.7:
		return "Most observed behavior replays, but some traces do not fit. Inspect the lowest-fitness traces and their missing activities — they indicate paths the model does not allow."
	default:
		return "The model cannot reproduce most of the observed behavior. Check that activity names match transition names, then reconsider the model structure — consider rediscovering it from the log."
	}
}

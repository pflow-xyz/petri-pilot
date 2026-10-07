package eventgen

import (
	"encoding/csv"
	"encoding/json"
	"io"
	"sort"
	"time"

	"github.com/pflow-xyz/go-pflow/eventlog"
)

// go-pflow's eventlog package parses CSV and JSONL but writes neither; these
// are the encoder halves, emitting exactly the column/field shapes its own
// parsers accept, so a written log re-reads through ParseCSV/ParseJSONL
// unchanged. Output order is deterministic — cases sorted by id, events by
// timestamp — because a dataset endpoint that returns different bytes for
// the same seed is a cache-buster and a diff-noise generator.

// WriteCSV emits case_id,activity,timestamp,resource,lifecycle rows.
func WriteCSV(w io.Writer, log *eventlog.EventLog) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"case_id", "activity", "timestamp", "resource", "lifecycle"}); err != nil {
		return err
	}
	for _, trace := range sortedTraces(log) {
		for _, ev := range trace.Events {
			if err := cw.Write([]string{
				ev.CaseID, ev.Activity, ev.Timestamp.UTC().Format(time.RFC3339Nano), ev.Resource, ev.Lifecycle,
			}); err != nil {
				return err
			}
		}
	}
	cw.Flush()
	return cw.Error()
}

// WriteJSONL emits one event object per line.
func WriteJSONL(w io.Writer, log *eventlog.EventLog) error {
	enc := json.NewEncoder(w)
	for _, trace := range sortedTraces(log) {
		for _, ev := range trace.Events {
			rec := map[string]any{
				"case_id":   ev.CaseID,
				"activity":  ev.Activity,
				"timestamp": ev.Timestamp.UTC().Format(time.RFC3339Nano),
			}
			if ev.Resource != "" {
				rec["resource"] = ev.Resource
			}
			if ev.Lifecycle != "" {
				rec["lifecycle"] = ev.Lifecycle
			}
			if err := enc.Encode(rec); err != nil {
				return err
			}
		}
	}
	return nil
}

func sortedTraces(log *eventlog.EventLog) []*eventlog.Trace {
	ids := make([]string, 0, len(log.Cases))
	for id := range log.Cases {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]*eventlog.Trace, 0, len(ids))
	for _, id := range ids {
		out = append(out, log.Cases[id])
	}
	return out
}

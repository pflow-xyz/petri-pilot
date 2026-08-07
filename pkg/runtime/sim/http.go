package sim

import (
	"encoding/json"
	"net/http"

	"github.com/pflow-xyz/go-pflow/metamodel"

	"github.com/pflow-xyz/petri-pilot/pkg/runtime/api"
)

// The scenario endpoints live here rather than in a template.
//
// Nothing about answering a hypothetical depends on the shape of the
// application asking it: a scenario supplies its own marking, so there is no
// aggregate to load, no store to read and no entity layout to know about. The
// single-net and composed generators mount the same handlers, which is what
// keeps a fix to one from being a fix to only one — the failure mode that let
// the composed app ship with no simulation at all.

// ModelFunc supplies the model a scenario runs against. Generated apps pass the
// accessor for the model embedded at generation time; it returns an error
// rather than a model so a decode failure surfaces as a 500 with a reason
// instead of a nil dereference.
type ModelFunc func() (*metamodel.Model, error)

// HandleScenario answers one hypothetical.
//
//	POST /api/scenario
//	{"marking": {"staff/available": 3}, "schedule": {...}, "hours": 8}
//
// Pure: no aggregate is loaded, no event is appended, no store is touched.
func HandleScenario(model ModelFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m, err := model()
		if err != nil {
			api.Error(w, http.StatusInternalServerError, "SCHEMA_ERROR", err.Error())
			return
		}
		var s Scenario
		if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
			api.Error(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
			return
		}
		res, err := Run(m, s)
		if err != nil {
			// A rejected scenario is the caller's mistake, not a server fault:
			// almost always a place or transition name that does not exist.
			// Reporting it beats running the unmodified model and answering a
			// question nobody asked.
			api.Error(w, http.StatusBadRequest, "INVALID_SCENARIO", err.Error())
			return
		}
		api.JSON(w, http.StatusOK, res)
	}
}

// HandleCompare answers several hypotheticals together.
//
//	POST /api/scenario/compare
//	{"scenarios": [{"name": "today", ...}, {"name": "one more barista", ...}]}
//
// One request rather than several, because the scenarios have to share a seed
// to be comparable at all, and a client cannot be relied on to arrange that.
func HandleCompare(model ModelFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m, err := model()
		if err != nil {
			api.Error(w, http.StatusInternalServerError, "SCHEMA_ERROR", err.Error())
			return
		}
		var body struct {
			Scenarios []Scenario `json:"scenarios"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			api.Error(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
			return
		}
		cmp, err := Compare(m, body.Scenarios)
		if err != nil {
			api.Error(w, http.StatusBadRequest, "INVALID_SCENARIO", err.Error())
			return
		}
		api.JSON(w, http.StatusOK, cmp)
	}
}

// HandleModelRates reports the rates and the initial marking the model
// declares, so a client can render controls seeded from the net rather than
// from a table of its own that will quietly drift from it.
func HandleModelRates(model ModelFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m, err := model()
		if err != nil {
			api.Error(w, http.StatusInternalServerError, "SCHEMA_ERROR", err.Error())
			return
		}
		api.JSON(w, http.StatusOK, map[string]any{
			"rates":   Rates(m),
			"initial": m.InitialMarking(),
			"caveats": m.Gating(),
		})
	}
}

// RegisterScenarioRoutes mounts the hypothetical-answering endpoints.
func RegisterScenarioRoutes(r *api.Router, model ModelFunc) {
	r.POST("/api/scenario", "Run a hypothetical: your own marking, rates and rate schedule. Changes nothing.", HandleScenario(model))
	r.POST("/api/scenario/compare", "Run several scenarios on one seed and return them side by side.", HandleCompare(model))
	r.GET("/api/rates", "Rates and initial marking declared by the model.", HandleModelRates(model))
}

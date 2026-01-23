// Financial analytics for vet clinic

package main

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/pflow-xyz/petri-pilot/pkg/runtime"
	"github.com/pflow-xyz/petri-pilot/pkg/runtime/api"
)

// FinancialMetrics represents aggregate financial data
type FinancialMetrics struct {
	TotalVisits        int                        `json:"total_visits"`
	AvgServicesPct     float64                    `json:"avg_services_pct"`
	AvgProductsPct     float64                    `json:"avg_products_pct"`
	ByCategory         CategoryBreakdown          `json:"by_category"`
	ByAppointmentType  map[string]TypeMetrics     `json:"by_appointment_type"`
	ByProvider         map[string]ProviderMetrics `json:"by_provider"`
	IndustryBenchmarks Benchmarks                 `json:"industry_benchmarks"`
}

// CategoryBreakdown shows average % by revenue category
type CategoryBreakdown struct {
	ExamPct        float64 `json:"exam_pct"`
	ProceduresPct  float64 `json:"procedures_pct"`
	LabPct         float64 `json:"lab_pct"`
	MedicationsPct float64 `json:"medications_pct"`
	SuppliesPct    float64 `json:"supplies_pct"`
}

// TypeMetrics shows metrics by appointment type
type TypeMetrics struct {
	VisitCount     int     `json:"visit_count"`
	AvgServicesPct float64 `json:"avg_services_pct"`
	AvgProductsPct float64 `json:"avg_products_pct"`
	TotalWeight    float64 `json:"total_weight"`
}

// ProviderMetrics shows metrics by provider
type ProviderMetrics struct {
	VisitCount     int     `json:"visit_count"`
	AvgServicesPct float64 `json:"avg_services_pct"`
	AvgProductsPct float64 `json:"avg_products_pct"`
	TotalWeight    float64 `json:"total_weight"`
}

// Benchmarks shows industry standard targets
type Benchmarks struct {
	TargetServicesPct float64 `json:"target_services_pct"`
	TargetProductsPct float64 `json:"target_products_pct"`
	TargetCOGSPct     float64 `json:"target_cogs_pct"`
	Notes             string  `json:"notes"`
}

// VisitFinancials extracts financial data from a completed visit
type VisitFinancials struct {
	AggregateID      string  `json:"aggregate_id"`
	PatientName      string  `json:"patient_name"`
	AppointmentType  string  `json:"appointment_type"`
	Provider         string  `json:"provider"`
	ExamPct          float64 `json:"exam_pct"`
	ProceduresPct    float64 `json:"procedures_pct"`
	LabPct           float64 `json:"lab_pct"`
	MedicationsPct   float64 `json:"medications_pct"`
	SuppliesPct      float64 `json:"supplies_pct"`
	ServicesTotalPct float64 `json:"services_total_pct"`
	ProductsTotalPct float64 `json:"products_total_pct"`
	VisitWeight      float64 `json:"visit_weight"`
}

// HandleGetAnalytics returns aggregate financial analytics
func HandleGetAnalytics(app *Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get all completed visits
		visits, err := getCompletedVisits(ctx, app)
		if err != nil {
			api.Error(w, http.StatusInternalServerError, "ANALYTICS_ERROR", err.Error())
			return
		}

		metrics := computeMetrics(visits)
		api.JSON(w, http.StatusOK, metrics)
	}
}

func getCompletedVisits(ctx context.Context, app *Application) ([]VisitFinancials, error) {
	// Get all checkout events using ReadAll with filter
	filter := runtime.EventFilter{
		Types: []string{"Checkouted"},
	}

	checkoutEvents, err := app.store.ReadAll(ctx, filter)
	if err != nil {
		return nil, err
	}

	var visits []VisitFinancials

	for _, checkoutEvt := range checkoutEvents {
		streamID := checkoutEvt.StreamID

		// Read all events for this stream to get full visit data
		allEvents, err := app.store.Read(ctx, streamID, 0)
		if err != nil {
			continue
		}

		visit := VisitFinancials{
			AggregateID: streamID,
			VisitWeight: 1.0,
		}

		// Extract data from events
		for _, evt := range allEvents {
			var data map[string]interface{}
			if err := json.Unmarshal(evt.Data, &data); err != nil {
				continue
			}

			switch evt.Type {
			case "Scheduleed":
				if v, ok := data["patient_name"].(string); ok {
					visit.PatientName = v
				}
				if v, ok := data["appointment_type"].(string); ok {
					visit.AppointmentType = v
				}
				if v, ok := data["assigned_provider"].(string); ok {
					visit.Provider = v
				}

			case "Checkouted":
				// Get financial breakdown
				if v, ok := data["exam_pct"].(float64); ok {
					visit.ExamPct = v
				}
				if v, ok := data["procedures_pct"].(float64); ok {
					visit.ProceduresPct = v
				}
				if v, ok := data["lab_pct"].(float64); ok {
					visit.LabPct = v
				}
				if v, ok := data["medications_pct"].(float64); ok {
					visit.MedicationsPct = v
				}
				if v, ok := data["supplies_pct"].(float64); ok {
					visit.SuppliesPct = v
				}

				// Services/products totals
				if v, ok := data["services_total_pct"].(float64); ok {
					visit.ServicesTotalPct = v
				} else if v, ok := data["services_pct"].(float64); ok {
					visit.ServicesTotalPct = v
				}

				if v, ok := data["products_total_pct"].(float64); ok {
					visit.ProductsTotalPct = v
				} else if v, ok := data["products_pct"].(float64); ok {
					visit.ProductsTotalPct = v
				}

				if v, ok := data["visit_weight"].(float64); ok {
					visit.VisitWeight = v
				}
			}
		}

		visits = append(visits, visit)
	}

	return visits, nil
}

func computeMetrics(visits []VisitFinancials) FinancialMetrics {
	metrics := FinancialMetrics{
		TotalVisits:       len(visits),
		ByAppointmentType: make(map[string]TypeMetrics),
		ByProvider:        make(map[string]ProviderMetrics),
		IndustryBenchmarks: Benchmarks{
			TargetServicesPct: 71.0,
			TargetProductsPct: 29.0,
			TargetCOGSPct:     22.0,
			Notes:             "Industry benchmarks: ~71% services, ~29% products. Target COGS: 20-24%",
		},
	}

	if len(visits) == 0 {
		return metrics
	}

	// Compute weighted averages
	var totalWeight float64
	var weightedServices, weightedProducts float64
	var totalExam, totalProcedures, totalLab, totalMeds, totalSupplies float64

	for _, v := range visits {
		weight := v.VisitWeight
		if weight <= 0 {
			weight = 1.0
		}
		totalWeight += weight

		weightedServices += v.ServicesTotalPct * weight
		weightedProducts += v.ProductsTotalPct * weight

		totalExam += v.ExamPct
		totalProcedures += v.ProceduresPct
		totalLab += v.LabPct
		totalMeds += v.MedicationsPct
		totalSupplies += v.SuppliesPct

		// By appointment type
		typeKey := v.AppointmentType
		if typeKey == "" {
			typeKey = "unknown"
		}
		tm := metrics.ByAppointmentType[typeKey]
		tm.VisitCount++
		tm.TotalWeight += weight
		tm.AvgServicesPct = (tm.AvgServicesPct*float64(tm.VisitCount-1) + v.ServicesTotalPct) / float64(tm.VisitCount)
		tm.AvgProductsPct = (tm.AvgProductsPct*float64(tm.VisitCount-1) + v.ProductsTotalPct) / float64(tm.VisitCount)
		metrics.ByAppointmentType[typeKey] = tm

		// By provider
		provKey := v.Provider
		if provKey == "" {
			provKey = "unknown"
		}
		pm := metrics.ByProvider[provKey]
		pm.VisitCount++
		pm.TotalWeight += weight
		pm.AvgServicesPct = (pm.AvgServicesPct*float64(pm.VisitCount-1) + v.ServicesTotalPct) / float64(pm.VisitCount)
		pm.AvgProductsPct = (pm.AvgProductsPct*float64(pm.VisitCount-1) + v.ProductsTotalPct) / float64(pm.VisitCount)
		metrics.ByProvider[provKey] = pm
	}

	if totalWeight > 0 {
		metrics.AvgServicesPct = weightedServices / totalWeight
		metrics.AvgProductsPct = weightedProducts / totalWeight
	}

	n := float64(len(visits))
	metrics.ByCategory = CategoryBreakdown{
		ExamPct:        totalExam / n,
		ProceduresPct:  totalProcedures / n,
		LabPct:         totalLab / n,
		MedicationsPct: totalMeds / n,
		SuppliesPct:    totalSupplies / n,
	}

	return metrics
}

// HandleGetFinancialReport returns a detailed financial report
func HandleGetFinancialReport(app *Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		visits, err := getCompletedVisits(ctx, app)
		if err != nil {
			api.Error(w, http.StatusInternalServerError, "REPORT_ERROR", err.Error())
			return
		}

		// Return detailed visit data
		report := map[string]interface{}{
			"visits":  visits,
			"summary": computeMetrics(visits),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(report)
	}
}

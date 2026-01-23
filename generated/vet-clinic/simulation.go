// Simulation mode for vet clinic
// Generates realistic appointment data and runs through workflows

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/pflow-xyz/petri-pilot/pkg/runtime/api"
)

// SimulationConfig controls simulation parameters
type SimulationConfig struct {
	VisitsToGenerate int  `json:"visits_to_generate"`
	DelayMs          int  `json:"delay_ms"`
	AutoComplete     bool `json:"auto_complete"`
}

// SimulationStatus tracks simulation progress
type SimulationStatus struct {
	Running        bool   `json:"running"`
	TotalPlanned   int    `json:"total_planned"`
	Completed      int    `json:"completed"`
	InProgress     int    `json:"in_progress"`
	Failed         int    `json:"failed"`
	CurrentPatient string `json:"current_patient"`
	Message        string `json:"message"`
}

// Simulation state
var (
	simMutex  sync.Mutex
	simStatus = SimulationStatus{}
	simStop   = make(chan bool, 1)
)

// Sample data for realistic generation
var (
	petNames = []string{
		"Max", "Bella", "Charlie", "Luna", "Cooper", "Daisy", "Buddy", "Sadie",
		"Rocky", "Molly", "Bear", "Bailey", "Duke", "Maggie", "Tucker", "Sophie",
		"Jack", "Chloe", "Oliver", "Penny", "Leo", "Zoe", "Milo", "Lily",
	}

	ownerLastNames = []string{
		"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller",
		"Davis", "Rodriguez", "Martinez", "Anderson", "Taylor", "Thomas", "Moore",
	}

	appointmentTypes = []struct {
		Type           string
		Weight         int
		ProviderType   string
		DurationMin    int
		ServicesPctAvg float64
		ServicesPctVar float64
	}{
		{"wellness", 40, "dvm", 30, 75, 5},
		{"sick", 30, "dvm", 30, 60, 10},
		{"dental", 10, "dvm", 60, 70, 8},
		{"surgery", 10, "dvm", 90, 85, 5},
		{"vaccination", 10, "rvt", 15, 80, 5},
	}

	providers = []struct {
		Name string
		Type string
	}{
		{"Dr. Johnson", "dvm"},
		{"Dr. Smith", "dvm"},
		{"Dr. Martinez", "dvm"},
		{"Sarah RVT", "rvt"},
		{"Mike RVT", "rvt"},
	}

	rooms = []string{
		"Exam Room 1", "Exam Room 2", "Exam Room 3",
		"Surgery Suite", "Dental Suite", "Treatment Area",
	}

	diagnoses = map[string][]string{
		"wellness":    {"Healthy - annual exam complete", "Healthy with minor dental tartar", "Healthy - vaccinations updated"},
		"sick":        {"Upper respiratory infection", "Gastrointestinal upset", "Skin allergies", "Ear infection", "Urinary tract infection"},
		"dental":      {"Grade 2 dental disease - cleaning complete", "Tooth extraction performed", "Periodontal treatment"},
		"surgery":     {"Spay/neuter complete", "Mass removal - sent to lab", "Orthopedic repair successful"},
		"vaccination": {"Vaccinations administered per schedule", "Boosters given", "Rabies vaccination updated"},
	}
)

// HandleStartSimulation starts the simulation
func HandleStartSimulation(app *Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var config SimulationConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			config = SimulationConfig{
				VisitsToGenerate: 10,
				DelayMs:          500,
				AutoComplete:     true,
			}
		}

		if config.VisitsToGenerate <= 0 {
			config.VisitsToGenerate = 10
		}
		if config.VisitsToGenerate > 100 {
			config.VisitsToGenerate = 100
		}
		if config.DelayMs < 100 {
			config.DelayMs = 100
		}

		simMutex.Lock()
		if simStatus.Running {
			simMutex.Unlock()
			api.Error(w, http.StatusConflict, "SIMULATION_RUNNING", "Simulation already running")
			return
		}
		simStatus = SimulationStatus{
			Running:      true,
			TotalPlanned: config.VisitsToGenerate,
			Message:      "Starting simulation...",
		}
		simMutex.Unlock()

		// Clear stop channel
		select {
		case <-simStop:
		default:
		}

		// Run simulation in background
		go runSimulation(app, config)

		api.JSON(w, http.StatusOK, simStatus)
	}
}

// HandleStopSimulation stops the simulation
func HandleStopSimulation(app *Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		simMutex.Lock()
		if !simStatus.Running {
			simMutex.Unlock()
			api.Error(w, http.StatusBadRequest, "NOT_RUNNING", "No simulation running")
			return
		}
		simMutex.Unlock()

		// Signal stop
		select {
		case simStop <- true:
		default:
		}

		api.JSON(w, http.StatusOK, map[string]string{"status": "stopping"})
	}
}

// HandleGetSimulationStatus returns current simulation status
func HandleGetSimulationStatus(app *Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		simMutex.Lock()
		status := simStatus
		simMutex.Unlock()

		api.JSON(w, http.StatusOK, status)
	}
}

func runSimulation(app *Application, config SimulationConfig) {
	ctx := context.Background()
	delay := time.Duration(config.DelayMs) * time.Millisecond

	for i := 0; i < config.VisitsToGenerate; i++ {
		// Check for stop signal
		select {
		case <-simStop:
			simMutex.Lock()
			simStatus.Running = false
			simStatus.Message = "Simulation stopped by user"
			simMutex.Unlock()
			return
		default:
		}

		// Generate visit data
		visit := generateVisit()

		simMutex.Lock()
		simStatus.CurrentPatient = visit.PatientName
		simStatus.InProgress = 1
		simStatus.Message = fmt.Sprintf("Processing %s (%s)", visit.PatientName, visit.AppointmentType)
		simMutex.Unlock()

		// Run through workflow
		err := processVisit(ctx, app, visit, delay)

		simMutex.Lock()
		simStatus.InProgress = 0
		if err != nil {
			simStatus.Failed++
			simStatus.Message = fmt.Sprintf("Failed: %s - %v", visit.PatientName, err)
		} else {
			simStatus.Completed++
			simStatus.Message = fmt.Sprintf("Completed: %s", visit.PatientName)
		}
		simMutex.Unlock()

		time.Sleep(delay)
	}

	simMutex.Lock()
	simStatus.Running = false
	simStatus.Message = fmt.Sprintf("Simulation complete: %d visits processed", simStatus.Completed)
	simMutex.Unlock()
}

type simulatedVisit struct {
	PatientName     string
	OwnerName       string
	Phone           string
	AppointmentType string
	ProviderType    string
	Provider        string
	Room            string
	DurationMinutes int
	ServicesPct     float64
	ProductsPct     float64
	Diagnosis       string
	Treatment       string
}

func generateVisit() simulatedVisit {
	// Pick appointment type weighted by frequency
	totalWeight := 0
	for _, t := range appointmentTypes {
		totalWeight += t.Weight
	}
	r := rand.Intn(totalWeight)
	var apptType struct {
		Type           string
		Weight         int
		ProviderType   string
		DurationMin    int
		ServicesPctAvg float64
		ServicesPctVar float64
	}
	for _, t := range appointmentTypes {
		r -= t.Weight
		if r < 0 {
			apptType = t
			break
		}
	}

	// Pick provider of correct type
	var eligibleProviders []struct {
		Name string
		Type string
	}
	for _, p := range providers {
		if p.Type == apptType.ProviderType || p.Type == "dvm" {
			eligibleProviders = append(eligibleProviders, p)
		}
	}
	provider := eligibleProviders[rand.Intn(len(eligibleProviders))]

	// Generate financial breakdown with some variance
	servicesPct := apptType.ServicesPctAvg + (rand.Float64()*2-1)*apptType.ServicesPctVar
	if servicesPct > 95 {
		servicesPct = 95
	}
	if servicesPct < 50 {
		servicesPct = 50
	}

	// Pick diagnosis
	diagList := diagnoses[apptType.Type]
	diagnosis := diagList[rand.Intn(len(diagList))]

	return simulatedVisit{
		PatientName:     petNames[rand.Intn(len(petNames))],
		OwnerName:       ownerLastNames[rand.Intn(len(ownerLastNames))],
		Phone:           fmt.Sprintf("555-%04d", rand.Intn(10000)),
		AppointmentType: apptType.Type,
		ProviderType:    apptType.ProviderType,
		Provider:        provider.Name,
		Room:            rooms[rand.Intn(len(rooms))],
		DurationMinutes: apptType.DurationMin,
		ServicesPct:     servicesPct,
		ProductsPct:     100 - servicesPct,
		Diagnosis:       diagnosis,
		Treatment:       "Treatment administered per protocol",
	}
}

func processVisit(ctx context.Context, app *Application, visit simulatedVisit, delay time.Duration) error {
	// Create aggregate
	id, err := app.Create(ctx)
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}

	// Schedule
	_, err = app.Execute(ctx, id, TransitionSchedule, map[string]interface{}{
		"patient_name":      visit.PatientName,
		"owner_name":        visit.OwnerName,
		"phone":             visit.Phone,
		"appointment_type":  visit.AppointmentType,
		"scheduled_time":    time.Now().Format(time.RFC3339),
		"duration_minutes":  float64(visit.DurationMinutes),
		"assigned_provider": visit.Provider,
		"provider_type":     visit.ProviderType,
		"assigned_room":     visit.Room,
		"notes":             "Simulated appointment",
	})
	if err != nil {
		return fmt.Errorf("schedule: %w", err)
	}
	time.Sleep(delay / 4)

	// Check in
	_, err = app.Execute(ctx, id, TransitionCheckIn, map[string]interface{}{
		"arrival_time": time.Now().Format(time.RFC3339),
		"weight_lbs":   float64(10 + rand.Intn(90)),
	})
	if err != nil {
		return fmt.Errorf("check_in: %w", err)
	}
	time.Sleep(delay / 4)

	// Start exam
	_, err = app.Execute(ctx, id, TransitionStartExam, map[string]interface{}{
		"provider_name": visit.Provider,
		"room":          visit.Room,
	})
	if err != nil {
		return fmt.Errorf("start_exam: %w", err)
	}
	time.Sleep(delay / 4)

	// Complete
	_, err = app.Execute(ctx, id, TransitionComplete, map[string]interface{}{
		"diagnosis":        visit.Diagnosis,
		"treatment":        visit.Treatment,
		"follow_up_needed": rand.Float32() < 0.3,
	})
	if err != nil {
		return fmt.Errorf("complete: %w", err)
	}
	time.Sleep(delay / 4)

	// Checkout
	_, err = app.Execute(ctx, id, TransitionCheckout, map[string]interface{}{
		"services_pct":     visit.ServicesPct,
		"products_pct":     visit.ProductsPct,
		"next_appointment": "",
	})
	if err != nil {
		return fmt.Errorf("checkout: %w", err)
	}

	return nil
}

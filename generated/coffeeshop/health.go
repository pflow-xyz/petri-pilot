// Health state tracking for coffee shop simulation
// Mirrors go-pflow/examples/coffeeshop/simulator.go health classification

package coffeeshop

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// SystemHealth represents the overall health state of the coffee shop
type SystemHealth string

const (
	HealthHealthy         SystemHealth = "healthy"          // Everything running smoothly
	HealthBusy            SystemHealth = "busy"             // High traffic but managing
	HealthStressed        SystemHealth = "stressed"         // Falling behind, queues growing
	HealthSLACrisis       SystemHealth = "sla_crisis"       // Significant SLA breaches
	HealthInventoryCrisis SystemHealth = "inventory_crisis" // Running out of ingredients
	HealthCritical        SystemHealth = "critical"         // Multiple crises or menu empty
)

// HealthStateInfo provides display information for a health state
type HealthStateInfo struct {
	Key         string `json:"key"`
	Emoji       string `json:"emoji"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Severity    int    `json:"severity"`
}

// HealthStates maps health states to their display info
var HealthStates = map[SystemHealth]HealthStateInfo{
	HealthHealthy:         {Key: "healthy", Emoji: "💚", Label: "Healthy", Description: "Operating smoothly", Severity: 0},
	HealthBusy:            {Key: "busy", Emoji: "💛", Label: "Busy", Description: "High traffic, managing well", Severity: 1},
	HealthStressed:        {Key: "stressed", Emoji: "🟠", Label: "Stressed", Description: "Falling behind, queues growing", Severity: 2},
	HealthSLACrisis:       {Key: "sla_crisis", Emoji: "🔴", Label: "SLA Crisis", Description: "SLA targets being missed", Severity: 3},
	HealthInventoryCrisis: {Key: "inventory_crisis", Emoji: "📦", Label: "Inventory Crisis", Description: "Running low on ingredients", Severity: 3},
	HealthCritical:        {Key: "critical", Emoji: "🚨", Label: "Critical", Description: "Immediate action needed", Severity: 4},
}

// Resource max values for health calculations
const (
	MaxCoffeeBeans = 2000.0
	MaxMilk        = 1000.0
	MaxCups        = 500.0
)

// HealthMetricsData holds the serializable health metrics (no mutex)
type HealthMetricsData struct {
	// Rolling window metrics
	QueueHistory    []int   `json:"queueHistory"`
	QueueTrend      float64 `json:"queueTrend"`
	InventoryHealth float64 `json:"inventoryHealth"` // 0.0 - 1.0 (min across resources)
	SLABreachRate   float64 `json:"slaBreachRate"`   // 0.0 - 1.0

	// Counters
	TotalOrders     int `json:"totalOrders"`
	CompletedOrders int `json:"completedOrders"`
	SLABreaches     int `json:"slaBreaches"`

	// Current snapshot
	CurrentQueueLength int          `json:"currentQueueLength"`
	CurrentHealth      SystemHealth `json:"currentHealth"`
	PreviousHealth     SystemHealth `json:"previousHealth"`
	HealthChangedAt    *time.Time   `json:"healthChangedAt,omitempty"`
}

// HealthMetrics tracks rolling metrics for health classification (with thread safety)
type HealthMetrics struct {
	mu   sync.RWMutex
	data HealthMetricsData

	// Configuration (not serialized)
	windowSize int
}

// HealthResponse is the API response for /api/health
type HealthResponse struct {
	Health    SystemHealth      `json:"health"`
	Info      HealthStateInfo   `json:"info"`
	Metrics   HealthMetricsData `json:"metrics"`
	State     map[string]int    `json:"state"`
	Timestamp time.Time         `json:"timestamp"`
}

// Global health tracker
var healthTracker = NewHealthMetrics()

// NewHealthMetrics creates a new health metrics tracker
func NewHealthMetrics() *HealthMetrics {
	return &HealthMetrics{
		windowSize: 10, // 10-sample rolling window
		data: HealthMetricsData{
			QueueHistory:   make([]int, 0, 15),
			CurrentHealth:  HealthHealthy,
			PreviousHealth: HealthHealthy,
		},
	}
}

// Update updates health metrics based on current state
func (h *HealthMetrics) Update(state map[string]int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	queueLength := state["orders_pending"]

	// Update queue history
	h.data.QueueHistory = append(h.data.QueueHistory, queueLength)
	if len(h.data.QueueHistory) > h.windowSize {
		h.data.QueueHistory = h.data.QueueHistory[1:]
	}
	h.data.CurrentQueueLength = queueLength

	// Calculate queue trend (positive = growing)
	if len(h.data.QueueHistory) >= 3 {
		recent := h.data.QueueHistory[len(h.data.QueueHistory)-3:]
		h.data.QueueTrend = float64(recent[2]-recent[0]) / 3.0
	}

	// Calculate inventory health (minimum across key resources)
	coffeeBeans := float64(state["coffee_beans"]) / MaxCoffeeBeans
	milk := float64(state["milk"]) / MaxMilk
	cups := float64(state["cups"]) / MaxCups

	h.data.InventoryHealth = min(coffeeBeans, min(milk, cups))
	if h.data.InventoryHealth < 0 {
		h.data.InventoryHealth = 0
	}

	// Update order counts from state
	h.data.CompletedOrders = state["orders_complete"]

	// Estimate SLA breach rate based on queue buildup
	// In a real system, track actual wait times
	if h.data.CompletedOrders > 10 {
		totalProcessed := h.data.CompletedOrders + queueLength
		if totalProcessed > 0 {
			h.data.SLABreachRate = float64(queueLength) / float64(totalProcessed)
			if h.data.SLABreachRate > 1.0 {
				h.data.SLABreachRate = 1.0
			}
		}
	}

	// Classify health state
	h.data.PreviousHealth = h.data.CurrentHealth
	h.data.CurrentHealth = h.classifyHealth()

	if h.data.CurrentHealth != h.data.PreviousHealth {
		now := time.Now()
		h.data.HealthChangedAt = &now
	}
}

// classifyHealth determines the current health state based on metrics
// Mirrors go-pflow/examples/coffeeshop/simulator.go classifyHealth()
func (h *HealthMetrics) classifyHealth() SystemHealth {
	// Critical: Any resource depleted (menu would be empty)
	if h.data.InventoryHealth <= 0 {
		return HealthCritical
	}

	// Inventory crisis: Any ingredient below 10%
	if h.data.InventoryHealth < 0.10 {
		return HealthInventoryCrisis
	}

	// SLA crisis: More than 30% breach rate
	if h.data.SLABreachRate > 0.30 {
		return HealthSLACrisis
	}

	// Stressed: Queue > 10 or growing fast, or 15-30% breach rate
	if h.data.CurrentQueueLength > 10 || h.data.QueueTrend > 2.0 || h.data.SLABreachRate > 0.15 {
		return HealthStressed
	}

	// Busy: Queue > 5 or moderate growth, or 5-15% breach rate
	if h.data.CurrentQueueLength > 5 || h.data.QueueTrend > 1.0 || h.data.SLABreachRate > 0.05 {
		return HealthBusy
	}

	// Healthy: Everything under control
	return HealthHealthy
}

// GetHealth returns the current health state and metrics data
func (h *HealthMetrics) GetHealth() (SystemHealth, HealthMetricsData) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Return a copy of metrics data
	dataCopy := HealthMetricsData{
		QueueHistory:       append([]int{}, h.data.QueueHistory...),
		QueueTrend:         h.data.QueueTrend,
		InventoryHealth:    h.data.InventoryHealth,
		SLABreachRate:      h.data.SLABreachRate,
		TotalOrders:        h.data.TotalOrders,
		CompletedOrders:    h.data.CompletedOrders,
		SLABreaches:        h.data.SLABreaches,
		CurrentQueueLength: h.data.CurrentQueueLength,
		CurrentHealth:      h.data.CurrentHealth,
		PreviousHealth:     h.data.PreviousHealth,
		HealthChangedAt:    h.data.HealthChangedAt,
	}

	return h.data.CurrentHealth, dataCopy
}

// ClassifyHealthFromState calculates health state from a state map (stateless version)
// Can be used by frontend or backend without maintaining history
func ClassifyHealthFromState(state map[string]int) SystemHealth {
	// Calculate inventory health
	coffeeBeans := float64(state["coffee_beans"]) / MaxCoffeeBeans
	milk := float64(state["milk"]) / MaxMilk
	cups := float64(state["cups"]) / MaxCups
	inventoryHealth := min(coffeeBeans, min(milk, cups))

	queueLength := state["orders_pending"]
	completedOrders := state["orders_complete"]

	// Estimate SLA breach rate
	slaBreachRate := 0.0
	if completedOrders > 10 {
		totalProcessed := completedOrders + queueLength
		if totalProcessed > 0 {
			slaBreachRate = float64(queueLength) / float64(totalProcessed)
		}
	}

	// Critical: Any resource depleted
	if inventoryHealth <= 0 {
		return HealthCritical
	}

	// Inventory crisis: Below 10%
	if inventoryHealth < 0.10 {
		return HealthInventoryCrisis
	}

	// SLA crisis: >30% breach rate
	if slaBreachRate > 0.30 {
		return HealthSLACrisis
	}

	// Stressed: Queue > 10 or >15% breach rate
	if queueLength > 10 || slaBreachRate > 0.15 {
		return HealthStressed
	}

	// Busy: Queue > 5 or >5% breach rate
	if queueLength > 5 || slaBreachRate > 0.05 {
		return HealthBusy
	}

	return HealthHealthy
}

// UpdateHealthFromState updates the global health tracker with new state
func UpdateHealthFromState(state map[string]int) {
	healthTracker.Update(state)
}

// GetCurrentHealth returns the current health from the global tracker
func GetCurrentHealth() (SystemHealth, HealthMetricsData) {
	return healthTracker.GetHealth()
}

// HandleHealth is the HTTP handler for GET /api/health
func HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get current state from aggregate (need to fetch from store)
	// For now, use the health tracker's last known state
	health, metrics := GetCurrentHealth()
	info := HealthStates[health]

	response := HealthResponse{
		Health:    health,
		Info:      info,
		Metrics:   metrics,
		State:     nil, // Will be populated by API handler with actual state
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HealthTimestamp returns the current time for health responses
func HealthTimestamp() time.Time {
	return time.Now()
}

// min returns the minimum of two float64 values
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

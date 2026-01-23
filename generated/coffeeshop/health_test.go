package coffeeshop

import (
	"testing"
)

// TestHealthStateClassification tests that each health state is correctly
// classified based on the state conditions. These thresholds mirror
// go-pflow/examples/coffeeshop/simulator.go
func TestHealthStateClassification(t *testing.T) {
	tests := []struct {
		name           string
		state          map[string]int
		expectedHealth SystemHealth
		description    string
	}{
		{
			name: "Healthy - all resources good, no queue",
			state: map[string]int{
				"coffee_beans":    1000,
				"milk":            500,
				"cups":            200,
				"orders_pending":  0,
				"orders_complete": 0,
			},
			expectedHealth: HealthHealthy,
			description:    "Full inventory, no pending orders",
		},
		{
			name: "Healthy - moderate inventory",
			state: map[string]int{
				"coffee_beans":    400,  // 20% of 2000
				"milk":            200,  // 20% of 1000
				"cups":            100,  // 20% of 500
				"orders_pending":  2,
				"orders_complete": 100, // 2/(100+2) = 2% breach rate (under 5%)
			},
			expectedHealth: HealthHealthy,
			description:    "20% inventory, small queue, low breach rate",
		},
		{
			name: "Busy - queue > 5",
			state: map[string]int{
				"coffee_beans":    1000,
				"milk":            500,
				"cups":            200,
				"orders_pending":  6,
				"orders_complete": 200, // 6/(200+6) = 3% breach rate (under 5%)
			},
			expectedHealth: HealthBusy,
			description:    "Queue length 6 triggers busy state",
		},
		{
			name: "Busy - SLA breach rate > 5%",
			state: map[string]int{
				"coffee_beans":    1000,
				"milk":            500,
				"cups":            200,
				"orders_pending":  2,
				"orders_complete": 20, // 2/(20+2) = 9% breach rate
			},
			expectedHealth: HealthBusy,
			description:    "9% SLA breach rate triggers busy state",
		},
		{
			name: "Stressed - queue > 10",
			state: map[string]int{
				"coffee_beans":    1000,
				"milk":            500,
				"cups":            200,
				"orders_pending":  12,
				"orders_complete": 50,
			},
			expectedHealth: HealthStressed,
			description:    "Queue length 12 triggers stressed state",
		},
		{
			name: "Stressed - SLA breach rate > 15%",
			state: map[string]int{
				"coffee_beans":    1000,
				"milk":            500,
				"cups":            200,
				"orders_pending":  5,
				"orders_complete": 20, // 5/(20+5) = 20% breach rate
			},
			expectedHealth: HealthStressed,
			description:    "20% SLA breach rate triggers stressed state",
		},
		{
			name: "SLA Crisis - breach rate > 30%",
			state: map[string]int{
				"coffee_beans":    1000,
				"milk":            500,
				"cups":            200,
				"orders_pending":  15,
				"orders_complete": 30, // 15/(30+15) = 33% breach rate
			},
			expectedHealth: HealthSLACrisis,
			description:    "33% SLA breach rate triggers SLA crisis",
		},
		{
			name: "Inventory Crisis - coffee beans < 10%",
			state: map[string]int{
				"coffee_beans":    150, // 7.5% of 2000
				"milk":            500,
				"cups":            200,
				"orders_pending":  0,
				"orders_complete": 10,
			},
			expectedHealth: HealthInventoryCrisis,
			description:    "Coffee beans at 7.5% triggers inventory crisis",
		},
		{
			name: "Inventory Crisis - milk < 10%",
			state: map[string]int{
				"coffee_beans":    1000,
				"milk":            80, // 8% of 1000
				"cups":            200,
				"orders_pending":  0,
				"orders_complete": 10,
			},
			expectedHealth: HealthInventoryCrisis,
			description:    "Milk at 8% triggers inventory crisis",
		},
		{
			name: "Inventory Crisis - cups < 10%",
			state: map[string]int{
				"coffee_beans":    1000,
				"milk":            500,
				"cups":            40, // 8% of 500
				"orders_pending":  0,
				"orders_complete": 10,
			},
			expectedHealth: HealthInventoryCrisis,
			description:    "Cups at 8% triggers inventory crisis",
		},
		{
			name: "Critical - coffee beans depleted",
			state: map[string]int{
				"coffee_beans":    0,
				"milk":            500,
				"cups":            200,
				"orders_pending":  0,
				"orders_complete": 10,
			},
			expectedHealth: HealthCritical,
			description:    "Zero coffee beans triggers critical state",
		},
		{
			name: "Critical - milk depleted",
			state: map[string]int{
				"coffee_beans":    1000,
				"milk":            0,
				"cups":            200,
				"orders_pending":  0,
				"orders_complete": 10,
			},
			expectedHealth: HealthCritical,
			description:    "Zero milk triggers critical state",
		},
		{
			name: "Critical - cups depleted",
			state: map[string]int{
				"coffee_beans":    1000,
				"milk":            500,
				"cups":            0,
				"orders_pending":  0,
				"orders_complete": 10,
			},
			expectedHealth: HealthCritical,
			description:    "Zero cups triggers critical state",
		},
		{
			name: "Critical - all resources depleted",
			state: map[string]int{
				"coffee_beans":    0,
				"milk":            0,
				"cups":            0,
				"orders_pending":  20,
				"orders_complete": 100,
			},
			expectedHealth: HealthCritical,
			description:    "All resources depleted triggers critical",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			health := ClassifyHealthFromState(tt.state)
			if health != tt.expectedHealth {
				t.Errorf("ClassifyHealthFromState() = %v, want %v\n  %s", health, tt.expectedHealth, tt.description)
			}
		})
	}
}

// TestHealthMetricsTracking tests the rolling metrics tracker
func TestHealthMetricsTracking(t *testing.T) {
	metrics := NewHealthMetrics()

	// Initial state should be healthy
	health, data := metrics.GetHealth()
	if health != HealthHealthy {
		t.Errorf("Initial health = %v, want %v", health, HealthHealthy)
	}

	// Update with healthy state
	metrics.Update(map[string]int{
		"coffee_beans":    1000,
		"milk":            500,
		"cups":            200,
		"orders_pending":  0,
		"orders_complete": 0,
	})

	health, data = metrics.GetHealth()
	if health != HealthHealthy {
		t.Errorf("After healthy update: health = %v, want %v", health, HealthHealthy)
	}
	if len(data.QueueHistory) != 1 {
		t.Errorf("QueueHistory length = %d, want 1", len(data.QueueHistory))
	}

	// Update multiple times to build queue trend
	for i := 0; i < 5; i++ {
		metrics.Update(map[string]int{
			"coffee_beans":    1000,
			"milk":            500,
			"cups":            200,
			"orders_pending":  i * 3, // Growing queue
			"orders_complete": 10,
		})
	}

	health, data = metrics.GetHealth()
	if len(data.QueueHistory) != 6 {
		t.Errorf("QueueHistory length = %d, want 6", len(data.QueueHistory))
	}
	if data.QueueTrend <= 0 {
		t.Errorf("QueueTrend = %v, want > 0 for growing queue", data.QueueTrend)
	}
}

// TestHealthStateTransitions tests transitions between health states
func TestHealthStateTransitions(t *testing.T) {
	metrics := NewHealthMetrics()

	// Start healthy
	metrics.Update(map[string]int{
		"coffee_beans":    1000,
		"milk":            500,
		"cups":            200,
		"orders_pending":  0,
		"orders_complete": 0,
	})

	health, data := metrics.GetHealth()
	if health != HealthHealthy {
		t.Fatalf("Initial health = %v, want %v", health, HealthHealthy)
	}

	// Transition to busy (queue grows)
	metrics.Update(map[string]int{
		"coffee_beans":    1000,
		"milk":            500,
		"cups":            200,
		"orders_pending":  8,
		"orders_complete": 200, // 8/(200+8) = 4% breach rate (under 5%)
	})

	health, data = metrics.GetHealth()
	if health != HealthBusy {
		t.Errorf("After queue growth: health = %v, want %v", health, HealthBusy)
	}
	if data.PreviousHealth != HealthHealthy {
		t.Errorf("PreviousHealth = %v, want %v", data.PreviousHealth, HealthHealthy)
	}
	if data.HealthChangedAt == nil {
		t.Error("HealthChangedAt should be set after state change")
	}

	// Transition to stressed (queue > 10)
	metrics.Update(map[string]int{
		"coffee_beans":    1000,
		"milk":            500,
		"cups":            200,
		"orders_pending":  12,
		"orders_complete": 200, // 12/(200+12) = 5.7% breach rate (under 15%)
	})

	health, _ = metrics.GetHealth()
	if health != HealthStressed {
		t.Errorf("After more queue growth: health = %v, want %v", health, HealthStressed)
	}

	// Transition to inventory crisis
	metrics.Update(map[string]int{
		"coffee_beans":    100, // 5% of max
		"milk":            500,
		"cups":            200,
		"orders_pending":  0,
		"orders_complete": 50,
	})

	health, _ = metrics.GetHealth()
	if health != HealthInventoryCrisis {
		t.Errorf("After inventory drop: health = %v, want %v", health, HealthInventoryCrisis)
	}

	// Transition to critical
	metrics.Update(map[string]int{
		"coffee_beans":    0,
		"milk":            500,
		"cups":            200,
		"orders_pending":  0,
		"orders_complete": 50,
	})

	health, _ = metrics.GetHealth()
	if health != HealthCritical {
		t.Errorf("After resource depletion: health = %v, want %v", health, HealthCritical)
	}
}

// TestHealthStateInfo verifies all health states have proper info
func TestHealthStateInfo(t *testing.T) {
	allStates := []SystemHealth{
		HealthHealthy,
		HealthBusy,
		HealthStressed,
		HealthSLACrisis,
		HealthInventoryCrisis,
		HealthCritical,
	}

	for _, state := range allStates {
		info, ok := HealthStates[state]
		if !ok {
			t.Errorf("HealthStates missing info for %v", state)
			continue
		}

		if info.Key == "" {
			t.Errorf("HealthStates[%v].Key is empty", state)
		}
		if info.Emoji == "" {
			t.Errorf("HealthStates[%v].Emoji is empty", state)
		}
		if info.Label == "" {
			t.Errorf("HealthStates[%v].Label is empty", state)
		}
		if info.Description == "" {
			t.Errorf("HealthStates[%v].Description is empty", state)
		}
	}
}

// TestHealthSeverityOrder verifies severity increases with worse health
func TestHealthSeverityOrder(t *testing.T) {
	orderedStates := []SystemHealth{
		HealthHealthy,
		HealthBusy,
		HealthStressed,
		HealthSLACrisis,      // Same severity as inventory crisis
		HealthInventoryCrisis, // Same severity as SLA crisis
		HealthCritical,
	}

	prevSeverity := -1
	for _, state := range orderedStates {
		info := HealthStates[state]
		if info.Severity < prevSeverity {
			t.Errorf("Severity for %v (%d) is less than previous state (%d)", state, info.Severity, prevSeverity)
		}
		prevSeverity = info.Severity
	}
}

// TestInventoryHealthCalculation verifies inventory health is minimum across resources
func TestInventoryHealthCalculation(t *testing.T) {
	tests := []struct {
		name     string
		state    map[string]int
		minRatio float64
	}{
		{
			name: "coffee beans is minimum",
			state: map[string]int{
				"coffee_beans": 200,  // 10% of 2000
				"milk":         500,  // 50% of 1000
				"cups":         250,  // 50% of 500
			},
			minRatio: 0.10,
		},
		{
			name: "milk is minimum",
			state: map[string]int{
				"coffee_beans": 1000, // 50% of 2000
				"milk":         100,  // 10% of 1000
				"cups":         250,  // 50% of 500
			},
			minRatio: 0.10,
		},
		{
			name: "cups is minimum",
			state: map[string]int{
				"coffee_beans": 1000, // 50% of 2000
				"milk":         500,  // 50% of 1000
				"cups":         50,   // 10% of 500
			},
			minRatio: 0.10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Add required fields
			tt.state["orders_pending"] = 0
			tt.state["orders_complete"] = 20

			metrics := NewHealthMetrics()
			metrics.Update(tt.state)

			_, data := metrics.GetHealth()
			// Allow small floating point tolerance
			if data.InventoryHealth < tt.minRatio-0.01 || data.InventoryHealth > tt.minRatio+0.01 {
				t.Errorf("InventoryHealth = %v, want ~%v", data.InventoryHealth, tt.minRatio)
			}
		})
	}
}

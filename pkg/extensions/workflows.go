package extensions

import (
	"encoding/json"
	"fmt"

	goflowmodel "github.com/pflow-xyz/go-pflow/metamodel"
)

const (
	// WorkflowsExtensionName is the extension name for workflow definitions.
	WorkflowsExtensionName = "petri-pilot/workflows"
)

// WorkflowExtension adds cross-entity workflow orchestration to a Petri net model.
type WorkflowExtension struct {
	goflowmodel.BaseExtension
	Workflows []Workflow `json:"workflows"`
}

// Workflow defines cross-entity orchestration.
type Workflow struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`

	// Trigger defines what starts the workflow.
	Trigger WorkflowTrigger `json:"trigger"`

	// Steps define the workflow sequence.
	Steps []WorkflowStep `json:"steps"`
}

// WorkflowTrigger defines what initiates a workflow.
type WorkflowTrigger struct {
	Type   string `json:"type"` // event, schedule, manual
	Entity string `json:"entity,omitempty"`
	Action string `json:"action,omitempty"`
	Cron   string `json:"cron,omitempty"`
}

// WorkflowStep defines a single step in a workflow.
type WorkflowStep struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"` // action, condition, parallel, wait
	Entity    string            `json:"entity,omitempty"`
	Action    string            `json:"action,omitempty"`
	Condition string            `json:"condition,omitempty"`
	Duration  string            `json:"duration,omitempty"` // For wait steps
	Input     map[string]string `json:"input,omitempty"`    // Mapping from workflow context
	OnSuccess string            `json:"on_success,omitempty"`
	OnFailure string            `json:"on_failure,omitempty"`
}

// NewWorkflowExtension creates a new WorkflowExtension.
func NewWorkflowExtension() *WorkflowExtension {
	return &WorkflowExtension{
		BaseExtension: goflowmodel.NewBaseExtension(WorkflowsExtensionName),
		Workflows:     make([]Workflow, 0),
	}
}

// Validate checks that all workflows are valid.
func (w *WorkflowExtension) Validate(model *goflowmodel.Model) error {
	seen := make(map[string]bool)
	for _, wf := range w.Workflows {
		if seen[wf.ID] {
			return fmt.Errorf("duplicate workflow ID: %s", wf.ID)
		}
		seen[wf.ID] = true

		// Validate trigger type
		validTriggers := map[string]bool{
			"event": true, "schedule": true, "manual": true,
		}
		if !validTriggers[wf.Trigger.Type] {
			return fmt.Errorf("workflow %s: invalid trigger type: %s", wf.ID, wf.Trigger.Type)
		}

		// Validate step IDs are unique
		stepIDs := make(map[string]bool)
		for _, step := range wf.Steps {
			if stepIDs[step.ID] {
				return fmt.Errorf("workflow %s: duplicate step ID: %s", wf.ID, step.ID)
			}
			stepIDs[step.ID] = true
		}

		// Validate step references
		for _, step := range wf.Steps {
			if step.OnSuccess != "" && !stepIDs[step.OnSuccess] {
				return fmt.Errorf("workflow %s step %s: unknown on_success step: %s",
					wf.ID, step.ID, step.OnSuccess)
			}
			if step.OnFailure != "" && !stepIDs[step.OnFailure] {
				return fmt.Errorf("workflow %s step %s: unknown on_failure step: %s",
					wf.ID, step.ID, step.OnFailure)
			}
		}
	}
	return nil
}

// MarshalJSON serializes the workflows.
func (w *WorkflowExtension) MarshalJSON() ([]byte, error) {
	return json.Marshal(w.Workflows)
}

// UnmarshalJSON deserializes the workflows.
func (w *WorkflowExtension) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &w.Workflows)
}

// AddWorkflow adds a workflow to the extension.
func (w *WorkflowExtension) AddWorkflow(wf Workflow) {
	w.Workflows = append(w.Workflows, wf)
}

// WorkflowByID returns a workflow by ID, or nil if not found.
func (w *WorkflowExtension) WorkflowByID(id string) *Workflow {
	for i := range w.Workflows {
		if w.Workflows[i].ID == id {
			return &w.Workflows[i]
		}
	}
	return nil
}

// init registers the workflow extension with the default registry.
func init() {
	goflowmodel.Register(WorkflowsExtensionName, func() goflowmodel.ModelExtension {
		return NewWorkflowExtension()
	})
}

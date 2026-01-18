// Package mcp provides MCP server prompt handlers for guided Petri net workflows.
//
// This file implements three prompts that guide LLMs through complex multi-step tasks:
//   - design-workflow: Guide through creating a new Petri net model from requirements
//   - add-access-control: Guide adding roles and permissions to an existing model
//   - add-views: Guide designing UI views for a model
//
// Each prompt returns structured guidance messages that help LLMs work through
// the design process step-by-step, including JSON Schema examples and best practices.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// handleDesignWorkflowPrompt guides through designing a new workflow from requirements.
func handleDesignWorkflowPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	description := request.Params.Arguments["description"]
	if description == "" {
		description = "a new workflow"
	}

	// Create guidance messages
	messages := []mcp.PromptMessage{
		mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(fmt.Sprintf(`I need to design a Petri net workflow for: %s

Please guide me through the design process step by step.`, description))),
		mcp.NewPromptMessage(mcp.RoleAssistant, mcp.NewTextContent(`I'll help you design a Petri net workflow. Let's break this down into steps:

## Step 1: Identify Places (States)

Places represent the states your workflow can be in. Think about:
- What are the different stages or conditions in your workflow?
- What is the initial state?
- What are the terminal/final states?
- What intermediate states exist?

For example, in a task workflow: pending, in_progress, review, completed

## Step 2: Identify Transitions (Actions/Events)

Transitions are the actions or events that move the workflow between states:
- What actions can users take?
- What events trigger state changes?
- What operations need to happen?

For example: start_task, submit_for_review, approve, reject

## Step 3: Define Arcs (Flow)

Arcs connect places to transitions and transitions to places:
- Which states can trigger which transitions? (place → transition)
- Which transitions produce which states? (transition → place)

For example:
- pending → start_task
- start_task → in_progress
- in_progress → submit_for_review
- submit_for_review → review

## Step 4: Set Initial Marking

Specify which places should have tokens initially:
- Usually one place has initial: 1 (the starting state)
- Other places have initial: 0

## JSON Schema Reference

Here's the structure you should use:

` + "```json" + `
{
  "name": "workflow-name",
  "version": "1.0.0",
  "description": "Description of the workflow",
  "places": [
    {
      "id": "place_id",
      "description": "What this state represents",
      "initial": 0
    }
  ],
  "transitions": [
    {
      "id": "transition_id",
      "description": "What this action does",
      "http_method": "POST",
      "http_path": "/api/resource/{id}/action"
    }
  ],
  "arcs": [
    {"from": "place_id", "to": "transition_id"},
    {"from": "transition_id", "to": "next_place_id"}
  ]
}
` + "```" + `

Now, based on your workflow description, let's start:

1. **What are the main states/stages in your workflow?** (List them out)
2. **Which state should be the starting point?**
3. **What are the possible end states?**`)),
	}

	return mcp.NewGetPromptResult(
		"Guide through designing a new Petri net workflow from requirements",
		messages,
	), nil
}

// handleAddAccessControlPrompt guides adding roles and permissions to an existing model.
func handleAddAccessControlPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	modelJSON := request.Params.Arguments["model"]
	if modelJSON == "" {
		// Provide general guidance without a specific model
		messages := []mcp.PromptMessage{
			mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent("I need to add access control to my Petri net model. How should I structure roles and permissions?")),
			mcp.NewPromptMessage(mcp.RoleAssistant, mcp.NewTextContent(`I'll help you add role-based access control to your Petri net model.

## Step 1: Identify Actors/Users

Think about who interacts with your workflow:
- What types of users exist?
- What are their responsibilities?
- Do some users have overlapping permissions?

For example: regular users, managers, administrators, system processes

## Step 2: Define Roles

Create role definitions with:
- **id**: Unique identifier (e.g., "user", "admin", "reviewer")
- **name**: Human-readable name
- **description**: What this role can do
- **inherits**: Optional list of parent roles for inheritance

Example:
` + "```json" + `
"roles": [
  {"id": "user", "name": "Regular User", "description": "Basic workflow access"},
  {"id": "reviewer", "name": "Reviewer", "description": "Can approve/reject items"},
  {"id": "admin", "name": "Administrator", "inherits": ["user", "reviewer"]}
]
` + "```" + `

## Step 3: Map Transitions to Roles

For each transition in your model, decide:
- Who should be allowed to execute it?
- Are there any additional conditions (guards)?

Create access rules:
` + "```json" + `
"access": [
  {"transition": "submit", "roles": ["user"]},
  {"transition": "approve", "roles": ["reviewer", "admin"]},
  {"transition": "reject", "roles": ["reviewer", "admin"]},
  {"transition": "*", "roles": ["admin"]}
]
` + "```" + `

## Step 4: Add Guard Conditions (Optional)

Guards are expressions that further restrict access:
` + "```json" + `
{"transition": "edit", "roles": ["user"], "guard": "user.id == author_id"}
` + "```" + `

## Questions to Answer:

1. **What user types exist in your system?**
2. **What transitions should each role be allowed to execute?**
3. **Are there any role hierarchies?** (e.g., admin inherits all other roles)
4. **Do any transitions need guard conditions?** (e.g., users can only edit their own items)

Provide your model or describe your access requirements, and I'll help structure the roles and access rules.`)),
		}
		return mcp.NewGetPromptResult(
			"Guide through adding role-based access control to a Petri net model",
			messages,
		), nil
	}

	// Parse the model to provide specific guidance
	var model map[string]interface{}
	if err := json.Unmarshal([]byte(modelJSON), &model); err != nil {
		return nil, fmt.Errorf("invalid model JSON: %w", err)
	}

	// Extract transition IDs
	var transitionIDs []string
	if transitions, ok := model["transitions"].([]interface{}); ok {
		for _, t := range transitions {
			if trans, ok := t.(map[string]interface{}); ok {
				if id, ok := trans["id"].(string); ok {
					transitionIDs = append(transitionIDs, id)
				}
			}
		}
	}

	transitionList := "No transitions found in model"
	if len(transitionIDs) > 0 {
		transitionList = "Available transitions:\n"
		for _, id := range transitionIDs {
			transitionList += fmt.Sprintf("- %s\n", id)
		}
	}

	messages := []mcp.PromptMessage{
		mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(fmt.Sprintf("I need to add access control to this Petri net model:\n\n```json\n%s\n```", modelJSON))),
		mcp.NewPromptMessage(mcp.RoleAssistant, mcp.NewTextContent(fmt.Sprintf(`I'll help you add access control to your model.

## Your Model's Transitions

%s

## Step 1: Define Roles

Based on your workflow, what user types should exist? Consider:
- Who creates/submits items?
- Who reviews/approves items?
- Who has administrative access?

Suggested roles to start:
` + "```json" + `
"roles": [
  {"id": "user", "name": "Regular User"},
  {"id": "admin", "name": "Administrator"}
]
` + "```" + `

## Step 2: Assign Transitions to Roles

For each transition above, decide which roles can execute it:
` + "```json" + `
"access": [
  {"transition": "transition_id", "roles": ["role1", "role2"]}
]
` + "```" + `

## Step 3: Add Guard Conditions (if needed)

If certain transitions should only be allowed under specific conditions:
` + "```json" + `
{"transition": "edit", "roles": ["user"], "guard": "user.id == owner_id"}
` + "```" + `

What roles do you want to define for this workflow?`, transitionList))),
	}

	return mcp.NewGetPromptResult(
		"Guide through adding access control to the provided Petri net model",
		messages,
	), nil
}

// handleAddViewsPrompt guides designing UI views for a model.
func handleAddViewsPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	modelJSON := request.Params.Arguments["model"]
	if modelJSON == "" {
		// Provide general guidance without a specific model
		messages := []mcp.PromptMessage{
			mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent("I need to design UI views for my Petri net model. How should I structure them?")),
			mcp.NewPromptMessage(mcp.RoleAssistant, mcp.NewTextContent(`I'll help you design UI views for your Petri net model.

## Step 1: Identify Data Fields

For each workflow state, what data needs to be displayed or collected?
- What information describes an instance of your workflow?
- What fields are required vs optional?
- What field types are needed? (text, number, date, select, etc.)

Example fields: title, description, assignee, priority, due_date, status

## Step 2: Design View Types

Views can have different purposes:

### List/Table View (kind: "table")
Shows multiple workflow instances in a tabular format:
` + "```json" + `
{
  "id": "task_list",
  "name": "Task List",
  "kind": "table",
  "groups": [
    {
      "id": "main",
      "fields": [
        {"binding": "title", "label": "Title", "type": "text"},
        {"binding": "status", "label": "Status", "type": "text"},
        {"binding": "assignee", "label": "Assignee", "type": "text"}
      ]
    }
  ]
}
` + "```" + `

### Detail/Form View (kind: "form" or "detail")
Shows or edits a single workflow instance:
` + "```json" + `
{
  "id": "task_form",
  "name": "Task Details",
  "kind": "form",
  "groups": [
    {
      "id": "basic_info",
      "name": "Basic Information",
      "fields": [
        {"binding": "title", "label": "Title", "type": "text", "required": true},
        {"binding": "description", "label": "Description", "type": "textarea"}
      ]
    },
    {
      "id": "metadata",
      "name": "Metadata",
      "fields": [
        {"binding": "assignee", "label": "Assigned To", "type": "select"},
        {"binding": "due_date", "label": "Due Date", "type": "date"}
      ]
    }
  ],
  "actions": ["submit", "save_draft"]
}
` + "```" + `

### Card View (kind: "card")
Shows summary information in a card layout for dashboards

## Step 3: Map UI Actions to Transitions

The "actions" array in a view specifies which workflow transitions can be triggered:
` + "```json" + `
"actions": ["start_task", "submit", "approve"]
` + "```" + `

## Field Type Options

- **text**: Single-line text input
- **textarea**: Multi-line text input
- **number**: Numeric input
- **date**: Date picker
- **datetime**: Date and time picker
- **select**: Dropdown selection
- **checkbox**: Boolean checkbox
- **email**: Email input with validation
- **url**: URL input with validation

## Field Properties

- **binding**: The data field name
- **label**: Human-readable label
- **type**: Field type (see above)
- **required**: Boolean, whether field is required
- **readonly**: Boolean, whether field is read-only
- **placeholder**: Placeholder text

## Questions to Answer:

1. **What data fields exist in your workflow?**
2. **Do you need a list view, form view, or both?**
3. **How should fields be grouped?** (e.g., "Basic Info", "Advanced Settings")
4. **Which transitions should be available from each view?**

Provide your model or describe your UI needs, and I'll help structure the views.`)),
		}
		return mcp.NewGetPromptResult(
			"Guide through designing UI views for a Petri net model",
			messages,
		), nil
	}

	// Parse the model to provide specific guidance
	var model map[string]interface{}
	if err := json.Unmarshal([]byte(modelJSON), &model); err != nil {
		return nil, fmt.Errorf("invalid model JSON: %w", err)
	}

	// Extract transition IDs for actions
	var transitionIDs []string
	if transitions, ok := model["transitions"].([]interface{}); ok {
		for _, t := range transitions {
			if trans, ok := t.(map[string]interface{}); ok {
				if id, ok := trans["id"].(string); ok {
					transitionIDs = append(transitionIDs, id)
				}
			}
		}
	}

	transitionList := "No transitions found"
	if len(transitionIDs) > 0 {
		transitionList = "Available transitions for actions:\n"
		for _, id := range transitionIDs {
			transitionList += fmt.Sprintf("- %s\n", id)
		}
	}

	// Extract place IDs to suggest state-based views
	var placeIDs []string
	if places, ok := model["places"].([]interface{}); ok {
		for _, p := range places {
			if place, ok := p.(map[string]interface{}); ok {
				if id, ok := place["id"].(string); ok {
					placeIDs = append(placeIDs, id)
				}
			}
		}
	}

	placeList := "No places found"
	if len(placeIDs) > 0 {
		placeList = "States in your workflow:\n"
		for _, id := range placeIDs {
			placeList += fmt.Sprintf("- %s\n", id)
		}
	}

	messages := []mcp.PromptMessage{
		mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(fmt.Sprintf("I need to design UI views for this Petri net model:\n\n```json\n%s\n```", modelJSON))),
		mcp.NewPromptMessage(mcp.RoleAssistant, mcp.NewTextContent(fmt.Sprintf(`I'll help you design UI views for your model.

## Your Model Overview

%s

%s

## Step 1: Define Data Fields

What information should be displayed/collected for workflow instances?
Common fields might include: title, description, created_at, updated_at, assignee, etc.

## Step 2: Create a List View

A table view to show multiple instances:
` + "```json" + `
{
  "id": "list_view",
  "name": "List View",
  "kind": "table",
  "groups": [
    {
      "id": "columns",
      "fields": [
        {"binding": "id", "label": "ID", "type": "text", "readonly": true},
        {"binding": "title", "label": "Title", "type": "text"},
        {"binding": "status", "label": "Status", "type": "text"}
      ]
    }
  ]
}
` + "```" + `

## Step 3: Create a Detail/Form View

A form view for viewing/editing a single instance:
` + "```json" + `
{
  "id": "detail_view",
  "name": "Detail View",
  "kind": "form",
  "groups": [
    {
      "id": "main_info",
      "name": "Main Information",
      "fields": [
        {"binding": "title", "label": "Title", "type": "text", "required": true},
        {"binding": "description", "label": "Description", "type": "textarea"}
      ]
    }
  ],
  "actions": ["transition_id_1", "transition_id_2"]
}
` + "```" + `

## Step 4: Map Actions

The "actions" array should reference transition IDs that can be triggered from this view.

What data fields do you want to display, and which transitions should be available in your views?`, placeList, transitionList))),
	}

	return mcp.NewGetPromptResult(
		"Guide through designing UI views for the provided Petri net model",
		messages,
	), nil
}

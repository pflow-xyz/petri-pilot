package delegate

import (
	"context"
	"fmt"
	"sync"
)

// Task represents a task to delegate to Copilot.
type Task struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Labels      []string `json:"labels,omitempty"`
}

// BatchResult contains the results of batch delegation.
type BatchResult struct {
	Succeeded []Issue `json:"succeeded"`
	Failed    []struct {
		Task  Task   `json:"task"`
		Error string `json:"error"`
	} `json:"failed"`
}

// DelegateTasks creates multiple issues and assigns them to Copilot in parallel.
func (c *Client) DelegateTasks(ctx context.Context, tasks []Task) (*BatchResult, error) {
	result := &BatchResult{}
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, task := range tasks {
		wg.Add(1)
		go func(t Task) {
			defer wg.Done()

			issue, err := c.createTaskIssue(ctx, t)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				result.Failed = append(result.Failed, struct {
					Task  Task   `json:"task"`
					Error string `json:"error"`
				}{Task: t, Error: err.Error()})
			} else {
				result.Succeeded = append(result.Succeeded, *issue)
			}
		}(task)
	}

	wg.Wait()
	return result, nil
}

func (c *Client) createTaskIssue(ctx context.Context, task Task) (*Issue, error) {
	labels := task.Labels
	if len(labels) == 0 {
		labels = []string{"copilot"}
	}

	payload := map[string]any{
		"title":  task.Title,
		"body":   task.Description,
		"labels": labels,
	}

	issue, err := c.doCreateIssue(ctx, payload)
	if err != nil {
		return nil, err
	}

	if err := c.assignToCopilot(ctx, issue.Number); err != nil {
		fmt.Printf("warning: failed to assign #%d to Copilot: %v\n", issue.Number, err)
	}

	return issue, nil
}

// DelegateRoadmap reads a ROADMAP.md file and delegates all "Next Phase" items.
func (c *Client) DelegateRoadmap(ctx context.Context, roadmapContent string) (*BatchResult, error) {
	tasks := ParseRoadmapTasks(roadmapContent)
	if len(tasks) == 0 {
		return nil, fmt.Errorf("no tasks found in roadmap")
	}
	return c.DelegateTasks(ctx, tasks)
}

// ParseRoadmapTasks extracts tasks from ROADMAP.md content.
// Looks for "## Next Phase" section and extracts ### headings as tasks.
func ParseRoadmapTasks(content string) []Task {
	// Simple parser - looks for ### headings under ## Next Phase
	var tasks []Task
	inNextPhase := false
	lines := splitLines(content)

	var currentTask *Task
	var currentDescription string

	for _, line := range lines {
		if len(line) >= 13 && line[:13] == "## Next Phase" {
			inNextPhase = true
			continue
		}

		if inNextPhase && len(line) >= 2 && line[:2] == "## " {
			// Hit next major section, stop
			break
		}

		if inNextPhase && len(line) >= 4 && line[:4] == "### " {
			// Save previous task if exists
			if currentTask != nil {
				currentTask.Description = currentDescription
				tasks = append(tasks, *currentTask)
			}

			// Start new task
			title := line[4:]
			// Remove numbering like "1. " or "2. "
			if len(title) > 3 && title[1] == '.' && title[2] == ' ' {
				title = title[3:]
			}
			currentTask = &Task{
				Title:  title,
				Labels: []string{"copilot", "roadmap"},
			}
			currentDescription = ""
			continue
		}

		if currentTask != nil {
			currentDescription += line + "\n"
		}
	}

	// Don't forget the last task
	if currentTask != nil {
		currentTask.Description = currentDescription
		tasks = append(tasks, *currentTask)
	}

	return tasks
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

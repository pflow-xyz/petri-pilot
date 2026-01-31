// Package delegate provides a client for delegating app generation to GitHub Copilot.
package delegate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Client communicates with GitHub to delegate work to Copilot.
type Client struct {
	httpClient *http.Client
	token      string
	owner      string
	repo       string
	baseURL    string
}

// NewClient creates a new GitHub delegate client.
// Token can be empty to use GITHUB_TOKEN env var.
func NewClient(owner, repo, token string) *Client {
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		token:      token,
		owner:      owner,
		repo:       repo,
		baseURL:    "https://api.github.com",
	}
}

// AppRequest describes a new application to generate.
type AppRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Features    []string `json:"features,omitempty"` // auth, rbac, admin, events, e2e
	Complexity  string   `json:"complexity,omitempty"` // simple, medium, complex
}

// Issue represents a GitHub issue.
type Issue struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	HTMLURL   string    `json:"html_url"`
	CreatedAt time.Time `json:"created_at"`
}

// PullRequest represents a GitHub pull request.
type PullRequest struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	Draft     bool      `json:"draft"`
	HTMLURL   string    `json:"html_url"`
	Additions int       `json:"additions"`
	Deletions int       `json:"deletions"`
	CreatedAt time.Time `json:"created_at"`
	Head      struct {
		Ref string `json:"ref"`
	} `json:"head"`
}

// WorkflowRun represents a GitHub Actions workflow run.
type WorkflowRun struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Status     string    `json:"status"` // queued, in_progress, completed
	Conclusion string    `json:"conclusion"` // success, failure, cancelled
	HeadBranch string    `json:"head_branch"`
	HTMLURL    string    `json:"html_url"`
	CreatedAt  time.Time `json:"created_at"`
}

// AgentStatus summarizes Copilot agent activity.
type AgentStatus struct {
	ActiveRuns  []WorkflowRun  `json:"active_runs"`
	OpenPRs     []PullRequest  `json:"open_prs"`
	RecentIssues []Issue       `json:"recent_issues"`
}

// CreateAppRequest creates a new issue requesting app generation.
func (c *Client) CreateAppRequest(ctx context.Context, req AppRequest) (*Issue, error) {
	body := c.formatAppRequestBody(req)

	payload := map[string]any{
		"title":  fmt.Sprintf("[App] %s", req.Name),
		"body":   body,
		"labels": []string{"app-request", "copilot"},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/repos/%s/%s/issues", c.baseURL, c.owner, c.repo)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create issue: %s - %s", resp.Status, string(body))
	}

	var issue Issue
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return nil, err
	}

	// Assign to Copilot
	if err := c.assignToCopilot(ctx, issue.Number); err != nil {
		// Don't fail if assignment fails, issue is still created
		fmt.Fprintf(os.Stderr, "warning: failed to assign to Copilot: %v\n", err)
	}

	return &issue, nil
}

func (c *Client) formatAppRequestBody(req AppRequest) string {
	features := ""
	for _, f := range req.Features {
		features += fmt.Sprintf("- [x] %s\n", f)
	}
	if features == "" {
		features = "- [ ] Authentication (GitHub OAuth)\n- [ ] Role-based access control\n- [ ] Admin dashboard\n- [ ] Event history viewer\n- [ ] E2E tests (Playwright)\n"
	}

	complexity := req.Complexity
	if complexity == "" {
		complexity = "Medium (5-10 states)"
	}

	return fmt.Sprintf(`### Application Description

%s

### App Name

%s

### Features

%s

### Complexity

%s
`, req.Description, req.Name, features, complexity)
}

func (c *Client) assignToCopilot(ctx context.Context, issueNumber int) error {
	payload := map[string]any{
		"assignees": []string{"Copilot"},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/repos/%s/%s/issues/%d/assignees", c.baseURL, c.owner, c.repo, issueNumber)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to assign: %s - %s", resp.Status, string(body))
	}

	return nil
}

// GetAgentStatus returns the current status of Copilot agents.
func (c *Client) GetAgentStatus(ctx context.Context) (*AgentStatus, error) {
	status := &AgentStatus{}

	// Get active workflow runs
	runs, err := c.getWorkflowRuns(ctx, "in_progress")
	if err != nil {
		return nil, fmt.Errorf("failed to get runs: %w", err)
	}
	for _, run := range runs {
		if run.Name == "Running Copilot coding agent" {
			status.ActiveRuns = append(status.ActiveRuns, run)
		}
	}

	// Get open PRs from Copilot branches
	prs, err := c.getPullRequests(ctx, "open")
	if err != nil {
		return nil, fmt.Errorf("failed to get PRs: %w", err)
	}
	for _, pr := range prs {
		if len(pr.Head.Ref) > 8 && pr.Head.Ref[:8] == "copilot/" {
			status.OpenPRs = append(status.OpenPRs, pr)
		}
	}

	// Get recent app-request issues
	issues, err := c.getIssues(ctx, "app-request")
	if err != nil {
		return nil, fmt.Errorf("failed to get issues: %w", err)
	}
	status.RecentIssues = issues

	return status, nil
}

func (c *Client) getWorkflowRuns(ctx context.Context, status string) ([]WorkflowRun, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/actions/runs?status=%s&per_page=20", c.baseURL, c.owner, c.repo, status)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		WorkflowRuns []WorkflowRun `json:"workflow_runs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.WorkflowRuns, nil
}

func (c *Client) getPullRequests(ctx context.Context, state string) ([]PullRequest, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls?state=%s&per_page=20", c.baseURL, c.owner, c.repo, state)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %s: %s", resp.Status, string(body))
	}

	var prs []PullRequest
	if err := json.NewDecoder(resp.Body).Decode(&prs); err != nil {
		return nil, err
	}

	return prs, nil
}

func (c *Client) getIssues(ctx context.Context, label string) ([]Issue, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/issues?labels=%s&per_page=10", c.baseURL, c.owner, c.repo, label)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var issues []Issue
	if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
		return nil, err
	}

	return issues, nil
}

// WaitForCompletion blocks until all active Copilot runs complete.
func (c *Client) WaitForCompletion(ctx context.Context, pollInterval time.Duration) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		status, err := c.GetAgentStatus(ctx)
		if err != nil {
			return err
		}

		if len(status.ActiveRuns) == 0 {
			return nil
		}

		fmt.Printf("Waiting for %d agent(s)...\n", len(status.ActiveRuns))
		time.Sleep(pollInterval)
	}
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

// doCreateIssue creates an issue from a payload map.
func (c *Client) doCreateIssue(ctx context.Context, payload map[string]any) (*Issue, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/repos/%s/%s/issues", c.baseURL, c.owner, c.repo)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create issue: %s - %s", resp.Status, string(body))
	}

	var issue Issue
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return nil, err
	}

	return &issue, nil
}

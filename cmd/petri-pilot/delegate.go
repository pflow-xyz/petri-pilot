package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/pflow-xyz/petri-pilot/pkg/delegate"
)

func cmdDelegate(args []string) {
	if len(args) == 0 {
		printDelegateUsage()
		os.Exit(1)
	}

	subcmd := args[0]
	switch subcmd {
	case "app":
		cmdDelegateApp(args[1:])
	case "roadmap":
		cmdDelegateRoadmap(args[1:])
	case "status":
		cmdDelegateStatus(args[1:])
	case "wait":
		cmdDelegateWait(args[1:])
	case "help", "-h", "--help":
		printDelegateUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown delegate subcommand: %s\n", subcmd)
		printDelegateUsage()
		os.Exit(1)
	}
}

func printDelegateUsage() {
	fmt.Println(`petri-pilot delegate - Delegate tasks to GitHub Copilot

Usage:
  petri-pilot delegate <subcommand> [options]

Subcommands:
  app       Request a new application to be generated
  roadmap   Delegate all tasks from ROADMAP.md
  status    Check status of Copilot agents
  wait      Wait for all Copilot agents to complete

Examples:
  # Request a new app
  petri-pilot delegate app my-app -d "A task management workflow"

  # Delegate roadmap tasks
  petri-pilot delegate roadmap

  # Check agent status
  petri-pilot delegate status

  # Wait for completion
  petri-pilot delegate wait --timeout 1h`)
}

func cmdDelegateApp(args []string) {
	fs := flag.NewFlagSet("delegate app", flag.ExitOnError)
	description := fs.String("d", "", "Application description (required)")
	features := fs.String("f", "", "Features: auth,rbac,admin,events,e2e (comma-separated)")
	complexity := fs.String("c", "medium", "Complexity: simple, medium, complex")
	owner := fs.String("owner", "pflow-xyz", "GitHub repository owner")
	repo := fs.String("repo", "petri-pilot", "GitHub repository name")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "Error: app name required")
		fmt.Fprintln(os.Stderr, "Usage: petri-pilot delegate app [options] <name>")
		os.Exit(1)
	}

	if *description == "" {
		fmt.Fprintln(os.Stderr, "Error: -d (description) is required")
		os.Exit(1)
	}

	name := fs.Arg(0)

	var featureList []string
	if *features != "" {
		for _, f := range splitComma(*features) {
			featureList = append(featureList, f)
		}
	}

	client := delegate.NewClient(*owner, *repo, "")

	req := delegate.AppRequest{
		Name:        name,
		Description: *description,
		Features:    featureList,
		Complexity:  *complexity,
	}

	ctx := context.Background()
	issue, err := client.CreateAppRequest(ctx, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created issue #%d: %s\n", issue.Number, issue.HTMLURL)
	fmt.Println("Copilot will work on this autonomously and create a PR when ready.")
}

func cmdDelegateRoadmap(args []string) {
	fs := flag.NewFlagSet("delegate roadmap", flag.ExitOnError)
	owner := fs.String("owner", "pflow-xyz", "GitHub repository owner")
	repo := fs.String("repo", "petri-pilot", "GitHub repository name")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	filename := "ROADMAP.md"
	if fs.NArg() > 0 {
		filename = fs.Arg(0)
	}

	content, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", filename, err)
		os.Exit(1)
	}

	tasks := delegate.ParseRoadmapTasks(string(content))
	if len(tasks) == 0 {
		fmt.Fprintf(os.Stderr, "No tasks found in %s\n", filename)
		os.Exit(1)
	}

	fmt.Printf("Found %d tasks in %s:\n", len(tasks), filename)
	for i, task := range tasks {
		fmt.Printf("  %d. %s\n", i+1, task.Title)
	}
	fmt.Println()

	client := delegate.NewClient(*owner, *repo, "")
	ctx := context.Background()

	result, err := client.DelegateTasks(ctx, tasks)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Delegated %d tasks:\n", len(result.Succeeded))
	for _, issue := range result.Succeeded {
		fmt.Printf("  #%d: %s\n", issue.Number, issue.Title)
	}

	if len(result.Failed) > 0 {
		fmt.Printf("\nFailed %d tasks:\n", len(result.Failed))
		for _, f := range result.Failed {
			fmt.Printf("  %s: %s\n", f.Task.Title, f.Error)
		}
	}
}

func cmdDelegateStatus(args []string) {
	fs := flag.NewFlagSet("delegate status", flag.ExitOnError)
	owner := fs.String("owner", "pflow-xyz", "GitHub repository owner")
	repo := fs.String("repo", "petri-pilot", "GitHub repository name")
	jsonOutput := fs.Bool("json", false, "Output as JSON")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	client := delegate.NewClient(*owner, *repo, "")
	ctx := context.Background()

	status, err := client.GetAgentStatus(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(status)
		return
	}

	if len(status.ActiveRuns) > 0 {
		fmt.Printf("Active Copilot Agents: %d\n", len(status.ActiveRuns))
		for _, run := range status.ActiveRuns {
			duration := time.Since(run.CreatedAt).Round(time.Second)
			fmt.Printf("  - %s (running %s)\n", run.HeadBranch, duration)
			fmt.Printf("    %s\n", run.HTMLURL)
		}
	} else {
		fmt.Println("No active Copilot agents")
	}

	if len(status.OpenPRs) > 0 {
		fmt.Printf("\nOpen PRs from Copilot: %d\n", len(status.OpenPRs))
		for _, pr := range status.OpenPRs {
			draft := ""
			if pr.Draft {
				draft = " [DRAFT]"
			}
			fmt.Printf("  #%d: %s%s\n", pr.Number, pr.Title, draft)
			fmt.Printf("    +%d -%d | %s\n", pr.Additions, pr.Deletions, pr.HTMLURL)
		}
	}

	if len(status.RecentIssues) > 0 {
		fmt.Printf("\nRecent App Requests: %d\n", len(status.RecentIssues))
		for _, issue := range status.RecentIssues {
			fmt.Printf("  #%d: %s [%s]\n", issue.Number, issue.Title, issue.State)
		}
	}
}

func cmdDelegateWait(args []string) {
	fs := flag.NewFlagSet("delegate wait", flag.ExitOnError)
	owner := fs.String("owner", "pflow-xyz", "GitHub repository owner")
	repo := fs.String("repo", "petri-pilot", "GitHub repository name")
	timeout := fs.Duration("timeout", 30*time.Minute, "Maximum time to wait")
	jsonOutput := fs.Bool("json", false, "Output as JSON when complete")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	client := delegate.NewClient(*owner, *repo, "")

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	fmt.Println("Waiting for Copilot agents to complete...")
	if err := client.WaitForCompletion(ctx, 30*time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("All agents completed!")

	// Show final status
	status, err := client.GetAgentStatus(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to get final status: %v\n", err)
		return
	}

	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(status)
		return
	}

	if len(status.OpenPRs) > 0 {
		fmt.Printf("\nReady for review: %d PRs\n", len(status.OpenPRs))
		for _, pr := range status.OpenPRs {
			fmt.Printf("  #%d: %s\n", pr.Number, pr.HTMLURL)
		}
	}
}

func splitComma(s string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			if i > start {
				result = append(result, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}

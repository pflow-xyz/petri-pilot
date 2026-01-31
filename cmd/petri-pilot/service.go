package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/pflow-xyz/petri-pilot/pkg/mcp"
)

func cmdService(args []string) {
	if len(args) == 0 {
		printServiceUsage()
		os.Exit(1)
	}

	subcmd := args[0]
	switch subcmd {
	case "list", "ls":
		cmdServiceList(args[1:])
	case "stop":
		cmdServiceStop(args[1:])
	case "logs":
		cmdServiceLogs(args[1:])
	case "stats":
		cmdServiceStats(args[1:])
	case "health":
		cmdServiceHealth(args[1:])
	case "help", "-h", "--help":
		printServiceUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown service subcommand: %s\n", subcmd)
		printServiceUsage()
		os.Exit(1)
	}
}

func printServiceUsage() {
	fmt.Println(`petri-pilot service - Manage running services

Usage:
  petri-pilot service <command> [arguments]

Commands:
  list, ls    List all running services
  stop        Stop a running service by ID
  logs        View logs from a service
  stats       Get runtime statistics for a service
  health      Check health endpoint of a service

Examples:
  petri-pilot service list
  petri-pilot service stop svc-1
  petri-pilot service logs svc-1 -n 100
  petri-pilot service stats svc-1
  petri-pilot service health svc-1`)
}

func cmdServiceList(args []string) {
	fs := flag.NewFlagSet("service list", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "Output as JSON")

	fs.Usage = func() {
		w := fs.Output()
		fmt.Fprintln(w, `petri-pilot service list - List all running services

Usage:
  petri-pilot service list [options]

Options:`)
		fs.PrintDefaults()
		fmt.Fprintln(w, `
Examples:
  petri-pilot service list            List services in table format
  petri-pilot service list -json      List services as JSON`)
	}

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	services, err := mcp.ListServices()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing services: %v\n", err)
		os.Exit(1)
	}

	if *jsonOutput {
		output, _ := json.MarshalIndent(services, "", "  ")
		fmt.Println(string(output))
		return
	}

	if len(services) == 0 {
		fmt.Println("No running services.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tURL\tPID\tSTATUS")
	for _, svc := range services {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", svc.ID, svc.Name, svc.URL, svc.PID, svc.Status)
	}
	w.Flush()
}

func cmdServiceStop(args []string) {
	fs := flag.NewFlagSet("service stop", flag.ExitOnError)

	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `petri-pilot service stop - Stop a running service

Usage:
  petri-pilot service stop <service-id>

Arguments:
  service-id    The ID of the service to stop (e.g., svc-1)

Examples:
  petri-pilot service stop svc-1`)
	}

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "Error: service ID required")
		fmt.Fprintln(os.Stderr, "Usage: petri-pilot service stop <service-id>")
		os.Exit(1)
	}

	serviceID := fs.Arg(0)

	if err := mcp.StopService(serviceID); err != nil {
		fmt.Fprintf(os.Stderr, "Error stopping service: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Service %s stopped successfully\n", serviceID)
}

func cmdServiceLogs(args []string) {
	fs := flag.NewFlagSet("service logs", flag.ExitOnError)
	lines := fs.Int("n", 50, "Number of lines to show")
	stream := fs.String("stream", "both", "Log stream: stdout, stderr, or both")

	fs.Usage = func() {
		w := fs.Output()
		fmt.Fprintln(w, `petri-pilot service logs - View logs from a service

Usage:
  petri-pilot service logs [options] <service-id>

Arguments:
  service-id    The ID of the service (e.g., svc-1)

Options:`)
		fs.PrintDefaults()
		fmt.Fprintln(w, `
Examples:
  petri-pilot service logs svc-1              Show last 50 lines
  petri-pilot service logs -n 100 svc-1       Show last 100 lines
  petri-pilot service logs -stream stderr svc-1  Show only stderr`)
	}

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "Error: service ID required")
		fmt.Fprintln(os.Stderr, "Usage: petri-pilot service logs <service-id> [-n lines] [-stream stdout|stderr|both]")
		os.Exit(1)
	}

	serviceID := fs.Arg(0)

	logLines, err := mcp.ServiceLogs(serviceID, *lines, *stream)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting logs: %v\n", err)
		os.Exit(1)
	}

	for _, line := range logLines {
		fmt.Println(line)
	}
}

func cmdServiceStats(args []string) {
	fs := flag.NewFlagSet("service stats", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "Output as JSON")

	fs.Usage = func() {
		w := fs.Output()
		fmt.Fprintln(w, `petri-pilot service stats - Get runtime statistics for a service

Usage:
  petri-pilot service stats [options] <service-id>

Arguments:
  service-id    The ID of the service (e.g., svc-1)

Options:`)
		fs.PrintDefaults()
		fmt.Fprintln(w, `
Examples:
  petri-pilot service stats svc-1        Show stats in human-readable format
  petri-pilot service stats -json svc-1  Show stats as JSON`)
	}

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "Error: service ID required")
		fmt.Fprintln(os.Stderr, "Usage: petri-pilot service stats <service-id>")
		os.Exit(1)
	}

	serviceID := fs.Arg(0)

	stats, err := mcp.GetServiceStats(serviceID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting stats: %v\n", err)
		os.Exit(1)
	}

	if *jsonOutput {
		output, _ := json.MarshalIndent(stats, "", "  ")
		fmt.Println(string(output))
		return
	}

	fmt.Printf("Service:  %s (%s)\n", stats.Name, stats.ID)
	fmt.Printf("Status:   %s\n", stats.Status)
	fmt.Printf("URL:      %s\n", stats.URL)
	fmt.Printf("PID:      %d\n", stats.PID)
	fmt.Printf("Uptime:   %s\n", stats.Uptime)
	if stats.MemoryRSS > 0 {
		fmt.Printf("Memory:   %.1f MB (RSS)\n", float64(stats.MemoryRSS)/1024/1024)
	}
	if stats.Health != nil {
		fmt.Printf("Health:   %s\n", stats.Health.Status)
	}
}

func cmdServiceHealth(args []string) {
	fs := flag.NewFlagSet("service health", flag.ExitOnError)

	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `petri-pilot service health - Check health endpoint of a service

Usage:
  petri-pilot service health <service-id>

Arguments:
  service-id    The ID of the service (e.g., svc-1)

Examples:
  petri-pilot service health svc-1`)
	}

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "Error: service ID required")
		fmt.Fprintln(os.Stderr, "Usage: petri-pilot service health <service-id>")
		os.Exit(1)
	}

	serviceID := fs.Arg(0)

	health, err := mcp.CheckServiceHealth(serviceID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking health: %v\n", err)
		os.Exit(1)
	}

	output, _ := json.MarshalIndent(health, "", "  ")
	fmt.Println(string(output))
}

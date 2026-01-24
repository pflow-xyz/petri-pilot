package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/pflow-xyz/petri-pilot/pkg/serve"
)

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 0, "Port to run the service on (default: 8080 or PORT env)")

	fs.Usage = func() {
		w := fs.Output()
		fmt.Fprintln(w, `petri-pilot serve - Run a registered service

Usage:
  petri-pilot serve [options] [service-name]

  Without a service name, lists all available services.
  With a service name, starts that service.

Options:`)
		fs.PrintDefaults()
		fmt.Fprintln(w, `
Examples:
  petri-pilot serve                   List available services
  petri-pilot serve blog-post         Run the blog-post service
  petri-pilot serve -port 3000 myapp  Run myapp on port 3000`)
	}

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// If no service name provided, list available services
	if fs.NArg() == 0 {
		services := serve.List()
		if len(services) == 0 {
			fmt.Println("No services registered.")
			fmt.Println("\nTo generate services, use:")
			fmt.Println("  petri-pilot codegen -submodule -o generated/<name> <model.json>")
			os.Exit(0)
		}

		fmt.Println("Available services:")
		for _, name := range services {
			fmt.Printf("  %s\n", name)
		}
		fmt.Println("\nUsage: petri-pilot serve [options] <service-name>")
		fmt.Println("\nOptions:")
		fmt.Println("  -port int    Port to run the service on (default: 8080 or PORT env)")
		os.Exit(0)
	}

	serviceName := fs.Arg(0)

	// Check if service exists
	if _, ok := serve.Get(serviceName); !ok {
		fmt.Fprintf(os.Stderr, "Error: service %q not found\n", serviceName)
		fmt.Fprintf(os.Stderr, "\nAvailable services:\n")
		for _, name := range serve.List() {
			fmt.Fprintf(os.Stderr, "  %s\n", name)
		}
		os.Exit(1)
	}

	// Run the service
	opts := serve.DefaultOptions()
	if *port > 0 {
		opts.Port = *port
	}

	if err := serve.Run(serviceName, opts); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

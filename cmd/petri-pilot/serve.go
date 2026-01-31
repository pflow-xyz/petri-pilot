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
		fmt.Fprintln(w, `petri-pilot serve - Run registered services

Usage:
  petri-pilot serve [options] [service-names...]

  Without service names, lists all available services.
  With one service name, starts that service.
  With multiple service names, runs all on one port (mounted at /{name}/).

Options:`)
		fs.PrintDefaults()
		fmt.Fprintln(w, `
Examples:
  petri-pilot serve                        List available services
  petri-pilot serve blog-post              Run the blog-post service
  petri-pilot serve -port 3000 myapp       Run myapp on port 3000
  petri-pilot serve tic-tac-toe coffeeshop Run both services together`)
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

	serviceNames := fs.Args()

	// Check if all services exist
	for _, name := range serviceNames {
		if _, ok := serve.Get(name); !ok {
			fmt.Fprintf(os.Stderr, "Error: service %q not found\n", name)
			fmt.Fprintf(os.Stderr, "\nAvailable services:\n")
			for _, n := range serve.List() {
				fmt.Fprintf(os.Stderr, "  %s\n", n)
			}
			os.Exit(1)
		}
	}

	// Run the service(s)
	opts := serve.DefaultOptions()
	if *port > 0 {
		opts.Port = *port
	}

	if err := serve.RunMultiple(serviceNames, opts); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

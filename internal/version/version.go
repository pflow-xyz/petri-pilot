// Package version provides build-time version information.
package version

// Version is set at build time via ldflags:
//
//	go build -ldflags "-X github.com/pflow-xyz/petri-pilot/internal/version.Version=v1.0.0"
var Version = "dev"

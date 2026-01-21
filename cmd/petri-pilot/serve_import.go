//go:build !noserve

package main

// Import generated services for auto-registration.
// This file is excluded when building with -tags noserve,
// which allows the codegen command to run without depending on generated packages.
import _ "github.com/pflow-xyz/petri-pilot/generated"

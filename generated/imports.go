// Package generated registers the apps produced by the current generator.
//
// examples/ holds the frozen reference apps; this tree holds output from the
// generator as it stands today. Importing a package here is what makes its
// init() run and register the service, so an app that is not listed exists on
// disk and is reachable from nothing.
package generated

import (
	// cafe is the composed app: counter, pantry and staff as three nets, with
	// the scenario endpoints mounted on the composition root.
	_ "github.com/pflow-xyz/petri-pilot/generated/cafe"
)

// Package generated provides auto-discovery of all generated Petri-pilot services.
// This file imports all service packages to trigger their init() functions,
// which register them with the serve package.
//
// This file is auto-generated. Do not edit manually.
// Regenerate with: make codegen-all
package generated

import (
	_ "github.com/pflow-xyz/petri-pilot/generated/blogpost"
	_ "github.com/pflow-xyz/petri-pilot/generated/coffeeshop"
	_ "github.com/pflow-xyz/petri-pilot/generated/ecommercecheckout"
	_ "github.com/pflow-xyz/petri-pilot/generated/erc20token"
	_ "github.com/pflow-xyz/petri-pilot/generated/jobapplication"
	_ "github.com/pflow-xyz/petri-pilot/generated/loanapplication"
	_ "github.com/pflow-xyz/petri-pilot/generated/orderprocessing"
	_ "github.com/pflow-xyz/petri-pilot/generated/supportticket"
	_ "github.com/pflow-xyz/petri-pilot/generated/taskmanager"
	_ "github.com/pflow-xyz/petri-pilot/generated/testaccess"
	_ "github.com/pflow-xyz/petri-pilot/generated/tictactoe"
	_ "github.com/pflow-xyz/petri-pilot/generated/tictactoev2"
)

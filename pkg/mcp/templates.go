package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mark3labs/mcp-go/mcp"
)

// petri_template returns ready-to-use Petri net models for common DeFi
// primitives. The LLM-driven design loop benefits hugely from concrete
// starting points; building "Uniswap V2" from scratch in chat is hard,
// loading the template and asking "now add a fee tier" is easy.
//
// Templates are plain model JSON with description metadata. Each is small
// enough to read in one sitting and contains the minimum places/
// transitions/arcs to represent the mechanism. Compose freely with
// petri_extend, petri_visualize, petri_ode, petri_optimize, etc.

func templateTool() mcp.Tool {
	return mcp.NewTool("petri_template",
		mcp.WithDescription("DeFi/tokenomics model templates ready for analysis. Without a name argument, lists all available templates with one-line descriptions. With a name, returns the full model JSON ready to feed into petri_visualize / petri_ode / petri_optimize / etc."),
		mcp.WithString("name",
			mcp.Description("Template name (e.g. 'constant_product_amm'). Omit to list all templates with descriptions"),
		),
	)
}

type defiTemplate struct {
	Name        string
	Category    string
	Description string
	Notes       string
	Model       string
}

func defiTemplates() map[string]defiTemplate {
	return map[string]defiTemplate{
		"constant_product_amm": {
			Name:        "constant_product_amm",
			Category:    "amm",
			Description: "Uniswap V2-style constant-product AMM. Two reserves (x, y) trade against each other; swap_x_for_y and swap_y_for_x are mass-action transitions whose rates set trade intensity. Equilibrium under mass-action approximates the x·y=k invariant.",
			Notes:       "For real AMM math you'd want discrete-step swap traces (use petri_simulate). The ODE view is good for showing flow direction under continuous trading pressure.",
			Model: `{
  "modelType": "petriNet",
  "version": "v0",
  "places": [
    {"id": "reserve_x", "initial": 100, "x": 80, "y": 80},
    {"id": "reserve_y", "initial": 100, "x": 80, "y": 240},
    {"id": "lp_shares", "initial": 100, "x": 80, "y": 400},
    {"id": "fees_x", "x": 360, "y": 80},
    {"id": "fees_y", "x": 360, "y": 240}
  ],
  "transitions": [
    {"id": "swap_x_for_y", "x": 220, "y": 80},
    {"id": "swap_y_for_x", "x": 220, "y": 240},
    {"id": "collect_fee_x", "x": 460, "y": 80},
    {"id": "collect_fee_y", "x": 460, "y": 240}
  ],
  "arcs": [
    {"from": "reserve_x", "to": "swap_x_for_y"},
    {"from": "swap_x_for_y", "to": "reserve_y"},
    {"from": "swap_x_for_y", "to": "fees_x"},
    {"from": "reserve_y", "to": "swap_y_for_x"},
    {"from": "swap_y_for_x", "to": "reserve_x"},
    {"from": "swap_y_for_x", "to": "fees_y"},
    {"from": "fees_x", "to": "collect_fee_x"},
    {"from": "fees_y", "to": "collect_fee_y"}
  ]
}`,
		},

		"lending_pool": {
			Name:        "lending_pool",
			Category:    "lending",
			Description: "Aave/Compound-style lending pool. Suppliers deposit into available_liquidity; borrowers draw against it producing outstanding_debt. Interest accrues to suppliers; liquidation closes underwater positions.",
			Notes:       "Use petri_rate_scan over the borrow rate to see how utilization affects equilibrium liquidity.",
			Model: `{
  "modelType": "petriNet",
  "version": "v0",
  "places": [
    {"id": "supplier_balance", "initial": 1000, "x": 80, "y": 80},
    {"id": "available_liquidity", "x": 280, "y": 80},
    {"id": "outstanding_debt", "x": 480, "y": 80},
    {"id": "borrower_balance", "x": 680, "y": 80},
    {"id": "interest_earned", "x": 280, "y": 240},
    {"id": "liquidations", "x": 480, "y": 240}
  ],
  "transitions": [
    {"id": "supply", "x": 180, "y": 80},
    {"id": "borrow", "x": 380, "y": 80},
    {"id": "repay", "x": 580, "y": 80},
    {"id": "accrue_interest", "x": 380, "y": 200},
    {"id": "liquidate", "x": 480, "y": 320}
  ],
  "arcs": [
    {"from": "supplier_balance", "to": "supply"},
    {"from": "supply", "to": "available_liquidity"},
    {"from": "available_liquidity", "to": "borrow"},
    {"from": "borrow", "to": "borrower_balance"},
    {"from": "borrow", "to": "outstanding_debt"},
    {"from": "borrower_balance", "to": "repay"},
    {"from": "repay", "to": "available_liquidity"},
    {"from": "outstanding_debt", "to": "accrue_interest"},
    {"from": "accrue_interest", "to": "interest_earned"},
    {"from": "outstanding_debt", "to": "liquidate"},
    {"from": "liquidate", "to": "liquidations"}
  ]
}`,
		},

		"staking_pool": {
			Name:        "staking_pool",
			Category:    "yield",
			Description: "Staking pool with reward accrual. Users stake tokens that earn rewards over time; rewards can be claimed or restaked (compound). Standard yield-farming primitive.",
			Notes:       "Pair with petri_ode_sweep over the emission rate to find the sweet spot between stake retention and treasury drain.",
			Model: `{
  "modelType": "petriNet",
  "version": "v0",
  "places": [
    {"id": "wallet", "initial": 1000, "x": 80, "y": 80},
    {"id": "staked", "x": 280, "y": 80},
    {"id": "pending_rewards", "x": 480, "y": 80},
    {"id": "claimed_rewards", "x": 680, "y": 80},
    {"id": "treasury", "initial": 10000, "x": 280, "y": 240}
  ],
  "transitions": [
    {"id": "stake", "x": 180, "y": 80},
    {"id": "unstake", "x": 280, "y": 160},
    {"id": "emit_rewards", "x": 380, "y": 240},
    {"id": "claim", "x": 580, "y": 80},
    {"id": "compound", "x": 480, "y": 160}
  ],
  "arcs": [
    {"from": "wallet", "to": "stake"},
    {"from": "stake", "to": "staked"},
    {"from": "staked", "to": "unstake"},
    {"from": "unstake", "to": "wallet"},
    {"from": "treasury", "to": "emit_rewards"},
    {"from": "emit_rewards", "to": "pending_rewards"},
    {"from": "pending_rewards", "to": "claim"},
    {"from": "claim", "to": "claimed_rewards"},
    {"from": "pending_rewards", "to": "compound"},
    {"from": "compound", "to": "staked"}
  ]
}`,
		},

		"vesting_schedule": {
			Name:        "vesting_schedule",
			Category:    "tokenomics",
			Description: "Linear vesting with cliff. Locked tokens are released continuously to claimable balance; the holder can claim them at will. Adjust vest rate to model 12/24/48-month vesting.",
			Notes:       "Use petri_ode to plot the unlock curve. For investor/team vesting modeling, run several instances at different cliff dates with petri_extend.",
			Model: `{
  "modelType": "petriNet",
  "version": "v0",
  "places": [
    {"id": "locked", "initial": 1000, "x": 80, "y": 150},
    {"id": "claimable", "x": 280, "y": 150},
    {"id": "claimed", "x": 480, "y": 150}
  ],
  "transitions": [
    {"id": "vest", "x": 180, "y": 150},
    {"id": "claim", "x": 380, "y": 150}
  ],
  "arcs": [
    {"from": "locked", "to": "vest"},
    {"from": "vest", "to": "claimable"},
    {"from": "claimable", "to": "claim"},
    {"from": "claim", "to": "claimed"}
  ]
}`,
		},

		"dao_governance": {
			Name:        "dao_governance",
			Category:    "governance",
			Description: "DAO proposal lifecycle. Proposals enter the queue, accumulate yes/no votes, then execute or fail. Use to study quorum dynamics, voter participation, and threshold effects.",
			Notes:       "Pair with petri_optimize: max execution rate + min failure rate as a function of quorum and voting period rates.",
			Model: `{
  "modelType": "petriNet",
  "version": "v0",
  "places": [
    {"id": "draft", "initial": 5, "x": 80, "y": 150},
    {"id": "active", "x": 280, "y": 150},
    {"id": "yes_votes", "x": 480, "y": 80},
    {"id": "no_votes", "x": 480, "y": 220},
    {"id": "passed", "x": 680, "y": 80},
    {"id": "failed", "x": 680, "y": 220},
    {"id": "executed", "x": 880, "y": 80}
  ],
  "transitions": [
    {"id": "submit", "x": 180, "y": 150},
    {"id": "vote_yes", "x": 380, "y": 80},
    {"id": "vote_no", "x": 380, "y": 220},
    {"id": "tally_pass", "x": 580, "y": 80},
    {"id": "tally_fail", "x": 580, "y": 220},
    {"id": "execute", "x": 780, "y": 80}
  ],
  "arcs": [
    {"from": "draft", "to": "submit"},
    {"from": "submit", "to": "active"},
    {"from": "active", "to": "vote_yes"},
    {"from": "vote_yes", "to": "yes_votes"},
    {"from": "vote_yes", "to": "active"},
    {"from": "active", "to": "vote_no"},
    {"from": "vote_no", "to": "no_votes"},
    {"from": "vote_no", "to": "active"},
    {"from": "yes_votes", "to": "tally_pass"},
    {"from": "tally_pass", "to": "passed"},
    {"from": "no_votes", "to": "tally_fail"},
    {"from": "tally_fail", "to": "failed"},
    {"from": "passed", "to": "execute"},
    {"from": "execute", "to": "executed"}
  ]
}`,
		},

		"liquidation_cascade": {
			Name:        "liquidation_cascade",
			Category:    "risk",
			Description: "Cascading liquidations: healthy positions become risky, then liquidatable, then absorbed by the protocol. Sets up the classic 'liquidations beget more liquidations' loop that drives DeFi crashes.",
			Notes:       "Use petri_stochastic to see how variance in price shocks propagates through the cascade — the deterministic ODE smooths over the tail risk.",
			Model: `{
  "modelType": "petriNet",
  "version": "v0",
  "places": [
    {"id": "healthy", "initial": 100, "x": 80, "y": 150},
    {"id": "at_risk", "x": 280, "y": 150},
    {"id": "liquidatable", "x": 480, "y": 150},
    {"id": "liquidated", "x": 680, "y": 80},
    {"id": "recovered", "x": 680, "y": 220},
    {"id": "protocol_loss", "x": 880, "y": 80}
  ],
  "transitions": [
    {"id": "price_drop", "x": 180, "y": 150},
    {"id": "deepen", "x": 380, "y": 150},
    {"id": "execute_liq", "x": 580, "y": 80},
    {"id": "rebound", "x": 580, "y": 220},
    {"id": "bad_debt", "x": 780, "y": 80}
  ],
  "arcs": [
    {"from": "healthy", "to": "price_drop"},
    {"from": "price_drop", "to": "at_risk"},
    {"from": "at_risk", "to": "deepen"},
    {"from": "deepen", "to": "liquidatable"},
    {"from": "liquidatable", "to": "execute_liq"},
    {"from": "execute_liq", "to": "liquidated"},
    {"from": "at_risk", "to": "rebound"},
    {"from": "rebound", "to": "recovered"},
    {"from": "liquidated", "to": "bad_debt"},
    {"from": "bad_debt", "to": "protocol_loss"}
  ]
}`,
		},

		"yield_aggregator": {
			Name:        "yield_aggregator",
			Category:    "yield",
			Description: "Yearn-style yield aggregator routing capital across N strategies. Compounds harvested rewards back into principal. Use petri_optimize to find capital allocation that maximizes APY across strategies with different yield-vs-risk profiles.",
			Notes:       "Pair with petri_ode_sensitivity to identify which strategy's rate matters most.",
			Model: `{
  "modelType": "petriNet",
  "version": "v0",
  "places": [
    {"id": "user_deposits", "initial": 1000, "x": 80, "y": 150},
    {"id": "principal", "x": 280, "y": 150},
    {"id": "strategy_a", "x": 480, "y": 50},
    {"id": "strategy_b", "x": 480, "y": 150},
    {"id": "strategy_c", "x": 480, "y": 250},
    {"id": "harvested", "x": 680, "y": 150}
  ],
  "transitions": [
    {"id": "deposit", "x": 180, "y": 150},
    {"id": "route_a", "x": 380, "y": 50},
    {"id": "route_b", "x": 380, "y": 150},
    {"id": "route_c", "x": 380, "y": 250},
    {"id": "harvest_a", "x": 580, "y": 50},
    {"id": "harvest_b", "x": 580, "y": 150},
    {"id": "harvest_c", "x": 580, "y": 250},
    {"id": "compound", "x": 480, "y": 350}
  ],
  "arcs": [
    {"from": "user_deposits", "to": "deposit"},
    {"from": "deposit", "to": "principal"},
    {"from": "principal", "to": "route_a"}, {"from": "route_a", "to": "strategy_a"},
    {"from": "principal", "to": "route_b"}, {"from": "route_b", "to": "strategy_b"},
    {"from": "principal", "to": "route_c"}, {"from": "route_c", "to": "strategy_c"},
    {"from": "strategy_a", "to": "harvest_a"}, {"from": "harvest_a", "to": "harvested"},
    {"from": "strategy_b", "to": "harvest_b"}, {"from": "harvest_b", "to": "harvested"},
    {"from": "strategy_c", "to": "harvest_c"}, {"from": "harvest_c", "to": "harvested"},
    {"from": "harvested", "to": "compound"},
    {"from": "compound", "to": "principal"}
  ]
}`,
		},
	}
}

func handleTemplate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	templates := defiTemplates()
	name := request.GetString("name", "")
	if name == "" {
		return listTemplates(templates)
	}
	tpl, ok := templates[name]
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("unknown template %q. Call petri_template with no arguments to list available templates", name)), nil
	}

	// Verify the model parses so we don't hand back a broken JSON if someone
	// edited the definitions and a brace went missing.
	if _, err := parseModelV2(tpl.Model); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("internal: template %q has invalid model JSON: %v", name, err)), nil
	}

	result := struct {
		Name        string `json:"name"`
		Category    string `json:"category"`
		Description string `json:"description"`
		Notes       string `json:"notes,omitempty"`
		Model       json.RawMessage `json:"model"`
	}{
		Name:        tpl.Name,
		Category:    tpl.Category,
		Description: tpl.Description,
		Notes:       tpl.Notes,
		Model:       json.RawMessage(tpl.Model),
	}
	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal template: %v", err)), nil
	}
	return mcp.NewToolResultText(string(out)), nil
}

func listTemplates(templates map[string]defiTemplate) (*mcp.CallToolResult, error) {
	names := make([]string, 0, len(templates))
	for k := range templates {
		names = append(names, k)
	}
	sort.Strings(names)

	type listEntry struct {
		Name        string `json:"name"`
		Category    string `json:"category"`
		Description string `json:"description"`
	}
	entries := make([]listEntry, 0, len(names))
	for _, n := range names {
		t := templates[n]
		entries = append(entries, listEntry{Name: t.Name, Category: t.Category, Description: t.Description})
	}
	wrapper := struct {
		Total     int         `json:"total"`
		Templates []listEntry `json:"templates"`
		Hint      string      `json:"hint"`
	}{
		Total:     len(entries),
		Templates: entries,
		Hint:      "Load any template by calling petri_template with name=<template_name>. The returned model can be fed directly into petri_visualize, petri_ode, petri_optimize, etc.",
	}
	out, err := json.MarshalIndent(wrapper, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal list: %v", err)), nil
	}
	return mcp.NewToolResultText(string(out)), nil
}


package serve

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	seoBaseURL  = "https://pilot.pflow.xyz"
	seoSiteName = "Petri-Pilot"
	seoOGImage  = "https://pilot.pflow.xyz/logo-square.png"
)

type pageMeta struct {
	Title       string
	Description string
}

// frontendMeta maps a frontend directory name to per-page SEO metadata used
// for share cards and search snippets. Routes not in this map are left
// untouched — keeps SEO injection opt-in per page.
var frontendMeta = map[string]pageMeta{
	// Tools
	"code-to-flow": {
		Title:       "Code to Flow — Source Code as a Petri Net",
		Description: "Paste source code from any language and get a formal, executable Petri net. Detects state machines, control flow, concurrency, and resource pools.",
	},

	// Concepts
	"learn": {
		Title:       "What is a Petri Net? — A Visual Primer",
		Description: "Places, transitions, arcs, and tokens — the four concepts behind every Petri net model. Interactive examples, no math required.",
	},
	"patterns": {
		Title:       "Thinking in Petri Nets — Patterns and Properties",
		Description: "State machines, fork/join, resource pools, mutual exclusion, deadlock — the design patterns behind Petri net modeling, with a suggested learning path.",
	},
	"advanced": {
		Title:       "ODE Simulation and Prediction with Petri Nets",
		Description: "How mass-action kinetics turns a Petri net into a system of differential equations, and how the Tic-Tac-Toe net discovers strategy without search.",
	},
	"zk-intro": {
		Title:       "Zero-Knowledge Proofs for Petri Nets",
		Description: "How gnark circuits verify Petri net transitions cryptographically — MiMC hashing, enabledness checks, and Groth16 proofs from net topology alone.",
	},
	"optimal-play": {
		Title:       "Verifiable Computation from Petri Nets",
		Description: "Petri net → ODE → ZK → on-chain. A 33-place, 35-transition net plus a Groth16 proof verify each move on-chain at ~450k gas.",
	},

	// Games (GameNet)
	"tic-tac-toe": {
		Title:       "Tic-Tac-Toe Petri Net Simulator",
		Description: "Nine board cells as places, player moves as transitions. ODE simulation computes strategic value from net topology alone — no search, no minimax.",
	},
	"zk-tic-tac-toe": {
		Title:       "ZK Tic-Tac-Toe — Verified Moves with gnark",
		Description: "The same game as tic-tac-toe, but every move is verified cryptographically using gnark circuits. Proves valid play without revealing strategy.",
	},
	"texas-holdem": {
		Title:       "Texas Hold'em — ODE Strategic Analysis",
		Description: "Multi-player state machine with five players, four betting rounds, and role-based actions. Turn tokens control whose transition fires next.",
	},

	// Resources (ResourceNet)
	"coffeeshop": {
		Title:       "Coffee Shop — Inventory and Flow as a Petri Net",
		Description: "Weighted arcs consume beans, milk, and cups per order. Rates drive continuous flow. The ODE predicts when supplies run out.",
	},
	"knapsack": {
		Title:       "Knapsack Optimizer — Petri Net + Mass-Action",
		Description: "Classic 0/1 knapsack as a Petri net. Items compete for limited capacity via mass-action kinetics, and the ODE reveals optimal item selection.",
	},
	"producer-consumer": {
		Title:       "Producer-Consumer — Bounded Buffer Petri Net",
		Description: "A producer and consumer share a fixed-size buffer. Capacity tokens block the producer when full and the consumer when empty.",
	},
	"vet-clinic": {
		Title:       "Vet Clinic Scheduling Simulator",
		Description: "Staff, rooms, and equipment as resource tokens. Service-mix routing with piecewise ODE, plus a financial dashboard for revenue and utilization.",
	},
	"predator-prey": {
		Title:       "Predator-Prey — Lotka-Volterra as a Petri Net",
		Description: "Two places, three transitions, and mass-action kinetics produce classic population oscillations. Tune parameters and observe phase-space orbits.",
	},
	"enzyme-kinetics": {
		Title:       "Enzyme Kinetics — Michaelis-Menten as a Petri Net",
		Description: "Substrate binds enzyme to form a complex, which either unbinds or catalyzes into product. Four places and three transitions reproduce the saturation curve.",
	},

	// Workflows (WorkflowNet)
	"loan-approval": {
		Title:       "Loan Approval — Fork-Join Pipeline",
		Description: "Parallel credit and employment reviews with role-based access. The Petri net enforces the synchronization barrier before final decision.",
	},
	"hiring-pipeline": {
		Title:       "Hiring Pipeline — Multi-Stage Petri Net",
		Description: "Phone screen, parallel technical and culture interviews, then offer. Four roles control different stages of a kanban-style pipeline.",
	},

	// Computation (ComputationNet)
	"tcp-handshake": {
		Title:       "TCP Handshake — Protocol State Machine",
		Description: "The classic 3-way handshake (SYN, SYN-ACK, ACK) and connection teardown as two parallel state machines linked by shared transitions.",
	},
	"thermostat": {
		Title:       "Thermostat — Bang-Bang Feedback Control",
		Description: "A bang-bang controller with ODE dynamics. Temperature rises when the heater is on and decays naturally — watch the feedback loop oscillate.",
	},
	"dining-philosophers": {
		Title:       "Dining Philosophers — Concurrency and Deadlock",
		Description: "Five philosophers, five forks, one classic problem. Step through transitions to observe mutual exclusion and circular wait.",
	},
	"stoplight": {
		Title:       "Stoplight — Cyclic Petri Net State Machine",
		Description: "A traffic light cycling through red, green, and yellow. The simplest example of a live, bounded Petri net with no deadlocks.",
	},
	"galton-board": {
		Title:       "Galton Board — Binomial Distribution from Topology",
		Description: "256 balls drop through 8 rows of pegs. Each peg is two competing transitions; topology alone produces Pascal's triangle.",
	},
	"march-madness-2026": {
		Title:       "March Madness 2026 — NCAA Bracket as a Petri Net",
		Description: "96 places and 240 transitions encode the full tournament. The incidence matrix bridges ODE, Monte Carlo, and an analytical formula.",
	},

	// Classification (ClassificationNet)
	"poker-hand": {
		Title:       "Poker Hand — Structural Classifier",
		Description: "Cards are tokens in rank and suit places. Pattern-detection transitions fire when the right combination is present — pairs, flushes, straights.",
	},
}

var (
	descriptionMetaRE = regexp.MustCompile(`(?i)<meta\s+name=["']description["']`)
	titleElementRE    = regexp.MustCompile(`(?is)<title>.*?</title>`)
)

// frontendNameFromPath extracts the frontend directory name from a path like
// "frontends/tic-tac-toe" → "tic-tac-toe". Returns "" for landing or unknown.
func frontendNameFromPath(frontendPath string) string {
	parts := strings.Split(filepath.ToSlash(strings.TrimSuffix(frontendPath, "/")), "/")
	for i, part := range parts {
		if part == "frontends" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// injectSEO adds per-page meta tags before </head> when the frontend has
// curated metadata and the HTML doesn't already declare its own description
// (the landing page declares its own and is left untouched).
func injectSEO(html []byte, frontendName string) []byte {
	if frontendName == "" {
		return html
	}
	meta, ok := frontendMeta[frontendName]
	if !ok {
		return html
	}
	if descriptionMetaRE.Match(html) {
		return html
	}

	canonical := seoBaseURL + "/" + frontendName + "/"
	desc := htmlAttrEscape(meta.Description)
	title := htmlAttrEscape(meta.Title)

	tags := fmt.Sprintf(`  <meta name="description" content="%s">
  <meta name="robots" content="index, follow, max-image-preview:large">
  <link rel="canonical" href="%s">
  <meta property="og:type" content="website">
  <meta property="og:site_name" content="%s">
  <meta property="og:title" content="%s">
  <meta property="og:description" content="%s">
  <meta property="og:url" content="%s">
  <meta property="og:image" content="%s">
  <meta property="og:image:width" content="256">
  <meta property="og:image:height" content="256">
  <meta property="og:image:alt" content="Petri-Pilot logo">
  <meta property="og:locale" content="en_US">
  <meta name="twitter:card" content="summary">
  <meta name="twitter:title" content="%s">
  <meta name="twitter:description" content="%s">
  <meta name="twitter:image" content="%s">
`, desc, canonical, seoSiteName, title, desc, canonical, seoOGImage, title, desc, seoOGImage)

	html = bytes.Replace(html, []byte("</head>"), []byte(tags+"</head>"), 1)

	// Replace <title>...</title> with the curated title so the browser tab and
	// the OG title agree. Skipped if no <title> element exists.
	titleTag := []byte(fmt.Sprintf("<title>%s</title>", htmlBodyEscape(meta.Title)))
	html = titleElementRE.ReplaceAll(html, titleTag)

	return html
}

func htmlAttrEscape(s string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		`"`, "&quot;",
		"<", "&lt;",
		">", "&gt;",
	).Replace(s)
}

func htmlBodyEscape(s string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	).Replace(s)
}

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// petri_explain returns structured explanations of the math the MCP uses.
// Each concept is designed as a "digestible chunk": intuition first, then
// the formula in plain Unicode (no LaTeX assumed), then a short derivation
// and a worked example with concrete numbers, then suggested follow-up
// tools to try. The format optimizes for chat readability without
// depending on math-typography rendering.
//
// Used by both end users ("explain X to me") and LLMs driving the MCP
// ("before I run this, what does it actually compute?"). Concept names
// are stable and lowercase_snake so they're predictable to call.

func explainTool() mcp.Tool {
	return mcp.NewTool("petri_explain",
		mcp.WithDescription("Explain the math behind any concept used in this MCP — formulas, intuition, derivations, worked examples, and what tool to try next. Without arguments, lists all available concepts. With a topic name, returns the full explanation."),
		mcp.WithString("topic",
			mcp.Description("Concept name (e.g. 'impermanent_loss', 'constant_product_amm'). Omit to list all available topics with one-line summaries"),
		),
	)
}

type concept struct {
	Name       string
	Category   string
	Summary    string   // one-line for the list view
	Intuition  string   // 1-2 sentence plain-English version
	Formula    string   // Unicode-formatted core equation
	Derivation string   // why the formula has the shape it does
	Example    string   // concrete numbers
	SeeAlso    []string // related MCP tools
}

func defiConcepts() map[string]concept {
	return map[string]concept{
		"petri_net_basics": {
			Name:       "petri_net_basics",
			Category:   "core",
			Summary:    "Places hold tokens, transitions move them. The bedrock model.",
			Intuition:  "A Petri net is a directed bipartite graph where token-holding 'places' (circles) connect to 'transitions' (bars) via arcs. A transition fires when all its inputs have enough tokens; firing consumes input tokens and produces output tokens.",
			Formula:    "Transition t can fire if   ∀ p ∈ •t : m(p) ≥ w(p, t)\nAfter firing:   m'(p) = m(p) − w(p, t) + w(t, p)",
			Derivation: "The marking m: P → ℕ assigns a token count to each place. Pre-arcs •t are inputs to transition t with weight w(p, t). Post-arcs t• are outputs. Firing is atomic — all inputs consumed, all outputs produced, in one indivisible step.",
			Example:    "Coffee shop: place 'order_pending' has 2 tokens, place 'barista_idle' has 1. Transition 'start_brew' has both as inputs (weight 1 each). It can fire: consumes 1 from each, produces 1 in 'brewing'. After: order_pending=1, barista_idle=0, brewing=1.",
			SeeAlso:    []string{"petri_visualize", "petri_validate", "petri_simulate"},
		},

		"mass_action_kinetics": {
			Name:       "mass_action_kinetics",
			Category:   "ode",
			Summary:    "petri_ode's continuous-time firing rate = rate constant × product of input concentrations, once each — NOT the SSA propensity's C(m, weight).",
			Intuition:  "When we go from discrete firings to continuous flow, a transition's instantaneous rate becomes the product of its reactants — but petri_ode multiplies each input place's concentration in exactly once, regardless of that arc's weight. Weight only decides how much moves once the transition fires, not how fast it fires.",
			Formula:    "rate(t) = k_t · ∏_(p ∈ •t) m(p)          (petri_ode: weight NOT in the rate)\n\nCompare petri_stochastic's propensity, which IS weight-sensitive:\n  a(t) = k_t · ∏_(p ∈ •t, kinetic) C(m(p), w(p,t))",
			Derivation: "This is go-pflow's actual implementation (solver/ode.go buildODEFunction), not the textbook chemistry convention — verified directly against source, since the two are easy to conflate: it is not 'weight-1 mass action generalizes to C(m,w) at higher weight,' it is 'petri_ode never looks at weight when computing the rate at all.' A weight-20 arc feeding a transition contributes the same single factor of the source place's concentration as a weight-1 arc would; the 20 only appears when applying the computed flux to update du/dt. petri_stochastic's propensity is the real combinatorial mass action, C(m,w), which coincides with petri_ode's rate only at weight 1. Above weight 1 the two engines are computing genuinely different rate laws, not just a continuous vs. discrete version of the same one — see petri_ode's own model-refusal notes and docs/engine-selection.md (go-pflow) for a worked case (stochastic/consistency_test.go's dimerisation case exists specifically to assert the two disagree on a weighted arc, as a regression guard).\n\nThe non-kinetic exception applies identically to both engines: an input arc marked non-kinetic is left out of the product entirely (in petri_ode's single-power term and in petri_stochastic's C(m,w) term alike). Mass action is the right law for chemistry and the wrong one for a service system — a barista is a prerequisite for making a drink, not a catalyst, and a fuller pantry does not make a drink pour faster. A non-kinetic input still gates the firing and is still consumed by it; it simply does not scale the rate.",
			Example:    "Coffee shop, coffee_beans feeds make_espresso at weight 20, k=1.0:\n  petri_ode:         rate = 1.0 × order_pending × coffee_beans × cups   (weight 20 absent)\n  petri_stochastic:  a    = 1.0 × order_pending × C(coffee_beans, 20) × cups\nWith coffee_beans near its declared capacity, petri_ode refuses the whole\nmodel outright (Diverged=true) rather than pretend the pantry is infinite —\nsee petri_ode's caveats and the simulation_choice topic.",
			SeeAlso:    []string{"petri_ode", "petri_stochastic", "petri_rate_scan", "petri_ode_sensitivity", "simulation_choice"},
		},

		"constant_product_amm": {
			Name:       "constant_product_amm",
			Category:   "defi",
			Summary:    "The Uniswap V2 invariant: x · y = k. Trades preserve the product.",
			Intuition:  "A liquidity pool holds reserves of two assets, X and Y. Every swap must keep the product x·y equal to its pre-trade value k. This invariant determines exactly how much Y you get for a given amount of X.",
			Formula:    "Spot price:           P = y / x\nOutput for input Δx:  Δy = y · Δx / (x + Δx)\nWith fee f:           Δy = y · Δx · (1−f) / (x + Δx · (1−f))",
			Derivation: "Pre-trade invariant: x · y = k. Post-trade: (x + Δx)(y − Δy) = k. Solve for Δy: Δy = y · Δx / (x + Δx). The fee removes a fraction of the input from the effective trade size before applying the invariant.",
			Example:    "Pool: 100 ETH, 200,000 USDC. Spot price = 2000 USDC/ETH.\nSwap 1 ETH with 0.3% fee:\n  Δx_after_fee = 1 × 0.997 = 0.997\n  Δy = 200000 × 0.997 / (100 + 0.997) = 1974.69 USDC\n  Effective price: 1974.69 USDC/ETH\n  Slippage: 1 − 1974.69/2000 = 1.27%",
			SeeAlso:    []string{"petri_amm_quote", "petri_amm_depth", "petri_template"},
		},

		"impermanent_loss": {
			Name:       "impermanent_loss",
			Category:   "defi",
			Summary:    "Why LPs underperform HODL when prices move: IL(r) = 2√r/(1+r) − 1.",
			Intuition:  "An LP holds half value in each token. The AMM auto-rebalances: as one price rises, the LP ends up holding less of the appreciating asset than a passive holder. The shortfall is 'impermanent' — it only crystallizes when you withdraw.",
			Formula:    "IL(r) = 2√r / (1 + r) − 1,   where r = P_new / P_old\n\nKnown points:\n  r = 1:    IL =   0%\n  r = 2:    IL ≈  −5.72%\n  r = 4:    IL = −20.0%\n  r = 5:    IL ≈ −25.5%\n  r = 0.5:  IL ≈  −5.72%  (symmetric in log(r))",
			Derivation: "Pool reserves satisfy x·y = k, so the LP holds √(k·P) of each (geometric-mean value). HODL gives arithmetic mean of the new portfolio. AM ≥ GM, with equality only at r=1. The ratio LP_value / HODL_value works out to 2√r/(1+r). Subtract 1 to express as a loss.",
			Example:    "Start: 100 ETH @ $1000 = $100K, plus 100K USDC = $200K total LP.\nETH price doubles (r=2): pool rebalances to ~70.7 ETH and ~141,400 USDC. Value: 70.7·2000 + 141400 ≈ $282,800.\nHODL would be: 100·2000 + 100,000 = $300,000.\nLP is $17,200 short = IL of ≈ −5.7%.",
			SeeAlso:    []string{"petri_amm_il", "petri_template"},
		},

		"gillespie_ssa": {
			Name:       "gillespie_ssa",
			Category:   "stochastic",
			Summary:    "Discrete random firings. Sample wait time, sample which transition.",
			Intuition:  "When token counts are small, individual firings matter — you can't smooth them into a continuous flow. The Stochastic Simulation Algorithm runs each firing as a random event in continuous time.",
			Formula:    "Per step:\n  a_i = k_i · ∏_(p ∈ •t_i, kinetic) C(m(p), w(p, t_i))    propensities\n  A   = Σ a_i                          total rate\n  τ   ~ Exp(A)                         wait time to next event\n  P(t_i) = a_i / A                     which transition fires\n\nThe product runs over the KINETIC inputs only. An input arc marked\nnon-kinetic is left out of it — but if that input is unsatisfied the\ntransition is not enabled and a_i = 0 regardless, and when t_i fires the\ninput is consumed like any other.",
			Derivation: "In a continuous-time Markov chain with total leaving rate A, the time to the next jump is Exponential(A) — that's a basic Markov property. Which jump happens is independent of when, and proportional to each transition's propensity.\n\nWhy some inputs are excluded from the product: multiplying every input into the rate is the law of mass action, which is right for chemistry and wrong for a service system. A barista is a prerequisite for making a drink, not a reactant, and a fuller pantry does not make a drink pour faster. Left kinetic, a staff pool made two drinks in progress each finish twice as fast, and a queue arc made pickup scale with the number of people waiting — so the wait, and the walkouts, stopped responding to staffing at all. Marking those arcs non-kinetic keeps them as gates and as consumption while taking them out of the rate law. Both SSA engines honour this; a continuous solve cannot express it, which is why petri_ode caveats or refuses such a model.",
			Example:    "Two transitions: a_1 = 2, a_2 = 3. Total A = 5.\n  Wait time: τ ~ Exp(5), mean = 1/5 = 0.2 time units.\n  Probability t_1 fires: 2/5 = 40%.\n  Probability t_2 fires: 3/5 = 60%.\nDraw u_1, u_2 ∈ U(0,1) → τ = −ln(u_1)/5; if u_2 < 0.4 fire t_1 else t_2.\n\nNon-kinetic input: 'start_brew' takes 1 order (kinetic) and 1 free barista\n(non-kinetic), k = 720/h. With 4 orders waiting and 3 baristas free:\n  a = 720 × 4 = 2880/h        (the 3 does not enter)\n  With 0 baristas free: a = 0 — not enabled.\nFiring still consumes one order and one barista.",
			SeeAlso:    []string{"petri_stochastic", "petri_simulate"},
		},

		"euler_maruyama": {
			Name:       "euler_maruyama",
			Category:   "stochastic",
			Summary:    "Simple SDE integrator: drift × dt + noise × √dt × Z.",
			Intuition:  "An SDE has both a smooth 'drift' direction and a noisy 'diffusion' kick. Euler-Maruyama takes one small step of each at every time interval — drift over dt, plus a Gaussian kick scaled by √dt.",
			Formula:    "dx = μ(x, t) dt + σ(x, t) dW\n\nDiscretized:   x(t+dt) = x(t) + μ · dt + σ · √dt · Z,   Z ~ N(0,1)\n\nGeometric Brownian Motion (σ scales with state):\n               x(t+dt) = x(t) + μ · dt + σ · x · √dt · Z",
			Derivation: "Brownian increments satisfy dW ~ N(0, dt), so √dt·N(0,1). For multiplicative noise — appropriate when the quantity can't go negative (prices, balances, populations) — σ scales with the current state. GBM is the canonical choice for asset prices.",
			Example:    "S(0) = 100, σ = 0.3, dt = 0.01 (one path step):\n  Drift = 0 (no transitions firing in this pure-noise example)\n  Diffusion = 0.3 × 100 × √0.01 × Z = 3 · Z\n  S(0.01) ≈ 100 + 3Z\nOne stdev range: 97 to 103. Over 100 steps the path random-walks with variance summing.",
			SeeAlso:    []string{"petri_sde"},
		},

		"correlated_sde": {
			Name:       "correlated_sde",
			Category:   "stochastic",
			Summary:    "Multi-asset SDE with correlated noise via Cholesky decomposition.",
			Intuition:  "Real portfolios have correlated assets — BTC and ETH move together. To simulate them, draw independent normals and multiply by the Cholesky factor of the correlation matrix to get correlated noise with the right structure.",
			Formula:    "Correlation matrix R (N×N, symmetric, PSD)\n  R = L · Lᵀ                            Cholesky factor\n  Z ~ N(0, I_N)                          independent normals\n  W = L · Z                              correlated, E[W] = 0, Cov(W) = R",
			Derivation: "If Z is iid N(0,1), then for any matrix L, the vector W = LZ has covariance Cov(W) = E[LZZᵀLᵀ] = L·I·Lᵀ = LLᵀ. So to get noise with correlation R, decompose R = LLᵀ and apply L.",
			Example:    "Two assets with ρ = 0.7:\n  R = [[1, 0.7], [0.7, 1]]\n  L = [[1, 0], [0.7, 0.714]]  (lower triangular Cholesky)\n  Draw Z = (z_1, z_2) ~ N(0, I).\n  W = (z_1, 0.7·z_1 + 0.714·z_2)\n  W is bivariate normal with the requested 0.7 correlation.",
			SeeAlso:    []string{"petri_sde"},
		},

		"simulation_choice": {
			Name:       "simulation_choice",
			Category:   "guide",
			Summary:    "When to use ODE vs SSA vs SDE — four rules, not a size threshold. Full version: go-pflow docs/engine-selection.md.",
			Intuition:  "Population size is the SECOND question, not the first. The first is whether the model has a firing instant the ODE cannot see at all — a read arc, an inhibitor, a reached capacity, a guard. If it does, petri_ode does not give a slightly-wrong answer; it refuses outright (Diverged=true), because a continuous flow has nothing to test the constraint against.",
			Formula:    "1. Read arc / inhibitor / reached capacity / guard present?  →  petri_ode refuses; use petri_stochastic\n2. No gating, large population, want the average?            →  petri_ode\n3. No gating, small counts, want the actual distribution?     →  petri_stochastic\n4. Any input arc weight > 1?  →  expect the two engines to disagree quantitatively (see mass_action_kinetics) — not a bug, a different rate law\n5. Exogenous continuous noise on top of the ODE (price, demand)?  →  petri_sde, today\n6. Intrinsic firing noise, cheap enough to sweep?  →  not shipped yet (go-pflow ROADMAP G6)",
			Derivation: "Verified against the shared coffeeshop.json fixture, not asserted: it declares capacity on coffee_beans/milk/cups, and its restock transitions actually reach it, so petri_ode's Forecast refuses the whole model rather than silently assume an infinite pantry — the exact text is 'capacity is declared on [coffee_beans milk cups] but is a post-firing bound, which has no continuous analogue.' Where the ODE DOES answer, it is the true mean-field limit of the SSA ensemble mean (go-pflow's LLN consistency gate proves this, not just states it) — and the gap between them at finite population is a real, provably-shrinking effect: ~5% of N at N=1000, provably below 2% and provably smaller at N=10,000 on the same net scaled up, on go-pflow's SIR test case. So '10,000 is fine, 2 is not' is directionally right but not the FIRST filter — a net with only 2 tokens and no gating at all is still a legitimate petri_ode question if you only want the average, and a net with a million tokens but a reachable capacity still gets refused.",
			Example:    "Coffee shop as shipped (has reachable capacities): petri_ode refuses; petri_stochastic answers ~840 beans remaining, ~32 orders/hour (seed 42, 20 realizations).\nSame shop with capacity removed, 2 orders pending: no refusal, but variance is the answer → petri_stochastic anyway.\nSame shop, capacity removed, 10,000 orders queued: petri_ode is the mean-field answer and 1000× faster.\ncoffee_beans feeding make_espresso at weight 20: petri_ode and petri_stochastic compute genuinely different rate laws above weight 1 — see mass_action_kinetics.\nETH price over 1 year: continuous + exogenous noise → petri_sde.",
			SeeAlso:    []string{"petri_ode", "petri_stochastic", "petri_sde", "mass_action_kinetics"},
		},

		"pareto_optimization": {
			Name:       "pareto_optimization",
			Category:   "optimization",
			Summary:    "When objectives conflict, the frontier shows the unavoidable trade-offs.",
			Intuition:  "With multiple competing objectives, no single answer is 'best.' The Pareto frontier is the set of solutions where every alternative is at least as good in all objectives, and strictly better in at least one — you can't move along the frontier without losing somewhere.",
			Formula:    "A dominates B  iff  ∀ obj: A.obj ≥ B.obj  (≤ for min)\n                AND  ∃ obj: A.obj > B.obj  (< for min)\n\nPareto-optimal: not dominated by any other sample.",
			Derivation: "Monte Carlo samples the parameter space (faster than gradient methods, no convexity needed). For each sample pair, check dominance. Non-dominated samples form the frontier. With N samples the check is O(N²) — fine for N ≤ a few thousand.",
			Example:    "Shared budget B = 100 split between ads and engineering. Maximize both.\n  Sample 1: (ads=70, eng=30). Sample 2: (ads=40, eng=60).\n  Neither dominates the other — to gain on one, lose on the other.\n  Both are Pareto-optimal. The frontier traces the trade-off curve.",
			SeeAlso:    []string{"petri_optimize"},
		},

		"nelder_mead": {
			Name:       "nelder_mead",
			Category:   "optimization",
			Summary:    "Simplex search without gradients. Reflect, expand, contract, shrink.",
			Intuition:  "Maintain N+1 points (a simplex) in parameter space. Each step, find the worst point and try to improve it by reflecting through the centroid of the others. If reflection helps, push further. If not, pull back. Eventually the simplex collapses onto a minimum.",
			Formula:    "Per iteration:\n  1. Sort vertices by loss\n  2. Centroid c = mean of best N vertices\n  3. Reflect:   x_r = c + α(c − x_worst)\n  4. If f(x_r) < best: Expand to x_e = c + γ(x_r − c). Take whichever is better.\n  5. Else if f(x_r) < second worst: replace worst with x_r.\n  6. Else Contract or Shrink.\n\nStandard coefficients: α=1, γ=2, ρ=0.5, σ=0.5",
			Derivation: "Pure function evaluation — no gradients required. Works on noisy losses where gradient methods fail. The simplex 'rolls downhill' through reflection, expanding when finding new territory, contracting when stuck. Convergence is heuristic: stop when the simplex spread (vertex value range) drops below tolerance.",
			Example:    "Fitting 3 ODE rates against 14 observations. Initial simplex: 4 points around guess [1, 1, 1]. After ~200 iterations of reflect/expand/contract, simplex converges to [2.00, 1.49, 1.00] with loss ≈ 1e-7.",
			SeeAlso:    []string{"petri_fit"},
		},

		"equilibrium_detection": {
			Name:       "equilibrium_detection",
			Category:   "ode",
			Summary:    "Tell when an ODE has truly settled (not just briefly slow).",
			Intuition:  "An ODE is at equilibrium when its state stops changing. We detect this by watching the largest derivative — if it stays below tolerance for several checks in a row, the system has settled.",
			Formula:    "Per check (every k integration steps):\n  maxChange = max_i |dx_i / dt|\n  If maxChange < tol AND t > minTime:\n    consecutive++; if consecutive ≥ N: reached\n  Else:\n    consecutive := 0\n\nFastEquilibriumOptions: tol = 1e-4, N = 3, minTime = 0.01.",
			Derivation: "A single sub-tolerance step could be coincidence (e.g., the ODE is briefly passing through a slow region). Requiring N consecutive sub-tolerance checks filters out false positives. The minTime guard rejects 't=0 with zero rates' as a spurious equilibrium.",
			Example:    "Coffee shop at t=15 has all rates below 1e-6 (well under 1e-4 tol). The detector confirms reached=true after 3 checks below tolerance. petri_ode reports the equilibrium marking and the time when it was detected.",
			SeeAlso:    []string{"petri_ode"},
		},

		"sensitivity_elasticity": {
			Name:       "sensitivity_elasticity",
			Category:   "analysis",
			Summary:    "Dimensionless 'which knob matters?': % change in output per % change in input.",
			Intuition:  "If I nudge one transition's rate up 5%, how much does the answer move? Normalize both numerator and denominator by their base values so the answer is dimensionless and comparable across rates of any scale.",
			Formula:    "E_i = (Δy/y) / (Δk_i/k_i)\n\nFinite-difference recipe:\n  y_base = solve(k_base)\n  for each i:\n    k_i' = k_i · (1 + δ)         (e.g. δ = 0.05)\n    y_i' = solve(k_i')\n    E_i = (y_i' − y_base) / y_base / δ",
			Derivation: "From economics: elasticity is dy/dk · k/y, which is unitless and tells you 'percentage points per percentage point.' Magnitudes are directly comparable: E_i = 2 means 'twice as influential as a baseline 1-to-1 response.' Negative E means inhibiting.",
			Example:    "petri_ode_sensitivity on coffee shop, observable=delivered:\n  All three transitions show E > 0 (faster rate → higher delivered at fixed time).\n  Equilibrium values may give E ≈ 0 because mass conservation pins the final state.\n  Transient sensitivity reveals which rate controls the timescale to equilibrium.",
			SeeAlso:    []string{"petri_ode_sensitivity", "petri_analyze"},
		},
	}
}

func handleExplain(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	concepts := defiConcepts()
	topic := strings.TrimSpace(request.GetString("topic", ""))
	if topic == "" {
		return listConcepts(concepts)
	}
	c, ok := concepts[topic]
	if !ok {
		// Soft-match by category or substring.
		candidates := []string{}
		for k := range concepts {
			if strings.Contains(k, topic) || strings.Contains(topic, k) {
				candidates = append(candidates, k)
			}
		}
		if len(candidates) == 1 {
			c = concepts[candidates[0]]
		} else {
			msg := fmt.Sprintf("unknown topic %q.", topic)
			if len(candidates) > 1 {
				sort.Strings(candidates)
				msg += fmt.Sprintf(" Did you mean: %s?", strings.Join(candidates, ", "))
			} else {
				msg += " Call petri_explain with no arguments to list all topics."
			}
			return mcp.NewToolResultError(msg), nil
		}
	}

	out := map[string]any{
		"name":       c.Name,
		"category":   c.Category,
		"intuition":  c.Intuition,
		"formula":    c.Formula,
		"derivation": c.Derivation,
		"example":    c.Example,
		"seeAlso":    c.SeeAlso,
	}
	text, _ := json.MarshalIndent(out, "", "  ")
	return mcp.NewToolResultText(string(text)), nil
}

// verboseAnnotation returns short formula/algorithm text suitable for
// inclusion in a tool's response when verbose=true. The text is meant to
// run alongside numeric output, not replace it: 5-10 lines, plain Unicode
// math, terminated with a pointer at the petri_explain topic for deeper
// reading. Each annotation pairs with a topic name that exists in
// defiConcepts(), so the user can drill in.
//
// kind is the tool family: "ode", "ssa", "sde", "fit", "optimize",
// "sensitivity". Unknown kinds return the empty string so callers can
// safely treat the result as optional.
func verboseAnnotation(kind string, runSummary string) string {
	header := func(algorithm, formula, topic string) string {
		out := algorithm + "\n\nFormula:\n" + indent(formula, "  ")
		if runSummary != "" {
			out += "\n\nThis run:\n" + indent(runSummary, "  ")
		}
		if topic != "" {
			out += "\n\nDeeper reading: call petri_explain topic=" + topic
		}
		return out
	}

	switch kind {
	case "ode":
		return header(
			"Algorithm: Tsit5 (Tsitouras 5/4) adaptive Runge-Kutta integration of mass-action ODE.",
			"For each transition t:   rate(t) = k_t · ∏_(p ∈ •t) m(p)\nDerivatives:            du_p/dt = Σ_(t input p) rate(t) − Σ_(t output p) rate(t)\nIntegrator step error:   adaptive, abstol=1e-6, reltol=1e-3 (JSParityOptions matches pflow.xyz).",
			"mass_action_kinetics",
		)
	case "equilibrium":
		return header(
			"Equilibrium detection on top of ODE solve: stop when max derivative stays below tolerance.",
			"At each check:   maxChange = max_i |du_i/dt|\nIf maxChange < tol for N consecutive checks → reached.\nFastEquilibriumOptions: tol=1e-4, N=3, minTime=0.01.\nIf maxChange falls below tolerance but the consecutive gate hasn't\nfired by tspan end, the response sets effectiveReached=true so the\nstate is reported honestly.",
			"equilibrium_detection",
		)
	case "ssa":
		return header(
			"Algorithm: Gillespie Stochastic Simulation Algorithm — exact discrete-event Markov chain simulation.",
			"Per step:\n  a_i = k_i · ∏_(p ∈ •t_i, kinetic) C(m(p), w(p,t_i))    propensities\n  A   = Σ a_i                          total rate\n  τ   = −ln(u) / A,  u ~ U(0,1)        wait time ~ Exp(A)\n  P(t_i) = a_i / A                     which transition fires\nFire t_i: subtract input weights, add output weights, advance t by τ.\nThe product runs over kinetic inputs only. A non-kinetic input still gates\nt_i (unsatisfied ⇒ a_i = 0) and is still consumed; it just does not scale\nthe rate — a server or a stockpile is a prerequisite, not an accelerant.",
			"gillespie_ssa",
		)
	case "sde":
		return header(
			"Algorithm: Euler-Maruyama integration of SDE with geometric Brownian motion on volatile places.",
			"dx_i = drift_i(x, t) dt + σ_i · x_i · dW_i\n\nDiscretized per step (dt = T/N):\n  drift  = mass-action derivative (same as petri_ode)\n  noise  = σ_i · x_i · √dt · W_i,  W ~ N(0, R)\nMulti-asset noise W = L · Z where Z is iid N(0, I_N) and R = L · L^T\n(R is the user-supplied correlation matrix, Cholesky-factored once).",
			"euler_maruyama",
		)
	case "fit":
		return header(
			"Algorithm: Nelder-Mead simplex — gradient-free minimization of squared residual loss.",
			"Simplex: N+1 points in N-dim parameter space.\nPer iteration:\n  1. Sort by loss\n  2. Reflect worst through centroid of others (α=1)\n  3. If reflection beats best: expand further (γ=2)\n  4. Else if reflection beats second-worst: replace worst\n  5. Else: contract (ρ=0.5) or shrink (σ=0.5)\nLoss: Σ (model(t_i) − observed_i)² over observation points.",
			"nelder_mead",
		)
	case "optimize":
		return header(
			"Algorithm: Monte Carlo sampling + O(N²) Pareto frontier filter.",
			"Sample N rate combos uniformly in parameter bounds.\nFor each combo: run ODE to equilibrium, record observable values.\nDominance: A dominates B iff ∀ obj: A.obj ≥ B.obj (≤ for min),\nand strictly better on at least one. Pareto-optimal: no other\nsample dominates this one.",
			"pareto_optimization",
		)
	case "sensitivity":
		return header(
			"Algorithm: finite-difference dimensionless elasticities of an observable vs each rate.",
			"E_i = (Δy / y) / (Δk_i / k_i)\n\nFor each transition i:\n  Bump k_i → k_i · (1 + δ)\n  Re-run ODE to equilibrium\n  Δy = y_perturbed − y_base\n  E_i = (Δy / y_base) / δ",
			"sensitivity_elasticity",
		)
	case "rate_scan":
		return header(
			"Algorithm: parameter sweep — one rate varies over a range, each value run to equilibrium.",
			"For each rate value k in values:\n  prob = NewProblem(net, initial, tspan, rates ∪ {tid: k})\n  sol  = SolveUntilEquilibrium(prob, Tsit5, FastEquilibriumOptions)\n  record sol.GetFinalState()\nUseful for finding regime boundaries, computing dose-response.",
			"mass_action_kinetics",
		)
	}
	return ""
}

// indent prefixes each non-empty line with the given prefix.
func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l == "" {
			continue
		}
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}

func listConcepts(concepts map[string]concept) (*mcp.CallToolResult, error) {
	names := make([]string, 0, len(concepts))
	for k := range concepts {
		names = append(names, k)
	}
	sort.Strings(names)

	type entry struct {
		Name     string `json:"name"`
		Category string `json:"category"`
		Summary  string `json:"summary"`
	}
	entries := make([]entry, 0, len(names))
	for _, n := range names {
		c := concepts[n]
		entries = append(entries, entry{Name: c.Name, Category: c.Category, Summary: c.Summary})
	}
	wrapper := struct {
		Total  int     `json:"total"`
		Topics []entry `json:"topics"`
		Hint   string  `json:"hint"`
	}{
		Total:  len(entries),
		Topics: entries,
		Hint:   "Call petri_explain with topic=<name> to get the full explanation: intuition, formula, derivation, worked example, and related tools.",
	}
	text, _ := json.MarshalIndent(wrapper, "", "  ")
	return mcp.NewToolResultText(string(text)), nil
}

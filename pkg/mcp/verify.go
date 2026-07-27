package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"
	"github.com/pflow-xyz/go-pflow/petri"
	"github.com/pflow-xyz/go-pflow/verify"
	"github.com/pflow-xyz/petri-pilot/pkg/validator"
)

// verifyTool exposes declarative property checking.
//
// The other analysis tools describe a model; this one judges it. Given a set of
// properties the model is supposed to satisfy, it returns proved / refuted /
// unknown per property — with a replayable firing sequence on refutation, and
// the proof method on success. That distinction is the point: a "structural"
// proof holds for every initial marking, while "exhaustive" holds only for the
// one analyzed, and "unknown" means the question is still open rather than
// answered in the affirmative.
func verifyTool() mcp.Tool {
	return mcp.NewTool("petri_verify",
		mcp.WithDescription(
			"Check whether a Petri net model satisfies stated correctness properties. "+
				"Returns proved/refuted/unknown per property. Refutations include a firing sequence "+
				"you can replay with petri_simulate to reproduce the failure. Use this to answer "+
				"'is this model correct?' against explicit requirements, rather than eyeballing a simulation."),
		mcp.WithString("model",
			mcp.Required(),
			mcp.Description("The Petri net model as JSON or tokenmodel DSL (S-expression format starting with '(')"),
		),
		mcp.WithString("properties",
			mcp.Required(),
			mcp.Description(`JSON array of properties to check. Each entry is either a shorthand string or an object.

Shorthand strings:
  "deadlock-free"            no reachable marking is a deadlock
  "bounded"                  no place accumulates tokens without limit
  "live"                     every transition can fire from some reachable marking
  "terminating"              every execution eventually stops
  "conserves"                total token count never changes
  "reachable:done=1"         some reachable marking matches (partial markings allowed)
  "unreachable:a=1,b=1"      no reachable marking matches — the safety form
  "mutex:busy1,busy2"        at most one of these places holds a token ("mutex:a,b,c<=2" for a higher bound)
  "a + 2*b == 10"            a linear relation over places holding at every reachable marking

Object form (equivalent, more explicit):
  {"kind":"invariant","name":"supply is conserved","expr":"minted == circulating + burned"}
  {"kind":"unreachable","name":"no double spend","target":{"spent":2}}
  {"kind":"mutual-exclusion","places":["busy1","busy2"],"bound":1}

Example: ["deadlock-free","bounded","mutex:busy1,busy2","minted == circulating + burned"]`),
		),
		mcp.WithString("max_states",
			mcp.Description("State exploration limit for exhaustive checks (default 20000). Structural proofs ignore this."),
		),
	)
}

// propertySpec is the wire form of a property in object notation.
type propertySpec struct {
	Kind   string         `json:"kind"`
	Name   string         `json:"name,omitempty"`
	Expr   string         `json:"expr,omitempty"`
	Target map[string]int `json:"target,omitempty"`
	Places []string       `json:"places,omitempty"`
	Bound  int            `json:"bound,omitempty"`
}

func handleVerify(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	modelJSON, err := request.RequireString("model")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing model parameter: %v", err)), nil
	}

	propsJSON, err := request.RequireString("properties")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("missing properties parameter: %v", err)), nil
	}

	parsed, err := parseModelV2(modelJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model: %v", err)), nil
	}

	props, err := parseProperties(propsJSON)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid properties: %v", err)), nil
	}
	if len(props) == 0 {
		return mcp.NewToolResultError("no properties given — supply at least one, e.g. [\"deadlock-free\"]"), nil
	}

	net := buildVerifyNet(parsed.Model)

	v := verify.New(net)
	if raw := request.GetString("max_states", ""); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			return mcp.NewToolResultError(fmt.Sprintf("max_states must be a positive integer, got %q", raw)), nil
		}
		v = v.WithMaxStates(n)
	}

	report := v.Check(props...)

	output, err := json.MarshalIndent(struct {
		*verify.Report
		Summary string `json:"summary"`
	}{Report: report, Summary: report.Summary()}, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(output)), nil
}

// buildVerifyNet converts a metamodel Model into a petri.PetriNet.
func buildVerifyNet(model *goflowmetamodel.Model) *petri.PetriNet {
	builder := petri.Build()

	for _, p := range model.Places {
		builder = builder.Place(p.ID, float64(p.Initial))
	}
	for _, t := range model.Transitions {
		builder = builder.Transition(t.ID)
	}
	for _, arc := range model.Arcs {
		weight := arc.Weight
		if weight == 0 {
			weight = 1
		}
		builder = builder.Arc(arc.From, arc.To, float64(weight))
	}

	return builder.Done()
}

// parseProperties accepts a JSON array whose entries are either shorthand
// strings or property objects, so callers can use whichever is clearer.
func parseProperties(src string) ([]verify.Property, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal([]byte(src), &raw); err != nil {
		return nil, fmt.Errorf("properties must be a JSON array: %w", err)
	}

	props := make([]verify.Property, 0, len(raw))
	for i, entry := range raw {
		// Try the shorthand string form first.
		var shorthand string
		if err := json.Unmarshal(entry, &shorthand); err == nil {
			p, err := parsePropertyShorthand(shorthand)
			if err != nil {
				return nil, fmt.Errorf("property %d (%q): %w", i, shorthand, err)
			}
			props = append(props, p)
			continue
		}

		var spec propertySpec
		if err := json.Unmarshal(entry, &spec); err != nil {
			return nil, fmt.Errorf("property %d is neither a string nor an object: %w", i, err)
		}
		p, err := specToProperty(spec)
		if err != nil {
			return nil, fmt.Errorf("property %d: %w", i, err)
		}
		props = append(props, p)
	}

	return props, nil
}

func specToProperty(spec propertySpec) (verify.Property, error) {
	kind := verify.Kind(strings.TrimSpace(spec.Kind))
	switch kind {
	case verify.KindDeadlockFree, verify.KindBounded, verify.KindLive,
		verify.KindTerminating, verify.KindConserves:
		return verify.Property{Kind: kind, Name: spec.Name}, nil

	case verify.KindReachable, verify.KindUnreachable:
		if len(spec.Target) == 0 {
			return verify.Property{}, fmt.Errorf("%s requires a non-empty \"target\" marking", kind)
		}
		return verify.Property{Kind: kind, Name: spec.Name, Target: spec.Target}, nil

	case verify.KindInvariant:
		if strings.TrimSpace(spec.Expr) == "" {
			return verify.Property{}, fmt.Errorf("invariant requires an \"expr\"")
		}
		if _, err := verify.ParseExpr(spec.Expr); err != nil {
			return verify.Property{}, err
		}
		return verify.Property{Kind: kind, Name: spec.Name, Expr: spec.Expr}, nil

	case verify.KindMutualExclusion:
		if len(spec.Places) == 0 {
			return verify.Property{}, fmt.Errorf("mutual-exclusion requires \"places\"")
		}
		return verify.Property{Kind: kind, Name: spec.Name, Places: spec.Places, Bound: spec.Bound}, nil

	case "":
		return verify.Property{}, fmt.Errorf("missing \"kind\"")
	default:
		return verify.Property{}, fmt.Errorf("unknown kind %q", spec.Kind)
	}
}

// parsePropertyShorthand converts the compact string form into a Property.
func parsePropertyShorthand(spec string) (verify.Property, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return verify.Property{}, fmt.Errorf("empty property")
	}

	switch strings.ToLower(spec) {
	case "deadlock-free", "deadlockfree":
		return verify.Property{Kind: verify.KindDeadlockFree, Name: spec}, nil
	case "bounded":
		return verify.Property{Kind: verify.KindBounded, Name: spec}, nil
	case "live", "liveness":
		return verify.Property{Kind: verify.KindLive, Name: spec}, nil
	case "terminating", "terminates":
		return verify.Property{Kind: verify.KindTerminating, Name: spec}, nil
	case "conserves", "conservation":
		return verify.Property{Kind: verify.KindConserves, Name: spec}, nil
	}

	switch {
	case strings.HasPrefix(spec, "reachable:"):
		target, err := parseMarkingShorthand(strings.TrimPrefix(spec, "reachable:"))
		if err != nil {
			return verify.Property{}, err
		}
		return verify.Property{Kind: verify.KindReachable, Name: spec, Target: target}, nil

	case strings.HasPrefix(spec, "unreachable:"):
		target, err := parseMarkingShorthand(strings.TrimPrefix(spec, "unreachable:"))
		if err != nil {
			return verify.Property{}, err
		}
		return verify.Property{Kind: verify.KindUnreachable, Name: spec, Target: target}, nil

	case strings.HasPrefix(spec, "mutex:"):
		body := strings.TrimPrefix(spec, "mutex:")
		bound := 1
		if idx := strings.Index(body, "<="); idx >= 0 {
			n, err := strconv.Atoi(strings.TrimSpace(body[idx+2:]))
			if err != nil {
				return verify.Property{}, fmt.Errorf("bad bound after '<=': %w", err)
			}
			bound = n
			body = body[:idx]
		}
		var places []string
		for _, p := range strings.Split(body, ",") {
			if p = strings.TrimSpace(p); p != "" {
				places = append(places, p)
			}
		}
		if len(places) == 0 {
			return verify.Property{}, fmt.Errorf("mutex requires at least one place")
		}
		return verify.Property{Kind: verify.KindMutualExclusion, Name: spec, Places: places, Bound: bound}, nil
	}

	// Fall through to a linear expression. Parsing here means a typo is
	// reported against the property text rather than surfacing later as a
	// confusing "unknown" verdict.
	if _, err := verify.ParseExpr(spec); err != nil {
		return verify.Property{}, err
	}
	return verify.Property{Kind: verify.KindInvariant, Name: spec, Expr: spec}, nil
}

// parseMarkingShorthand parses "a=1,b=0" into a partial marking.
func parseMarkingShorthand(s string) (map[string]int, error) {
	target := make(map[string]int)
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("expected place=tokens, got %q", part)
		}
		n, err := strconv.Atoi(strings.TrimSpace(kv[1]))
		if err != nil {
			return nil, fmt.Errorf("bad token count in %q: %w", part, err)
		}
		target[strings.TrimSpace(kv[0])] = n
	}
	if len(target) == 0 {
		return nil, fmt.Errorf("empty marking — expected place=tokens pairs")
	}
	return target, nil
}

// analyzeInvariants returns the model's minimal-support P- and T-invariants,
// delegating to pkg/validator so petri_validate and petri_analyze cannot drift
// apart on what they report.
func analyzeInvariants(model *goflowmetamodel.Model) (pInvariants, tInvariants []string) {
	opts := validator.DefaultOptions()
	opts.EnableSensitivity = false
	return validator.New(opts).Invariants(model)
}

# Petri Pilot

LLM-augmented design and validation for Petri net models.

## Concept

Petri Pilot combines the creative power of LLMs with the formal verification capabilities of [go-pflow](https://github.com/pflow-xyz/go-pflow) to create validated process models through an iterative refinement loop:

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Natural       │     │   LLM           │     │   Petri Net     │
│   Language      │────▶│   Generator     │────▶│   Model         │
│   Requirements  │     │                 │     │   (DSL)         │
└─────────────────┘     └─────────────────┘     └────────┬────────┘
                                                         │
        ┌────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Validated     │     │   Feedback      │     │   Formal        │
│   Model         │◀────│   Loop          │◀────│   Validation    │
│                 │     │                 │     │   (go-pflow)    │
└─────────────────┘     └─────────────────┘     └─────────────────┘
```

## Why This Works

| LLM Strengths | Formal Methods Strengths |
|---------------|--------------------------|
| Natural language understanding | Guaranteed correctness proofs |
| Creative structure generation | Deadlock detection |
| Domain knowledge synthesis | Liveness verification |
| Rapid prototyping | Conservation law checking |
| Pattern recognition | Sensitivity analysis |

**Together**: LLM generates candidate models quickly, formal methods catch errors the LLM missed, feedback refines until correct.

## Validation Pipeline

1. **Structural Validation**
   - Parse DSL syntax
   - Check node connectivity
   - Verify arc weights

2. **Behavioral Validation**
   - Reachability analysis (deadlocks, livelocks)
   - Boundedness checking
   - Conservation invariants

3. **Sensitivity Analysis**
   - Element importance ranking
   - Symmetry group detection
   - Isolated element detection
   - Critical path identification

4. **Simulation Validation**
   - ODE trajectory analysis
   - Equilibrium detection
   - Expected behavior verification

## Example Workflow

```bash
# Generate a model from requirements
petri-pilot generate "A simple order processing workflow with
  receive order, validate, process payment, and ship steps.
  Orders can fail validation and be rejected."

# Output: Generated model with validation results
# ✓ Syntax valid
# ✓ All nodes connected
# ✓ No deadlocks detected
# ✓ Bounded (max 1 token per place)
# ⚠ Warning: 'rejected' is a sink (no outgoing arcs)
#
# Symmetry groups detected:
#   [process_payment, ship] - similar structural role
#
# Critical elements (high importance):
#   - validate (0.89) - removal breaks 3 paths
#   - receive_order (0.95) - entry point
```

## Installation

```bash
go install github.com/pflow-xyz/petri-pilot/cmd/petri-pilot@latest
```

## Usage

### Generate Model

```bash
# Interactive generation
petri-pilot generate "your requirements here"

# From file
petri-pilot generate -f requirements.txt

# With specific output format
petri-pilot generate -o model.go "requirements"
```

### Validate Existing Model

```bash
# Validate a go-pflow model
petri-pilot validate model.go

# Full analysis with sensitivity
petri-pilot validate -full model.go
```

### Refine Model

```bash
# Iterative refinement loop
petri-pilot refine model.go "add error handling for payment failures"
```

## Architecture

```
petri-pilot/
├── cmd/petri-pilot/     # CLI entry point
├── pkg/
│   ├── generator/       # LLM-based model generation
│   ├── validator/       # Formal validation using go-pflow
│   ├── feedback/        # Structured feedback for LLM refinement
│   └── schema/          # Model schema definitions
├── internal/
│   └── llm/             # LLM provider abstraction
└── examples/            # Example requirements and models
```

## Supported LLM Providers

- Claude (Anthropic) - recommended
- OpenAI GPT-4
- Local models via Ollama

## Configuration

```yaml
# ~/.petri-pilot.yaml
llm:
  provider: claude
  model: claude-sonnet-4-20250514

validation:
  max_states: 10000
  sensitivity:
    parallel: true
    workers: 0  # auto-detect

output:
  format: go  # go, jsonld, svg
```

## License

MIT

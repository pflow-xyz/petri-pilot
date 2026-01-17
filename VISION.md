# Petri Pilot

**From requirements to verified models, in plain language.**

## The Gap

Good models require two things: domain knowledge and formal expertise. You understand your system—the states, the transitions, the constraints. But translating that understanding into a valid Petri net means learning a formalism, avoiding subtle pitfalls, and iterating until the math agrees with your intuition.

Most people skip the model entirely. They go straight to code, where the structure lives only in their heads.

## The Bridge

Petri Pilot closes this gap. Describe what you want in natural language. The tool generates a model, validates it against formal rules, and refines until it's correct.

```
"A checkout workflow with cart, payment, and shipping"
```

becomes a verified Petri net—places, transitions, arcs—with deadlock analysis, connectivity checks, and boundedness guarantees.

## The Loop

Generation alone isn't enough. LLMs make mistakes. They create unconnected places, invalid references, impossible state spaces.

Petri Pilot catches these through formal validation, then feeds structured errors back to the model for refinement. Each iteration gets closer to correctness.

```
Requirements → Generate → Validate → Feedback → Refine
     ↑                                            │
     └────────────────────────────────────────────┘
```

The human provides intent. The validator provides rigor. The LLM bridges the two.

## Why This Matters

**For designers**: Skip the learning curve. Express your system in words you already use.

**For developers**: Get a formal specification before you write code. Catch deadlocks and race conditions at design time.

**For teams**: A shared model everyone can read, simulate, and discuss—not buried in implementation details.

## The Formalism

Petri nets are minimal: places hold state, transitions move it, arcs connect them. Four concepts, one firing rule. This simplicity is the point—complex behavior emerges from simple composition.

The same vocabulary describes mutex locks, producer-consumer queues, checkout flows, approval workflows. Different domains, same underlying structure.

## What We're Building

A CLI that treats model generation as a conversation between human intent and formal verification:

- **Generate**: Natural language to Petri net
- **Validate**: Structural and behavioral checks via go-pflow
- **Refine**: Targeted fixes from validation feedback
- **Iterate**: Automatic refinement until valid

The goal isn't to replace understanding—it's to accelerate the path from understanding to specification.

## Try It

```bash
petri-pilot generate -auto -v "User registration with email verification"
```

Watch the model evolve. See what the validator catches. Understand your system better by watching it take shape.

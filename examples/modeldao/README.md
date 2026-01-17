# Why modeldao

I've been circling the same problem for years without seeing it clearly.

## Factom

I watched a DAO die from the inside. Not from lack of funding or technical failure—from governance rot.

Whales voted absentmindedly, throwing random weight at proposals. They weren't playing the cooperation game. They were noise in the system—low marginal cost to vote, diversified holdings, no operational stake in outcomes.

Meanwhile, community members discovered tit-for-tat reward farming. Cooperation emerged beautifully... around the wrong objective function. Small tasks that triggered reciprocal rewards. A closed loop with no drainage toward protocol goals. The system selected for mutual back-scratching, not useful work.

The declared model—tokens flow, votes aggregate, decisions execute—didn't match actual dynamics. Hidden state (whale attention levels), stochastic transitions (random weight submission), missing feedback loops (no reputation decay for careless voting).

We couldn't fix it because we couldn't *see* it. The governance structure was entangled with implicit context. You can't debug what you can't represent.

## Christakis

Reading "Blueprint" connected some dots. The social suite—friendship, repeated interaction, mild hierarchy, shared fate—these aren't policy choices. They're structural preconditions for stable cooperation.

DAOs do the opposite. Token-weighted voting, pseudonymity, global async coordination. We took everything evolution spent millions of years tuning and said "what if we didn't do that."

But the mechanisms that *do* work in crypto—staking, lockups, skin in the game—they work because they create structural conditions the social suite predicts. Shared fate. Mutual vulnerability. Shadow of the future.

The question isn't "how do we get people to cooperate." It's "what structures make cooperation an attractor rather than a transient state."

That's a modeling question.

## Petri Nets

I've spent years building tools for Petri net modeling. The formalism sits at a sweet spot: visual enough for human intuition, mathematical enough for verification, composable enough to scale without losing structural semantics.

The key insight came from using them with LLMs. The net becomes the artifact, not the code. Development is monotonic—never delete, only grow. Extension preserves what came before.

This works because the semantics are explicit. State is a vector. Transitions are enabled or not. Invariants hold or they don't. There's nothing to misinterpret because the model *is* the source of truth.

If governance lived in a Petri net, you could simulate outcomes before committing. Verify properties hold under composition. Diff changes between versions. See exactly where the structure allows cooperation to decay.

## arcnet

So I built a toolchain that compiles Petri net schemas to executable smart contracts.

The state model is minimal:

```go
type State struct {
    Vector   []int
    Sequence uint64
    Hash     Hash
}
```

Vector for the marking, sequence for ordering, hash for integrity. The entire state of a contract is "how many tokens are in each place"—which is exactly what a Petri net marking is.

The VM is portable. Run it in a browser, on-chain, in a validator. The execution is just "check if this transition is enabled, update the vector."

Forking becomes instantiation with modified parameters. Composition has defined semantics. Evolution is legible—you can diff organizational changes like you diff code.

## The Convergence

Every investigation pointed to the same gap.

Factom failed partly because you couldn't diff the governance. The structure was implicit, entangled with specific people and tacit knowledge. When things went wrong, there was nothing to examine.

DAOs fail because cooperation conditions aren't encoded. The mechanisms assume strategic actors paying attention; they get noise from distracted whales and gaming from reward farmers.

Traditional corporations can't fork at all. The pattern is inseparable from the instance. Successful structures die with their hosts. Every new org reinvents from scratch.

Organizations can't propagate because their structure is implicit.

## modeldao

L2s proved that composable sovereignty works. Before rollups, the assumption was one chain or fragmentation. Turns out there's a middle path: child chains that inherit security guarantees, differentiate on execution, and propagate without the original team's involvement.

We haven't tested this for organizations. The sample size is zero.

modeldao is the experiment. Membership requires building a model. Not because gatekeeping is good, but because it ensures every member can read and write the genome. A community where participation *is* demonstrated capacity for structural reasoning.

If it works, the pattern propagates. Purpose-specific child orgs instantiate from composable schemas. Cooperation conditions are encoded, not hoped for. Governance runs on formal models, not prose reconstructed into intent by algorithms that don't understand context.

Nobody knows if actual decentralized organization—self-propagating software-as-orgs—is possible at all.

That's what we're testing.

[modeldao.org](https://modeldao.org)

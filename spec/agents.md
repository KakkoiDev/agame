# Agents and turns

## Agent contract

Conceptually:

```text
observe(snapshot) -> decide(tools) -> 0..N orders
```

The interface must support LLM agents and non-LLM agents.

The model proposes decisions. Only the engine can validate and execute orders.

## Turn lifecycle

Every turn follows this barrier:

1. Freeze authoritative world snapshot S(t).
2. Derive each ruler's legal observation from S(t).
3. Invoke every ruler independently against S(t).
4. Collect all submitted order sets.
5. Close the decision barrier.
6. Validate orders.
7. Resolve all orders deterministically.
8. Produce S(t+1) and structured events.
9. Deliver resulting observations/messages.
10. Allow rulers to update their Jikko knowledge/memory.
11. Record audit data.

No agent may see another agent's current-turn order before the barrier closes.

## Sequential inference, simultaneous game time

A single local model may physically run Cassian, then Malrec, then Aya because GPU inference is sequential.

That execution order has **no game meaning**.

Every invocation receives the same pre-resolution world turn and cannot observe side effects from earlier inference calls.

Transient model context must be reset between rulers.

## Shared model experiment

Canonical mode:

```text
same model weights
+ ruler A personality/history/knowledge -> ruler A
+ ruler B personality/history/knowledge -> ruler B
+ ruler C personality/history/knowledge -> ruler C
```

This isolates context and accumulated experience as experimental variables.

Different-model tournaments are also valid but must be labeled separately.

## Identity

The harness tells the model which Jikko Identity authenticated it.

Authentication identity is not itself a biography dump.

The ruler may inspect its own accessible Identity document if it wants authored information about itself.

## Retrieval loop

The default agent begins with:

- system/game contract;
- authenticated identity name;
- current engine observation;
- permission-filtered Jikko file tree;
- available tool/action schemas.

It does **not** automatically receive every accessible Jikko file.

It may request:

- one file;
- multiple files in a batch;
- searches/listings where available;
- additional retrieval in subsequent tool calls.

The model decides what is worth reading.

## Decision output

Orders must use a compact structured format, e.g. JSON.

Each turn may additionally include a concise public or auditable rationale/summary. AGame must never require or store hidden chain-of-thought.

Example shape:

```json
{
  "orders": [
    {"type": "spy", "target": "vega-2"},
    {"type": "move", "fleet": "home-1", "destination": "sol-3"}
  ],
  "statement": "We will verify Vega before committing the fleet."
}
```

The exact schema belongs to the implementation ruleset.

## Memory/reflection phase

After meaningful outcomes, an agent may write or update Jikko documents/tasks.

The harness may invite reflection after significant events, but should not dictate the interpretation.

Examples:

- betrayal;
- death/destruction of a major fleet;
- loss/capture of a capital;
- unexpected alliance assistance;
- victory in a long war;
- exile and restoration.

Character evolution should emerge through authored memory and changed plans rather than hidden engine personality numbers unless a later experiment explicitly adds them.

## Failure handling

A malformed model response must not corrupt the universe.

The harness should support bounded repair attempts. If no valid decision is obtained, the safe fallback is zero orders for that ruler.

Timeouts, parse failures and repair attempts are benchmark data.

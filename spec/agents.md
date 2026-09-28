# Agents and turns

## Agent contract

Conceptually:

```text
observe(snapshot) -> inspect(jikko/tools) -> decide() -> 0..N orders
```

The interface supports LLM and non-LLM agents. Models propose decisions; only the engine validates and executes them.

## Turn lifecycle

1. Freeze authoritative S(t).
2. Derive each ruler's legal observation from S(t).
3. Invoke every ruler independently against S(t).
4. Collect all submitted order sets.
5. Close the decision barrier.
6. Validate orders.
7. Resolve all orders deterministically.
8. Produce S(t+1) and structured events.
9. Deliver resulting observations/messages.
10. Run reflection when triggered.
11. Record audit data.

No agent sees another agent's current-turn order before the barrier closes.

## Sequential inference, simultaneous game time

A shared local model physically runs rulers sequentially, but invocation order has no game meaning. Transient model context is reset between rulers.

## Canonical shared-model mode

```text
same model weights/config
+ ruler A identity/history/knowledge -> ruler A
+ ruler B identity/history/knowledge -> ruler B
...
```

Different-model tournaments are allowed but labeled separately.

## Initial invocation context

The harness supplies:

- concise immutable game/rules contract;
- authenticated Jikko identity;
- current legal engine observation;
- permission-filtered Jikko tree;
- tool schemas.

It does not inject all memories or personality documents automatically.

## Tool surface

The canonical conceptual tools are:

```text
jikko.tree() -> [{path,title,type}]
jikko.read(path) -> document
jikko.read_many(paths[]) -> documents[]
jikko.search(query) -> matches[]
jikko.create(path, content, type?)
jikko.update(path, expected_revision, content)
jikko.mentions() -> documents[]

game.inspect(ref) -> structured observable game object
game.submit(orders[], statement?)
```

The implementation may expose these through CLI, HTTP or native tool calling, but semantics must remain equivalent.

The engine observation contains stable IDs needed to construct orders; the model should not guess opaque identifiers.

## Order schema

Every order has:

```json
{
  "type": "move",
  "actor": "fleet-17",
  "target": "system-vega",
  "params": {}
}
```

`type`, `actor`, `target` and `params` are interpreted by per-order schemas. Fields not needed by an order may be omitted.

Required v1 order types:

```text
construct, build_ships, research,
form_fleet, split_fleet, move, attack, colonize,
spy, transport, recycle,
message, alliance_create, alliance_join, alliance_leave
```

Submission envelope:

```json
{
  "orders": [],
  "statement": "Concise auditable explanation, not hidden reasoning."
}
```

## Small-model budgets

Canonical 2B–8B benchmark budget per ruler/turn:

- initial prompt + observation + tree target: **<= 8,000 tokens**;
- maximum **8 tool-call rounds**;
- `read_many` may request up to **16 files** in one call;
- maximum **24 Jikko files returned** in a turn;
- maximum **24,000 total input tokens** across the turn;
- final decision output target **<= 2,000 tokens**;
- at most **2 repair attempts** for malformed structured output.

These are benchmark limits, not engine laws. Other experiments may configure them and must record the values.

If the accessible tree becomes too large for the initial budget, Jikko returns a bounded-depth tree plus directory counts; the model may list deeper paths.

## Reflection triggers

A reflection phase is automatically offered after:

- homeworld/capital captured or recovered;
- empire enters exile;
- empire is restored from exile;
- alliance is joined, left or broken by hostile action;
- battle destroys >= 50% of the empire's pre-battle fleet combat value;
- empire captures another empire's homeworld;
- a ruler receives a message explicitly marked as a major diplomatic proposal;
- every 12 turns as an annual review.

Reflection is optional: the model may decide no durable memory/update is needed.

The reflection phase has a separate 4-tool-call-round budget and cannot submit game orders.

## Memory integrity

Jikko remains source-first Markdown, but AGame imposes historical discipline:

- agents may update current plans/tasks;
- agents may append corrections or changed interpretations to memory/intelligence documents;
- agents must not silently rewrite an old observation as though it had always been known;
- deletion of durable memory/intelligence is permitted only by creating an auditable tombstone/retraction event;
- Git/Jikko history preserves prior text.

A ruler may be mistaken. Corrections should say what changed and when.

## Ruler succession

v1 rulers do **not** die of age and there is no automatic succession system.

The same ruler Identity may persist for the full 50-year canonical run. This deliberately isolates memory/personality effects.

Succession can later be introduced as a separate experiment rather than confounding v1.

## Failure handling

Malformed responses cannot corrupt the universe.

After at most two repair attempts, failure becomes zero orders for that ruler. Timeouts, parse failures and repairs are benchmark data.

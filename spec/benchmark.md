# Reproducibility and benchmarking

## Reproducible universe

Every run has:

- ruleset version;
- universe seed;
- initial-state definition/hash;
- agent roster;
- model identifier and quantization;
- inference configuration;
- prompt/harness version;
- Jikko revision;
- media generation configuration if enabled.

Engine randomness uses seeded deterministic RNG.

## Event log

Every authoritative state transition emits a structured append-only event.

A complete event log plus initial state/ruleset must be sufficient to reconstruct or verify the world trajectory.

## Decision record

For every ruler/turn, record:

- invocation order (operational only);
- identity;
- observation;
- visible tree/revision;
- files actually retrieved;
- messages available;
- model/config;
- latency;
- inference usage where available;
- parse/repair failures;
- final structured orders;
- concise supplied statement/rationale if any.

Do not store hidden chain-of-thought.

## Comparison modes

### Same model, different minds

All rulers use identical model/configuration with different personality/history/knowledge.

Primary question: how much behavioral diversity emerges from persistent context?

### Same universe, different models

Replay the same initial seed/personality set using different local models.

Primary question: how does model capability alter long-horizon performance?

### Ablations

Useful experiments include:

- no persistent memory;
- no diplomacy;
- tree-only retrieval versus search availability;
- different context limits;
- different sampling parameters;
- personality removed;
- scripted baseline agents.

## Metrics

At minimum:

### Survival
- turns survived;
- elimination/restoration events;
- planets controlled over time.

### Economy
- resource production;
- resource efficiency;
- infrastructure growth;
- idle/wasted resources.

### Military
- fleet strength over time;
- losses inflicted/suffered;
- successful defenses;
- successful captures;
- salvage efficiency.

### Expansion
- colonies founded;
- planets captured/lost;
- time to expansion.

### Intelligence
- espionage attempts/success;
- age of intelligence used in decisions;
- retrieval behavior;
- relevant-memory retrieval where objectively measurable.

### Diplomacy
- alliances joined/left;
- agreements made;
- agreements honored/broken where machine-detectable;
- assistance given/received;
- message volume.

### Agent operation
- inference latency;
- tokens/usage where available;
- tool calls;
- files read;
- malformed outputs;
- repair attempts;
- zero-order turns.

## No single intelligence score

AGame should expose measurements, not collapse all behavior into one arbitrary universal score.

Benchmarks may define experiment-specific aggregate metrics, but raw measurements remain available.

## Replay

The dashboard should be able to replay a completed run without invoking agents.

A replay must distinguish authoritative events from ruler knowledge at that historical moment.

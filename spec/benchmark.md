# Reproducibility and benchmarking

## Canonical benchmark

Unless an experiment says otherwise:

- 8 empires;
- 32 systems × 4 planetary slots;
- 600 monthly turns maximum;
- identical material starting conditions;
- one shared model/configuration for all rulers;
- different authored ruler identities/personality seeds;
- deterministic universe seed and engine resolution;
- small-model tool/context budgets defined in `agents.md`.

A benchmark suite should run **multiple universe seeds**. One run is a story, not a statistically meaningful model comparison.

## Reproducible universe

Every run records:

- ruleset version;
- universe seed;
- initial-state definition/hash;
- agent roster;
- model identifier/checkpoint and quantization;
- inference configuration and sampling seed where supported;
- prompt/harness version;
- Jikko starting revision;
- tool/context budgets;
- media configuration if enabled.

Engine randomness uses seeded deterministic RNG.

## Event log

Every authoritative state transition emits a structured append-only event.

Initial state + ruleset + event log must be sufficient to reconstruct/verify the world trajectory.

## Decision record

For every ruler/turn record:

- operational invocation order;
- identity;
- observation;
- visible tree/revision;
- files actually retrieved;
- messages available;
- model/config;
- latency and inference usage;
- parse/repair failures;
- final orders;
- concise supplied statement.

Never store hidden chain-of-thought.

## Comparison modes

### Same model, different minds
Identical model/configuration, different personality/history/knowledge.

### Same universe, different models
Same initial seeds/personality set, different local models.

### Ablations
Useful variants include no persistent memory, no diplomacy, tree-only retrieval, different context limits, different sampling, personality removed and scripted baselines.

## Metrics

### Survival
Turns survived; exile/restoration; planets controlled.

### Economy
Production; efficiency; infrastructure growth; idle/wasted resources.

### Military
Fleet value; losses inflicted/suffered; defenses; captures; salvage.

### Expansion
Colonies; captures/losses; time to expansion.

### Intelligence
Espionage; age of intelligence used; retrieval behavior; measurable relevant-memory retrieval.

### Diplomacy
Alliances; agreements; detectable honored/broken commitments; aid; message volume.

### Agent operation
Latency; tokens; tool calls; files read; malformed outputs; repairs; zero-order turns.

## No universal intelligence score

Expose raw measurements. Experiment-specific aggregates are allowed but must not replace underlying metrics.

## Replay

Completed runs replay without invoking agents and distinguish objective historical truth from each ruler's historical knowledge.

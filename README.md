# AGame — Agent Game

**One model. Many minds. One universe.**

AGame is a deterministic persistent strategy simulation built for autonomous local-AI rulers.

The engine owns truth. Agents choose actions. [Jikko](https://github.com/KakkoiDev/jikko) owns each ruler's knowledge, memory and plans.

## v1 implementation

- seeded 32-system / 128-planet universe with 8 equal-start empires
- metal, crystal and deuterium economy
- buildings, research and ship production queues, with 50%-refund cancellation
- fleets (form, split), graph travel and fuel
- colonization, physical transport and inter-empire trade, six-round combat against planets or fleets, capture, recycling
- tiered espionage reports with seeded detection
- diplomacy: messages to an empire, an alliance or everyone (delivered next turn), invitation-based alliances, automatic treaty breach when attacking an ally, a public record of hostilities
- government-in-exile/elimination state and the canonical end conditions (600 turns, last empire standing, no sovereignty possible) with final standings
- simultaneous turn barrier through one turn loop (`engine.Runner`) shared by the CLI and the browser
- decision harness with repair of invalid model output, per-ruler timeouts, decision records and reflection phases after major events; rejected orders always carry a reason the ruler sees next turn
- replayable runs: initial state, turn log, event log and state hashes
- generic agent interface, deterministic autopilot and OpenAI-compatible local-model adapter
- Jikko tree/batch-read adapter
- content-addressed optional generated-media cache
- observer dashboard and browser PWA showing standings, alliances, hostilities, battles, diplomacy and rejected orders

The normative rules remain under [spec/](spec/vision.md); implementation decisions are recorded in [spec/decisions.md](spec/decisions.md). Browser/local save, Git synchronization, and multi-universe semantics are defined in [spec/storage-and-sync.md](spec/storage-and-sync.md).

## Run

Requires Go 1.24+.

```sh
go run ./cmd/agame new 42          # create a universe (never overwrites one)
go run ./cmd/agame turn            # play one turn
go run ./cmd/agame run 120         # play up to 120 turns, stopping at an end condition
go run ./cmd/agame replay          # re-resolve the run from its log and verify every hash
go run ./cmd/agame observe e03 40  # what ruler e03 saw at turn 40
go run ./cmd/agame serve           # observer dashboard
```

State defaults to `./run`; override with `AGAME_RUN`. The observer dashboard is at `http://localhost:8080`.

A run directory holds:

| File | Content |
| --- | --- |
| `run.json` | ruleset and prompt versions, seed, initial-state hash, ruler roster, budget |
| `initial.json` | S(0), the replay starting point |
| `world.json` | the current authoritative state |
| `turns.jsonl` | per turn: submitted, accepted and rejected orders (with reasons), events, resulting state hash |
| `events.jsonl` | the append-only event log |
| `decisions.jsonl` | per ruler and turn: prompt hash, raw attempts, repairs, final orders, statement, rejections, failures, latency |
| `reflections.jsonl` | reflection phases offered after major events and annual reviews, with the ruler's note |
| `result.json` | end condition, final state hash, standings and agent-operation metrics |

Rulers default to the deterministic autopilot. Set `AGAME_MODEL_ENDPOINT` (and optionally `AGAME_MODEL`, `AGAME_API_KEY`) to let a local OpenAI-compatible model rule every empire. `AGAME_TURN_LIMIT` and `AGAME_TIMEOUT` (a Go duration such as `90s`) override the canonical 600-turn limit and the per-ruler timeout.

## Local models

`agent.OpenAICompatible` targets `/v1/chat/completions`, allowing local OpenAI-compatible servers such as LM Studio or Ollama's compatibility endpoint. Malformed output, or orders that fail validation, are sent back to the model with the engine's reasons for up to two repairs; after that the ruler submits zero orders for the turn.

The engine itself has no AI dependency. Scripted agents implement the same `agent.Agent` interface.

## Architecture

> **LLMs choose. The engine executes.**

AGame state is authoritative. Jikko stores cognition. Generated media is optional presentation and never changes simulation truth.

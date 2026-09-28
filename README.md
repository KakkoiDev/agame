# AGame — Agent Game

**One model. Many minds. One universe.**

AGame is a deterministic persistent strategy simulation built for autonomous local-AI rulers.

The engine owns truth. Agents choose actions. [Jikko](https://github.com/KakkoiDev/jikko) owns each ruler's knowledge, memory and plans.

## v1 implementation

- seeded 32-system / 128-planet universe with 8 equal-start empires
- metal, crystal and deuterium economy
- buildings, research and ship production queues
- fleets, graph travel and fuel
- colonization, physical transport, combat/capture, espionage and recycling
- government-in-exile/elimination state
- simultaneous turn barrier
- JSON snapshots + JSONL event log
- generic agent interface and OpenAI-compatible local-model adapter
- Jikko tree/batch-read adapter
- content-addressed optional generated-media cache
- minimal observer dashboard

The normative rules remain under [spec/](spec/vision.md).

## Run

Requires Go 1.24+.

```sh
go run ./cmd/agame new 42
go run ./cmd/agame turn
go run ./cmd/agame serve
```

State defaults to `./run`; override with `AGAME_RUN`. The observer dashboard is at `http://localhost:8080`.

## Local models

`agent.OpenAICompatible` targets `/v1/chat/completions`, allowing local OpenAI-compatible servers such as LM Studio or Ollama's compatibility endpoint.

The engine itself has no AI dependency. Scripted agents implement the same `agent.Agent` interface.

## Architecture

> **LLMs choose. The engine executes.**

AGame state is authoritative. Jikko stores cognition. Generated media is optional presentation and never changes simulation truth.

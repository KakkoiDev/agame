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
- swappable structured-decision/classifier providers and independent text generators
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

## Swappable local cognition

AGame separates **decision** from **language**:

```text
world observation -> DecisionProvider -> structured gameplay orders / writing intents
                                      -> TextGenerator -> messages, reports, memories
```

`DecisionProvider` is model-neutral. `agent.ClassifierHTTP` targets a tiny local structured-classification service, so GLiNER2.5/GLiNER2.5-Decide or another classifier can be swapped without changing the engine. `ScriptedDecision` supports deterministic bots/tests.

`TextGenerator` is independent. `agent.OpenAICompatible` can point at a local 2B model through LM Studio, Ollama's OpenAI-compatible endpoint, or another compatible server. It is invoked only for requested prose.

`JikkoSink` writes generated reports, memories, plans and other prose to Jikko. The decision provider does not need to generate Markdown.

`OpenAICompatible` also implements `DecisionProvider`, so experiments can swap the classifier out for a generative decision model without changing the runner.

The engine itself has no AI dependency.

## Architecture

> **LLMs choose. The engine executes.**

AGame state is authoritative. Jikko stores cognition. Generated media is optional presentation and never changes simulation truth.

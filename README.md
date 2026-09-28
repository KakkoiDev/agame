# AGame — Agent Game

**One model. Many minds. One universe.**

AGame is a persistent strategy simulation designed primarily for autonomous AI players.

It is inspired by browser empire games such as OGame, but its purpose is different: create a small deterministic universe in which local language models can build empires, remember history, negotiate, deceive, cooperate, wage war, lose their homeland, recover, and change over long periods.

AGame is also an experiment. A single local model may role-play every ruler. Because every ruler uses the same underlying model, differences in outcome can be attributed to personality, accumulated knowledge, memory, relationships and decisions rather than raw model strength.

This repository is currently **specification only. No implementation should begin until the v1 specification is accepted.**

## Core rule

> **LLMs choose. The engine executes.**

The game engine owns objective truth and deterministically validates/resolves orders. Models never directly modify world state.

## Architecture

AGame has three deliberately separate layers:

1. **World** — objective game state owned by AGame.
2. **Knowledge** — what a ruler is allowed to know or currently believes, stored through Jikko.
3. **Memory** — how a ruler interprets its experience, also stored through Jikko.

A ruler can therefore be wrong. The engine may know that an enemy owns 53 cruisers while another emperor remembers observing 27 cruisers 32 turns ago.

## Specifications

- [Vision and principles](spec/vision.md)
- [Game rules](spec/game.md)
- [Agents and turns](spec/agents.md)
- [Jikko cognition and knowledge](spec/jikko.md)
- [Diplomacy](spec/diplomacy.md)
- [Reproducibility and benchmarking](spec/benchmark.md)
- [Dashboard and history](spec/dashboard.md)
- [Generated media](spec/media.md)
- [Decisions and open questions](spec/decisions.md)

## Initial implementation direction

The current intended implementation stack is Go + SQLite + server-rendered HTML/HTMX. Local model adapters should support at least Ollama and LM Studio.

These are implementation directions, not game semantics. The engine must remain usable without an LLM through the same agent interface so scripted bots and tests can play.

## Non-goals for v1

- Human-controlled empires.
- Real-time combat.
- 3D gameplay.
- LLM-authored game rules.
- LLM access to hidden world state.
- Cloud AI as a requirement.
- Embedding/RAG infrastructure.
- Making generated images/audio necessary for simulation correctness.

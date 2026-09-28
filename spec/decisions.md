# Decisions and open questions

This file records design decisions made before implementation.

## D1 — Deterministic engine owns truth

**Decision:** AGame, not the LLM, owns authoritative world state.

**Reason:** reproducibility, testability and prevention of narrative hallucinations becoming game facts.

## D2 — Simultaneous strategic turns

**Decision:** all rulers decide from the same frozen snapshot; orders resolve only after all decisions are collected.

**Reason:** sequential local inference must not give later-invoked rulers an information advantage.

## D3 — One or more orders

**Decision:** a turn may contain zero, one or multiple orders.

**Reason:** strategy requires concurrent economic, fleet and diplomatic decisions; forcing one artificial action per turn is unnecessary.

## D4 — Planet capture preserves sovereignty target

**Decision:** enemy planets can be captured intact rather than only destroyed.

**Reason:** territorial history and changing borders are central to the simulation.

## D5 — Government in exile

**Decision:** loss of the final planet is not elimination while viable colony/escape-capable assets remain.

**Reason:** exile, recolonization and restoration create meaningful long-horizon history.

## D6 — Shared model, isolated minds

**Decision:** canonical experiments may use one local model for every ruler, invoked independently with isolated context.

**Reason:** separates personality/memory/context effects from model capability.

## D7 — Jikko for cognition, not reality

**Decision:** authoritative world truth stays in AGame; Jikko stores identity, knowledge, beliefs, memories, plans and communication.

**Reason:** preserves imperfect knowledge and prevents two sources of truth.

## D8 — Retrieval is agent responsibility

**Decision:** no automatic Jikko context assembly. Agents receive a permission-filtered file tree and choose files to read/batch-read.

**Reason:** retrieval and memory management are themselves aspects of agent intelligence.

## D9 — No new Jikko primitives for AGame

**Decision:** use Identity, Document, Task and View.

**Reason:** AGame is a useful stress test of Jikko's minimal model; concepts such as memory/message/faction do not yet justify new primitives.

## D10 — No hidden relationship score

**Decision:** social interpretation lives primarily in agent-authored Jikko knowledge rather than an engine friendship scalar.

**Reason:** makes evolving relationships inspectable and permits disagreement/deception.

## D11 — Messages affect the next turn by default

**Decision:** messages written during turn t arrive after the decision barrier and can affect t+1.

**Reason:** preserves simultaneous decisions without adding negotiation sub-phases.

## D12 — Media is non-authoritative

**Decision:** local image/music/SFX generation is optional presentation.

**Reason:** simulation reproducibility must not depend on expensive generative media.

## D13 — No implementation yet

**Decision:** this repository begins as specifications only.

**Reason:** settle the game/agent boundaries before code makes accidental design decisions permanent.

# Open questions before v1 implementation

These should be resolved through specification work rather than silently chosen in code.

1. Exact building list and cost/progression formulas.
2. Exact research tree.
3. Exact ship roster and combat statistics.
4. Combat/capture/retreat resolution algorithm.
5. Espionage noise and information tiers.
6. Maximum/default number of orders per turn, if any.
7. Construction/research queue semantics.
8. Fleet travel-time model and whether movement spans multiple turns.
9. Resource transfer/trade mechanics.
10. Mechanical representation, if any, of formal alliances and treaties.
11. Universe topology and initial placement rules.
12. Default number of rulers and planets for the canonical benchmark.
13. Rules for ruler succession, if characters can die/retire.
14. Which events automatically invite a reflection phase.
15. Exact structured agent/tool schemas.
16. Context/tool-call budget for small models.
17. Rules for editing/deleting past memories versus appending corrections.
18. What constitutes a viable escape asset for elimination.
19. Default end condition for showcase runs.
20. Whether generated media belongs inside the run directory or an external cache.

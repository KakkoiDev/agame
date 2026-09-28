# Vision and principles

## Purpose

AGame is a long-horizon AI strategy sandbox and benchmark.

The interesting object is not a single prompt response. It is the history produced when agents must repeatedly make decisions while operating with limited information, persistent memory, personalities, relationships, resource constraints and consequences.

## Design principles

### One model, many minds

The canonical experiment runs all rulers sequentially through the same local language model. Each invocation receives only that ruler's identity, accessible Jikko workspace, observation and chosen retrieved files.

A model must never inherit another ruler's transient context.

### Persistent consequences

The universe persists across turns. Empires grow and collapse. Planets change hands. Diplomatic history matters. Memories can affect decisions decades later.

### Imperfect knowledge is a feature

World truth and agent knowledge must remain separate.

No omniscient prompt should summarize the world for a ruler.

### Intelligence includes retrieval

Jikko exposes a permission-filtered file tree. The model decides what to read, potentially in batches. AGame does not choose which memories are relevant on its behalf.

Failure to retrieve an important memory is a meaningful agent failure.

### Deterministic engine, nondeterministic minds

Given a seed, initial state and complete set of submitted orders, the engine's resolution must be reproducible.

Model sampling may vary independently and must be recorded.

### Simultaneous decisions

All rulers decide from the same turn snapshot. No ruler receives information caused by another ruler's orders from the current turn before submitting its own orders.

### Small-model first

The agent protocol and observations should be compact enough to make 2B–8B local models useful.

### AI is optional to the engine

An agent is an interface, not an LLM. Scripted agents, random agents and test fixtures must be able to participate.

### Generated media is presentation

Images, music and sound effects can enrich the historical simulation but cannot change authoritative game state.

## Success

AGame succeeds if a long run produces a history that can be inspected and quantitatively compared:

- why an empire prospered or failed;
- what a ruler knew when it acted;
- whether it remembered relevant history;
- whether alliances were honored;
- how personalities diverged despite a shared base model;
- how different models behave in the same seeded universe.

# Generated media

## Status

Generated media is optional and must not affect simulation correctness.

A run must remain fully playable and replayable with media generation disabled.

## Purpose

Local generative models can turn the simulation's history into a living audiovisual chronicle.

Potential assets:

- ruler portraits;
- empire flags/emblems;
- planet/system illustrations;
- propaganda posters;
- historical-event art;
- ambient empire themes;
- peace/war/victory/mourning music;
- UI sounds;
- engines, battle ambience and other SFX.

## Image generation

The media adapter should be model-agnostic.

A practical Apple-Silicon target can use a local FLUX-family model, but AGame's specification must not depend on a specific checkpoint.

Prompts may derive from public/historical facts plus the relevant empire's authored aesthetic identity.

Generated images never reveal hidden world truth to an agent unless deliberately delivered as knowledge.

## Music

Music may be generated locally and cached.

A ruler/empire can maintain an authored musical identity, for example:

```text
restrained orchestral ambient
warm analog synth
distant choir
slow military percussion
melancholic but hopeful
```

The system can derive tracks for eras/events such as:

- peace;
- war;
- victory;
- mourning;
- exile;
- restoration.

ACE-Step-class local models are a likely implementation target, but the adapter remains generic.

## Sound effects

A Stable-Audio-class local model may generate UI/ambient SFX.

Again, this is an adapter choice, not a core dependency.

## Historical generation

Significant events may enqueue media jobs after authoritative resolution.

Example:

```text
capital captured
-> event committed
-> optional media job
-> event illustration + mourning theme
-> cached and attached to dashboard history
```

Failure to generate media never rolls back or changes the event.

## Resource management

On unified-memory Macs, large generative models should normally load on demand rather than remain resident together.

The decision LLM may remain resident while image/audio generators are loaded for queued media jobs and then released.

Media generation should preferably be asynchronous relative to simulation progress.

## Provenance

Generated assets should record:

- run/event/empire association;
- model;
- model version/checkpoint;
- generation parameters/seed where supported;
- source prompt;
- creation time.

This allows regeneration and comparison.

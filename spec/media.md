# Generated media

## Status

Generated media is optional and never affects simulation correctness.

## Storage

Authoritative run data and generated media are separated.

A run directory contains **media manifests/provenance and stable asset references**, not necessarily the heavy binary assets themselves.

Generated binaries live in a content-addressed external media cache, e.g. by SHA-256. This avoids duplicating large files across reruns while keeping a run reproducible.

Exporting/archiving a run may optionally bundle referenced assets.

## Purpose

Potential assets include ruler portraits, flags, planets, propaganda, historical art, empire themes, event music, UI sounds and ambience.

## Image generation

The adapter is model-agnostic. Local FLUX-class models are a practical Apple-Silicon target but not a specification dependency.

Prompts may derive from public/historical facts and authored aesthetic identity. Generated images never reveal hidden truth unless deliberately delivered as knowledge.

## Music

Music is generated locally and cached. An empire can maintain an authored musical identity, with tracks for peace, war, victory, mourning, exile and restoration.

ACE-Step-class models are a likely adapter target, not a dependency.

## Sound effects

Stable-Audio-class local models may generate UI/ambient SFX through the same generic media-job approach.

## Historical generation

Significant resolved events may enqueue media jobs. Failure never rolls back or alters simulation state.

## Resource management

On unified-memory machines, large media models should load on demand. Media jobs should normally run asynchronously relative to simulation turns.

## Provenance

Every asset records:

- content hash;
- run/event/empire association;
- model/checkpoint;
- generation parameters/seed;
- source prompt;
- creation time.

This permits regeneration and comparison.

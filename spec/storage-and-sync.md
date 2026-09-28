# Storage, Git, saves and synchronization

## Decision

AGame owns the meaning of Git branches. Jikko owns generic Git capability.

The canonical mapping is:

```text
repository = one player's AGame library
branch     = one universe
commit     = one atomic universe transaction / completed turn
OPFS       = local working copies
remote Git = optional synchronized backup/library
```

This is an AGame convention, not a Jikko primitive.

## Why one universe per branch

A player must be able to create multiple independent games from the start without creating multiple repositories. A single Git repository keeps the complete AGame library easy to back up, move and inspect.

Each universe receives a stable ID and a human-readable branch name under a reserved namespace, for example:

```text
universe/01J...-first-run
universe/01J...-qwen-benchmark
universe/01J...-roman-empire
```

The stable ID is authoritative; the readable suffix may change.

A new game creates a new universe branch, initializes the complete world/Jikko state, and records a Turn 0 commit. It must never overwrite or reuse an existing universe branch.

## Local-first behavior

OPFS is the live working store. Remote Git is optional durability and synchronization.

Gameplay never waits for the network:

1. open the locally available universe immediately;
2. commit completed transactions locally;
3. enqueue a background push of the current universe branch;
4. if offline or the push fails, retain the commits locally and retry later.

Remote failure must not make a playable local universe unavailable.

## Selective synchronization

A browser/device does not need every universe locally.

On connection to a remote library, AGame may discover remote `universe/*` branch names without materializing every branch. The game library can therefore show:

- local universes;
- synced universes;
- remote-only universes.

Selecting a remote-only universe fetches only that branch and materializes it into the local OPFS working store. AGame must use Jikko's generic branch-scoped Git operations; Jikko must not contain `universe/*` policy.

## Commit and push boundary

Do not commit individual file writes. One coherent game transaction is one Git commit.

For the canonical turn loop, all state, event-log and Jikko changes belonging to a completed turn are committed together. Only after the local commit succeeds may AGame schedule a remote push.

This gives meaningful history and prevents a remote from observing half-written turns.

## Multiple browsers and divergence

Before advancing a synchronized universe, AGame should fetch its remote branch. A simple fast-forward is safe.

If two devices advance the same universe independently, authoritative world state must **not** be silently content-merged. Divergence represents two different histories.

AGame must surface the divergence and preserve both histories. The user may:

- keep the local history;
- keep the remote history; or
- fork one history into a newly named universe branch.

A forked universe retains Git ancestry but becomes an independent universe from that point onward.

Automatic textual merge may eventually be appropriate for non-authoritative content, but it must never silently reconcile divergent authoritative world histories.

## Authentication and secrets

Jikko exposes provider-neutral remote Git operations and accepts authentication from its caller. AGame owns provider login and credential lifecycle.

Credentials must not be written into:

- universe files;
- Jikko documents;
- commits;
- ZIP exports;
- Git remote URLs containing embedded secrets.

GitHub can be the first synchronization provider, but AGame's universe model must not depend on GitHub-specific semantics.

## Backup and portability

Two backup levels remain useful:

- repository backup: preserves every synchronized universe branch and full Git history;
- universe ZIP: exports one checked-out universe without requiring Git history.

Git synchronization complements ZIP export; it does not replace it.

## Library UX

The initial AGame screen is a universe library, not an implicit single save.

Required actions:

- New Game
- Continue a local universe
- Download/continue a remote-only universe
- Import Universe ZIP
- Export Universe
- connect/disconnect synchronization

The library must make local/synced/remote-only state visible without requiring Git terminology from ordinary players.

## Jikko boundary

Jikko owns:

- Git repository initialization;
- local commits and history;
- branch creation/listing/checkout/merge primitives;
- remote configuration;
- remote branch discovery;
- branch-scoped fetch/pull/push;
- browser OPFS Git adapter;
- native Git adapter;
- portable workspace ZIP export.

AGame owns:

- the `universe/*` namespace;
- one universe per branch;
- universe IDs and names;
- New Game behavior;
- local universe catalog;
- selective universe synchronization;
- push-after-transaction policy;
- divergence/fork UX;
- which files constitute authoritative game state;
- when a turn/transaction is complete enough to commit.

The invariant is:

> Jikko knows Git. AGame knows universes.

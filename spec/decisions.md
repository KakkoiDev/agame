# Decisions — v1 pre-implementation

All previously open v1 design questions are resolved here. Implementation must not silently change these semantics; new uncertainty should be recorded as a new decision.

## D1 — Deterministic engine owns truth
AGame, not the LLM, owns authoritative world state.

## D2 — Simultaneous strategic turns
All rulers decide from the same frozen snapshot; resolution begins only after all decisions are collected.

## D3 — No global order cap
A turn may contain zero or many orders. Physical queues, fleets and resources constrain concurrency instead of arbitrary action points.

## D4 — Planet capture preserves infrastructure
Enemy planets can be captured intact; capture rules determine surviving resources/queues.

## D5 — Government in exile
Final-planet loss is not elimination while a viable Colony Ark can still restore sovereignty.

## D6 — Shared model, isolated minds
Canonical experiments use one model/config for every ruler with completely isolated transient contexts.

## D7 — Jikko for cognition, not reality
World truth is AGame state; Jikko stores identity, knowledge, belief, memory, plans and communication.

## D8 — Retrieval is agent responsibility
No automatic memory/context selection. Permission-filtered tree + explicit reads/search.

## D9 — No AGame-specific Jikko primitives
Use Identity, Document, Task and View.

## D10 — No hidden relationship score
Social interpretation remains inspectable agent-authored knowledge.

## D11 — Messages affect the next turn by default
Turn-t messages are delivered after the decision barrier.

## D12 — Media is non-authoritative
Images/music/SFX are optional presentation.

## D13 — Specification before implementation
No implementation until v1 questions are closed.

## D14 — Canonical universe and clock
One turn is one month. 8 empires inhabit a seeded graph of 32 systems with 4 planetary slots each (128 slots). Homeworlds use fair seeded max-distance placement and equal material starts.

## D15 — Buildings and progression
Seven building families: Metal Mine, Crystal Mine, Deuterium Extractor, Infrastructure, Research Lab, Shipyard, Defense Grid. Costs double each level; exact base costs/effects are normative in `game.md`.

## D16 — Research tree
Six technologies: Industry, Propulsion, Weapons, Shields, Sensors, Colonization. Costs double each level; exact effects are in `game.md`.

## D17 — Ship roster
Six classes: Scout, Transport, Colony Ark, Frigate, Cruiser, Recycler. Their v1 costs/stats are normative in `game.md`.

## D18 — Combat/capture/retreat
Combat lasts at most six simultaneous-damage rounds with seeded target allocation. Surviving attackers retreat after a six-round stalemate. A surviving combat ship captures an undefended planet. Exact capture effects are in `game.md`.

## D19 — Espionage
Scout espionage uses Sensors-level difference to produce four information tiers with bounded deterministic noise. Intelligence is timestamped and can become stale.

## D20 — Queues
One construction and one shipyard queue per planet; one research queue per empire. Progress spans turns. Resources are paid on queueing; cancellation refunds 50%.

## D21 — Travel
Movement follows graph routes. Base speed is one edge/turn; Propulsion improves speed. Deuterium fuel is paid at departure. Multi-turn travel is normal.

## D22 — Trade
Resources move physically in cargo. Negotiated exchange is not engine-enforced; betrayal/default is possible.

## D23 — Alliance mechanics
The engine recognizes alliance membership and hostility but does not enforce most promises. Attacking an ally is legal, automatically leaves the alliance and records a breach.

## D24 — Rulers
v1 has no aging/death/succession. Persistent ruler identities survive the canonical 50-year horizon unless their empire is eliminated.

## D25 — Reflection
Major territorial, military, exile/restoration and alliance events trigger optional reflection; an annual review occurs every 12 turns.

## D26 — Agent/tool schema
Canonical tools are Jikko tree/read/read-many/search/create/update/mentions plus observable game inspection and one structured order submission envelope. Stable game IDs are supplied by the engine.

## D27 — Small-model budget
Canonical 2B–8B runs target 8k initial context, 8 tool-call rounds, 16 files/read-many, 24 files returned, 24k aggregate input tokens, 2k final output and at most two repair attempts.

## D28 — Memory correction, not silent revisionism
Plans may be updated. Historical observations/memories must preserve corrections/retractions in auditable history; durable deletion requires a tombstone/retraction event.

## D29 — Viable exile
A planetless empire survives only if it has a Colony Ark capable of reaching a planet under the rules. Ordinary military fleets alone do not prevent elimination.

## D30 — Canonical end condition
A canonical run ends at 600 turns (50 years), one remaining empire, a true engine-detectable sovereignty dead-end, or explicit observer stop for non-benchmark runs.

## D31 — Media cache
Run data stores media manifests/references; heavy generated binaries live in a content-addressed external cache. Portable exports may bundle them.

## D32 — Reproducible benchmarks
Canonical comparisons use multiple seeds and record model/checkpoint, quantization, sampling, harness, Jikko revision and context/tool budgets.


## D33 — Jikko owns Git capability; AGame owns branch semantics
Jikko provides generic Git operations in native and browser runtimes. It must not learn AGame concepts such as universes or save slots. AGame assigns meaning to those branches: **Jikko knows Git; AGame knows universes.**

## D34 — One library repository, one universe per branch
One Git repository represents the player's AGame library. Every independent game is a complete `universe/*` branch. This keeps the library easy to back up and permits branch-scoped selective synchronization. Branches are not reserved for alternate timelines.

## D35 — Multiple games are first-class from Turn 0
AGame has no implicit singleton save. New Game creates a new stable universe ID and a new universe branch, initializes complete state, and records a Turn 0 commit without changing existing universes.

## D36 — OPFS is live state; remote Git is optional synchronization
Browser gameplay uses OPFS immediately and remains fully usable offline. A remote Git provider is a synchronized durable replica/library, not the gameplay backend. Failed synchronization never invalidates a successful local turn.

## D37 — Push atomic transactions, not individual file writes
A completed turn/transaction and all associated authoritative/Jikko changes form one local commit. Only a successful local commit may be queued for push. This prevents remote half-turn states and gives meaningful history.

## D38 — Selective universe synchronization
A device may discover remote `universe/*` branches without downloading every universe. Only universes selected for play need to be fetched/materialized locally.

## D39 — Divergent authoritative histories never auto-merge
If two devices independently advance the same universe, the resulting authoritative world histories must not be content-merged automatically. AGame surfaces the divergence and lets the player retain one history or fork the other into a new universe branch while preserving Git ancestry.

## D40 — Provider authentication stays outside Jikko source
Jikko accepts provider-neutral Git authentication from its caller. AGame owns GitHub/provider login and credential lifecycle. Secrets never enter universe/Jikko source, commits, ZIP exports, or credential-bearing remote URLs.

## D41 — ZIP remains an independent portability path
Repository synchronization provides convenient continuous backup of the library. A universe ZIP remains the provider-independent single-universe export/import format.


## D42 — Static-browser GitHub sync uses Jikko's GitHub REST transport
GitHub's Git smart-HTTP endpoints do not provide the CORS behavior required for direct browser isomorphic-git synchronization without a proxy. AGame must not send private save credentials through a public CORS proxy. For GitHub, the static browser therefore uses Jikko's browser-safe Git Database REST transport to create trees/commits and move `universe/*` refs atomically. Local history remains ordinary isomorphic-git in OPFS; AGame still owns universe branch semantics.

## D43 — GitHub credentials are ephemeral by default
The first browser UI accepts a GitHub credential only for the active page lifetime and passes it directly to the Jikko transport. It is not written to OPFS, Web Storage, universe files, commits, or exports. A future OAuth/device-flow UI may replace manual credential entry without changing storage semantics.


## D44 — Immediate-play fallback decision provider
The browser must be playable before optional local ML assets finish downloading. The canonical browser runner therefore has a deterministic zero-download autopilot decision provider that emits ordinary engine orders. Classifier/model decision providers remain swappable and may replace it per ruler or benchmark. The fallback never bypasses engine validation.

## D45 — PWA shell and GitHub Pages distribution
The browser build is a first-class PWA deployed as a static GitHub Pages artifact. The service worker caches only application/runtime assets; authoritative universes remain in OPFS and Git. GitHub backup remains optional and must never become a prerequisite for local play.

## D46 — Atomic cognition record
Each completed browser turn stores the world snapshot plus the rulers' auditable statements and resulting events before the Git turn commit. This is the first browser bridge between decision output and durable Jikko-style cognitive history; richer per-ruler Markdown organization can evolve without changing the atomic turn boundary.

## D47 — Combat resolution details (refines D18)
Each arriving attack fleet fights alone, in fleet-ID order. Defenders are the ships docked at the target planet plus the defender's fleets idle in that system. Each round every living ship, and each Defense Grid level as one battery of 20 power (Shields-scaled), hits one living enemy ship chosen by a battle RNG seeded from world seed + turn + planet + attacking fleet; damage is simultaneous and accumulates against Shields-scaled hull. Combat ends after 6 rounds, when the attacker is gone, or when the defender has no ships and no grid. A surviving attacker that does not capture retreats to its previous route node at no fuel cost, or stays if it attacked in place. Debris is 30% of destroyed metal/crystal cost, rounded down once per side per battle. Fleets left with zero ships are removed; a consumed Colony Ark's cargo stays on the new colony.

## D48 — Observation visibility (until espionage reports feed observations)
Every ruler sees the public hyperspace graph and every planet's id, system, slot, owner and homeworld flag. Its own planets and fleets are shown in full. Foreign fleets are seen only in systems where the ruler owns a planet or has a fleet, as owner plus a size band (1-4, 5-19, 20-49, 50+); debris is seen only in those systems. Foreign planet contents (resources, buildings, docked ships, queues) stay hidden and are learned through espionage. The observation also lists the legal order types with their field shapes.

## D49 — Fuel from carried deuterium; viable Ark test (refines D21, D29)
At departure, fuel is paid by the empire's own planet in the fleet's system when that planet can cover fuel plus any cargo being loaded; otherwise it is paid from deuterium the fleet already carries (cargo is still loaded only from a planet). A planetless empire's Colony Ark is viable if it is on a paid route, or if it can pay the fuel to reach some unowned planet it could colonize.

# Status

**There are no unresolved v1 questions from the original pre-implementation list.**

Future balancing discovered through simulation is expected, but it must be an explicit ruleset/version change rather than an undocumented implementation choice.

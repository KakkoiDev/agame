# Game rules

## Universe

A game is a persistent seeded universe containing star systems, planets, empires, fleets and resources.

A strategic turn represents a configurable period such as a week or month. The exact narrative duration does not alter resolution semantics.

## Resources

v1 uses three resources:

- **Metal** — structures and hulls.
- **Crystal** — electronics, research and advanced construction.
- **Deuterium** — energy/fuel and strategic movement.

Production belongs to planets. Storage and spending are authoritative engine state.

## Economy

The engine should use a deliberately small building set. The first implementation should prefer a few meaningful choices over a large technology tree.

Required capability classes:

- metal production;
- crystal production;
- deuterium production;
- general infrastructure/construction capacity;
- research capacity;
- shipyard capacity;
- defensive capability.

Exact names, costs and formulas remain balancing parameters rather than additional conceptual systems.

## Research

Research unlocks or improves capabilities. The initial tree should remain small and legible to small language models.

Required areas:

- economy;
- propulsion;
- weapons;
- defenses;
- sensors/espionage;
- colonization.

## Fleets

The ship roster should be small.

Required roles:

- scout/spy;
- transport;
- colony/escape vessel;
- light combat;
- heavy combat;
- recycler/salvage.

Ships are concrete world assets. Fleets are groupings of ships with an owner, location and mission.

## Orders

An empire may submit zero or more orders per turn, subject to resources, capacities and game limits.

Core order families:

- construct;
- research;
- move fleet;
- colonize;
- spy;
- attack;
- defend/redeploy;
- transport resources;
- recycle/salvage;
- diplomatic action.

Doing nothing is always legal.

Invalid orders do not become true because an LLM requested them. The engine rejects or deterministically normalizes them according to the order contract.

## Colonization and sovereignty

Planets have sovereign owners.

Unclaimed valid planets may be colonized.

A planet may also be captured from another empire. Capture transfers the planet **intact** according to combat/capture rules: ownership changes rather than the planet being automatically destroyed.

Buildings, stored resources and other surviving planetary assets become subject to the new sovereignty rules.

## Combat

Combat is engine-resolved and deterministic given the snapshot, submitted orders and seeded randomness.

The first ruleset should prioritize explainability over tactical simulation complexity.

Resolution must emit enough structured evidence to explain losses, captures, retreats and salvage.

## Espionage

Espionage produces observations, not direct access to truth.

Reports may expose partial or noisy information depending on capability and circumstances. Once delivered, the report becomes knowledge available to the receiving ruler and may become stale.

## Recycling and salvage

Destroyed fleets may produce recoverable debris. Recycler/salvage assets can claim it according to deterministic rules.

## Elimination and government in exile

Losing the final planet does **not** automatically eliminate an empire.

An empire remains viable while it possesses a realistic means of continuing sovereign existence, especially a colony/escape-capable asset.

A government in exile may:

- flee;
- hide;
- negotiate;
- seek protection;
- recolonize;
- capture a new planet;
- attempt to retake its homeland.

An empire is eliminated only when it owns no planets **and** has no viable colony/escape-capable assets or other rules-defined path to re-establish sovereignty.

This rule is important because losing a capital should create history rather than necessarily terminate a character.

## Victory

v1 should not require a single universal victory condition.

Runs may end by:

- configured turn limit;
- one surviving sovereign empire;
- benchmark-specific condition;
- explicit observer stop.

Metrics are recorded independently of narrative victory.

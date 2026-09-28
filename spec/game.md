# Game rules

## Canonical universe

The canonical v1 benchmark is:

- **1 turn = 1 month** of narrative time.
- **8 empires**.
- **32 star systems**.
- **4 planetary slots per system**, for **128 planetary slots**.
- Every empire begins with one inhabited homeworld.
- The universe is a seeded connected graph of star systems joined by hyperspace routes.
- Home systems are distributed by a seeded max-distance placement pass so no two starts are immediate neighbors when the graph permits it.
- Each homeworld starts with the same base infrastructure/resources. Personality and knowledge differ; starting material advantage does not.

The graph is intentionally more important than geometric coordinates. Routes create borders, chokepoints, interiors and contested systems that small models can reason about.

## Resources

v1 has three resources:

- **Metal** — structures and hulls.
- **Crystal** — electronics, research and advanced construction.
- **Deuterium** — fuel, advanced systems and strategic movement.

Resource amounts are integers.

Each inhabited planet produces resources at the end of every turn after ownership/combat resolution.

## Economy

There are exactly seven building families in v1.

| Building | Base cost M/C/D | Effect per level |
| --- | ---: | --- |
| Metal Mine | 60/15/0 | +30 metal/turn |
| Crystal Mine | 48/24/0 | +20 crystal/turn |
| Deuterium Extractor | 36/30/0 | +12 deuterium/turn |
| Infrastructure | 80/40/0 | +1 construction point/turn |
| Research Lab | 60/90/30 | contributes +1 research point/turn |
| Shipyard | 100/60/20 | +1 shipyard point/turn |
| Defense Grid | 120/80/20 | +20 planetary defense power |

Every homeworld starts at level 1 in each building.

Level L costs `base × 2^(L-1)`. Building a level requires construction points equal to `ceil(total resource cost / 100)`. A planet has one construction queue. Unused points carry progress on the queued item, not as banked capacity.

Mines have no energy subgame in v1.

## Research

Research is empire-wide. Labs on all owned planets contribute research points to one empire research queue.

There are exactly six technologies:

| Technology | Base cost M/C/D | Effect per level |
| --- | ---: | --- |
| Industry | 150/100/20 | +10% resource production |
| Propulsion | 100/150/50 | +1 route-edge fleet speed per 3 levels; -5% fuel/level |
| Weapons | 100/180/40 | +10% ship attack |
| Shields | 120/160/40 | +10% ship/planet effective defense |
| Sensors | 80/200/60 | improves espionage accuracy/range |
| Colonization | 200/250/120 | unlocks Colony Ark at L1; +1 sustainable colony per level |

Level L costs `base × 2^(L-1)`. Research points required are `ceil(total resource cost / 100)`.

Resources are paid when construction/research is queued. Canceling returns 50% of the paid resources and loses progress.

## Ships

v1 has six ship classes.

| Ship | Cost M/C/D | Hull | Attack | Cargo | Fuel/edge | Notes |
| --- | ---: | ---: | ---: | ---: | ---: | --- |
| Scout | 20/40/20 | 20 | 5 | 0 | 2 | espionage asset |
| Transport | 60/30/30 | 60 | 5 | 250 | 4 | resource movement |
| Colony Ark | 200/150/100 | 150 | 10 | 100 | 8 | colonize/escape; requires Colonization 1 |
| Frigate | 100/50/30 | 100 | 40 | 10 | 5 | light combat |
| Cruiser | 240/120/80 | 260 | 110 | 20 | 10 | heavy combat |
| Recycler | 80/60/40 | 80 | 5 | 200 | 5 | collects debris |

Shipyard points required are `ceil(total resource cost / 100)`. Each planet has one shipyard queue. Ships of one class may be batched in a queue entry.

Weapons modifies Attack; Shields modifies Hull for combat calculation.

## Fleets and movement

Ships at the same planet may be grouped/split into fleets while idle.

A fleet may receive at most one mission per turn.

Movement follows hyperspace routes. Base travel time is **one turn per route edge**. Propulsion 3/6/9... increases movement by one additional edge per turn.

A fleet in transit is committed to its route until the next node it reaches; at a node it may be given a new route on a later turn.

Fuel is paid at departure for the planned path:
`sum(ship fuel/edge) × route edges`, modified by Propulsion.

Insufficient deuterium makes the move invalid.

## Orders and concurrency

There is **no global action-point or order-count limit**.

Concurrency is constrained by the world:

- one construction queue per planet;
- one shipyard queue per planet;
- one research queue per empire;
- one mission per fleet per turn;
- any reasonable number of diplomatic messages.

This is both more physical and easier to explain than an arbitrary order cap.

## Colonization

A Colony Ark at an unclaimed planet may establish a colony if the empire is below its sustainable colony limit.

The Ark is consumed. The new colony begins with level-1 Infrastructure and no other buildings.

An empire's sustainable colony count is `1 + Colonization level`, including the homeworld. Existing colonies are not destroyed if research access is somehow lost.

## Combat

Combat occurs when an attack mission reaches a hostile planet/fleet.

Combat is resolved in up to **6 rounds**.

For each side each round:

1. Compute total effective attack from surviving ships.
2. Add planetary Defense Grid power for the defender when fighting at a defended planet.
3. Apply seeded target allocation across enemy ships.
4. Damage is simultaneous.
5. A ship is destroyed when accumulated damage reaches its effective Hull.

Seeded target allocation uses the run RNG/event seed, making the same state and orders reproducible.

If both sides retain ships after round 6, the attacker retreats to the previous route node. If the attacker has no valid retreat, it remains blocked outside sovereignty and must depart next turn.

### Planet capture

If the defending combat fleet is destroyed/absent and the attacker has at least one surviving combat ship (Frigate or Cruiser), the planet is captured.

On capture:

- buildings survive;
- 50% of stored resources survive and transfer;
- queued construction/ships/research contributions on that planet are canceled;
- defending ships under construction are lost;
- the planet changes sovereign owner immediately.

Defense Grid levels survive but do not fire again in the capture turn.

## Debris and recycling

Destroyed ships create debris equal to **30% of their original Metal + Crystal cost**. Deuterium is not salvageable.

Debris remains at the battle location until collected. Recycler cargo capacity limits collection.

If multiple hostile recyclers attempt the same debris in the same turn, collection is proportional to available recycler capacity, with deterministic remainder allocation by seeded ordering.

## Espionage

A Scout may spy on a planet at its current system or an adjacent system.

Define `intel = attacker Sensors level - defender Sensors level`.

Reports are tiered:

- intel <= -2: ownership and coarse activity only;
- -1..0: resource bands, building bands, fleet size band;
- 1..2: exact resources/building levels, ship counts rounded to nearest 5;
- >=3: exact visible ships, resources, buildings, current research level summary and known outgoing fleet direction.

Noise is deterministic and bounded by tier. A report includes turn observed and confidence/tier.

Espionage never exposes private Jikko content, diplomatic messages, hidden model context or future orders.

A Scout has a detection chance based on the same intel difference. Detection reveals the spying empire after resolution; failure to detect does not prove no espionage occurred.

## Resource transfer and trade

A Transport (or cargo-capable fleet) may carry resources between planets.

Transfers are physical: resources leave the source at departure and arrive with the fleet.

Trade is not a magical market. Empires negotiate terms diplomatically and send cargo. The engine records transfers but does not enforce promises or fair exchange.

## Diplomacy mechanics

The engine recognizes only:

- war/hostility state;
- alliance membership;
- explicit resource transfer;
- messages.

Alliances do not prevent betrayal. An alliance member may still attack another member; doing so automatically leaves the alliance and creates a treaty-breach event.

Non-aggression pacts, borders, debts and promises are social agreements recorded in messages/Jikko, not engine-enforced contracts.

## Elimination and government in exile

An empire with planets is sovereign.

If it loses its final planet, it becomes a government in exile if it owns at least one **Colony Ark that is not trapped in an unwinnable combat resolution and has enough carried/accessible deuterium to reach at least one currently reachable planet**.

A government in exile may hide, negotiate, receive fuel/resources, colonize or participate in capturing a planet.

It is eliminated when it has no planets and no viable Colony Ark.

Other military fleets alone do not preserve sovereignty indefinitely.

## End conditions

The canonical showcase/benchmark run ends at the first of:

- **600 turns (50 years)**;
- only one non-eliminated empire remains;
- all remaining empires are unable to establish or contest sovereignty (engine-detectable terminal state);
- observer explicitly stops a non-benchmark run.

A benchmark never stops merely because one empire has a dominant score.

Metrics are recorded independently of narrative victory.

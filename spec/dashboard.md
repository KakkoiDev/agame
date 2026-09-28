# Dashboard and history viewer

## Role

The dashboard is primarily an observer interface.

Humans initially watch rather than control empires.

## Live universe view

The dashboard should make it possible to understand:

- current turn/date;
- systems and planets;
- sovereignty;
- empire economy;
- fleets and movement;
- wars and attacks;
- alliances;
- recent messages/events;
- current benchmark metrics.

Hidden information requires observer semantics. A full-observer view may see truth, but ruler-specific views must respect what that ruler knew.

## Timeline

Long runs may span centuries.

The primary history interface should support:

1. zoomed-out timeline of major events;
2. filtering by empire, planet, war, alliance or event class;
3. opening a specific turn/event;
4. inspecting the decisions that preceded it;
5. inspecting what a ruler knew/read at the time.

Examples of major events:

- empire founded;
- first colony;
- alliance formed;
- war declared;
- capital captured;
- ruler enters exile;
- empire restored;
- major betrayal;
- empire eliminated.

## Decision inspection

For a ruler turn, show separately:

- objective pre-turn observation supplied by engine;
- accessible Jikko tree;
- files the model actually chose to retrieve;
- delivered messages;
- final orders;
- concise model statement;
- engine validation;
- resulting consequences.

Never display fabricated chain-of-thought.

## Historical knowledge view

The observer should be able to select an empire and historical turn and see its knowledge state/revisions then, not today's corrected world truth.

This is central to explaining decisions.

## Generated media

Where available, timeline entries may display generated portraits, flags, planet art, propaganda or music associated with the era/event.

Media is decoration/history presentation and must be clearly separable from authoritative simulation evidence.

## Technology direction

Initial intended UI: server-rendered HTML + HTMX.

Avoid a heavy client framework unless concrete dashboard requirements later justify one.

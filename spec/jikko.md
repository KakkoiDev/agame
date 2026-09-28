# Jikko integration

## Boundary

> **AGame owns reality. Jikko owns what agents know, believe, remember, plan and communicate.**

Do not duplicate authoritative world state into Jikko as a second source of truth.

## Mapping

| AGame concept | Jikko representation |
| --- | --- |
| Emperor/ruler | Identity |
| Alliance/group | Identity with members |
| Personality/backstory | Document |
| Memory/reflection | Document |
| Intelligence report | Document |
| Current plan/objective | Task |
| Long-term objective | Task or Document |
| Relationship interpretation | Document + links |
| Shared alliance intelligence | Documents + permissions |
| Private knowledge | Documents + permissions |
| Dynamic “what matters now?” collections | View |
| Historical authored changes | Git/Jikko audit |

Do **not** introduce Jikko primitives named Memory, Agent, Faction, Relationship, Message, Conversation or Event for AGame.

## Discovery rather than automatic context

AGame uses Jikko's permission-aware agent operations.

The normal flow is:

1. authenticate ruler Identity;
2. retrieve accessible file tree;
3. model decides what it needs;
4. model reads one or several files;
5. model acts;
6. model may create/update knowledge, memories and tasks;
7. logical changes may be committed/audited.

AGame must not implement a hidden relevance scorer that chooses “essential memories.”

## Permission isolation

The file tree itself is permission filtered.

If Cassian cannot read Malrec's invasion plan, Cassian must not learn that the file exists through tree/list/search metadata.

Alliance membership may transitively grant access through Jikko Identity membership.

## Imperfect knowledge

A Jikko intelligence document may be stale or wrong.

Example:

```text
Authoritative engine:
Malrec fleet = 53 cruisers

Cassian Jikko:
Observed Malrec fleet: 27 cruisers
Observation turn: 412
Confidence: medium
```

AGame does not silently synchronize Cassian's document with truth.

## Agent-created organization

Agents should be allowed to organize their accessible Jikko knowledge themselves within safe workspace/permission constraints.

One ruler may maintain careful folders and tasks. Another may accumulate disorganized notes.

That difference is potentially benchmark-relevant.

## Messages

Diplomatic messages are authored information, not a new Jikko type.

They may be ordinary Documents addressed via mentions and permissions.

Delivery timing remains an AGame rule; storage and later retrieval can use Jikko.

## Audit

For a reconstructable decision, a run should be able to associate:

- game/run ID;
- turn;
- ruler;
- model/configuration;
- world snapshot hash;
- observation;
- accessible tree revision;
- retrieved Jikko files/revisions;
- submitted orders;
- validation/resolution;
- resulting events;
- Jikko writes;
- audit/Git revision where used.

The goal is to answer: **what could this ruler know when it made this decision?**

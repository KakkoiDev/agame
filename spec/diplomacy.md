# Diplomacy

## Purpose

Diplomacy exists to test long-horizon social reasoning between persistent agents.

Agents may cooperate, threaten, deceive, negotiate, betray and reconcile.

## Communication

Rulers may send messages to other rulers or groups.

Messages:

- have sender and intended recipients;
- are delivered according to deterministic turn timing;
- become recipient knowledge only when delivered;
- can be stored as Jikko Documents;
- may mention recipient identities;
- are auditable.

A message is not an engine command that forces the recipient to comply.

## Agreements

Agents may discuss:

- alliances;
- non-aggression pacts;
- trade;
- territorial agreements;
- mutual defense;
- ceasefires;
- intelligence sharing;
- surrender/protection;
- assistance to governments in exile.

The engine may represent a small subset of agreements mechanically when necessary for rules such as shared access or legal transfers.

Promises and trust, however, should not automatically become engine-enforced merely because the agents wrote them.

This preserves the possibility of betrayal.

## Alliances

An alliance may map to a Jikko group Identity.

Membership can grant access to shared documents/intelligence.

Joining an alliance must not automatically reveal private historical knowledge unless permissions explicitly grant it.

## Deception

Agents may lie in diplomatic prose.

The engine must not rewrite a false statement into truth.

Mechanically forged engine reports are not permitted unless a future espionage rule explicitly defines them.

## Relationships

There is no required engine-level “friendship score.”

A ruler may record its interpretation of another empire in Jikko and change that interpretation over time.

This makes social memory inspectable rather than hidden in a scalar.

## Same-turn messaging

Messages composed during turn t are not visible to recipients during their turn-t decision phase unless a future multi-phase turn explicitly defines negotiation rounds.

Default v1 behavior: messages are delivered after the simultaneous decision barrier and can influence turn t+1.

This preserves simultaneous action semantics.

package world

import (
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"sort"
)

// Combat constants (spec/game.md, Combat; Debris and recycling).
const (
	CombatRounds     = 6
	DebrisPercent    = 30
	DefenseGridPower = 20 // per Defense Grid level
)

// combatant is one ship in a battle. Ships are tracked individually so damage
// accumulates across rounds until it reaches the ship's effective hull.
type combatant struct {
	kind    string
	attack  int
	hull    int
	damage  int
	pending int   // damage received this round, applied simultaneously
	dock    Ships // the planet dock or fleet this ship belongs to
}

func (c *combatant) alive() bool { return c.damage < c.hull }

func living(units []*combatant) []*combatant {
	var out []*combatant
	for _, u := range units {
		if u.alive() {
			out = append(out, u)
		}
	}
	return out
}

func sortedKinds(s Ships) []string {
	kinds := make([]string, 0, len(s))
	for k := range s {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	return kinds
}

// enlist appends one combatant per ship, in kind order, with Weapons applied to
// attack and Shields applied to hull (spec/game.md, Ships).
func enlist(units []*combatant, s Ships, t Tech) []*combatant {
	for _, k := range sortedKinds(s) {
		spec, ok := ShipSpecs[k]
		if !ok {
			continue
		}
		atk := spec.Attack * (100 + 10*t.Weapons) / 100
		hull := spec.Hull * (100 + 10*t.Shields) / 100
		for i := 0; i < s[k]; i++ {
			units = append(units, &combatant{kind: k, attack: atk, hull: hull, dock: s})
		}
	}
	return units
}

// battleRNG derives the battle's random stream from the world seed, the turn,
// the location and the attacking fleet, so the same state and orders always
// resolve identically and no global randomness is involved.
func battleRNG(w *World, location, attacker string) *rand.Rand {
	return seededRNG(w, "battle", location, attacker)
}

// seededRNG is a deterministic stream keyed by the world seed, the turn and
// the given labels. All engine randomness goes through it.
func seededRNG(w *World, labels ...string) *rand.Rand {
	h := fnv.New64a()
	fmt.Fprintf(h, "%s|%d|%d", labels[0], w.Seed, w.Turn)
	for _, l := range labels[1:] {
		fmt.Fprintf(h, "|%s", l)
	}
	s := h.Sum64()
	return rand.New(rand.NewPCG(s, s^0x9e3779b97f4a7c15))
}

// seededOrder is a deterministic permutation of 0..n-1.
func seededOrder(w *World, label string, n int) []int {
	return seededRNG(w, label).Perm(n)
}

func hasCombatShip(s Ships) bool { return s[ShipFrigate]+s[ShipCruiser] > 0 }

// fight runs up to CombatRounds simultaneous-damage rounds. Each round every
// living ship (and grid battery of power shot) picks one living enemy ship
// with rng and deals its full attack; damage is applied at the end of the
// round. It stops early when the attacker is gone or the defender has
// neither ships nor batteries. It returns the number of rounds fought.
func fight(rng *rand.Rand, attackers, defenders []*combatant, batteries, shot int) int {
	rounds := 0
	for rounds < CombatRounds {
		att, dfn := living(attackers), living(defenders)
		if len(att) == 0 || (len(dfn) == 0 && batteries == 0) {
			break
		}
		rounds++
		if len(dfn) > 0 {
			for _, u := range att {
				dfn[rng.IntN(len(dfn))].pending += u.attack
			}
		}
		for _, u := range dfn {
			att[rng.IntN(len(att))].pending += u.attack
		}
		for i := 0; i < batteries; i++ {
			att[rng.IntN(len(att))].pending += shot
		}
		for _, u := range att {
			u.damage, u.pending = u.damage+u.pending, 0
		}
		for _, u := range dfn {
			u.damage, u.pending = u.damage+u.pending, 0
		}
	}
	return rounds
}

// defendingFleets are the defender's fleets idle in, or finishing their
// route at, the system (D47).
func defendingFleets(w *World, owner, system string) []*Fleet {
	var out []*Fleet
	for _, id := range fleetIDs(w) {
		f := w.Fleets[id]
		if f.OwnerID == owner && f.SystemID == system && f.Ships.Count() > 0 && (len(f.Route) == 0 || f.RouteIndex == len(f.Route)-1) {
			out = append(out, f)
		}
	}
	return out
}

// battleBookkeeping records hostility, stats and the battle event shared by
// planet and fleet battles. A battle against an ally is a treaty breach (D23).
func battleBookkeeping(w *World, a *Fleet, def, target string, rounds, attLost, defLost int) {
	breachIfAllied(w, a.OwnerID, def, "battle at "+target)
	recordHostility(w, a.OwnerID, def)
	ea, ed := w.Empires[a.OwnerID], w.Empires[def]
	ea.Stats.Battles++
	ed.Stats.Battles++
	ea.Stats.ShipsLost += attLost
	ea.Stats.ShipsDestroyed += defLost
	ed.Stats.ShipsLost += defLost
	ed.Stats.ShipsDestroyed += attLost
	w.emit(Event{Type: "battle", EmpireID: a.OwnerID, Target: target, Other: def,
		Detail: fmt.Sprintf("defender=%s rounds=%d attacker_lost=%d defender_lost=%d", def, rounds, attLost, defLost)})
}

// retreat sends a surviving, non-capturing attacker back to the route node it
// arrived from (D47), or leaves it in place if it attacked in place.
func retreat(w *World, a *Fleet, prevNode, target string) {
	detail := "no retreat"
	if prevNode != "" && w.Systems[prevNode] != nil {
		a.SystemID = prevNode
		detail = "retreated to " + prevNode
	}
	w.emit(Event{Type: "attack_repulsed", EmpireID: a.OwnerID, Target: target, Detail: detail})
}

// combat resolves an attack mission that reached its target planet.
//
// Sides: the arriving fleet attacks alone (simultaneous arrivals fight one
// after another in fleet-ID order). The defender is every ship docked at the
// target planet plus every defender fleet that is idle in, or has finished
// its route at, the planet's system. The Defense Grid fires one shot of
// DefenseGridPower (scaled by Shields, "planet effective defense") per level,
// unless the planet was captured earlier this turn.
//
// Outcome: if no defending ship survives and the attacker kept a frigate or
// cruiser, the planet is captured. Otherwise a surviving attacker retreats to
// prevNode (the route node it arrived from), or stays put if it attacked in
// place. Destroyed ships leave DebrisPercent of their metal and crystal cost
// as debris in the system.
func combat(w *World, a *Fleet, prevNode string, capturedThisTurn map[string]bool) {
	p := w.Planets[a.Target]
	if p == nil || p.OwnerID == "" || p.OwnerID == a.OwnerID || p.SystemID != a.SystemID {
		w.emit(Event{Type: "attack_target_lost", EmpireID: a.OwnerID, Target: a.Target, Detail: a.ID + " found no hostile planet"})
		return
	}
	def := p.OwnerID
	defTech := w.Empires[def].Tech

	attackers := enlist(nil, a.Ships, w.Empires[a.OwnerID].Tech)
	defenders := enlist(nil, p.Ships, defTech)
	defFleets := defendingFleets(w, def, p.SystemID)
	for _, f := range defFleets {
		defenders = enlist(defenders, f.Ships, defTech)
	}
	batteries, shot := 0, 0
	if !capturedThisTurn[p.ID] {
		batteries = p.Buildings.DefenseGrid
		shot = DefenseGridPower * (100 + 10*defTech.Shields) / 100
	}

	rounds := fight(battleRNG(w, p.ID, a.ID), attackers, defenders, batteries, shot)
	attLost := removeDestroyed(w, p.SystemID, attackers)
	defLost := removeDestroyed(w, p.SystemID, defenders)
	battleBookkeeping(w, a, def, p.ID, rounds, attLost, defLost)

	defenderLeft := p.Ships.Count()
	for _, f := range defFleets {
		defenderLeft += f.Ships.Count()
	}
	switch {
	case defenderLeft == 0 && hasCombatShip(a.Ships):
		p.OwnerID = a.OwnerID
		p.Resources = Resources{p.Resources.Metal / 2, p.Resources.Crystal / 2, p.Resources.Deuterium / 2}
		p.Construction = nil
		p.ShipyardQueue = nil
		capturedThisTurn[p.ID] = true
		w.Empires[a.OwnerID].Stats.Captures++
		w.Empires[def].Stats.PlanetsLost++
		w.emit(Event{Type: "captured", EmpireID: a.OwnerID, Target: p.ID, Other: def, Detail: def})
	case a.Ships.Count() == 0:
		w.emit(Event{Type: "attack_repulsed", EmpireID: a.OwnerID, Target: p.ID, Detail: "destroyed"})
	default:
		retreat(w, a, prevNode, p.ID)
	}
}

// fleetCombat resolves an attack on a fleet (spec/game.md: "an attack mission
// reaches a hostile planet/fleet"; D54). The target must be in the attacker's
// system when the attack arrives, whether idle or passing through; the
// defender's other fleets idle there join it. Planets, docked ships and the
// Defense Grid take no part and nothing is captured. A surviving attacker
// facing surviving defenders retreats like a repulsed planet attack.
func fleetCombat(w *World, a, t *Fleet, prevNode string) {
	if t.OwnerID == a.OwnerID || t.SystemID != a.SystemID || t.Ships.Count() == 0 {
		w.emit(Event{Type: "attack_target_lost", EmpireID: a.OwnerID, Target: t.ID, Detail: a.ID + " did not find the fleet"})
		return
	}
	def := t.OwnerID
	defTech := w.Empires[def].Tech
	attackers := enlist(nil, a.Ships, w.Empires[a.OwnerID].Tech)
	defFleets := []*Fleet{t}
	for _, f := range defendingFleets(w, def, t.SystemID) {
		if f != t {
			defFleets = append(defFleets, f)
		}
	}
	var defenders []*combatant
	for _, f := range defFleets {
		defenders = enlist(defenders, f.Ships, defTech)
	}
	rounds := fight(battleRNG(w, t.ID, a.ID), attackers, defenders, 0, 0)
	attLost := removeDestroyed(w, a.SystemID, attackers)
	defLost := removeDestroyed(w, a.SystemID, defenders)
	battleBookkeeping(w, a, def, t.ID, rounds, attLost, defLost)
	left := 0
	for _, f := range defFleets {
		left += f.Ships.Count()
	}
	if left > 0 && a.Ships.Count() > 0 {
		retreat(w, a, prevNode, t.ID)
	}
}

// removeDestroyed takes destroyed ships out of their docks and leaves their
// debris in the system. It returns the number of ships destroyed.
func removeDestroyed(w *World, system string, units []*combatant) int {
	var cost Resources
	lost := 0
	for _, u := range units {
		if u.alive() {
			continue
		}
		lost++
		u.dock[u.kind]--
		if u.dock[u.kind] <= 0 {
			delete(u.dock, u.kind)
		}
		c := ShipSpecs[u.kind].Cost
		cost.Metal += c.Metal
		cost.Crystal += c.Crystal
	}
	if s := w.Systems[system]; s != nil {
		s.Debris = s.Debris.Add(debrisFor(cost))
	}
	return lost
}

// debrisFor is DebrisPercent of a destroyed cost's metal and crystal, rounded
// down once per side per battle. Deuterium is never salvageable.
func debrisFor(cost Resources) Resources {
	return Resources{Metal: cost.Metal * DebrisPercent / 100, Crystal: cost.Crystal * DebrisPercent / 100}
}

// pruneEmptyFleets removes fleets that no longer have any ship.
func pruneEmptyFleets(w *World) {
	for _, id := range fleetIDs(w) {
		if w.Fleets[id].Ships.Count() == 0 {
			delete(w.Fleets, id)
		}
	}
}

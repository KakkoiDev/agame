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
	h := fnv.New64a()
	fmt.Fprintf(h, "battle|%d|%d|%s|%s", w.Seed, w.Turn, location, attacker)
	s := h.Sum64()
	return rand.New(rand.NewPCG(s, s^0x9e3779b97f4a7c15))
}

func hasCombatShip(s Ships) bool { return s[ShipFrigate]+s[ShipCruiser] > 0 }

// combat resolves an attack mission that reached its target planet.
//
// Sides: the arriving fleet attacks alone (simultaneous arrivals fight one
// after another in fleet-ID order). The defender is every ship docked at the
// target planet plus every defender fleet that is idle in, or has finished
// its route at, the planet's system. The Defense Grid fires one shot of
// DefenseGridPower (scaled by Shields, "planet effective defense") per level,
// unless the planet was captured earlier this turn.
//
// Each round every living ship (and grid battery) picks one living enemy
// ship with the battle RNG and deals its full attack; damage is applied
// simultaneously at the end of the round. The battle stops after
// CombatRounds rounds, when the attacker is gone, or when the defender has
// neither ships nor a grid left.
//
// Outcome: if no defending ship survives and the attacker kept a frigate or
// cruiser, the planet is captured. Otherwise a surviving attacker retreats to
// prevNode (the route node it arrived from), or stays put if it attacked in
// place. Destroyed ships leave DebrisPercent of their metal and crystal cost
// as debris in the system.
func combat(w *World, a *Fleet, prevNode string, capturedThisTurn map[string]bool) {
	p := w.Planets[a.Target]
	if p == nil || p.OwnerID == "" || p.OwnerID == a.OwnerID || p.SystemID != a.SystemID {
		return
	}
	def := p.OwnerID
	defTech := w.Empires[def].Tech

	attackers := enlist(nil, a.Ships, w.Empires[a.OwnerID].Tech)
	defenders := enlist(nil, p.Ships, defTech)
	var defFleets []*Fleet
	for _, id := range fleetIDs(w) {
		f := w.Fleets[id]
		if f.OwnerID == def && f.SystemID == p.SystemID && (len(f.Route) == 0 || f.RouteIndex == len(f.Route)-1) {
			defFleets = append(defFleets, f)
			defenders = enlist(defenders, f.Ships, defTech)
		}
	}
	batteries, shot := 0, 0
	if !capturedThisTurn[p.ID] {
		batteries = p.Buildings.DefenseGrid
		shot = DefenseGridPower * (100 + 10*defTech.Shields) / 100
	}

	rng := battleRNG(w, p.ID, a.ID)
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

	attLost := removeDestroyed(w, p.SystemID, attackers)
	defLost := removeDestroyed(w, p.SystemID, defenders)
	w.Events = append(w.Events, Event{Turn: w.Turn, Type: "battle", EmpireID: a.OwnerID, Target: p.ID,
		Detail: fmt.Sprintf("defender=%s rounds=%d attacker_lost=%d defender_lost=%d", def, rounds, attLost, defLost)})

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
		w.Events = append(w.Events, Event{Turn: w.Turn, Type: "captured", EmpireID: a.OwnerID, Target: p.ID, Detail: def})
	case a.Ships.Count() == 0:
		w.Events = append(w.Events, Event{Turn: w.Turn, Type: "attack_repulsed", EmpireID: a.OwnerID, Target: p.ID, Detail: "destroyed"})
	default:
		detail := "no retreat"
		if prevNode != "" && w.Systems[prevNode] != nil {
			a.SystemID = prevNode
			detail = "retreated to " + prevNode
		}
		w.Events = append(w.Events, Event{Turn: w.Turn, Type: "attack_repulsed", EmpireID: a.OwnerID, Target: p.ID, Detail: detail})
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

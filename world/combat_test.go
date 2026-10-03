package world

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// stageAttack forms an e00 fleet at its homeworld and returns it with an
// e01-owned, undefended planet in a neighbouring system as the target.
func stageAttack(t *testing.T, w *World, ships map[string]int) (*Fleet, *Planet) {
	t.Helper()
	p := home(w, "e00")
	for k, n := range ships {
		p.Ships[k] = n
	}
	p.Resources.Deuterium = 10000
	f := formFleet(t, w, ships)
	sys := w.Systems[f.SystemID].Neighbors[0]
	target := w.Planets[w.Systems[sys].Planets[1]]
	target.OwnerID = "e01"
	target.Ships = Ships{}
	return f, target
}

func eventsOfType(r TurnResult, typ string) []Event {
	var out []Event
	for _, e := range r.Events {
		if e.Type == typ {
			out = append(out, e)
		}
	}
	return out
}

func TestCombatCapturesWithGridAttrition(t *testing.T) {
	w := newTestWorld(t, 1)
	p := home(w, "e00")
	p.Ships[ShipFrigate] = 10
	f := formFleet(t, w, map[string]int{ShipFrigate: 10})
	target := w.Planets[w.Systems[f.SystemID].Planets[1]]
	target.OwnerID = "e01"
	target.Resources = Resources{101, 50, 10}
	target.Construction = &Queue{Kind: "metal_mine", Required: 5}
	target.ShipyardQueue = &Queue{Kind: ShipFrigate, Quantity: 3, Required: 5}
	target.Buildings.DefenseGrid = 1
	r := resolve(t, w, Order{EmpireID: "e00", Type: OrderAttack, Actor: f.ID, Target: target.ID})
	if target.OwnerID != "e00" || target.Construction != nil || target.ShipyardQueue != nil {
		t.Fatalf("capture failed: owner=%s queues=%+v %+v", target.OwnerID, target.Construction, target.ShipyardQueue)
	}
	if target.Resources != (Resources{50, 25, 5}) || target.Buildings.DefenseGrid != 1 {
		t.Fatalf("capture effects wrong: %+v %+v", target.Resources, target.Buildings)
	}
	// The grid (20/round, 6 rounds = 120 damage) can destroy at most one frigate.
	if n := f.Ships[ShipFrigate]; n < 9 || n > 10 {
		t.Fatalf("frigates after grid fire = %d", n)
	}
	b := eventsOfType(r, "battle")
	c := eventsOfType(r, "captured")
	if len(b) != 1 || !strings.Contains(b[0].Detail, "rounds=6") || len(c) != 1 || c[0].Detail != "e01" {
		t.Fatalf("events=%+v", r.Events)
	}
}

func TestCombatPlanetDockedShipsDefend(t *testing.T) {
	w := newTestWorld(t, 1)
	att := formFleet(t, w, map[string]int{ShipFrigate: 1})
	target := home(w, "e01")
	target.Buildings.DefenseGrid = 0
	target.Ships[ShipCruiser] = 10
	att.SystemID = target.SystemID
	r := resolve(t, w, Order{EmpireID: "e00", Type: OrderAttack, Actor: att.ID, Target: target.ID})
	if target.OwnerID != "e01" {
		t.Fatal("homeworld with 10 docked cruisers captured by one frigate")
	}
	if _, ok := w.Fleets[att.ID]; ok {
		t.Fatal("destroyed attacker fleet still exists")
	}
	if rp := eventsOfType(r, "attack_repulsed"); len(rp) != 1 || rp[0].Detail != "destroyed" {
		t.Fatalf("events=%+v", r.Events)
	}
}

func TestCombatDestroyedShipsLeaveDebrisAndEmptyFleetsVanish(t *testing.T) {
	w := newTestWorld(t, 1)
	f := formFleet(t, w, map[string]int{ShipFrigate: 1})
	target := w.Planets[w.Systems[f.SystemID].Planets[1]]
	target.OwnerID = "e01"
	target.Buildings.DefenseGrid = 50
	resolve(t, w, Order{EmpireID: "e00", Type: OrderAttack, Actor: f.ID, Target: target.ID})
	if got, want := w.Systems[f.SystemID].Debris, (Resources{30, 15, 0}); got != want {
		t.Fatalf("debris=%+v want %+v (30%% of 100/50, no deuterium)", got, want)
	}
	if _, ok := w.Fleets[f.ID]; ok {
		t.Fatal("fleet with zero ships still exists")
	}
}

func TestCombatDebrisCanBeRecycled(t *testing.T) {
	w := newTestWorld(t, 1)
	p := home(w, "e00")
	p.Ships[ShipRecycler] = 1
	f := formFleet(t, w, map[string]int{ShipFrigate: 2})
	rec := formFleet(t, w, map[string]int{ShipRecycler: 1})
	target := w.Planets[w.Systems[f.SystemID].Planets[1]]
	target.OwnerID = "e01"
	target.Buildings.DefenseGrid = 50
	resolve(t, w, Order{EmpireID: "e00", Type: OrderAttack, Actor: f.ID, Target: target.ID})
	resolve(t, w, Order{EmpireID: "e00", Type: OrderRecycle, Actor: rec.ID, Target: rec.SystemID})
	if rec.Cargo != (Resources{60, 30, 0}) || w.Systems[rec.SystemID].Debris != (Resources{}) {
		t.Fatalf("cargo=%+v debris=%+v", rec.Cargo, w.Systems[rec.SystemID].Debris)
	}
}

func TestCombatStalemateRetreatsToPreviousNode(t *testing.T) {
	w := newTestWorld(t, 1)
	f, target := stageAttack(t, w, map[string]int{ShipTransport: 1})
	from := f.SystemID
	target.Ships[ShipTransport] = 1 // 5 attack vs 60 hull on both sides: nobody dies in 6 rounds
	r := resolve(t, w, Order{EmpireID: "e00", Type: OrderAttack, Actor: f.ID, Target: target.ID})
	if target.OwnerID != "e01" || f.SystemID != from || f.Mission != "" || len(f.Route) != 0 {
		t.Fatalf("owner=%s fleet at %s (want %s) mission=%q route=%v", target.OwnerID, f.SystemID, from, f.Mission, f.Route)
	}
	rp := eventsOfType(r, "attack_repulsed")
	if len(rp) != 1 || rp[0].Detail != "retreated to "+from {
		t.Fatalf("events=%+v", r.Events)
	}
	if w.Systems[target.SystemID].Debris != (Resources{}) {
		t.Fatal("debris without losses")
	}
}

func TestCombatInPlaceStalemateHasNoRetreat(t *testing.T) {
	w := newTestWorld(t, 1)
	f, target := stageAttack(t, w, map[string]int{ShipTransport: 1})
	f.SystemID = target.SystemID
	target.Ships[ShipTransport] = 1
	r := resolve(t, w, Order{EmpireID: "e00", Type: OrderAttack, Actor: f.ID, Target: target.ID})
	if rp := eventsOfType(r, "attack_repulsed"); len(rp) != 1 || !strings.HasPrefix(rp[0].Detail, "no retreat") || f.SystemID != target.SystemID || !f.Blocked {
		t.Fatalf("events=%+v at %s", r.Events, f.SystemID)
	}
	// spec/game.md: a blocked attacker must depart next turn; it may not
	// attack again from where it stands.
	f.Cargo.Deuterium = 100
	r = resolve(t, w, Order{EmpireID: "e00", Type: OrderAttack, Actor: f.ID, Target: target.ID})
	if len(r.Rejected) != 1 || !strings.Contains(r.Rejected[0].Reason, "blocked") {
		t.Fatalf("blocked fleet attacked again: %+v", r)
	}
	r = resolve(t, w, Order{EmpireID: "e00", Type: OrderMove, Actor: f.ID, Target: w.Systems[f.SystemID].Neighbors[0]})
	if len(r.Accepted) != 1 || f.Blocked {
		t.Fatalf("blocked fleet could not leave: %+v", r.Rejected)
	}
}

func TestCombatShieldsAndWeaponsApply(t *testing.T) {
	// One cruiser (110 attack) against one docked frigate (100 hull) kills it
	// in round 1; with defender Shields 2 (hull 120) it needs a second round.
	for _, tc := range []struct{ shields, rounds int }{{0, 1}, {2, 2}} {
		w := newTestWorld(t, 1)
		f, target := stageAttack(t, w, map[string]int{ShipCruiser: 1})
		target.Ships[ShipFrigate] = 1
		w.Empires["e01"].Tech.Shields = tc.shields
		r := resolve(t, w, Order{EmpireID: "e00", Type: OrderAttack, Actor: f.ID, Target: target.ID})
		b := eventsOfType(r, "battle")
		if len(b) != 1 || !strings.Contains(b[0].Detail, fmt.Sprintf("rounds=%d ", tc.rounds)) || target.OwnerID != "e00" {
			t.Fatalf("shields %d: events=%+v owner=%s", tc.shields, r.Events, target.OwnerID)
		}
	}
}

func TestCombatGridSilentAfterCaptureThisTurn(t *testing.T) {
	w := newTestWorld(t, 1)
	f, target := stageAttack(t, w, map[string]int{ShipFrigate: 1})
	f.SystemID, f.Target = target.SystemID, target.ID
	target.Buildings.DefenseGrid = 100
	combat(w, f, "", map[string]bool{target.ID: true})
	if target.OwnerID != "e00" || f.Ships[ShipFrigate] != 1 {
		t.Fatalf("owner=%s ships=%v: grid fired in its capture turn", target.OwnerID, f.Ships)
	}
}

// bigBattle sets up a battle whose outcome depends on target allocation.
func bigBattle(t *testing.T, seed int64) (*World, Order) {
	w := newTestWorld(t, seed)
	f, target := stageAttack(t, w, map[string]int{ShipFrigate: 12, ShipCruiser: 3, ShipScout: 1})
	target.Ships = Ships{ShipFrigate: 9, ShipCruiser: 2, ShipTransport: 2}
	target.Buildings.DefenseGrid = 3
	w.Fleets["fdef"] = &Fleet{ID: "fdef", OwnerID: "e01", SystemID: target.SystemID, Ships: Ships{ShipFrigate: 4}}
	return w, Order{EmpireID: "e00", Type: OrderAttack, Actor: f.ID, Target: target.ID}
}

func TestCombatIsDeterministic(t *testing.T) {
	var first string
	for i := 0; i < 20; i++ {
		w, o := bigBattle(t, 7)
		r := resolve(t, w, o)
		b, _ := json.Marshal(struct {
			W *World
			R TurnResult
		}{w, r})
		if i == 0 {
			first = string(b)
		} else if string(b) != first {
			t.Fatalf("run %d diverged", i)
		}
	}
}

func TestCombatSeedDependsOnWorldTurnAndLocation(t *testing.T) {
	w := newTestWorld(t, 1)
	draw := func(w *World, loc, f string) uint64 { return battleRNG(w, loc, f).Uint64() }
	base := draw(w, "s00-p1", "f000001")
	if draw(w, "s00-p1", "f000001") != base {
		t.Fatal("same inputs, different stream")
	}
	if draw(w, "s00-p2", "f000001") == base || draw(w, "s00-p1", "f000002") == base {
		t.Fatal("location/attacker not mixed into the seed")
	}
	w.Turn++
	if draw(w, "s00-p1", "f000001") == base {
		t.Fatal("turn not mixed into the seed")
	}
	w.Turn--
	w.Seed++
	if draw(w, "s00-p1", "f000001") == base {
		t.Fatal("world seed not mixed into the seed")
	}
}

func TestCombatConservesShipsAsDebris(t *testing.T) {
	for _, seed := range []int64{1, 2, 3, 4, 5} {
		w, o := bigBattle(t, seed)
		f := w.Fleets[o.Actor]
		target := w.Planets[o.Target]
		count := func() (Ships, Ships) {
			a := Ships{}
			for k, n := range f.Ships {
				a[k] += n
			}
			d := Ships{}
			for k, n := range target.Ships {
				d[k] += n
			}
			if df := w.Fleets["fdef"]; df != nil {
				for k, n := range df.Ships {
					d[k] += n
				}
			}
			return a, d
		}
		a0, d0 := count()
		sys := w.Systems[target.SystemID]
		debris0 := sys.Debris
		r := resolve(t, w, o)
		a1, d1 := count()
		var lost Resources
		lostShips := 0
		for _, pair := range [][2]Ships{{a0, a1}, {d0, d1}} {
			for k, n := range pair[0] {
				gone := n - pair[1][k]
				if gone < 0 {
					t.Fatalf("seed %d: ships of %s appeared", seed, k)
				}
				lostShips += gone
				c := ShipSpecs[k].Cost
				lost.Metal += c.Metal * gone
				lost.Crystal += c.Crystal * gone
			}
		}
		if lostShips == 0 {
			t.Fatalf("seed %d: battle destroyed nothing", seed)
		}
		// Every ship cost here is a multiple of 10, so 30% is exact.
		want := debris0.Add(Resources{lost.Metal * 3 / 10, lost.Crystal * 3 / 10, 0})
		if sys.Debris != want {
			t.Fatalf("seed %d: debris %+v want %+v", seed, sys.Debris, want)
		}
		b := eventsOfType(r, "battle")
		if len(b) != 1 {
			t.Fatalf("seed %d: events=%+v", seed, r.Events)
		}
		for k, n := range f.Ships {
			if n <= 0 {
				t.Fatalf("seed %d: zero/negative count kept for %s", seed, k)
			}
		}
		if df := w.Fleets["fdef"]; df != nil && df.Ships.Count() == 0 {
			t.Fatalf("seed %d: empty defender fleet kept", seed)
		}
	}
}

func TestColonizeUnloadsCargoAndRemovesEmptyFleet(t *testing.T) {
	w := newTestWorld(t, 1)
	f := colonyArkFleet(t, w)
	f.Cargo = Resources{40, 30, 20}
	target := w.Systems[f.SystemID].Planets[1]
	resolve(t, w, Order{EmpireID: "e00", Type: OrderColonize, Actor: f.ID, Target: target})
	if _, ok := w.Fleets[f.ID]; ok {
		t.Fatal("empty fleet kept after the ark was consumed")
	}
	if got := w.Planets[target].Resources; got != (Resources{40, 30, 20}) {
		t.Fatalf("colony resources %+v; ark cargo lost", got)
	}
}

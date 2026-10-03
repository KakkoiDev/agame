package world

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"testing"
)

func TestHomeSystemsAreNeverAdjacent(t *testing.T) {
	// spec/game.md: no two starts are immediate neighbours when the graph permits it.
	for seed := int64(0); seed < 100; seed++ {
		w := newTestWorld(t, seed)
		var hs []string
		for _, e := range w.Empires {
			hs = append(hs, w.Planets[e.HomeworldID].SystemID)
		}
		for i := range hs {
			for j := i + 1; j < len(hs); j++ {
				if adjacent(w, hs[i], hs[j]) {
					t.Fatalf("seed %d: home systems %s and %s are adjacent", seed, hs[i], hs[j])
				}
			}
		}
	}
}

func TestEqualMaterialStarts(t *testing.T) {
	w := newTestWorld(t, 11)
	var ref *Planet
	for _, e := range w.Empires {
		p := home(w, e.ID)
		if ref == nil {
			ref = p
			continue
		}
		if p.Resources != ref.Resources || p.Buildings != ref.Buildings || fmt.Sprint(p.Ships) != fmt.Sprint(ref.Ships) {
			t.Fatalf("unequal start: %s %+v vs %s %+v", p.ID, p, ref.ID, ref)
		}
	}
}

func TestGenerateRejectsWrongEmpireCount(t *testing.T) {
	if _, err := Generate(1, names[:7]); err == nil {
		t.Fatal("expected error for 7 empires")
	}
}

// graphFingerprint pins the seeded universe layout. If this changes (for
// example because math/rand/v2's derived methods change between Go
// releases), existing seeds no longer reproduce published runs; that must be
// an explicit ruleset/version change (spec/decisions.md, "Status").
func graphFingerprint(w *World) string {
	var b strings.Builder
	for _, id := range systemIDs(w) {
		fmt.Fprintf(&b, "%s:%s;", id, strings.Join(w.Systems[id].Neighbors, ","))
	}
	var homes []string
	for _, e := range w.Empires {
		homes = append(homes, e.ID+"@"+e.HomeworldID)
	}
	sort.Strings(homes)
	b.WriteString(strings.Join(homes, ","))
	h := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(h[:8])
}

func TestGenerateGoldenFingerprint(t *testing.T) {
	golden := map[int64]string{
		1:  "0aba5f7571e758c6",
		42: "d460a02d9dd31dfe",
	}
	for seed, want := range golden {
		if got := graphFingerprint(newTestWorld(t, seed)); got != want {
			t.Errorf("seed %d fingerprint %s, want %s", seed, got, want)
		}
	}
}

func TestShortestPathIsValidRoute(t *testing.T) {
	w := newTestWorld(t, 5)
	ids := systemIDs(w)
	for _, a := range ids[:4] {
		for _, b := range ids {
			r := shortest(w, a, b)
			if len(r) == 0 || r[0] != a || r[len(r)-1] != b {
				t.Fatalf("route %s->%s = %v", a, b, r)
			}
			if len(r)-1 != distance(w, a, b) {
				t.Fatalf("route %s->%s has %d edges, distance %d", a, b, len(r)-1, distance(w, a, b))
			}
			for i := 1; i < len(r); i++ {
				if !adjacent(w, r[i-1], r[i]) {
					t.Fatalf("route %v uses non-edge %s-%s", r, r[i-1], r[i])
				}
			}
		}
	}
}

func TestCostAndWorkFormulas(t *testing.T) {
	if got := scale(BuildingBase["metal_mine"], 3); got != (Resources{240, 60, 0}) {
		t.Fatalf("level-3 metal mine = %+v", got)
	}
	if got := work(Resources{60, 15, 0}); got != 1 {
		t.Fatalf("work(75)=%d", got)
	}
	if got := work(Resources{100, 1, 0}); got != 2 {
		t.Fatalf("work(101)=%d, want ceil", got)
	}
	if _, err := shipCost("frigate", 0); err == nil {
		t.Fatal("zero batch accepted")
	}
	if c, _ := shipCost("frigate", 3); c != (Resources{300, 150, 90}) {
		t.Fatalf("3 frigates = %+v", c)
	}
}

func TestValidationRejections(t *testing.T) {
	w := newTestWorld(t, 1)
	p := home(w, "e00")
	other := home(w, "e01")
	f := formFleet(t, w, map[string]int{"scout": 1})
	p.Resources = Resources{100000, 100000, 100000}
	w.Empires["e00"].Research = &Queue{Kind: "weapons", Required: 99}
	cases := map[string]Order{
		"unknown building":      {Type: "construct", Actor: p.ID, Target: "casino"},
		"foreign planet":        {Type: "construct", Actor: other.ID, Target: "metal_mine"},
		"missing planet":        {Type: "construct", Actor: "nope", Target: "metal_mine"},
		"research busy":         {Type: "research", Actor: p.ID, Target: "industry"},
		"ark without tech":      {Type: "build_ships", Actor: p.ID, Target: "colony_ark"},
		"unknown ship":          {Type: "build_ships", Actor: p.ID, Target: "deathstar"},
		"foreign fleet planet":  {Type: "form_fleet", Actor: other.ID, Params: map[string]any{"ships": map[string]int{"frigate": 1}}},
		"missing fleet":         {Type: "move", Actor: "f999999", Target: "s00"},
		"unknown recipient":     {Type: "message", Target: "e99"},
		"unsupported type":      {Type: "teleport", Actor: f.ID},
		"unreachable target":    {Type: "move", Actor: f.ID, Target: "nowhere"},
		"more ships than owned": {Type: "form_fleet", Actor: p.ID, Params: map[string]any{"ships": map[string]any{"frigate": float64(3)}}},
	}
	for name, o := range cases {
		o.EmpireID = "e00"
		r := resolve(t, w, o)
		if len(r.Accepted) != 0 || len(r.Rejected) != 1 {
			t.Errorf("%s: accepted=%v", name, r.Accepted)
		}
	}
	if _, err := ResolveTurn(nil, nil); err == nil {
		t.Fatal("nil world accepted")
	}
}

func TestOrderForUnknownEmpireOrEmptyTypeRejected(t *testing.T) {
	w := newTestWorld(t, 1)
	r, _ := ResolveTurn(w, map[string][]Order{
		"e99": {{EmpireID: "e99", Type: "message", Target: "e00"}},
		"e00": {{EmpireID: "e00"}},
	})
	if len(r.Accepted) != 0 || len(r.Rejected) != 2 {
		t.Fatalf("accepted=%v rejected=%v", r.Accepted, r.Rejected)
	}
}

func TestFleetInTransitCannotBeRedirected(t *testing.T) {
	w := newTestWorld(t, 4)
	p := home(w, "e00")
	f := formFleet(t, w, map[string]int{"scout": 1})
	p.Resources.Deuterium = 1000
	var far string
	for _, id := range systemIDs(w) {
		if distance(w, f.SystemID, id) >= 3 {
			far = id
			break
		}
	}
	resolve(t, w, Order{EmpireID: "e00", Type: "move", Actor: f.ID, Target: far})
	if len(f.Route) == 0 {
		t.Fatal("fleet should still be in transit")
	}
	r := resolve(t, w, Order{EmpireID: "e00", Type: "move", Actor: f.ID, Target: p.SystemID})
	if len(r.Accepted) != 0 {
		t.Fatal("in-transit fleet accepted a new mission")
	}
	for i := 0; i < 10 && len(f.Route) > 0; i++ {
		resolve(t, w)
	}
	if f.SystemID != far || f.Mission != "" {
		t.Fatalf("fleet at %s mission %q, want arrival at %s", f.SystemID, f.Mission, far)
	}
}

func TestPropulsionSpeedAndFuelDiscount(t *testing.T) {
	w := newTestWorld(t, 4)
	p := home(w, "e00")
	f := formFleet(t, w, map[string]int{"frigate": 2})
	w.Empires["e00"].Tech.Propulsion = 3 // speed 2 edges/turn, fuel -15%
	var far string
	for _, id := range systemIDs(w) {
		if distance(w, f.SystemID, id) == 2 {
			far = id
			break
		}
	}
	if far == "" {
		t.Skip("no system at distance 2")
	}
	p.Resources.Deuterium = 1000
	resolve(t, w, Order{EmpireID: "e00", Type: "move", Actor: f.ID, Target: far})
	if f.SystemID != far || len(f.Route) != 0 {
		t.Fatalf("Propulsion 3 fleet did not cover 2 edges in one turn: at %s route %v", f.SystemID, f.Route)
	}
	fuel := (2*5*2*85 + 99) / 100 // ships*fuel/edge*edges, -15%, rounded up
	if got := 1000 + 12 - p.Resources.Deuterium; got != fuel {
		t.Fatalf("fuel=%d want %d", got, fuel)
	}
}

func TestInsufficientFuelRejectsMove(t *testing.T) {
	w := newTestWorld(t, 4)
	p := home(w, "e00")
	f := formFleet(t, w, map[string]int{"frigate": 2})
	p.Resources.Deuterium = 9
	r := resolve(t, w, Order{EmpireID: "e00", Type: "move", Actor: f.ID, Target: w.Systems[f.SystemID].Neighbors[0]})
	if len(r.Accepted) != 0 || p.Resources.Deuterium != 9+12 {
		t.Fatalf("move without fuel: accepted=%d deut=%d", len(r.Accepted), p.Resources.Deuterium)
	}
}

func TestQueuesCompleteAndAdvanceOnePerTurn(t *testing.T) {
	w := newTestWorld(t, 1)
	p := home(w, "e00")
	p.Resources = Resources{5000, 5000, 5000}
	e := w.Empires["e00"]
	resolve(t, w,
		Order{EmpireID: "e00", Type: "build_ships", Actor: p.ID, Target: "frigate", Params: map[string]any{"quantity": float64(2)}},
		Order{EmpireID: "e00", Type: "research", Actor: p.ID, Target: "industry"},
	)
	// frigate x2 = 200/100/60 -> 4 shipyard points at 1/turn; industry = 270 -> 3 research points.
	if p.ShipyardQueue == nil || p.ShipyardQueue.Progress != 1 || p.ShipyardQueue.Required != 4 {
		t.Fatalf("shipyard queue %+v", p.ShipyardQueue)
	}
	if e.Research == nil || e.Research.Required != 3 {
		t.Fatalf("research queue %+v", e.Research)
	}
	resolve(t, w)
	resolve(t, w)
	if e.Research != nil || e.Tech.Industry != 1 {
		t.Fatalf("industry not researched after 3 turns: %+v %+v", e.Research, e.Tech)
	}
	resolve(t, w)
	if p.ShipyardQueue != nil || p.Ships["frigate"] != 4 {
		t.Fatalf("frigates not delivered after 4 turns: %+v ships=%v", p.ShipyardQueue, p.Ships)
	}
	// Industry 1 => +10% production.
	before := p.Resources.Metal
	resolve(t, w)
	if got := p.Resources.Metal - before; got != 33 {
		t.Fatalf("metal production with Industry 1 = %d, want 33", got)
	}
}

func TestResearchPointsSumLabsAcrossPlanets(t *testing.T) {
	w := newTestWorld(t, 1)
	p := home(w, "e00")
	col := w.Planets[w.Systems[p.SystemID].Planets[1]]
	col.OwnerID = "e00"
	col.Buildings.ResearchLab = 3
	w.Empires["e00"].Research = &Queue{Kind: "sensors", Required: 100}
	resolve(t, w)
	if got := w.Empires["e00"].Research.Progress; got != 4 {
		t.Fatalf("research progress %d, want 1+3 labs", got)
	}
}

func colonyArkFleet(t *testing.T, w *World) *Fleet {
	t.Helper()
	p := home(w, "e00")
	p.Ships["colony_ark"] = 1
	w.Empires["e00"].Tech.Colonization = 1
	p.Resources.Deuterium = 1000
	return formFleet(t, w, map[string]int{"colony_ark": 1})
}

func TestColonizeClaimsEmptyPlanetAndConsumesArk(t *testing.T) {
	w := newTestWorld(t, 1)
	f := colonyArkFleet(t, w)
	target := w.Systems[f.SystemID].Planets[1]
	r := resolve(t, w, Order{EmpireID: "e00", Type: "colonize", Actor: f.ID, Target: target})
	p := w.Planets[target]
	if p.OwnerID != "e00" || p.Buildings != (Buildings{Infrastructure: 1}) || f.Ships["colony_ark"] != 0 {
		t.Fatalf("colonize failed: owner=%q buildings=%+v ark=%d", p.OwnerID, p.Buildings, f.Ships["colony_ark"])
	}
	if len(r.Events) != 1 || r.Events[0].Type != "colonized" || r.Events[0].Target != target {
		t.Fatalf("events=%+v", r.Events)
	}
}

func TestColonizeRespectsSustainableColonyLimit(t *testing.T) {
	w := newTestWorld(t, 1)
	f := colonyArkFleet(t, w)
	sys := w.Systems[f.SystemID]
	w.Planets[sys.Planets[2]].OwnerID = "e00" // already at 1 + Colonization(1) = 2 planets
	r := resolve(t, w, Order{EmpireID: "e00", Type: "colonize", Actor: f.ID, Target: sys.Planets[1]})
	if w.Planets[sys.Planets[1]].OwnerID != "" || f.Ships["colony_ark"] != 1 || len(r.Events) != 0 {
		t.Fatal("colonized beyond sustainable limit")
	}
}

func TestStationaryDefendingFleetAddsDefense(t *testing.T) {
	w := newTestWorld(t, 1)
	p := home(w, "e00")
	p.Ships["frigate"] = 3
	att := formFleet(t, w, map[string]int{"frigate": 3}) // 120 attack
	target := w.Planets[w.Systems[att.SystemID].Planets[1]]
	target.OwnerID = "e01"
	w.Fleets["fdef"] = &Fleet{ID: "fdef", OwnerID: "e01", SystemID: target.SystemID, Ships: Ships{"cruiser": 2}} // 220 defense
	resolve(t, w, Order{EmpireID: "e00", Type: "attack", Actor: att.ID, Target: target.ID})
	if target.OwnerID != "e01" {
		t.Fatal("defending fleet ignored in combat")
	}
}

func TestSpyTiers(t *testing.T) {
	for _, tc := range []struct{ att, def, tier int }{{0, 2, 0}, {0, 1, 1}, {0, 0, 1}, {1, 0, 2}, {2, 0, 2}, {3, 0, 3}} {
		w := newTestWorld(t, 1)
		f := formFleet(t, w, map[string]int{"scout": 1})
		target := w.Planets[w.Systems[f.SystemID].Planets[1]]
		target.OwnerID = "e01"
		w.Empires["e00"].Tech.Sensors = tc.att
		w.Empires["e01"].Tech.Sensors = tc.def
		r := resolve(t, w, Order{EmpireID: "e00", Type: "spy", Actor: f.ID, Target: target.ID})
		want := fmt.Sprintf("tier=%d owner=e01", tc.tier)
		if len(r.Events) != 1 || r.Events[0].Type != "espionage" || r.Events[0].Detail != want {
			t.Fatalf("sensors %d vs %d: events=%+v want %s", tc.att, tc.def, r.Events, want)
		}
	}
}

func TestRecycleCollectsDebrisUpToCapacity(t *testing.T) {
	w := newTestWorld(t, 1)
	p := home(w, "e00")
	p.Ships["recycler"] = 1
	f := formFleet(t, w, map[string]int{"recycler": 1})
	w.Systems[f.SystemID].Debris = Resources{150, 100, 0}
	resolve(t, w, Order{EmpireID: "e00", Type: "recycle", Actor: f.ID, Target: f.SystemID})
	if f.Cargo != (Resources{150, 50, 0}) || w.Systems[f.SystemID].Debris != (Resources{0, 50, 0}) {
		t.Fatalf("cargo=%+v debris=%+v", f.Cargo, w.Systems[f.SystemID].Debris)
	}
}

func TestTransportDeliversToOwnPlanet(t *testing.T) {
	w := newTestWorld(t, 1)
	p := home(w, "e00")
	f := formFleet(t, w, map[string]int{"transport": 1})
	col := w.Planets[w.Systems[f.SystemID].Planets[1]]
	col.OwnerID = "e00"
	p.Resources = Resources{1000, 1000, 1000}
	resolve(t, w, Order{EmpireID: "e00", Type: "transport", Actor: f.ID, Target: col.ID,
		Params: map[string]any{"metal": float64(200), "deuterium": float64(50)}})
	if col.Resources != (Resources{200, 0, 50}) || f.Cargo != (Resources{}) {
		t.Fatalf("colony=%+v cargo=%+v", col.Resources, f.Cargo)
	}
	if p.Resources != (Resources{1000 - 200 + 30, 1000 + 20, 1000 - 50 + 12}) {
		t.Fatalf("source=%+v", p.Resources)
	}
}

func TestMessagesDeliveredNextTurn(t *testing.T) {
	w := newTestWorld(t, 1)
	resolve(t, w, Order{EmpireID: "e00", Type: "message", Target: "e01", Params: map[string]any{"body": "hi", "major": true}})
	if len(w.Messages) != 1 {
		t.Fatal("message lost")
	}
	m := w.Messages[0]
	if m.Turn != 1 || m.From != "e00" || m.To != "e01" || m.Body != "hi" || !m.Major {
		t.Fatalf("message=%+v", m)
	}
}

func TestSovereigntyExileAndElimination(t *testing.T) {
	w := newTestWorld(t, 1)
	home(w, "e01").OwnerID = ""
	home(w, "e02").OwnerID = ""
	w.Fleets["fark"] = &Fleet{ID: "fark", OwnerID: "e01", SystemID: "s00", Ships: Ships{"colony_ark": 1}}
	w.Fleets["fwar"] = &Fleet{ID: "fwar", OwnerID: "e02", SystemID: "s00", Ships: Ships{"cruiser": 9}}
	resolve(t, w)
	if e := w.Empires["e01"]; !e.Exile || e.Eliminated {
		t.Fatalf("e01 with ark: %+v", e)
	}
	if e := w.Empires["e02"]; e.Exile || !e.Eliminated {
		t.Fatalf("e02 with only warships: %+v", e)
	}
	if e := w.Empires["e00"]; e.Exile || e.Eliminated {
		t.Fatalf("e00 sovereign: %+v", e)
	}
}

// --- spec gaps found during the audit; reproduced but not fixed because the
// correct behaviour needs a design decision. ---

func TestKnownGapStrandedFleetCanLeave(t *testing.T) {
	t.Skip("spec gap: launch() takes fuel only from an owned planet in the fleet's system, so a fleet at a system without an owned planet (e.g. after a repulsed attack) can never move again; spec requires it to retreat/depart")
	w := newTestWorld(t, 1)
	f := formFleet(t, w, map[string]int{"frigate": 2})
	f.SystemID = w.Planets[w.Empires["e01"].HomeworldID].SystemID
	f.Cargo.Deuterium = 1000
	r := resolve(t, w, Order{EmpireID: "e00", Type: "move", Actor: f.ID, Target: w.Systems[f.SystemID].Neighbors[0]})
	if len(r.Accepted) != 1 {
		t.Fatal("fleet carrying deuterium cannot leave a system without an owned planet")
	}
}

func TestKnownGapTransportToAnotherEmpire(t *testing.T) {
	t.Skip("spec gap: resolveArrivals only unloads cargo at the sender's own planets, so the trade described in spec/game.md (send cargo to another empire) is impossible")
	w := newTestWorld(t, 1)
	p := home(w, "e00")
	f := formFleet(t, w, map[string]int{"transport": 1})
	col := w.Planets[w.Systems[f.SystemID].Planets[1]]
	col.OwnerID = "e01"
	p.Resources = Resources{1000, 1000, 1000}
	resolve(t, w, Order{EmpireID: "e00", Type: "transport", Actor: f.ID, Target: col.ID, Params: map[string]any{"metal": 100}})
	if col.Resources.Metal != 100 {
		t.Fatal("cargo not delivered to trading partner")
	}
}

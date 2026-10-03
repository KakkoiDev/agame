package agent

import (
	"encoding/json"
	"testing"

	"github.com/KakkoiDev/agame/world"
)

func TestAutopilotProducesEngineSafeTurn(t *testing.T) {
	w, err := world.Generate(7, []string{"A", "B", "C", "D", "E", "F", "G", "H"})
	if err != nil {
		t.Fatal(err)
	}
	submitted := map[string][]world.Order{}
	for id := range w.Empires {
		d := Autopilot(w, id)
		for i := range d.Orders {
			d.Orders[i].EmpireID = id
		}
		submitted[id] = d.Orders
	}
	res, err := world.ResolveTurn(w, submitted)
	if err != nil {
		t.Fatal(err)
	}
	if w.Turn != 1 {
		t.Fatalf("turn=%d", w.Turn)
	}
	if len(res.Accepted) == 0 {
		t.Fatal("autopilot produced no accepted actions")
	}
}

func TestAutopilotChangesStrategicStateAcross57Turns(t *testing.T) {
	w, err := world.Generate(57, []string{"A", "B", "C", "D", "E", "F", "G", "H"})
	if err != nil {
		t.Fatal(err)
	}
	initialOwned := ownedCount(w)
	initialTech := techTotal(w)
	initialFleets := len(w.Fleets)
	accepted := 0
	for turn := 0; turn < 57; turn++ {
		submitted := map[string][]world.Order{}
		for id := range w.Empires {
			d := Autopilot(w, id)
			for i := range d.Orders {
				d.Orders[i].EmpireID = id
			}
			submitted[id] = d.Orders
		}
		res, err := world.ResolveTurn(w, submitted)
		if err != nil {
			t.Fatal(err)
		}
		accepted += len(res.Accepted)
	}
	if accepted < 40 {
		t.Fatalf("only %d accepted orders in 57 turns", accepted)
	}
	if ownedCount(w) <= initialOwned {
		e := w.Empires["e00"]
		p := w.Planets[e.HomeworldID]
		t.Fatalf("no expansion after 57 turns: planets=%d colonization=%d resources=%+v ships=%+v shipq=%+v fleets=%d", ownedCount(w), e.Tech.Colonization, p.Resources, p.Ships, p.ShipyardQueue, len(w.Fleets))
	}
	if techTotal(w) <= initialTech {
		t.Fatalf("no research after 57 turns: tech=%d", techTotal(w))
	}
	if len(w.Fleets) <= initialFleets {
		t.Fatalf("no persistent fleet activity after 57 turns: fleets=%d", len(w.Fleets))
	}
}

func ownedCount(w *world.World) int {
	n := 0
	for _, p := range w.Planets {
		if p.OwnerID != "" {
			n++
		}
	}
	return n
}
func techTotal(w *world.World) int {
	n := 0
	for _, e := range w.Empires {
		n += e.Tech.Industry + e.Tech.Propulsion + e.Tech.Weapons + e.Tech.Shields + e.Tech.Sensors + e.Tech.Colonization
	}
	return n
}

func TestAutopilotIsDeterministic(t *testing.T) {
	a, _ := world.Generate(5, []string{"A", "B", "C", "D", "E", "F", "G", "H"})
	b, _ := world.Generate(5, []string{"A", "B", "C", "D", "E", "F", "G", "H"})
	for turn := 0; turn < 30; turn++ {
		for _, w := range []*world.World{a, b} {
			sub := map[string][]world.Order{}
			for id := range w.Empires {
				d := Autopilot(w, id)
				for i := range d.Orders {
					d.Orders[i].EmpireID = id
				}
				sub[id] = d.Orders
			}
			if _, err := world.ResolveTurn(w, sub); err != nil {
				t.Fatal(err)
			}
		}
	}
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Fatal("autopilot runs diverged")
	}
}

func TestAutopilotDoesNotRepeatImpossibleColonization(t *testing.T) {
	// A Colony Ark parked at a system where its empire owns no planet cannot
	// pay fuel to leave. Autopilot used to order it to colonize a planet
	// elsewhere every single turn; the order was always rejected and, since it
	// returned early, the empire never built or researched anything again.
	w, _ := world.Generate(5, []string{"A", "B", "C", "D", "E", "F", "G", "H"})
	h := w.Planets[w.Empires["e00"].HomeworldID]
	foreign := w.Planets[w.Empires["e01"].HomeworldID].SystemID
	for _, id := range w.Systems[foreign].Planets {
		if w.Planets[id].OwnerID == "" {
			w.Planets[id].OwnerID = "e02"
		}
	}
	w.Fleets["fark"] = &world.Fleet{ID: "fark", OwnerID: "e00", SystemID: foreign, Ships: world.Ships{"colony_ark": 1}}
	h.Resources = world.Resources{Metal: 5000, Crystal: 5000, Deuterium: 5000}
	d := Autopilot(w, "e00")
	if len(d.Orders) == 1 && d.Orders[0].Type == "colonize" {
		t.Fatalf("autopilot issued an unlaunchable colonize order: %+v", d.Orders[0])
	}
}

func TestAutopilotEliminatedAndUnknownEmpireDoNothing(t *testing.T) {
	w, _ := world.Generate(5, []string{"A", "B", "C", "D", "E", "F", "G", "H"})
	w.Empires["e03"].Eliminated = true
	if d := Autopilot(w, "e03"); len(d.Orders) != 0 {
		t.Fatal("eliminated empire acted")
	}
	if d := Autopilot(w, "zz"); len(d.Orders) != 0 {
		t.Fatal("unknown empire acted")
	}
}

func TestAutopilotExileRecolonizesInPlace(t *testing.T) {
	w, _ := world.Generate(5, []string{"A", "B", "C", "D", "E", "F", "G", "H"})
	h := w.Planets[w.Empires["e00"].HomeworldID]
	h.OwnerID = ""
	w.Empires["e00"].Tech.Colonization = 1
	w.Fleets["fark"] = &world.Fleet{ID: "fark", OwnerID: "e00", SystemID: h.SystemID, Ships: world.Ships{"colony_ark": 1}}
	d := Autopilot(w, "e00")
	if len(d.Orders) != 1 || d.Orders[0].Type != "colonize" {
		t.Fatalf("exile decision %+v", d)
	}
	d.Orders[0].EmpireID = "e00"
	if _, err := world.ResolveTurn(w, map[string][]world.Order{"e00": d.Orders}); err != nil {
		t.Fatal(err)
	}
	if ownedPlanets(w, "e00") == nil {
		t.Fatal("exile did not recolonize")
	}
}

func TestDescribeOrderAndHuman(t *testing.T) {
	if s := describeOrder(world.Order{Type: "move", Actor: "f1", Target: "s02"}); s != "move f1 → s02" {
		t.Fatal(s)
	}
	if s := describeOrder(world.Order{Type: "form_fleet", Actor: "p"}); s != "form_fleet p" {
		t.Fatal(s)
	}
	if human("metal_mine") != "Metal Mine" || human("shipyard") != "shipyard" {
		t.Fatal("human labels")
	}
}

func TestAutopilotDoesNotLaunchUnfueledAttacks(t *testing.T) {
	w, _ := world.Generate(5, []string{"A", "B", "C", "D", "E", "F", "G", "H"})
	h := w.Planets[w.Empires["e00"].HomeworldID]
	w.Fleets["fwar"] = &world.Fleet{ID: "fwar", OwnerID: "e00", SystemID: h.SystemID, Ships: world.Ships{"cruiser": 3}}
	h.Resources = world.Resources{Metal: 5000, Crystal: 5000}
	for _, o := range Autopilot(w, "e00").Orders {
		if o.Type == "attack" {
			t.Fatalf("autopilot attacked without fuel: %+v", o)
		}
	}
	h.Resources.Deuterium = 5000
	attacked := false
	for _, o := range Autopilot(w, "e00").Orders {
		attacked = attacked || o.Type == "attack"
	}
	if !attacked {
		t.Fatal("fuelled fleet did not attack")
	}
}

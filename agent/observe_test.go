package agent

import (
	"reflect"
	"sort"
	"testing"

	"github.com/KakkoiDev/agame/world"
)

var testNames = []string{"A", "B", "C", "D", "E", "F", "G", "H"}

func genWorld(t *testing.T, seed int64) *world.World {
	t.Helper()
	w, err := world.Generate(seed, testNames)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func TestObserveIsSortedAndStable(t *testing.T) {
	w := genWorld(t, 3)
	h := w.Planets[w.Empires["e00"].HomeworldID]
	for _, id := range w.Systems[h.SystemID].Planets {
		w.Planets[id].OwnerID = "e00"
	}
	for i := 0; i < 5; i++ {
		id := []string{"f9", "f1", "f5", "f3", "f7"}[i]
		w.Fleets[id] = &world.Fleet{ID: id, OwnerID: "e00", SystemID: h.SystemID, Ships: world.Ships{"scout": 1}}
	}
	first := Observe(w, "e00")
	if !sort.SliceIsSorted(first.Planets, func(i, j int) bool { return first.Planets[i].ID < first.Planets[j].ID }) {
		t.Fatal("planets not sorted by ID")
	}
	if !sort.SliceIsSorted(first.Fleets, func(i, j int) bool { return first.Fleets[i].ID < first.Fleets[j].ID }) {
		t.Fatal("fleets not sorted by ID")
	}
	for i := 0; i < 20; i++ {
		if !reflect.DeepEqual(first, Observe(w, "e00")) {
			t.Fatal("observation order depends on map iteration")
		}
	}
}

func TestObserveCannotMutateWorld(t *testing.T) {
	// spec/agents.md: all rulers decide against the frozen S(t). Agents run
	// sequentially, so an observation must not alias live world state.
	w := genWorld(t, 3)
	w.Empires["e00"].Research = &world.Queue{Kind: "weapons", Required: 3}
	w.Fleets["f1"] = &world.Fleet{ID: "f1", OwnerID: "e00", SystemID: "s00", Ships: world.Ships{"scout": 1}, Route: []string{"s00", "s01"}}
	o := Observe(w, "e00")
	o.Empire.Name = "hacked"
	o.Empire.Research.Progress = 99
	o.Planets[0].Resources.Metal = 1 << 30
	o.Planets[0].Ships["cruiser"] = 50
	o.Fleets[0].Ships["cruiser"] = 50
	o.Fleets[0].Route[1] = "s31"
	p := w.Planets[w.Empires["e00"].HomeworldID]
	if w.Empires["e00"].Name == "hacked" || w.Empires["e00"].Research.Progress == 99 ||
		p.Resources.Metal == 1<<30 || p.Ships["cruiser"] != 0 || w.Fleets["f1"].Ships["cruiser"] != 0 || w.Fleets["f1"].Route[1] != "s01" {
		t.Fatal("agent mutated authoritative world through its observation")
	}
}

func TestObserveDeliversOnlyOwnDueMessages(t *testing.T) {
	w := genWorld(t, 3)
	w.Turn = 4
	w.Messages = []world.Message{
		{Turn: 4, From: "e01", To: "e00", Body: "due"},
		{Turn: 5, From: "e01", To: "e00", Body: "future"},
		{Turn: 3, From: "e01", To: "e02", Body: "other"},
	}
	o := Observe(w, "e00")
	if len(o.Messages) != 1 || o.Messages[0].Body != "due" {
		t.Fatalf("messages=%+v", o.Messages)
	}
	if o := Observe(w, "nobody"); o.Empire != nil || len(o.Planets) != 0 {
		t.Fatal("unknown empire observed something")
	}
}

func TestObserveExposesPublicGraphAndPlanets(t *testing.T) {
	w := genWorld(t, 3)
	o := Observe(w, "e00")
	if len(o.Systems) != len(w.Systems) {
		t.Fatalf("systems=%d", len(o.Systems))
	}
	for i, s := range o.Systems {
		if i > 0 && o.Systems[i-1].ID >= s.ID {
			t.Fatal("systems not sorted")
		}
		ws := w.Systems[s.ID]
		if !reflect.DeepEqual(s.Neighbors, ws.Neighbors) || !reflect.DeepEqual(s.Planets, ws.Planets) {
			t.Fatalf("system %s: %+v vs %+v", s.ID, s, ws)
		}
		if s.Debris != nil {
			t.Fatalf("debris on %s at start", s.ID)
		}
	}
	if len(o.Planets)+len(o.OtherPlanets) != len(w.Planets) {
		t.Fatalf("planets %d + %d != %d", len(o.Planets), len(o.OtherPlanets), len(w.Planets))
	}
	homes := 0
	for _, p := range o.OtherPlanets {
		if p.OwnerID == "e00" || p.OwnerID != w.Planets[p.ID].OwnerID || p.SystemID != w.Planets[p.ID].SystemID {
			t.Fatalf("planet view %+v", p)
		}
		if p.Homeworld {
			homes++
		}
	}
	if homes != 7 {
		t.Fatalf("foreign homeworlds visible=%d, want 7", homes)
	}
	if len(o.Orders) == 0 {
		t.Fatal("no legal order types")
	}
}

func TestObserveForeignFleetsAndDebrisNeedPresence(t *testing.T) {
	w := genWorld(t, 3)
	h := w.Planets[w.Empires["e00"].HomeworldID]
	other := w.Planets[w.Empires["e01"].HomeworldID]
	remote := w.Planets[w.Empires["e02"].HomeworldID].SystemID
	w.Fleets["f1"] = &world.Fleet{ID: "f1", OwnerID: "e01", SystemID: h.SystemID, Ships: world.Ships{"frigate": 7}}
	w.Fleets["f2"] = &world.Fleet{ID: "f2", OwnerID: "e01", SystemID: other.SystemID, Ships: world.Ships{"frigate": 1}}
	w.Fleets["f3"] = &world.Fleet{ID: "f3", OwnerID: "e02", SystemID: remote, Ships: world.Ships{"cruiser": 60}}
	w.Fleets["f4"] = &world.Fleet{ID: "f4", OwnerID: "e00", SystemID: remote, Ships: world.Ships{"scout": 1}}
	w.Systems[h.SystemID].Debris = world.Resources{Metal: 30}
	w.Systems[other.SystemID].Debris = world.Resources{Metal: 99}
	o := Observe(w, "e00")
	want := []FleetSighting{
		{ID: "f1", OwnerID: "e01", SystemID: h.SystemID, Size: "5-19"},
		{ID: "f3", OwnerID: "e02", SystemID: remote, Size: "50+"},
	}
	if !reflect.DeepEqual(o.ForeignFleets, want) {
		t.Fatalf("foreign fleets %+v want %+v", o.ForeignFleets, want)
	}
	for _, s := range o.Systems {
		switch s.ID {
		case h.SystemID:
			if s.Debris == nil || s.Debris.Metal != 30 {
				t.Fatalf("own-system debris %+v", s.Debris)
			}
		case other.SystemID:
			if s.Debris != nil {
				t.Fatal("debris visible without presence")
			}
		}
	}
	for _, f := range o.Fleets {
		if f.OwnerID != "e00" {
			t.Fatal("foreign fleet in own fleets")
		}
	}
}

func TestObserveGraphIsADeepCopy(t *testing.T) {
	w := genWorld(t, 3)
	w.Systems["s00"].Debris = world.Resources{Metal: 5}
	w.Fleets["f1"] = &world.Fleet{ID: "f1", OwnerID: "e00", SystemID: "s00", Ships: world.Ships{"scout": 1}}
	o := Observe(w, "e00")
	before := append([]string(nil), w.Systems["s00"].Neighbors...)
	o.Systems[0].Neighbors[0] = "hacked"
	o.Systems[0].Planets[0] = "hacked"
	o.Systems[0].Debris.Metal = 1 << 20
	o.Orders[0].Type = "hacked"
	if !reflect.DeepEqual(w.Systems["s00"].Neighbors, before) || w.Systems["s00"].Planets[0] == "hacked" || w.Systems["s00"].Debris.Metal != 5 {
		t.Fatal("observation aliases the world graph")
	}
	if Observe(w, "e00").Orders[0].Type == "hacked" {
		t.Fatal("order specs are shared between observations")
	}
}

// TestObservationSuffices builds move and attack orders using only the
// observation and checks the engine accepts them.
func TestObservationSufficesForMoveAndAttack(t *testing.T) {
	w := genWorld(t, 3)
	w.Planets[w.Empires["e00"].HomeworldID].Resources.Deuterium = 5000
	form := Observe(w, "e00")
	home := form.Planets[0]
	if _, err := world.ResolveTurn(w, map[string][]world.Order{"e00": {{EmpireID: "e00", Type: world.OrderFormFleet, Actor: home.ID,
		Params: map[string]any{"ships": map[string]any{"frigate": float64(1)}}}, {EmpireID: "e00", Type: world.OrderFormFleet, Actor: home.ID,
		Params: map[string]any{"ships": map[string]any{"frigate": float64(1)}}}}}); err != nil {
		t.Fatal(err)
	}
	o := Observe(w, "e00")
	if len(o.Fleets) != 2 {
		t.Fatalf("fleets=%+v", o.Fleets)
	}
	var neighbor, enemy string
	for _, s := range o.Systems {
		if s.ID == o.Fleets[0].SystemID {
			neighbor = s.Neighbors[0]
		}
	}
	for _, p := range o.OtherPlanets {
		if p.OwnerID != "" {
			enemy = p.ID
			break
		}
	}
	res, err := world.ResolveTurn(w, map[string][]world.Order{"e00": {
		{EmpireID: "e00", Type: world.OrderMove, Actor: o.Fleets[0].ID, Target: neighbor},
		{EmpireID: "e00", Type: world.OrderAttack, Actor: o.Fleets[1].ID, Target: enemy},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Accepted) != 2 {
		t.Fatalf("rejected=%+v", res.Rejected)
	}
}

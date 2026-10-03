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

package world

import (
	"encoding/json"
	"reflect"
	"testing"
)

var worldNames = []string{"A", "B", "C", "D", "E", "F", "G", "H"}

func TestCanonicalAndDeterministic(t *testing.T) {
	a, e := Generate(42, worldNames)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := Generate(42, worldNames)
	if len(a.Systems) != 32 || len(a.Planets) != 128 || len(a.Empires) != 8 {
		t.Fatal("canonical size")
	}
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if !reflect.DeepEqual(ja, jb) {
		t.Fatal("not deterministic")
	}
	start := systemIDs(a)[0]
	for _, id := range systemIDs(a) {
		if distance(a, start, id) > 32 {
			t.Fatal("disconnected")
		}
	}
}
func TestEconomyProgress(t *testing.T) {
	w, _ := Generate(1, worldNames)
	p := w.Planets[w.Empires["e00"].HomeworldID]
	before := p.Resources.Metal
	_, e := ResolveTurn(w, map[string][]Order{})
	if e != nil {
		t.Fatal(e)
	}
	if p.Resources.Metal <= before {
		t.Fatal("no production")
	}
}
func TestConstruct(t *testing.T) {
	w, _ := Generate(1, worldNames)
	p := w.Planets[w.Empires["e00"].HomeworldID]
	p.Resources = Resources{10000, 10000, 10000}
	o := Order{EmpireID: "e00", Type: "construct", Actor: p.ID, Target: "metal_mine"}
	ResolveTurn(w, map[string][]Order{"e00": {o}})
	for p.Construction != nil {
		ResolveTurn(w, map[string][]Order{})
	}
	if p.Buildings.MetalMine != 2 {
		t.Fatal("build did not finish")
	}
}
func TestFleetMove(t *testing.T) {
	w, _ := Generate(2, worldNames)
	p := w.Planets[w.Empires["e00"].HomeworldID]
	p.Resources.Deuterium = 10000
	o := Order{EmpireID: "e00", Type: "form_fleet", Actor: p.ID, Params: map[string]any{"ships": map[string]int{"scout": 1}}}
	ResolveTurn(w, map[string][]Order{"e00": {o}})
	var f *Fleet
	for _, x := range w.Fleets {
		f = x
	}
	target := w.Systems[f.SystemID].Neighbors[0]
	m := Order{EmpireID: "e00", Type: "move", Actor: f.ID, Target: target}
	ResolveTurn(w, map[string][]Order{"e00": {m}})
	if f.SystemID != target {
		t.Fatal("fleet did not move")
	}
}
func TestSpoofRejected(t *testing.T) {
	w, _ := Generate(1, worldNames)
	r, _ := ResolveTurn(w, map[string][]Order{"e00": {{EmpireID: "e01", Type: "move"}}})
	if len(r.Accepted) != 0 {
		t.Fatal("spoof accepted")
	}
}

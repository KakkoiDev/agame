package world

import (
	"encoding/json"
	"reflect"
	"testing"
)

var names = []string{"A", "B", "C", "D", "E", "F", "G", "H"}

func TestGenerateCanonicalUniverse(t *testing.T) {
	w, err := Generate(42, names)
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Systems) != 32 {
		t.Fatalf("systems=%d", len(w.Systems))
	}
	if len(w.Planets) != 128 {
		t.Fatalf("planets=%d", len(w.Planets))
	}
	if len(w.Empires) != 8 {
		t.Fatalf("empires=%d", len(w.Empires))
	}
	for _, e := range w.Empires {
		p := w.Planets[e.HomeworldID]
		if p == nil || !p.Homeworld || p.OwnerID != e.ID {
			t.Fatalf("bad homeworld for %s", e.ID)
		}
	}
}

func TestGenerateIsDeterministic(t *testing.T) {
	a, _ := Generate(987654, names)
	b, _ := Generate(987654, names)
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if !reflect.DeepEqual(ja, jb) {
		t.Fatal("same seed produced different worlds")
	}
}

func TestGraphIsConnected(t *testing.T) {
	w, _ := Generate(7, names)
	start := systemIDs(w)[0]
	for _, id := range systemIDs(w) {
		if distance(w, start, id) > CanonicalSystems {
			t.Fatalf("%s unreachable", id)
		}
	}
}

func TestHomesAreDistinct(t *testing.T) {
	w, _ := Generate(99, names)
	seen := map[string]bool{}
	for _, e := range w.Empires {
		s := w.Planets[e.HomeworldID].SystemID
		if seen[s] {
			t.Fatalf("duplicate home system %s", s)
		}
		seen[s] = true
	}
}

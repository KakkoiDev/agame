package agent

import (
	"github.com/KakkoiDev/agame/world"
	"testing"
)

func TestObservationIsPrivate(t *testing.T) {
	w, _ := world.Generate(1, []string{"A", "B", "C", "D", "E", "F", "G", "H"})
	o := Observe(w, "e00")
	if len(o.Planets) != 1 {
		t.Fatal("expected own planet only")
	}
	if o.Empire.ID != "e00" {
		t.Fatal("identity")
	}
}

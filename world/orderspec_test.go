package world

import (
	"reflect"
	"testing"
)

func TestOrderSpecsAreAllSupported(t *testing.T) {
	w := newTestWorld(t, 1)
	seen := map[string]bool{}
	for _, s := range OrderSpecs() {
		if seen[s.Type] {
			t.Fatalf("duplicate spec %s", s.Type)
		}
		seen[s.Type] = true
		err := validateOrder(w, Order{EmpireID: "e00", Type: s.Type})
		if err != nil && err.Error() == "unsupported" {
			t.Fatalf("order spec %s is not supported by the engine", s.Type)
		}
	}
	if !reflect.DeepEqual(OrderSpecs(), OrderSpecs()) {
		t.Fatal("order specs are not stable")
	}
	a := OrderSpecs()
	a[2].Params["quantity"] = "changed"
	if OrderSpecs()[2].Params["quantity"] == "changed" {
		t.Fatal("order specs share mutable state")
	}
}

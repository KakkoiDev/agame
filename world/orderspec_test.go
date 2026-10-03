package world

import (
	"reflect"
	"strings"
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
		if err != nil && strings.HasPrefix(err.Error(), "unsupported") {
			t.Fatalf("order spec %s is not supported by the engine", s.Type)
		}
	}
	for _, typ := range []string{OrderConstruct, OrderBuildShips, OrderResearch, OrderFormFleet, OrderSplitFleet, OrderMove, OrderAttack, OrderColonize,
		OrderSpy, OrderTransport, OrderRecycle, OrderMessage, OrderAllianceCreate, OrderAllianceJoin, OrderAllianceLeave, OrderCancel} {
		if !seen[typ] { // spec/agents.md "Required v1 order types" plus cancel (D20)
			t.Fatalf("required order type %s has no spec", typ)
		}
	}
	if err := validateOrder(w, Order{EmpireID: "e00", Type: "teleport"}); err == nil || !strings.HasPrefix(err.Error(), "unsupported") {
		t.Fatalf("unknown type: %v", err)
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

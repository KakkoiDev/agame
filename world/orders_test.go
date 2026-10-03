package world

import (
	"encoding/json"
	"fmt"
	"testing"
)

// helpers shared by the order/turn tests in this package.

func newTestWorld(t *testing.T, seed int64) *World {
	t.Helper()
	w, err := Generate(seed, names)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func home(w *World, eid string) *Planet { return w.Planets[w.Empires[eid].HomeworldID] }

func resolve(t *testing.T, w *World, orders ...Order) TurnResult {
	t.Helper()
	sub := map[string][]Order{}
	for _, o := range orders {
		sub[o.EmpireID] = append(sub[o.EmpireID], o)
	}
	r, err := ResolveTurn(w, sub)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func nonNegative(t *testing.T, w *World) {
	t.Helper()
	for id, p := range w.Planets {
		r := p.Resources
		if r.Metal < 0 || r.Crystal < 0 || r.Deuterium < 0 {
			t.Fatalf("planet %s has negative resources %+v", id, r)
		}
		for k, n := range p.Ships {
			if n < 0 {
				t.Fatalf("planet %s has %d %s", id, n, k)
			}
		}
	}
}

// formFleet creates a fleet at the e00 homeworld and returns it.
func formFleet(t *testing.T, w *World, ships map[string]int) *Fleet {
	t.Helper()
	p := home(w, "e00")
	before := w.NextFleet
	r := resolve(t, w, Order{EmpireID: "e00", Type: "form_fleet", Actor: p.ID, Params: map[string]any{"ships": ships}})
	if len(r.Accepted) != 1 || w.NextFleet != before+1 {
		t.Fatalf("form_fleet not accepted: %+v", r)
	}
	for _, f := range w.Fleets {
		if f.ID == fleetID(before) {
			return f
		}
	}
	t.Fatal("fleet missing")
	return nil
}

func fleetID(n int) string { return fmt.Sprintf("f%06d", n) }

func inBoth(r TurnResult) bool {
	for _, a := range r.Accepted {
		for _, b := range r.Rejected {
			ja, _ := json.Marshal(a)
			jb, _ := json.Marshal(b)
			if string(ja) == string(jb) {
				return true
			}
		}
	}
	return false
}

// --- regression tests for order-validation holes ---

func TestDuplicateConstructChargesOnce(t *testing.T) {
	w := newTestWorld(t, 1)
	p := home(w, "e00")
	p.Resources = Resources{1000, 1000, 1000}
	o := Order{EmpireID: "e00", Type: "construct", Actor: p.ID, Target: "metal_mine"}
	r := resolve(t, w, o, o)
	if len(r.Accepted) != 1 || len(r.Rejected) != 1 {
		t.Fatalf("accepted=%d rejected=%d, want 1/1", len(r.Accepted), len(r.Rejected))
	}
	cost := scale(BuildingBase["metal_mine"], 2)
	// production after the turn is +30 metal (level-1 mine), +20 crystal, +12 deuterium.
	want := Resources{1000 - cost.Metal + 30, 1000 - cost.Crystal + 20, 1000 - cost.Deuterium + 12}
	if p.Resources != want {
		t.Fatalf("resources=%+v want %+v (double charge?)", p.Resources, want)
	}
}

func TestSameTurnOrdersCannotOverdraw(t *testing.T) {
	w := newTestWorld(t, 1)
	p := home(w, "e00")
	w.Empires["e00"].Tech.Weapons = 1
	// Each order is individually affordable from the snapshot (500/300/150),
	// but together they cost more metal than the planet holds.
	r := resolve(t, w,
		Order{EmpireID: "e00", Type: "construct", Actor: p.ID, Target: "metal_mine"},
		Order{EmpireID: "e00", Type: "research", Actor: p.ID, Target: "industry"},
		Order{EmpireID: "e00", Type: "build_ships", Actor: p.ID, Target: "cruiser", Params: map[string]any{"quantity": 1}},
	)
	nonNegative(t, w)
	if len(r.Rejected) == 0 {
		t.Fatal("expected at least one order to be rejected for insufficient resources")
	}
	if inBoth(r) {
		t.Fatal("an order is reported as both accepted and rejected")
	}
}

func TestFleetGetsAtMostOneMissionPerTurn(t *testing.T) {
	w := newTestWorld(t, 2)
	p := home(w, "e00")
	f := formFleet(t, w, map[string]int{"frigate": 2})
	p.Resources.Deuterium = 1000
	n := w.Systems[f.SystemID].Neighbors
	before := p.Resources.Deuterium
	r := resolve(t, w,
		Order{EmpireID: "e00", Type: "move", Actor: f.ID, Target: n[0]},
		Order{EmpireID: "e00", Type: "move", Actor: f.ID, Target: n[len(n)-1]},
	)
	if len(r.Accepted) != 1 {
		t.Fatalf("accepted %d missions for one fleet", len(r.Accepted))
	}
	fuel := 2 * ShipSpecs["frigate"].Fuel
	if got := before - p.Resources.Deuterium + 12; got != fuel {
		t.Fatalf("fuel charged %d, want %d", got, fuel)
	}
	if f.SystemID != n[0] {
		t.Fatalf("fleet at %s, want first ordered destination %s", f.SystemID, n[0])
	}
}

func TestFormFleetOverdrawIsRejectedNotAccepted(t *testing.T) {
	w := newTestWorld(t, 1)
	p := home(w, "e00")
	o := Order{EmpireID: "e00", Type: "form_fleet", Actor: p.ID, Params: map[string]any{"ships": map[string]int{"frigate": 2}}}
	r := resolve(t, w, o, o)
	if len(r.Accepted) != 1 || len(r.Rejected) != 1 {
		t.Fatalf("accepted=%v rejected=%v", r.Accepted, r.Rejected)
	}
	if len(w.Fleets) != 1 {
		t.Fatalf("fleets=%d", len(w.Fleets))
	}
}

func TestFormFleetRejectsEmptyOrUnknownShips(t *testing.T) {
	for name, ships := range map[string]map[string]int{
		"empty":   {},
		"zero":    {"scout": 0},
		"unknown": {"battlestar": 0},
	} {
		t.Run(name, func(t *testing.T) {
			w := newTestWorld(t, 1)
			p := home(w, "e00")
			r := resolve(t, w, Order{EmpireID: "e00", Type: "form_fleet", Actor: p.ID, Params: map[string]any{"ships": ships}})
			if len(r.Accepted) != 0 || len(w.Fleets) != 0 {
				t.Fatalf("empty fleet formed: %+v", w.Fleets)
			}
		})
	}
}

func TestBuildShipsQuantityOverflowRejected(t *testing.T) {
	for _, q := range []any{1 << 62, float64(1 << 62), -1, 0} {
		w := newTestWorld(t, 1)
		p := home(w, "e00")
		r := resolve(t, w, Order{EmpireID: "e00", Type: "build_ships", Actor: p.ID, Target: "scout", Params: map[string]any{"quantity": q}})
		if len(r.Accepted) != 0 {
			t.Fatalf("quantity %v accepted; ships=%v", q, p.Ships)
		}
		if p.Ships["scout"] != 1 {
			t.Fatalf("quantity %v produced scouts=%d", q, p.Ships["scout"])
		}
	}
}

func TestTransportRejectsNegativeCargo(t *testing.T) {
	w := newTestWorld(t, 2)
	p := home(w, "e00")
	f := formFleet(t, w, map[string]int{"transport": 1})
	before := p.Resources
	r := resolve(t, w, Order{EmpireID: "e00", Type: "transport", Actor: f.ID, Target: p.ID,
		Params: map[string]any{"metal": float64(-100000)}})
	if len(r.Accepted) != 0 {
		t.Fatal("negative cargo accepted")
	}
	if p.Resources.Metal > before.Metal+30 {
		t.Fatalf("metal minted: %d -> %d", before.Metal, p.Resources.Metal)
	}
	nonNegative(t, w)
	if f.Cargo != (Resources{}) {
		t.Fatalf("cargo=%+v", f.Cargo)
	}
}

func TestTransportRespectsCargoCapacity(t *testing.T) {
	w := newTestWorld(t, 2)
	p := home(w, "e00")
	f := formFleet(t, w, map[string]int{"transport": 1})
	p.Resources = Resources{5000, 5000, 5000}
	r := resolve(t, w, Order{EmpireID: "e00", Type: "transport", Actor: f.ID, Target: w.Systems[f.SystemID].Neighbors[0],
		Params: map[string]any{"metal": 251}})
	if len(r.Accepted) != 0 {
		t.Fatal("cargo above capacity accepted")
	}
	r = resolve(t, w, Order{EmpireID: "e00", Type: "transport", Actor: f.ID, Target: w.Systems[f.SystemID].Neighbors[0],
		Params: map[string]any{"metal": 250}})
	if len(r.Accepted) != 1 || f.Cargo.Metal != 250 {
		t.Fatalf("cargo at capacity rejected: %+v cargo=%+v", r.Rejected, f.Cargo)
	}
}

func TestFailedLaunchHasNoSideEffects(t *testing.T) {
	w := newTestWorld(t, 2)
	p := home(w, "e00")
	f := formFleet(t, w, map[string]int{"transport": 1})
	p.Resources = Resources{10, 10, 100}
	dest := w.Systems[f.SystemID].Neighbors[0]
	r := resolve(t, w, Order{EmpireID: "e00", Type: "transport", Actor: f.ID, Target: dest,
		Params: map[string]any{"metal": 200}})
	if len(r.Accepted) != 0 {
		t.Fatal("unaffordable cargo accepted")
	}
	if f.SystemID == dest || len(f.Route) != 0 || f.Mission != "" {
		t.Fatalf("rejected transport still launched: system=%s route=%v mission=%q", f.SystemID, f.Route, f.Mission)
	}
	if p.Resources.Deuterium != 100+12 {
		t.Fatalf("fuel charged for rejected launch: deuterium=%d", p.Resources.Deuterium)
	}
}

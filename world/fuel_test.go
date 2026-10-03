package world

import "testing"

func TestLaunchFuelPayer(t *testing.T) {
	fuel := 2 * ShipSpecs[ShipFrigate].Fuel // two frigates, one edge
	for _, tc := range []struct {
		name                          string
		planetDeut, cargoDeut         int
		accepted                      bool
		wantPlanetDeut, wantCargoDeut int
	}{
		{"planet pays first", 100, 50, true, 100 - fuel, 50},
		{"cargo pays when planet cannot", 5, 50, true, 5, 50 - fuel},
		{"neither can pay", 5, 5, false, 5, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newTestWorld(t, 4)
			p := home(w, "e00")
			f := formFleet(t, w, map[string]int{ShipFrigate: 2})
			p.Resources.Deuterium = tc.planetDeut
			f.Cargo.Deuterium = tc.cargoDeut
			from := f.SystemID
			r := resolve(t, w, Order{EmpireID: "e00", Type: OrderMove, Actor: f.ID, Target: w.Systems[from].Neighbors[0]})
			if got := len(r.Accepted) == 1; got != tc.accepted {
				t.Fatalf("accepted=%v rejected=%v", got, r.Rejected)
			}
			if got := p.Resources.Deuterium - 12; got != tc.wantPlanetDeut || f.Cargo.Deuterium != tc.wantCargoDeut {
				t.Fatalf("planet deut %d (want %d) cargo deut %d (want %d)", got, tc.wantPlanetDeut, f.Cargo.Deuterium, tc.wantCargoDeut)
			}
			if !tc.accepted && f.SystemID != from {
				t.Fatal("rejected move still moved")
			}
		})
	}
}

func TestTransportCargoStillComesFromPlanetWhenCargoPaysFuel(t *testing.T) {
	w := newTestWorld(t, 4)
	p := home(w, "e00")
	f := formFleet(t, w, map[string]int{ShipTransport: 1})
	p.Resources = Resources{500, 0, 0}
	f.Cargo.Deuterium = 10
	r := resolve(t, w, Order{EmpireID: "e00", Type: OrderTransport, Actor: f.ID, Target: w.Systems[f.SystemID].Neighbors[0],
		Params: map[string]any{"metal": 100}})
	if len(r.Accepted) != 1 || f.Cargo != (Resources{100, 0, 10 - ShipSpecs[ShipTransport].Fuel}) || p.Resources.Metal != 500-100+30 {
		t.Fatalf("accepted=%d cargo=%+v planet=%+v", len(r.Accepted), f.Cargo, p.Resources)
	}
}

// exileArk strips e01 of its planets and parks an ark in a system whose
// planets are all owned by someone else.
func exileArk(t *testing.T) (*World, *Fleet) {
	t.Helper()
	w := newTestWorld(t, 1)
	h := home(w, "e01")
	h.OwnerID = ""
	sys := w.Systems[home(w, "e02").SystemID]
	for _, id := range sys.Planets {
		w.Planets[id].OwnerID = "e02"
	}
	f := &Fleet{ID: "fark", OwnerID: "e01", SystemID: sys.ID, Ships: Ships{ShipColonyArk: 1}}
	w.Fleets[f.ID] = f
	return w, f
}

func TestExileNeedsFuelToReachAPlanet(t *testing.T) {
	w, _ := exileArk(t)
	resolve(t, w)
	if e := w.Empires["e01"]; e.Exile || !e.Eliminated {
		t.Fatalf("ark without fuel or a reachable planet kept the empire alive: %+v", e)
	}

	w, f := exileArk(t)
	f.Cargo.Deuterium = ShipSpecs[ShipColonyArk].Fuel // one edge
	resolve(t, w)
	if e := w.Empires["e01"]; !e.Exile || e.Eliminated {
		t.Fatalf("fuelled ark next to unowned planets: %+v", e)
	}
	if !FuelAvailable(w, f, w.Systems[f.SystemID].Neighbors[0]) {
		t.Fatal("FuelAvailable disagrees with sovereignty")
	}

	w, f = exileArk(t)
	for _, id := range systemIDs(w) {
		if r := shortest(w, f.SystemID, id); len(r) >= 3 {
			f.Route, f.RouteIndex, f.Mission = r, 0, OrderMove // committed: fuel was paid at departure
			break
		}
	}
	resolve(t, w)
	if e := w.Empires["e01"]; !e.Exile {
		t.Fatalf("ark on a paid route: %+v", e)
	}
}

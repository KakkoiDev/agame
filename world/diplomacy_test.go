package world

import (
	"fmt"
	"strings"
	"testing"
)

func rejectedWith(t *testing.T, r TurnResult, want string) {
	t.Helper()
	if len(r.Accepted) != 0 || len(r.Rejected) != 1 {
		t.Fatalf("accepted=%+v rejected=%+v, want one rejection", r.Accepted, r.Rejected)
	}
	if !strings.Contains(r.Rejected[0].Reason, want) {
		t.Fatalf("reason %q does not mention %q", r.Rejected[0].Reason, want)
	}
}

func accepted(t *testing.T, r TurnResult, n int) {
	t.Helper()
	if len(r.Accepted) != n {
		t.Fatalf("accepted %d, want %d; rejected=%+v", len(r.Accepted), n, r.Rejected)
	}
}

func ally(t *testing.T, w *World, founder string, members ...string) *Alliance {
	t.Helper()
	accepted(t, resolve(t, w, Order{EmpireID: founder, Type: OrderAllianceCreate, Params: map[string]any{"name": "Pact", "invite": members}}), 1)
	var joins []Order
	for _, m := range members {
		joins = append(joins, Order{EmpireID: m, Type: OrderAllianceJoin, Target: w.Empires[founder].AllianceID})
	}
	accepted(t, resolve(t, w, joins...), len(members))
	return w.Alliances[w.Empires[founder].AllianceID]
}

func TestAllianceCreateInviteJoinLeave(t *testing.T) {
	w := newTestWorld(t, 1)
	r := resolve(t, w, Order{EmpireID: "e00", Type: OrderAllianceCreate, Params: map[string]any{"name": "Northern Pact", "invite": []any{"e01"}}})
	accepted(t, r, 1)
	a := w.Alliances["a001"]
	if a == nil || a.Name != "Northern Pact" || a.Founder != "e00" || len(a.Members) != 1 || a.Invited[0] != "e01" || w.Empires["e00"].AllianceID != "a001" {
		t.Fatalf("alliance %+v", a)
	}
	if len(eventsOfType(r, "alliance_created")) != 1 || len(eventsOfType(r, "alliance_invited")) != 1 {
		t.Fatalf("events %+v", r.Events)
	}
	rejectedWith(t, resolve(t, w, Order{EmpireID: "e02", Type: OrderAllianceJoin, Target: "a001"}), "not invited")
	accepted(t, resolve(t, w, Order{EmpireID: "e00", Type: OrderAllianceInvite, Target: "e02"}), 1)
	r = resolve(t, w,
		Order{EmpireID: "e01", Type: OrderAllianceJoin, Target: "a001"},
		Order{EmpireID: "e02", Type: OrderAllianceJoin, Target: "a001"})
	accepted(t, r, 2)
	if got := strings.Join(a.Members, ","); got != "e00,e01,e02" || len(a.Invited) != 0 {
		t.Fatalf("members %s invited %v", got, a.Invited)
	}
	if !Allied(w, "e01", "e02") || Allied(w, "e01", "e03") || Allied(w, "e01", "e01") {
		t.Fatal("Allied")
	}
	for _, id := range []string{"e00", "e01", "e02"} {
		accepted(t, resolve(t, w, Order{EmpireID: id, Type: OrderAllianceLeave}), 1)
	}
	if w.Alliances["a001"] != nil || w.Empires["e02"].AllianceID != "" {
		t.Fatal("empty alliance not dissolved")
	}
	found := false
	for _, e := range w.Events {
		found = found || (e.Type == "alliance_dissolved" && e.Target == "a001")
	}
	if !found {
		t.Fatal("no alliance_dissolved event")
	}
}

func TestAllianceRejections(t *testing.T) {
	w := newTestWorld(t, 1)
	ally(t, w, "e00", "e01")
	home(w, "e05").OwnerID = ""
	resolve(t, w)
	cases := []struct {
		o    Order
		want string
	}{
		{Order{EmpireID: "e00", Type: OrderAllianceCreate}, "already a member"},
		{Order{EmpireID: "e02", Type: OrderAllianceCreate, Params: map[string]any{"invite": []any{"e99"}}}, "unknown empire"},
		{Order{EmpireID: "e02", Type: OrderAllianceCreate, Params: map[string]any{"invite": []any{"e02"}}}, "yourself"},
		{Order{EmpireID: "e02", Type: OrderAllianceCreate, Params: map[string]any{"name": strings.Repeat("x", 65)}}, "name longer"},
		{Order{EmpireID: "e02", Type: OrderAllianceInvite, Target: "e03"}, "not a member"},
		{Order{EmpireID: "e00", Type: OrderAllianceInvite, Target: "e01"}, "already a member"},
		{Order{EmpireID: "e00", Type: OrderAllianceInvite, Target: "e05"}, "eliminated"},
		{Order{EmpireID: "e01", Type: OrderAllianceJoin, Target: "a001"}, "already a member"},
		{Order{EmpireID: "e02", Type: OrderAllianceJoin, Target: "a999"}, "unknown alliance"},
		{Order{EmpireID: "e02", Type: OrderAllianceLeave}, "not a member"},
		{Order{EmpireID: "e00", Type: OrderAllianceLeave, Target: "a777"}, "not a member"},
	}
	for _, c := range cases {
		rejectedWith(t, resolve(t, w, c.o), c.want)
	}
	accepted(t, resolve(t, w, Order{EmpireID: "e00", Type: OrderAllianceInvite, Target: "e02"}), 1)
	rejectedWith(t, resolve(t, w, Order{EmpireID: "e01", Type: OrderAllianceInvite, Target: "e02"}), "already invited")
}

func TestAttackingAnAllyIsATreatyBreach(t *testing.T) {
	w := newTestWorld(t, 1)
	f, target := stageAttack(t, w, map[string]int{ShipFrigate: 4})
	ally(t, w, "e00", "e01", "e02")
	r := resolve(t, w, Order{EmpireID: "e00", Type: OrderAttack, Actor: f.ID, Target: target.ID})
	accepted(t, r, 1)
	b := eventsOfType(r, "treaty_breach")
	if len(b) != 1 || b[0].EmpireID != "e00" || b[0].Other != "e01" || b[0].Target != "a001" {
		t.Fatalf("breach events %+v", b)
	}
	if w.Empires["e00"].AllianceID != "" || w.Empires["e01"].AllianceID != "a001" || w.Empires["e00"].Stats.Breaches != 1 {
		t.Fatal("breaching attacker must leave, the victim stays")
	}
	if a := w.Alliances["a001"]; strings.Join(a.Members, ",") != "e01,e02" {
		t.Fatalf("members %v", a.Members)
	}
	left := eventsOfType(r, "alliance_left")
	if len(left) != 1 || left[0].Detail != "breach" {
		t.Fatalf("left %+v", left)
	}
}

func TestBattleWithAnAllyJoinedAfterLaunchIsABreach(t *testing.T) {
	w := newTestWorld(t, 1)
	f, target := stageAttack(t, w, map[string]int{ShipFrigate: 4})
	// Launch a two-edge attack, then ally while the fleet is in transit.
	far := ""
	for _, n := range w.Systems[target.SystemID].Neighbors {
		if n != f.SystemID && !adjacent(w, n, f.SystemID) {
			far = n
			break
		}
	}
	if far == "" {
		t.Skip("no two-edge target in this graph")
	}
	target.OwnerID = ""
	tp := w.Planets[w.Systems[far].Planets[2]]
	tp.OwnerID = "e01"
	accepted(t, resolve(t, w, Order{EmpireID: "e00", Type: OrderAllianceCreate, Params: map[string]any{"invite": []any{"e01"}}}), 1)
	accepted(t, resolve(t, w, Order{EmpireID: "e00", Type: OrderAttack, Actor: f.ID, Target: tp.ID}), 1)
	for f.Route != nil {
		r := resolve(t, w, Order{EmpireID: "e01", Type: OrderAllianceJoin, Target: "a001"})
		if len(eventsOfType(r, "battle")) > 0 {
			if len(eventsOfType(r, "treaty_breach")) != 1 {
				t.Fatalf("battle with an ally without breach: %+v", r.Events)
			}
			return
		}
	}
	t.Fatal("no battle")
}

func TestMessagesToAlliancesAndBroadcast(t *testing.T) {
	w := newTestWorld(t, 1)
	ally(t, w, "e00", "e01", "e02")
	home(w, "e07").OwnerID = ""
	resolve(t, w)
	r := resolve(t, w,
		Order{EmpireID: "e01", Type: OrderMessage, Target: "a001", Params: map[string]any{"body": "allies"}},
		Order{EmpireID: "e03", Type: OrderMessage, Target: "a001", Params: map[string]any{"body": "to the pact"}},
		Order{EmpireID: "e04", Type: OrderMessage, Target: "all", Params: map[string]any{"body": "hear ye", "major": true}})
	accepted(t, r, 3)
	got := map[string]string{}
	for _, m := range w.Messages {
		got[m.Body] = strings.Join(m.Recipients, ",")
	}
	if got["allies"] != "e00,e02" || got["to the pact"] != "e00,e01,e02" || got["hear ye"] != "e00,e01,e02,e03,e05,e06" {
		t.Fatalf("recipients %v", got)
	}
	if w.Empires["e04"].Stats.MessagesSent != 1 {
		t.Fatal("message stats")
	}
	rejectedWith(t, resolve(t, w, Order{EmpireID: "e00", Type: OrderMessage, Target: "e00", Params: map[string]any{"body": "me"}}), "yourself")
	rejectedWith(t, resolve(t, w, Order{EmpireID: "e00", Type: OrderMessage, Target: "e01", Params: map[string]any{"body": "  "}}), "non-empty")
	rejectedWith(t, resolve(t, w, Order{EmpireID: "e00", Type: OrderMessage, Target: "e01", Params: map[string]any{"body": strings.Repeat("x", MaxMessageBytes+1)}}), "longer")
	rejectedWith(t, resolve(t, w, Order{EmpireID: "e00", Type: OrderMessage, Target: "e07", Params: map[string]any{"body": "ghost"}}), "no living recipient")
}

func TestSplitFleet(t *testing.T) {
	w := newTestWorld(t, 1)
	p := home(w, "e00")
	p.Ships[ShipFrigate] = 4
	p.Ships[ShipTransport] = 1
	f := formFleet(t, w, map[string]int{ShipFrigate: 4, ShipTransport: 1})
	f.Cargo = Resources{Metal: 100}
	r := resolve(t, w, Order{EmpireID: "e00", Type: OrderSplitFleet, Actor: f.ID, Params: map[string]any{"ships": map[string]any{"frigate": float64(3)}}})
	accepted(t, r, 1)
	nf := w.Fleets[fleetID(1)]
	if nf == nil || nf.Ships[ShipFrigate] != 3 || nf.SystemID != f.SystemID || f.Ships[ShipFrigate] != 1 || f.Cargo.Metal != 100 || nf.Cargo != (Resources{}) {
		t.Fatalf("split: %+v / %+v", f, nf)
	}
	if len(eventsOfType(r, "fleet_split")) != 1 {
		t.Fatal("no fleet_split event")
	}
	rejectedWith(t, resolve(t, w, Order{EmpireID: "e00", Type: OrderSplitFleet, Actor: f.ID, Params: map[string]any{"ships": map[string]any{"frigate": float64(1), "transport": float64(1)}}}), "at least one ship")
	rejectedWith(t, resolve(t, w, Order{EmpireID: "e00", Type: OrderSplitFleet, Actor: f.ID, Params: map[string]any{"ships": map[string]any{"transport": float64(1)}}}), "cannot hold")
	rejectedWith(t, resolve(t, w, Order{EmpireID: "e00", Type: OrderSplitFleet, Actor: f.ID, Params: map[string]any{"ships": map[string]any{"cruiser": float64(1)}}}), "only 0 cruiser")
	rejectedWith(t, resolve(t, w, Order{EmpireID: "e00", Type: OrderSplitFleet, Actor: f.ID}), "params.ships")
	rejectedWith(t, resolve(t, w, Order{EmpireID: "e01", Type: OrderSplitFleet, Actor: f.ID, Params: map[string]any{"ships": map[string]any{"frigate": float64(1)}}}), "not one of your fleets")
	// A fleet that launched this turn cannot also be split.
	p.Resources.Deuterium = 1000
	r = resolve(t, w,
		Order{EmpireID: "e00", Type: OrderMove, Actor: nf.ID, Target: w.Systems[nf.SystemID].Neighbors[0]},
		Order{EmpireID: "e00", Type: OrderSplitFleet, Actor: nf.ID, Params: map[string]any{"ships": map[string]any{"frigate": float64(1)}}})
	if len(r.Accepted) != 1 || len(r.Rejected) != 1 || !strings.Contains(r.Rejected[0].Reason, "mission") {
		t.Fatalf("accepted=%+v rejected=%+v", r.Accepted, r.Rejected)
	}
}

func TestCancelRefundsHalf(t *testing.T) {
	w := newTestWorld(t, 1)
	p := home(w, "e00")
	p.Resources = Resources{10000, 10000, 10000}
	accepted(t, resolve(t, w,
		Order{EmpireID: "e00", Type: OrderConstruct, Actor: p.ID, Target: "research_lab"},
		Order{EmpireID: "e00", Type: OrderBuildShips, Actor: p.ID, Target: ShipCruiser, Params: map[string]any{"quantity": 3}},
		Order{EmpireID: "e00", Type: OrderResearch, Actor: p.ID, Target: "weapons"}), 3)
	before := p.Resources
	r := resolve(t, w,
		Order{EmpireID: "e00", Type: OrderCancel, Actor: p.ID, Target: QueueConstruction},
		Order{EmpireID: "e00", Type: OrderCancel, Actor: p.ID, Target: QueueShipyard},
		Order{EmpireID: "e00", Type: OrderCancel, Actor: p.ID, Target: QueueResearch})
	accepted(t, r, 3)
	lab := scale(BuildingBase["research_lab"], 2)
	refund := Resources{lab.Metal/2 + 3*240/2 + 100/2, lab.Crystal/2 + 3*120/2 + 180/2, lab.Deuterium/2 + 3*80/2 + 40/2}
	want := before.Add(refund).Add(Production(w, p))
	if p.Resources != want || p.Construction != nil || p.ShipyardQueue != nil || w.Empires["e00"].Research != nil {
		t.Fatalf("resources %+v want %+v", p.Resources, want)
	}
	if len(eventsOfType(r, "queue_cancelled")) != 3 {
		t.Fatal("cancel events")
	}
	if p.Buildings.ResearchLab != 1 || w.Empires["e00"].Tech.Weapons != 0 {
		t.Fatal("cancelled work completed anyway")
	}
	rejectedWith(t, resolve(t, w, Order{EmpireID: "e00", Type: OrderCancel, Actor: p.ID, Target: QueueShipyard}), "no shipyard queue")
	rejectedWith(t, resolve(t, w, Order{EmpireID: "e00", Type: OrderCancel, Actor: p.ID, Target: QueueConstruction}), "no construction queue")
	rejectedWith(t, resolve(t, w, Order{EmpireID: "e00", Type: OrderCancel, Actor: p.ID, Target: QueueResearch}), "no research queue")
	rejectedWith(t, resolve(t, w, Order{EmpireID: "e00", Type: OrderCancel, Actor: p.ID, Target: "everything"}), "must be construction")
	rejectedWith(t, resolve(t, w, Order{EmpireID: "e00", Type: OrderCancel, Actor: home(w, "e01").ID, Target: QueueResearch}), "not yours")
}

func TestAttackFleet(t *testing.T) {
	w := newTestWorld(t, 1)
	p := home(w, "e00")
	p.Ships[ShipCruiser] = 5
	att := formFleet(t, w, map[string]int{ShipCruiser: 5})
	w.Fleets["fz"] = &Fleet{ID: "fz", OwnerID: "e01", SystemID: att.SystemID, Ships: Ships{ShipScout: 2}}
	w.Fleets["fy"] = &Fleet{ID: "fy", OwnerID: "e01", SystemID: att.SystemID, Ships: Ships{ShipTransport: 1}}
	r := resolve(t, w, Order{EmpireID: "e00", Type: OrderAttack, Actor: att.ID, Target: "fz"})
	accepted(t, r, 1)
	b := eventsOfType(r, "battle")
	if len(b) != 1 || b[0].Target != "fz" || b[0].Other != "e01" {
		t.Fatalf("battle %+v", r.Events)
	}
	if w.Fleets["fz"] != nil || w.Fleets["fy"] != nil {
		t.Fatal("defending fleets in the system should have been destroyed by 5 cruisers")
	}
	if p.OwnerID != "e00" || w.Systems[att.SystemID].Debris == (Resources{}) {
		t.Fatal("fleet battle must leave debris and never capture")
	}
	if _, ok := w.Hostilities[HostilityKey("e01", "e00")]; len(eventsOfType(r, "war_began")) != 1 || !ok {
		t.Fatalf("hostility %v", w.Hostilities)
	}
	if s := w.Empires["e00"].Stats; s.ShipsDestroyed != 3 || s.Battles != 1 {
		t.Fatalf("stats %+v", s)
	}
	// A second battle between the same pair does not start a new war.
	w.Fleets["fx"] = &Fleet{ID: "fx", OwnerID: "e01", SystemID: att.SystemID, Ships: Ships{ShipScout: 1}}
	r = resolve(t, w, Order{EmpireID: "e00", Type: OrderAttack, Actor: att.ID, Target: "fx"})
	if len(eventsOfType(r, "war_began")) != 0 || w.Hostilities[HostilityKey("e00", "e01")] != w.Turn-1 {
		t.Fatalf("hostility %v events %+v", w.Hostilities, r.Events)
	}
	rejectedWith(t, resolve(t, w, Order{EmpireID: "e00", Type: OrderAttack, Actor: att.ID, Target: att.ID}), "your own")
	rejectedWith(t, resolve(t, w, Order{EmpireID: "e00", Type: OrderAttack, Actor: att.ID, Target: "nothing"}), "neither a planet nor a fleet")
	rejectedWith(t, resolve(t, w, Order{EmpireID: "e00", Type: OrderAttack, Actor: att.ID, Target: w.Systems[att.SystemID].Planets[3]}), "unowned")
	rejectedWith(t, resolve(t, w, Order{EmpireID: "e00", Type: OrderAttack, Actor: att.ID, Target: p.ID}), "your own")
}

func TestAttackFleetThatLeftFindsNothing(t *testing.T) {
	w := newTestWorld(t, 1)
	p := home(w, "e00")
	p.Ships[ShipCruiser] = 1
	p.Resources.Deuterium = 1000
	att := formFleet(t, w, map[string]int{ShipCruiser: 1})
	n := w.Systems[att.SystemID].Neighbors[0]
	w.Fleets["fz"] = &Fleet{ID: "fz", OwnerID: "e01", SystemID: n, Ships: Ships{ShipScout: 1}, Cargo: Resources{Deuterium: 100}}
	away := ""
	for _, x := range w.Systems[n].Neighbors {
		if x != att.SystemID {
			away = x
		}
	}
	r := resolve(t, w,
		Order{EmpireID: "e00", Type: OrderAttack, Actor: att.ID, Target: "fz"},
		Order{EmpireID: "e01", Type: OrderMove, Actor: "fz", Target: away})
	accepted(t, r, 2)
	if len(eventsOfType(r, "battle")) != 0 || len(eventsOfType(r, "attack_target_lost")) != 1 || att.SystemID != n {
		t.Fatalf("events %+v", r.Events)
	}
}

func TestSpyReportTiers(t *testing.T) {
	setup := func(att, def int) (*World, *Fleet, *Planet) {
		w := newTestWorld(t, 1)
		f := formFleet(t, w, map[string]int{ShipScout: 1})
		target := w.Planets[w.Systems[f.SystemID].Planets[1]]
		target.OwnerID = "e01"
		target.Resources = Resources{1234, 40, 12000}
		target.Buildings = Buildings{MetalMine: 4, Shipyard: 7}
		target.Ships = Ships{ShipFrigate: 7, ShipScout: 1}
		w.Fleets["fz"] = &Fleet{ID: "fz", OwnerID: "e01", SystemID: target.SystemID, Ships: Ships{ShipFrigate: 1}}
		w.Empires["e00"].Tech.Sensors = att
		w.Empires["e01"].Tech.Sensors = def
		w.Empires["e01"].Tech.Weapons = 5
		return w, f, target
	}
	report := func(w *World, f *Fleet, target *Planet) *SpyReport {
		r := resolve(t, w, Order{EmpireID: "e00", Type: OrderSpy, Actor: f.ID, Target: target.ID})
		ev := eventsOfType(r, "espionage")
		if len(ev) != 1 || ev[0].Report == nil {
			t.Fatalf("events %+v rejected %+v", r.Events, r.Rejected)
		}
		return ev[0].Report
	}
	w, f, p := setup(0, 2)
	r := report(w, f, p)
	if r.Tier != 0 || r.Owner != "e01" || r.Activity != "armed" || r.ResourceBands != nil || r.Resources != nil || r.Ships != nil || r.ObservedTurn != 1 {
		t.Fatalf("tier 0: %+v", r)
	}
	w, f, p = setup(0, 0)
	r = report(w, f, p)
	if r.Tier != 1 || r.ResourceBands["metal"] != "500-1999" || r.ResourceBands["deuterium"] != "10000+" || r.BuildingBands["shipyard"] != "6+" ||
		r.BuildingBands["metal_mine"] != "3-5" || r.FleetSize != "5-19" || r.Resources != nil || r.Ships != nil {
		t.Fatalf("tier 1: %+v", r)
	}
	w, f, p = setup(2, 1)
	r = report(w, f, p)
	if r.Tier != 2 || r.Resources.Metal != 1234 || r.Buildings.Shipyard != 7 || r.Ships[ShipFrigate] != 10 || r.Ships[ShipScout] != 0 || r.Tech != nil {
		t.Fatalf("tier 2: %+v", r)
	}
	w, f, p = setup(3, 0)
	r = report(w, f, p)
	if r.Tier != 3 || r.Ships[ShipFrigate] != 8 || r.Ships[ShipScout] != 1 || r.Tech == nil || r.Tech.Weapons != 5 || r.Confidence != "exact" {
		t.Fatalf("tier 3: %+v", r)
	}
}

func TestSpyValidationAndDetection(t *testing.T) {
	w := newTestWorld(t, 1)
	p := home(w, "e00")
	p.Ships[ShipFrigate] = 1
	frig := formFleet(t, w, map[string]int{ShipFrigate: 1})
	scout := formFleet(t, w, map[string]int{ShipScout: 1})
	near := w.Planets[w.Systems[w.Systems[p.SystemID].Neighbors[0]].Planets[0]]
	rejectedWith(t, resolve(t, w, Order{EmpireID: "e00", Type: OrderSpy, Actor: frig.ID, Target: near.ID}), "needs a scout")
	rejectedWith(t, resolve(t, w, Order{EmpireID: "e00", Type: OrderSpy, Actor: scout.ID, Target: p.ID}), "your own")
	rejectedWith(t, resolve(t, w, Order{EmpireID: "e00", Type: OrderSpy, Actor: scout.ID, Target: home(w, "e03").ID}), "beyond spy range 1")
	near.OwnerID = "e02"
	r := resolve(t, w, Order{EmpireID: "e00", Type: OrderSpy, Actor: scout.ID, Target: near.ID})
	accepted(t, r, 1)
	if scout.SystemID != p.SystemID || len(eventsOfType(r, "espionage")) != 1 {
		t.Fatal("a scout spies on an adjacent system without moving")
	}
	if SpyRange(0) != 1 || SpyRange(2) != 1 || SpyRange(3) != 2 || SpyRange(7) != 3 {
		t.Fatal("spy range")
	}
	if DetectionChance(-10) != 95 || DetectionChance(10) != 5 || DetectionChance(0) != 50 || DetectionChance(1) != 35 {
		t.Fatal("detection chance")
	}
	// Detection is seeded: over many turns at 50% both outcomes occur and the
	// detected empire learns who spied.
	detected := 0
	for i := 0; i < 30; i++ {
		r := resolve(t, w, Order{EmpireID: "e00", Type: OrderSpy, Actor: scout.ID, Target: near.ID})
		for _, e := range eventsOfType(r, "spy_detected") {
			if e.EmpireID != "e02" || e.Other != "e00" {
				t.Fatalf("detection event %+v", e)
			}
			detected++
		}
	}
	if detected == 0 || detected == 30 || w.Empires["e00"].Stats.Detected != detected {
		t.Fatalf("detected %d/30 stats %d", detected, w.Empires["e00"].Stats.Detected)
	}
}

func TestRecyclersShareDebrisProportionally(t *testing.T) {
	w := newTestWorld(t, 1)
	p := home(w, "e00")
	sys := p.SystemID
	w.Systems[sys].Debris = Resources{Metal: 150, Crystal: 151}
	w.Fleets["fa"] = &Fleet{ID: "fa", OwnerID: "e00", SystemID: sys, Ships: Ships{ShipRecycler: 2}}
	w.Fleets["fb"] = &Fleet{ID: "fb", OwnerID: "e01", SystemID: sys, Ships: Ships{ShipRecycler: 1}, Cargo: Resources{Deuterium: 10}}
	r := resolve(t, w,
		Order{EmpireID: "e00", Type: OrderRecycle, Actor: "fa", Target: sys},
		Order{EmpireID: "e01", Type: OrderRecycle, Actor: "fb", Target: sys})
	accepted(t, r, 2)
	a, b := w.Fleets["fa"].Cargo, w.Fleets["fb"].Cargo
	ga, gb := a.Metal+a.Crystal, b.Metal+b.Crystal
	// capacities 400 and 190 share 301 debris: 204.08 and 96.9 -> 204/96 + 1 remainder.
	if ga+gb != 301 || ga < 204 || ga > 205 || gb < 96 || gb > 97 || w.Systems[sys].Debris != (Resources{}) {
		t.Fatalf("shares %d/%d debris %+v", ga, gb, w.Systems[sys].Debris)
	}
	if len(eventsOfType(r, "debris_collected")) != 2 {
		t.Fatal("collection events")
	}
	p.Ships[ShipTransport] = 1
	tr := formFleet(t, w, map[string]int{ShipTransport: 1})
	rejectedWith(t, resolve(t, w, Order{EmpireID: "e00", Type: OrderRecycle, Actor: tr.ID, Target: sys}), "needs a recycler")
}

func TestRejectedOrdersCarryReasonsToTheEmpire(t *testing.T) {
	w := newTestWorld(t, 1)
	resolve(t, w,
		Order{EmpireID: "e00", Type: OrderConstruct, Actor: home(w, "e00").ID, Target: "casino"},
		Order{EmpireID: "e01", Type: OrderMessage, Target: "e02", Params: map[string]any{"body": "hi"}})
	rj := w.Empires["e00"].Rejected
	if len(rj) != 1 || !strings.Contains(rj[0].Reason, "unknown building") || rj[0].Target != "casino" {
		t.Fatalf("rejected %+v", rj)
	}
	if len(w.Empires["e01"].Rejected) != 0 {
		t.Fatal("accepted order reported as rejected")
	}
	// A spoofed order is reported to the submitter, not to the spoofed empire.
	r, _ := ResolveTurn(w, map[string][]Order{"e03": {{EmpireID: "e04", Type: OrderMessage, Target: "e01"}}})
	if len(r.Rejected) != 1 || !strings.Contains(r.Rejected[0].Reason, "does not match") || len(w.Empires["e03"].Rejected) != 1 || len(w.Empires["e04"].Rejected) != 0 {
		t.Fatalf("spoof %+v", r.Rejected)
	}
	if len(w.Empires["e00"].Rejected) != 0 {
		t.Fatal("rejections must only describe the latest turn")
	}
}

func TestEliminationLeavesAllianceAndEmitsEvents(t *testing.T) {
	w := newTestWorld(t, 1)
	ally(t, w, "e00", "e01")
	home(w, "e01").OwnerID = ""
	r := resolve(t, w)
	if ev := eventsOfType(r, "eliminated"); len(ev) != 1 || ev[0].EmpireID != "e01" {
		t.Fatalf("events %+v", r.Events)
	}
	if w.Empires["e01"].AllianceID != "" || strings.Join(w.Alliances["a001"].Members, ",") != "e00" || w.Empires["e01"].Stats.EliminatedTurn != 3 {
		t.Fatalf("eliminated empire %+v", w.Empires["e01"])
	}
	rejectedWith(t, resolve(t, w, Order{EmpireID: "e01", Type: OrderMessage, Target: "e00", Params: map[string]any{"body": "x"}}), "eliminated")
}

func TestQueueCompletionEvents(t *testing.T) {
	w := newTestWorld(t, 1)
	p := home(w, "e00")
	p.Resources = Resources{10000, 10000, 10000}
	r := resolve(t, w,
		Order{EmpireID: "e00", Type: OrderConstruct, Actor: p.ID, Target: "metal_mine"},
		Order{EmpireID: "e00", Type: OrderBuildShips, Actor: p.ID, Target: ShipScout, Params: map[string]any{"quantity": 1}},
		Order{EmpireID: "e00", Type: OrderResearch, Actor: p.ID, Target: "sensors"})
	kinds := map[string]int{}
	for i := 0; i < 5; i++ {
		for _, e := range r.Events {
			kinds[e.Type]++
		}
		r = resolve(t, w)
	}
	if kinds["queued"] != 3 {
		t.Fatalf("events %v", kinds)
	}
	if kinds["construction_complete"] != 1 || kinds["ships_built"] != 1 || kinds["research_complete"] != 1 || w.Empires["e00"].Stats.ShipsBuilt != 1 {
		t.Fatalf("events %v", kinds)
	}
}

func TestPlanetlessEmpireResearchWaits(t *testing.T) {
	w := newTestWorld(t, 1)
	w.Empires["e01"].Research = &Queue{Kind: "sensors", Level: 1, Required: 3}
	home(w, "e01").OwnerID = ""
	r := resolve(t, w)
	if w.Empires["e01"].Research.Progress != 0 || len(eventsOfType(r, "research_complete")) != 0 {
		t.Fatalf("research progressed without planets: %+v", w.Empires["e01"].Research)
	}
}

func TestProductionAndBattleDebrisAreLogged(t *testing.T) {
	w := newTestWorld(t, 1)
	want := Production(w, home(w, "e03"))
	r := resolve(t, w)
	found := false
	for _, e := range eventsOfType(r, "production") {
		if e.EmpireID == "e03" {
			found = e.Detail == fmt.Sprintf("metal=%d crystal=%d deuterium=%d", want.Metal, want.Crystal, want.Deuterium)
		}
	}
	if !found || len(eventsOfType(r, "production")) != 8 {
		t.Fatalf("production events %+v", eventsOfType(r, "production"))
	}
	f, target := stageAttack(t, w, map[string]int{ShipFrigate: 3})
	target.Ships = Ships{ShipFrigate: 1}
	r = resolve(t, w, Order{EmpireID: "e00", Type: OrderAttack, Actor: f.ID, Target: target.ID})
	for f.Route != nil {
		r = resolve(t, w)
	}
	b := eventsOfType(r, "battle")
	d := w.Systems[target.SystemID].Debris
	if len(b) != 1 || !strings.Contains(b[0].Detail, fmt.Sprintf("debris=%d/%d at %s", d.Metal, d.Crystal, target.SystemID)) || d == (Resources{}) {
		t.Fatalf("battle %+v debris %+v", b, d)
	}
}

func TestSensorsExtendSpyRange(t *testing.T) {
	w := newTestWorld(t, 1)
	scout := formFleet(t, w, map[string]int{ShipScout: 1})
	var two *Planet
	for _, id := range systemIDs(w) {
		if distance(w, scout.SystemID, id) == 2 {
			two = w.Planets[w.Systems[id].Planets[3]]
			break
		}
	}
	two.OwnerID = "e05"
	rejectedWith(t, resolve(t, w, Order{EmpireID: "e00", Type: OrderSpy, Actor: scout.ID, Target: two.ID}), "beyond spy range 1")
	w.Empires["e00"].Tech.Sensors = 3
	r := resolve(t, w, Order{EmpireID: "e00", Type: OrderSpy, Actor: scout.ID, Target: two.ID})
	if len(eventsOfType(r, "espionage")) != 1 {
		t.Fatalf("sensors 3 cannot spy two edges away: %+v", r.Rejected)
	}
}

func TestTransportToAFleetAidsAnExile(t *testing.T) {
	w := newTestWorld(t, 1)
	p := home(w, "e00")
	p.Resources = Resources{1000, 1000, 1000}
	f := formFleet(t, w, map[string]int{ShipTransport: 1})
	w.Fleets["fark"] = &Fleet{ID: "fark", OwnerID: "e01", SystemID: f.SystemID, Ships: Ships{ShipColonyArk: 1}, Cargo: Resources{Metal: 20}}
	r := resolve(t, w, Order{EmpireID: "e00", Type: OrderTransport, Actor: f.ID, Target: "fark", Params: map[string]any{"metal": 150, "deuterium": 60}})
	accepted(t, r, 1)
	// The ark holds 100 and carries 20: fuel first (60), then 20 metal.
	if got := w.Fleets["fark"].Cargo; got != (Resources{Metal: 40, Deuterium: 60}) || f.Cargo != (Resources{Metal: 130}) {
		t.Fatalf("ark %+v transport %+v", got, f.Cargo)
	}
	if tr := eventsOfType(r, "transfer"); len(tr) != 1 || tr[0].Other != "e01" || w.Empires["e01"].Stats.ResourcesReceived != 80 {
		t.Fatalf("transfer %+v", tr)
	}
	rejectedWith(t, resolve(t, w, Order{EmpireID: "e00", Type: OrderTransport, Actor: f.ID, Target: f.ID}), "itself")
	rejectedWith(t, resolve(t, w, Order{EmpireID: "e00", Type: OrderTransport, Actor: f.ID, Target: "zzz"}), "unknown planet, fleet or system")
}

func TestFleetCanBeRedirectedAtARouteNode(t *testing.T) {
	w := newTestWorld(t, 4)
	p := home(w, "e00")
	f := formFleet(t, w, map[string]int{ShipScout: 1})
	p.Resources.Deuterium = 1000
	var far string
	for _, id := range systemIDs(w) {
		if distance(w, f.SystemID, id) >= 3 {
			far = id
			break
		}
	}
	accepted(t, resolve(t, w, Order{EmpireID: "e00", Type: OrderMove, Actor: f.ID, Target: far}), 1)
	if len(f.Route) == 0 || f.RouteIndex != 1 {
		t.Fatal("fleet should be in transit at a route node")
	}
	f.Cargo.Deuterium = 50
	r := resolve(t, w,
		Order{EmpireID: "e00", Type: OrderMove, Actor: f.ID, Target: p.SystemID},
		Order{EmpireID: "e00", Type: OrderMove, Actor: f.ID, Target: far})
	if len(r.Accepted) != 1 || len(r.Rejected) != 1 || !strings.Contains(r.Rejected[0].Reason, "already has a move mission") {
		t.Fatalf("redirect: accepted=%+v rejected=%+v", r.Accepted, r.Rejected)
	}
	for i := 0; i < 10 && len(f.Route) > 0; i++ {
		resolve(t, w)
	}
	if f.SystemID != p.SystemID || f.Cargo.Deuterium != 50-ShipSpecs[ShipScout].Fuel {
		t.Fatalf("fleet at %s cargo %+v, want home after paying one edge from cargo", f.SystemID, f.Cargo)
	}
	rejectedWith(t, resolve(t, w, Order{EmpireID: "e00", Type: OrderMove, Actor: f.ID, Target: f.SystemID}), "already in")
}

func TestRejectionFeedbackIsBounded(t *testing.T) {
	w := newTestWorld(t, 1)
	var orders []Order
	for i := 0; i < MaxFeedbackRejections+10; i++ {
		orders = append(orders, Order{EmpireID: "e00", Type: OrderConstruct, Actor: strings.Repeat("x", 1000), Target: "casino",
			Params: map[string]any{"junk": strings.Repeat("y", 2000)}})
	}
	r := resolve(t, w, orders...)
	if len(r.Rejected) != len(orders) || len(r.Rejected[0].Params["junk"].(string)) != 2000 {
		t.Fatal("the turn result must keep the full submission")
	}
	rj := w.Empires["e00"].Rejected
	if len(rj) != MaxFeedbackRejections || rj[0].Params["junk"] != nil || len(rj[0].Actor) > 70 || len(rj[0].Reason) > 310 || !strings.Contains(rj[0].Reason, "unknown planet") {
		t.Fatalf("feedback %d %+v", len(rj), rj[0])
	}
}

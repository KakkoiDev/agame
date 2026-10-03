package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/KakkoiDev/agame/world"
)

func mustResolve(t *testing.T, w *world.World, orders ...world.Order) world.TurnResult {
	t.Helper()
	sub := map[string][]world.Order{}
	for _, o := range orders {
		sub[o.EmpireID] = append(sub[o.EmpireID], o)
	}
	r, err := world.ResolveTurn(w, sub)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestObserveDiplomacyTiming(t *testing.T) {
	w := genWorld(t, 3)
	mustResolve(t, w,
		world.Order{EmpireID: "e00", Type: world.OrderAllianceCreate, Params: map[string]any{"name": "Pact", "invite": []any{"e01"}}},
		world.Order{EmpireID: "e02", Type: world.OrderMessage, Target: "all", Params: map[string]any{"body": "hello all"}},
		world.Order{EmpireID: "e03", Type: world.OrderConstruct, Actor: "nope", Target: "metal_mine"})
	o1, o3, o5 := Observe(w, "e01"), Observe(w, "e03"), Observe(w, "e05")
	if len(o1.Invitations) != 1 || o1.Invitations[0] != "a001" || len(o5.Invitations) != 0 {
		t.Fatalf("invitations %v %v", o1.Invitations, o5.Invitations)
	}
	if len(o5.Alliances) != 1 || o5.Alliances[0].Members[0] != "e00" || o5.Alliances[0].Invited != nil {
		t.Fatalf("outsiders see members but not invitations: %+v", o5.Alliances)
	}
	if o0 := Observe(w, "e00"); len(o0.Alliances[0].Invited) != 1 {
		t.Fatalf("members see invitations: %+v", o0.Alliances)
	}
	if len(o5.Messages) != 1 || o5.Messages[0].Body != "hello all" || len(Observe(w, "e02").Messages) != 0 {
		t.Fatalf("broadcast delivery %+v", o5.Messages)
	}
	if len(o3.Rejected) != 1 || !strings.Contains(o3.Rejected[0].Reason, "unknown planet") || len(o5.Rejected) != 0 {
		t.Fatalf("rejections %+v", o3.Rejected)
	}
	seen := func(o Observation, typ string) bool {
		for _, e := range o.Events {
			if e.Type == typ {
				return true
			}
		}
		return false
	}
	if !seen(o5, "alliance_created") || seen(o5, "alliance_invited") || !seen(o1, "alliance_invited") || seen(o5, "message_sent") {
		t.Fatalf("event visibility: e05=%+v e01=%+v", o5.Events, o1.Events)
	}
	if len(o5.Rulers) != 8 || o5.Rulers[0].Alliance != "a001" || o5.Rulers[0].Status != "sovereign" {
		t.Fatalf("rulers %+v", o5.Rulers)
	}
	// One turn later the messages and events are gone (they were delivered).
	mustResolve(t, w)
	if o := Observe(w, "e05"); len(o.Messages) != 0 || seen(o, "alliance_created") || len(o.Rejected) != 0 {
		t.Fatalf("stale diplomacy %+v", o)
	}
}

func TestObserveEspionageAndBattlesArePrivate(t *testing.T) {
	w := genWorld(t, 3)
	h := w.Planets[w.Empires["e00"].HomeworldID]
	target := w.Planets[w.Systems[h.SystemID].Planets[1]]
	target.OwnerID = "e01"
	w.Fleets["fs"] = &world.Fleet{ID: "fs", OwnerID: "e00", SystemID: h.SystemID, Ships: world.Ships{"scout": 1}}
	w.Fleets["fa"] = &world.Fleet{ID: "fa", OwnerID: "e00", SystemID: h.SystemID, Ships: world.Ships{"cruiser": 3}}
	w.Fleets["fz"] = &world.Fleet{ID: "fz", OwnerID: "e02", SystemID: h.SystemID, Ships: world.Ships{"scout": 1}}
	w.Empires["e00"].Tech.Sensors = 9
	mustResolve(t, w,
		world.Order{EmpireID: "e00", Type: world.OrderSpy, Actor: "fs", Target: target.ID},
		world.Order{EmpireID: "e00", Type: world.OrderAttack, Actor: "fa", Target: "fz"})
	types := func(eid string) string {
		var ts []string
		for _, e := range Observe(w, eid).Events {
			ts = append(ts, e.Type)
		}
		return strings.Join(ts, ",")
	}
	if got := types("e00"); !strings.Contains(got, "espionage") || !strings.Contains(got, "battle") || !strings.Contains(got, "fleet_departed") {
		t.Fatalf("spy/attacker events %s", got)
	}
	if got := types("e01"); strings.Contains(got, "espionage") || strings.Contains(got, "battle") {
		t.Fatalf("spied-on empire saw %s", got)
	}
	if got := types("e02"); !strings.Contains(got, "battle") || strings.Contains(got, "fleet_departed") || strings.Contains(got, "espionage") {
		t.Fatalf("defender saw %s", got)
	}
	if got := types("e05"); got != "" {
		t.Fatalf("bystander saw %s", got)
	}
	o := Observe(w, "e00")
	if o.Hostilities["e02"] != 0 || len(o.Hostilities) != 1 || Observe(w, "e02").Hostilities["e00"] != 0 {
		t.Fatalf("hostilities %v", o.Hostilities)
	}
	for _, e := range o.Events {
		if e.Report != nil {
			e.Report.Owner = "hacked"
		}
	}
	for _, e := range Observe(w, "e00").Events {
		if e.Report != nil && e.Report.Owner != "e01" {
			t.Fatal("spy report aliases the world")
		}
	}
}

func TestObserveHidesDetectionFromTheSpy(t *testing.T) {
	w := genWorld(t, 3)
	w.Empires["e00"].Stats.Detected = 4
	if Observe(w, "e00").Empire.Stats.Detected != 0 || w.Empires["e00"].Stats.Detected != 4 {
		t.Fatal("detection count leaked or world mutated")
	}
}

func TestPromptRendersDiplomacy(t *testing.T) {
	w := genWorld(t, 3)
	mustResolve(t, w,
		world.Order{EmpireID: "e01", Type: world.OrderAllianceCreate, Params: map[string]any{"name": "Iron Pact", "invite": []any{"e00"}}},
		world.Order{EmpireID: "e01", Type: world.OrderMessage, Target: "e00", Params: map[string]any{"body": "join us", "major": true}},
		world.Order{EmpireID: "e00", Type: world.OrderResearch, Actor: "nowhere", Target: "industry"})
	p := Prompt(Observe(w, "e00"))
	for _, want := range []string{"invited to join: a001", `a001 "Iron Pact" members=e01`, "from e01 to e00 MAJOR: \"join us\"",
		"your orders rejected last turn:", "unknown planet", "alliance_created e01 a001", "other rulers: e01=B(sovereign)"} {
		if !strings.Contains(p, want) {
			t.Fatalf("prompt lacks %q:\n%s", want, p)
		}
	}
}

// TestAutopilotAgentSeesOnlyItsObservation plays a game where every ruler
// decides from its observation and checks the decisions match the autopilot
// run on the full world: the autopilot needs no hidden information.
func TestAutopilotAgentSeesOnlyItsObservation(t *testing.T) {
	w := genWorld(t, 11)
	for turn := 0; turn < 60; turn++ {
		sub := map[string][]world.Order{}
		for _, id := range sortedIDs(w.Empires) {
			full := Autopilot(w, id)
			d, err := AutopilotAgent{}.Decide(context.Background(), Observe(w, id))
			if err != nil {
				t.Fatal(err)
			}
			a, _ := json.Marshal(full)
			b, _ := json.Marshal(d)
			if string(a) != string(b) {
				t.Fatalf("turn %d %s: full-world %s vs observation %s", turn, id, a, b)
			}
			for i := range d.Orders {
				d.Orders[i].EmpireID = id
			}
			sub[id] = d.Orders
		}
		if _, err := world.ResolveTurn(w, sub); err != nil {
			t.Fatal(err)
		}
	}
	if len(w.Alliances) == 0 {
		t.Fatal("autopilot diplomacy formed no alliance")
	}
}

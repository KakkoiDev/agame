package engine

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/KakkoiDev/agame/agent"
	"github.com/KakkoiDev/agame/world"
)

var names = []string{"A", "B", "C", "D", "E", "F", "G", "H"}

type agentFunc func(context.Context, agent.Observation) (agent.Decision, error)

func (f agentFunc) Decide(ctx context.Context, o agent.Observation) (agent.Decision, error) {
	return f(ctx, o)
}

func newWorld(t *testing.T, seed int64) *world.World {
	t.Helper()
	w, err := world.Generate(seed, names)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func TestRunnerForcesEmpireIDAndSkipsFailures(t *testing.T) {
	w := newWorld(t, 1)
	var mu sync.Mutex
	var order []string
	rec := func(id string, d agent.Decision, err error) agent.Agent {
		return agentFunc(func(_ context.Context, o agent.Observation) (agent.Decision, error) {
			mu.Lock()
			order = append(order, o.Empire.ID)
			mu.Unlock()
			if o.Empire.ID != id {
				t.Errorf("agent for %s observed %s", id, o.Empire.ID)
			}
			return d, err
		})
	}
	spoof := agent.Decision{Orders: []world.Order{{EmpireID: "e05", Type: "message", Target: "e01", Params: map[string]any{"body": "x"}}}}
	w.Empires["e04"].Eliminated = true
	r := Runner{World: w, Agents: map[string]agent.Agent{
		"e02": rec("e02", spoof, nil),
		"e00": rec("e00", agent.Decision{Orders: []world.Order{{Type: "message", Target: "e01"}}}, errors.New("model timeout")),
		"e04": rec("e04", spoof, nil),
		"e01": nil,
	}}
	res, err := r.Turn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 || order[0] != "e00" || order[1] != "e02" {
		t.Fatalf("invocation order %v (eliminated/nil agents must be skipped, others sorted)", order)
	}
	if len(res.Accepted) != 1 || res.Accepted[0].EmpireID != "e02" {
		t.Fatalf("accepted %+v; spoofed EmpireID must be rewritten to the deciding empire", res.Accepted)
	}
	if len(w.Messages) != 1 || w.Messages[0].From != "e02" {
		t.Fatalf("messages %+v", w.Messages)
	}
	if w.Turn != 1 {
		t.Fatalf("turn %d", w.Turn)
	}
}

func TestRunnerAgentsShareOneSnapshot(t *testing.T) {
	// Every ruler must decide from the same S(t), regardless of invocation
	// order and of what earlier agents do to their observations.
	w := newWorld(t, 2)
	var seen []int
	a := map[string]agent.Agent{}
	for id := range w.Empires {
		a[id] = agentFunc(func(_ context.Context, o agent.Observation) (agent.Decision, error) {
			seen = append(seen, o.Planets[0].Resources.Metal)
			o.Planets[0].Resources.Metal = 0 // must not leak into the world
			return agent.Decision{Orders: []world.Order{{Type: "construct", Actor: o.Planets[0].ID, Target: "metal_mine"}}}, nil
		})
	}
	res, err := (&Runner{World: w, Agents: a}).Turn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range seen {
		if m != 500 {
			t.Fatalf("an agent observed metal=%d, want the turn-start 500", m)
		}
	}
	if len(res.Accepted) != 8 {
		t.Fatalf("accepted %d/8", len(res.Accepted))
	}
}

func autopilotAgents(get func() *world.World) map[string]agent.Agent {
	m := map[string]agent.Agent{}
	for _, id := range []string{"e00", "e01", "e02", "e03", "e04", "e05", "e06", "e07"} {
		id := id
		m[id] = agentFunc(func(context.Context, agent.Observation) (agent.Decision, error) {
			return agent.Autopilot(get(), id), nil
		})
	}
	return m
}

func TestLongRunIsReproducibleAcrossJSONRoundTrips(t *testing.T) {
	// The browser serialises the world to JSON between every turn; resolving
	// from a round-tripped world must give exactly the same history.
	const turns = 80
	direct := &Runner{World: newWorld(t, 77)}
	direct.Agents = autopilotAgents(func() *world.World { return direct.World })
	trip := &Runner{World: newWorld(t, 77)}
	trip.Agents = autopilotAgents(func() *world.World { return trip.World })
	for i := 0; i < turns; i++ {
		ra, err := direct.Turn(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(trip.World)
		var w world.World
		if err := json.Unmarshal(b, &w); err != nil {
			t.Fatal(err)
		}
		trip.World = &w
		rb, err := trip.Turn(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		ja, _ := json.Marshal(ra)
		jb, _ := json.Marshal(rb)
		if string(ja) != string(jb) {
			t.Fatalf("turn %d results diverged:\n%s\n%s", i, ja, jb)
		}
	}
	ja, _ := json.Marshal(direct.World)
	jb, _ := json.Marshal(trip.World)
	if string(ja) != string(jb) {
		t.Fatal("worlds diverged")
	}
	if direct.World.Turn != turns {
		t.Fatalf("turn=%d", direct.World.Turn)
	}
}

func TestLongRunInvariants(t *testing.T) {
	for _, seed := range []int64{1, 2, 3} {
		r := &Runner{World: newWorld(t, seed)}
		r.Agents = autopilotAgents(func() *world.World { return r.World })
		for i := 0; i < 120; i++ {
			if _, err := r.Turn(context.Background()); err != nil {
				t.Fatal(err)
			}
			w := r.World
			for id, p := range w.Planets {
				if p.Resources.Metal < 0 || p.Resources.Crystal < 0 || p.Resources.Deuterium < 0 {
					t.Fatalf("seed %d turn %d: %s resources %+v", seed, w.Turn, id, p.Resources)
				}
				if p.OwnerID != "" && w.Empires[p.OwnerID] == nil {
					t.Fatalf("planet %s owned by unknown %s", id, p.OwnerID)
				}
				for k, n := range p.Ships {
					if n < 0 {
						t.Fatalf("planet %s has %d %s", id, n, k)
					}
				}
			}
			for id, f := range w.Fleets {
				if w.Systems[f.SystemID] == nil || w.Empires[f.OwnerID] == nil {
					t.Fatalf("fleet %s in %q owned by %q", id, f.SystemID, f.OwnerID)
				}
				for k, n := range f.Ships {
					if n < 0 || world.ShipSpecs[k].Hull == 0 {
						t.Fatalf("fleet %s has %d %q", id, n, k)
					}
				}
			}
		}
	}
}

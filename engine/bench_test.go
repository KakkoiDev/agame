package engine

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/KakkoiDev/agame/agent"
	"github.com/KakkoiDev/agame/world"
)

// scripted is a Repairer that returns its outputs in sequence and records
// the problems it was told about.
type scripted struct {
	outputs  []func(agent.Observation) (agent.Decision, error)
	problems []string
	calls    int
}

func (s *scripted) next(o agent.Observation) (agent.Decision, error) {
	f := s.outputs[min(s.calls, len(s.outputs)-1)]
	s.calls++
	return f(o)
}
func (s *scripted) Decide(_ context.Context, o agent.Observation) (agent.Decision, error) {
	return s.next(o)
}
func (s *scripted) Repair(_ context.Context, o agent.Observation, prev agent.Decision, problem string) (agent.Decision, error) {
	s.problems = append(s.problems, problem)
	return s.next(o)
}

func malformed(agent.Observation) (agent.Decision, error) {
	return agent.Decision{Raw: "I think I will build"}, &agent.MalformedError{Raw: "I think I will build", Err: errors.New("no JSON object")}
}
func badOrder(agent.Observation) (agent.Decision, error) {
	return agent.Decision{Raw: `{"orders":[{"type":"construct","actor":"nowhere"}]}`, Orders: []world.Order{{Type: "construct", Actor: "nowhere", Target: "metal_mine"}}}, nil
}
func goodOrder(o agent.Observation) (agent.Decision, error) {
	return agent.Decision{Orders: []world.Order{{Type: "construct", Actor: o.Planets[0].ID, Target: "metal_mine"}}, Statement: "mines"}, nil
}

func runOne(t *testing.T, a agent.Agent, b Budget) (DecisionRecord, TurnRecord) {
	t.Helper()
	r := &Runner{World: newWorld(t, 1), Agents: map[string]agent.Agent{"e00": a}, Budget: b}
	rec, err := r.Step(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Decisions) != 1 {
		t.Fatalf("decisions %+v", rec.Decisions)
	}
	return rec.Decisions[0], rec
}

func TestRepairLoopFixesMalformedOutput(t *testing.T) {
	s := &scripted{outputs: []func(agent.Observation) (agent.Decision, error){malformed, badOrder, goodOrder}}
	d, rec := runOne(t, s, Budget{Repairs: DefaultRepairs})
	if d.Repairs != 2 || len(d.Attempts) != 3 || d.Failure != "" || d.Accepted != 1 || len(d.Orders) != 1 || d.Statement != "mines" {
		t.Fatalf("record %+v", d)
	}
	if len(s.problems) != 2 || !strings.Contains(s.problems[0], "no JSON") || !strings.Contains(s.problems[1], `unknown planet "nowhere"`) {
		t.Fatalf("problems fed back: %q", s.problems)
	}
	if d.Attempts[0].Raw != "I think I will build" || d.Attempts[0].Problem == "" || d.Attempts[2].Problem != "" {
		t.Fatalf("attempts %+v", d.Attempts)
	}
	if len(rec.Result.Accepted) != 1 || d.PromptHash == "" || d.Agent != "*engine.scripted" {
		t.Fatalf("result %+v", rec.Result)
	}
}

func TestRepairBudgetExhaustedMeansZeroOrders(t *testing.T) {
	s := &scripted{outputs: []func(agent.Observation) (agent.Decision, error){malformed}}
	d, _ := runOne(t, s, Budget{Repairs: DefaultRepairs})
	if s.calls != 3 || d.Repairs != 2 || d.Failure != "malformed" || len(d.Orders) != 0 {
		t.Fatalf("calls=%d record %+v", s.calls, d)
	}
	var ops Ops
	ops.Add(d)
	if ops.Invalid != 3 || ops.Repairs != 2 || ops.ZeroOrders != 1 {
		t.Fatalf("ops %+v", ops)
	}
}

func TestInvalidOrdersAfterRepairsAreRejectedWithReasons(t *testing.T) {
	s := &scripted{outputs: []func(agent.Observation) (agent.Decision, error){badOrder}}
	d, rec := runOne(t, s, Budget{Repairs: 1})
	if s.calls != 2 || d.Failure != "" || len(d.Orders) != 1 || d.Accepted != 0 || len(d.Rejected) != 1 {
		t.Fatalf("record %+v", d)
	}
	if !strings.Contains(d.Rejected[0].Reason, "unknown planet") || len(rec.Result.Rejected) != 1 {
		t.Fatalf("rejections %+v", d.Rejected)
	}
}

func TestNoRepairWithoutRepairerOrBudget(t *testing.T) {
	s := &scripted{outputs: []func(agent.Observation) (agent.Decision, error){malformed}}
	d, _ := runOne(t, s, Budget{})
	if s.calls != 1 || d.Failure != "malformed" {
		t.Fatalf("calls=%d %+v", s.calls, d)
	}
	plain := agentFunc(func(context.Context, agent.Observation) (agent.Decision, error) {
		return malformed(agent.Observation{})
	})
	d, _ = runOne(t, plain, Budget{Repairs: 2})
	if d.Repairs != 0 || d.Failure != "malformed" {
		t.Fatalf("%+v", d)
	}
	failing := agentFunc(func(context.Context, agent.Observation) (agent.Decision, error) {
		return agent.Decision{}, errors.New("connection refused")
	})
	d, _ = runOne(t, failing, Budget{Repairs: 2})
	if d.Repairs != 0 || d.Failure != "error" || d.Attempts[0].Error != "connection refused" {
		t.Fatalf("%+v", d)
	}
}

func TestTimeoutBoundsAStuckRuler(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	stuck := agentFunc(func(context.Context, agent.Observation) (agent.Decision, error) { // ignores its context
		<-block
		return agent.Decision{}, nil
	})
	start := time.Now()
	d, _ := runOne(t, stuck, Budget{Timeout: 50 * time.Millisecond})
	if d.Failure != "timeout" || len(d.Orders) != 0 || time.Since(start) > 5*time.Second {
		t.Fatalf("%+v", d)
	}
}

func TestLatencyUsesTheRunnerClock(t *testing.T) {
	now := time.Unix(0, 0)
	clock := func() time.Time { now = now.Add(1500 * time.Millisecond); return now }
	r := &Runner{World: newWorld(t, 1), Agents: map[string]agent.Agent{"e00": agentFunc(func(_ context.Context, o agent.Observation) (agent.Decision, error) { return goodOrder(o) })}, Now: clock}
	rec, err := r.Step(context.Background())
	if err != nil || rec.Decisions[0].LatencyMS != 1500 {
		t.Fatalf("%+v %v", rec.Decisions, err)
	}
}

func TestEndConditions(t *testing.T) {
	r := &Runner{World: newWorld(t, 1), Budget: Budget{TurnLimit: 2}}
	rec, err := r.Step(context.Background())
	if err != nil || rec.End != nil {
		t.Fatalf("turn 1 ended early: %+v %v", rec.End, err)
	}
	rec, err = r.Step(context.Background())
	if err != nil || rec.End == nil || rec.End.Reason != world.EndTurnLimit || rec.End.Turn != 2 || len(rec.End.Survivors) != 8 {
		t.Fatalf("end %+v %v", rec.End, err)
	}
	if _, err := r.Step(context.Background()); err == nil {
		t.Fatal("an ended run advanced")
	}

	w := newWorld(t, 1)
	for id, e := range w.Empires {
		if id != "e03" {
			w.Planets[e.HomeworldID].OwnerID = ""
		}
	}
	r = &Runner{World: w}
	rec, _ = r.Step(context.Background())
	if rec.End == nil || rec.End.Reason != world.EndLastStanding || rec.End.Survivors[0] != "e03" {
		t.Fatalf("end %+v", rec.End)
	}
	st := world.Standings(w)
	if st[0].Empire != "e03" || st[0].Rank != 1 || st[1].Status != "eliminated" || st[0].Score <= st[1].Score {
		t.Fatalf("standings %+v", st[:2])
	}

	w = newWorld(t, 1)
	for _, e := range w.Empires {
		w.Planets[e.HomeworldID].OwnerID = ""
	}
	r = &Runner{World: w}
	if rec, _ = r.Step(context.Background()); rec.End == nil || rec.End.Reason != world.EndNoSovereign {
		t.Fatalf("end %+v", rec.End)
	}
	if world.CheckEnd(newWorld(t, 1), 0) != nil {
		t.Fatal("fresh universe ended")
	}
}

func TestReplayReproducesARunAndDetectsTampering(t *testing.T) {
	w := newWorld(t, 5)
	initial, _ := Clone(w)
	r := &Runner{World: w, Agents: observationAutopilots()}
	var log []world.TurnResult
	for i := 0; i < 40; i++ {
		rec, err := r.Step(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(rec.Result) // the log is stored as JSON
		var tr world.TurnResult
		if err := json.Unmarshal(b, &tr); err != nil {
			t.Fatal(err)
		}
		log = append(log, tr)
	}
	final, err := Replay(initial, log)
	if err != nil {
		t.Fatal(err)
	}
	if world.StateHash(final) != world.StateHash(w) || initial.Turn != 0 {
		t.Fatal("replay diverged or mutated the initial state")
	}
	log[10].Submitted = nil
	if _, err := Replay(initial, log); err == nil || !strings.Contains(err.Error(), "turn 10") {
		t.Fatalf("tampered orders not detected: %v", err)
	}
	if _, err := Replay(initial, log[1:]); err == nil {
		t.Fatal("a gap in the log was not detected")
	}
	if _, err := Replay(nil, nil); err == nil {
		t.Fatal("nil initial state")
	}
}

func observationAutopilots() map[string]agent.Agent {
	m := map[string]agent.Agent{}
	for _, id := range []string{"e00", "e01", "e02", "e03", "e04", "e05", "e06", "e07"} {
		m[id] = agent.AutopilotAgent{}
	}
	return m
}

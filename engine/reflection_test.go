package engine

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/KakkoiDev/agame/agent"
	"github.com/KakkoiDev/agame/world"
)

func TestReflectionTriggers(t *testing.T) {
	w := newWorld(t, 1)
	w.Turn = 24
	h1 := w.Empires["e01"].HomeworldID
	h2 := w.Empires["e02"].HomeworldID
	w.Messages = []world.Message{{Turn: 24, From: "e05", To: "all", Recipients: []string{"e06"}, Major: true}, {Turn: 24, From: "e05", To: "e07", Recipients: []string{"e07"}}}
	w.Empires["e04"].Eliminated = true
	events := []world.Event{
		{Type: "captured", EmpireID: "e00", Target: h1, Other: "e01"},
		{Type: "captured", EmpireID: "e02", Target: h2, Other: "e03"},
		{Type: "exiled", EmpireID: "e03"},
		{Type: "restored", EmpireID: "e05"},
		{Type: "treaty_breach", EmpireID: "e06", Other: "e07"},
		{Type: "alliance_left", EmpireID: "e06"},
		{Type: "alliance_joined", EmpireID: "e04"}, // eliminated: never offered
		{Type: "battle", EmpireID: "e05", Other: "e07"},
	}
	before := fleetValues(w)
	before["e05"] = 2 * before["e05"] // as if e05 lost half its fleet value
	tr := ReflectionTriggers(before, w, events)
	want := map[string][]string{
		"e00": {TriggerAnnualReview, TriggerCapturedHomeworld},
		"e01": {TriggerAnnualReview, TriggerHomeworldLost},
		"e02": {TriggerAnnualReview, TriggerHomeworldRecovered},
		"e03": {TriggerAnnualReview, TriggerExile},
		"e05": {TriggerAnnualReview, TriggerFleetLosses, TriggerRestored},
		"e06": {TriggerAllianceLeft, TriggerAnnualReview, TriggerMajorProposal},
		"e07": {TriggerAllianceBroken, TriggerAnnualReview},
	}
	if !reflect.DeepEqual(tr, want) {
		t.Fatalf("triggers\n%v\nwant\n%v", tr, want)
	}
	w.Turn = 25
	if tr := ReflectionTriggers(fleetValues(w), w, nil); len(tr) != 0 {
		t.Fatalf("quiet turn triggered %v", tr)
	}
}

type reflector struct {
	agentFunc
	got  map[string][]string
	fail bool
}

func (r *reflector) Reflect(_ context.Context, o agent.Observation, triggers []string) (string, error) {
	r.got[o.Empire.ID] = triggers
	if r.fail {
		return "", errors.New("model down")
	}
	return "noted " + o.Empire.ID, nil
}

func TestRunnerOffersReflection(t *testing.T) {
	w := newWorld(t, 1)
	w.Turn = 11
	quiet := agentFunc(func(context.Context, agent.Observation) (agent.Decision, error) { return agent.Decision{}, nil })
	ref := &reflector{agentFunc: quiet, got: map[string][]string{}}
	bad := &reflector{agentFunc: quiet, got: map[string][]string{}, fail: true}
	r := &Runner{World: w, Agents: map[string]agent.Agent{"e00": ref, "e01": quiet, "e02": bad}}
	rec, err := r.Step(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Reflections) != 8 {
		t.Fatalf("annual review should be offered to all 8: %+v", rec.Reflections)
	}
	r0, r1, r2 := rec.Reflections[0], rec.Reflections[1], rec.Reflections[2]
	if !r0.Offered || r0.Note != "noted e00" || r0.Turn != 12 || r0.Triggers[0] != TriggerAnnualReview || len(ref.got["e00"]) != 1 {
		t.Fatalf("e00 %+v", r0)
	}
	if r1.Offered || r1.Note != "" {
		t.Fatalf("an agent without Reflect was offered: %+v", r1)
	}
	if !r2.Offered || r2.Error != "model down" {
		t.Fatalf("failing reflection %+v", r2)
	}
	if len(rec.Result.Accepted) != 0 {
		t.Fatal("reflection produced orders")
	}
}

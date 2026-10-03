package engine

import (
	"context"
	"sort"

	"github.com/KakkoiDev/agame/agent"
	"github.com/KakkoiDev/agame/world"
)

// Reflection triggers (spec/agents.md, Reflection triggers; D25, D63).
const (
	TriggerHomeworldLost      = "homeworld_captured"
	TriggerHomeworldRecovered = "homeworld_recovered"
	TriggerCapturedHomeworld  = "captured_enemy_homeworld"
	TriggerExile              = "entered_exile"
	TriggerRestored           = "restored_from_exile"
	TriggerAllianceJoined     = "alliance_joined"
	TriggerAllianceLeft       = "alliance_left"
	TriggerAllianceBroken     = "alliance_broken"
	TriggerFleetLosses        = "lost_half_fleet"
	TriggerMajorProposal      = "major_proposal"
	TriggerAnnualReview       = "annual_review"
)

// ReflectionRecord is the audit record of one reflection phase.
type ReflectionRecord struct {
	Turn     int      `json:"turn"`
	Empire   string   `json:"empire"`
	Triggers []string `json:"triggers"`
	// Offered is false when the ruler's agent cannot reflect.
	Offered   bool   `json:"offered"`
	Note      string `json:"note,omitempty"`
	Error     string `json:"error,omitempty"`
	LatencyMS int64  `json:"latency_ms"`
}

// fleetValues is each empire's total ship value (docked and in fleets).
func fleetValues(w *world.World) map[string]int {
	out := map[string]int{}
	for _, s := range world.Standings(w) {
		out[s.Empire] = s.FleetValue
	}
	return out
}

// ReflectionTriggers lists, per living empire, why a reflection phase is
// offered after the turn that turned S(t) into w: beforeFleet holds each
// empire's fleet value at S(t) and events are the turn's events.
func ReflectionTriggers(beforeFleet map[string]int, w *world.World, events []world.Event) map[string][]string {
	trig := map[string][]string{}
	add := func(eid, t string) {
		if e := w.Empires[eid]; e != nil && !e.Eliminated {
			for _, x := range trig[eid] {
				if x == t {
					return
				}
			}
			trig[eid] = append(trig[eid], t)
		}
	}
	homeOf := map[string]string{}
	for id, e := range w.Empires {
		homeOf[e.HomeworldID] = id
	}
	fought := map[string]bool{}
	for _, ev := range events {
		switch ev.Type {
		case "captured":
			if owner, ok := homeOf[ev.Target]; ok {
				if owner == ev.EmpireID {
					add(ev.EmpireID, TriggerHomeworldRecovered)
				} else {
					add(ev.EmpireID, TriggerCapturedHomeworld)
					if owner == ev.Other {
						add(owner, TriggerHomeworldLost)
					}
				}
			}
		case "exiled":
			add(ev.EmpireID, TriggerExile)
		case "restored":
			add(ev.EmpireID, TriggerRestored)
		case "alliance_joined", "alliance_created":
			add(ev.EmpireID, TriggerAllianceJoined)
		case "alliance_left":
			add(ev.EmpireID, TriggerAllianceLeft)
		case "treaty_breach":
			add(ev.Other, TriggerAllianceBroken)
		case "battle":
			fought[ev.EmpireID], fought[ev.Other] = true, true
		}
	}
	after := fleetValues(w)
	for eid := range fought {
		if b := beforeFleet[eid]; b > 0 && after[eid]*2 <= b {
			add(eid, TriggerFleetLosses)
		}
	}
	for _, m := range w.Messages {
		if m.Turn == w.Turn && m.Major {
			for _, r := range m.Recipients {
				add(r, TriggerMajorProposal)
			}
		}
	}
	if w.Turn > 0 && w.Turn%12 == 0 {
		for id := range w.Empires {
			add(id, TriggerAnnualReview)
		}
	}
	for id := range trig {
		sort.Strings(trig[id])
	}
	return trig
}

// reflect offers the reflection phase to every triggered ruler, in empire-ID
// order, on its observation of S(t+1). Reflection cannot submit orders.
func (r *Runner) reflect(ctx context.Context, triggers map[string][]string) []ReflectionRecord {
	var out []ReflectionRecord
	ids := make([]string, 0, len(triggers))
	for id := range triggers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		rec := ReflectionRecord{Turn: r.World.Turn, Empire: id, Triggers: triggers[id]}
		ref, ok := r.Agents[id].(agent.Reflector)
		if ok {
			rec.Offered = true
			rctx, cancel := ctx, context.CancelFunc(func() {})
			if r.Budget.Timeout > 0 {
				rctx, cancel = context.WithTimeout(ctx, r.Budget.Timeout)
			}
			start := r.now()
			o := agent.Observe(r.World, id)
			d, err := call(rctx, func(ctx context.Context) (agent.Decision, error) {
				note, err := ref.Reflect(ctx, o, rec.Triggers)
				return agent.Decision{Statement: note}, err
			})
			cancel()
			rec.Note = d.Statement
			if err != nil {
				rec.Error = err.Error()
			}
			rec.LatencyMS = r.now().Sub(start).Milliseconds()
		}
		out = append(out, rec)
	}
	return out
}

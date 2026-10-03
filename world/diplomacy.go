package world

import (
	"fmt"
	"sort"
)

// Alliances (spec/game.md, Diplomacy mechanics; D23, D50). The engine only
// records membership: an empire belongs to at most one alliance, joins only
// after a member invited it, and leaves voluntarily, by attacking an ally
// (treaty breach) or by elimination. Alliances never block orders.

func invitable(w *World, from, id string) error {
	e := w.Empires[id]
	if e == nil {
		return fmt.Errorf("unknown empire %q", id)
	}
	if id == from {
		return fmt.Errorf("cannot invite yourself")
	}
	if e.Eliminated {
		return fmt.Errorf("empire %s is eliminated", id)
	}
	return nil
}

func createAlliance(w *World, e *Empire, name string, invite []string) *Alliance {
	if w.Alliances == nil {
		w.Alliances = map[string]*Alliance{}
	}
	w.NextAlliance++
	id := fmt.Sprintf("a%03d", w.NextAlliance)
	if name == "" {
		name = e.Name + " Pact"
	}
	a := &Alliance{ID: id, Name: name, Founder: e.ID, Founded: w.Turn, Members: []string{e.ID}, Invited: sortedSet(append([]string(nil), invite...))}
	w.Alliances[id] = a
	e.AllianceID = id
	e.Stats.AlliancesJoined++
	w.emit(Event{Type: "alliance_created", EmpireID: e.ID, Target: id, Detail: name})
	for _, x := range a.Invited {
		w.emit(Event{Type: "alliance_invited", EmpireID: e.ID, Target: id, Other: x})
	}
	return a
}

// leaveAlliance removes e from its alliance; an alliance without members is
// dissolved. why is "left", "breach" or "eliminated".
func leaveAlliance(w *World, e *Empire, why string) {
	a := w.Alliances[e.AllianceID]
	e.AllianceID = ""
	if a == nil {
		return
	}
	a.Members = without(a.Members, e.ID)
	w.emit(Event{Type: "alliance_left", EmpireID: e.ID, Target: a.ID, Detail: why})
	if len(a.Members) == 0 {
		delete(w.Alliances, a.ID)
		w.emit(Event{Type: "alliance_dissolved", Target: a.ID, Detail: a.Name})
	}
}

// Allied reports whether two distinct empires share an alliance.
func Allied(w *World, a, b string) bool {
	ea, eb := w.Empires[a], w.Empires[b]
	return a != b && ea != nil && eb != nil && ea.AllianceID != "" && ea.AllianceID == eb.AllianceID
}

// breachIfAllied implements D23: attacking an ally is legal, but the
// attacker automatically leaves the alliance and a treaty breach is recorded.
func breachIfAllied(w *World, attacker, victim, what string) {
	if !Allied(w, attacker, victim) {
		return
	}
	e := w.Empires[attacker]
	w.emit(Event{Type: "treaty_breach", EmpireID: attacker, Target: e.AllianceID, Other: victim, Detail: what})
	e.Stats.Breaches++
	leaveAlliance(w, e, "breach")
}

// HostilityKey is the Hostilities map key for an empire pair.
func HostilityKey(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "|" + b
}

// recordHostility notes that two empires fought this turn; the first battle
// between a pair emits a war_began event (D52).
func recordHostility(w *World, a, b string) {
	if a == "" || b == "" || a == b {
		return
	}
	if w.Hostilities == nil {
		w.Hostilities = map[string]int{}
	}
	k := HostilityKey(a, b)
	if _, ok := w.Hostilities[k]; !ok {
		w.emit(Event{Type: "war_began", EmpireID: a, Other: b})
	}
	w.Hostilities[k] = w.Turn
}

// AllianceIDs returns the alliance ids in order.
func AllianceIDs(w *World) []string {
	ids := make([]string, 0, len(w.Alliances))
	for id := range w.Alliances {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// publicEvents are visible to every ruler: alliance membership, sovereignty
// and planet ownership are public facts (D48, D51).
var publicEvents = map[string]bool{
	"alliance_created": true, "alliance_joined": true, "alliance_left": true, "alliance_dissolved": true,
	"treaty_breach": true, "captured": true, "colonized": true, "eliminated": true, "exiled": true, "restored": true,
}

// EventVisibleTo reports whether empire eid learns of event e (D51). Public
// events reach everyone; espionage reports reach only the spy, detections
// only the spied-on empire; everything else reaches the acting empire and
// its counterpart.
func EventVisibleTo(e Event, eid string) bool {
	switch {
	case publicEvents[e.Type]:
		return true
	case e.Type == "espionage" || e.Type == "spy_detected":
		return e.EmpireID == eid
	}
	return e.EmpireID == eid || (e.Other != "" && e.Other == eid)
}

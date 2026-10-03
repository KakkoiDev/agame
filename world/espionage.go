package world

import (
	"fmt"
	"sort"
)

// SpyReport is the content of one espionage mission (spec/game.md,
// Espionage). What it contains depends on the tier derived from
// intel = spy Sensors - owner Sensors:
//
//	tier 0 (intel <= -2): ownership and coarse activity only;
//	tier 1 (-1..0):       resource bands, building bands, fleet size band;
//	tier 2 (1..2):        exact resources and building levels, ship counts
//	                      rounded to the nearest 5;
//	tier 3 (>= 3):        exact ships, resources and buildings, the owner's
//	                      technology levels and where its fleets in the
//	                      system are heading.
//
// The noise is the banding and rounding itself, so it is deterministic and
// bounded by the tier (D53). Ships seen are the planet's docked ships plus the
// owner's fleets in the planet's system.
type SpyReport struct {
	Planet        string            `json:"planet"`
	System        string            `json:"system"`
	Owner         string            `json:"owner,omitempty"`
	ObservedTurn  int               `json:"observed_turn"`
	Tier          int               `json:"tier"`
	Confidence    string            `json:"confidence"`
	Activity      string            `json:"activity"`
	ResourceBands map[string]string `json:"resource_bands,omitempty"`
	BuildingBands map[string]string `json:"building_bands,omitempty"`
	FleetSize     string            `json:"fleet_size,omitempty"`
	Resources     *Resources        `json:"resources,omitempty"`
	Buildings     *Buildings        `json:"buildings,omitempty"`
	Ships         Ships             `json:"ships,omitempty"`
	Tech          *Tech             `json:"tech,omitempty"`
	Outgoing      []string          `json:"outgoing,omitempty"`
}

var tierConfidence = []string{"minimal", "low", "high", "exact"}

// IntelTier maps a Sensors difference to a report tier.
func IntelTier(intel int) int {
	switch {
	case intel <= -2:
		return 0
	case intel <= 0:
		return 1
	case intel <= 2:
		return 2
	}
	return 3
}

// SpyRange is how many route edges a scout can observe across: its own
// system and adjacent ones, plus one more edge per 3 Sensors levels (D53).
func SpyRange(sensors int) int { return 1 + max(0, sensors)/3 }

// DetectionChance is the percent chance that the spied-on empire detects the
// scout: 50 - 15 x intel, clamped to 5..95 (D53).
func DetectionChance(intel int) int {
	return max(5, min(95, 50-15*intel))
}

// SizeBand coarsens a ship count the way low-tier reports and sightings do.
func SizeBand(n int) string {
	switch {
	case n == 0:
		return "0"
	case n < 5:
		return "1-4"
	case n < 20:
		return "5-19"
	case n < 50:
		return "20-49"
	}
	return "50+"
}

func resourceBand(n int) string {
	switch {
	case n < 500:
		return "0-499"
	case n < 2000:
		return "500-1999"
	case n < 10000:
		return "2000-9999"
	}
	return "10000+"
}

func buildingBand(n int) string {
	switch {
	case n == 0:
		return "0"
	case n <= 2:
		return "1-2"
	case n <= 5:
		return "3-5"
	}
	return "6+"
}

func spy(w *World, f *Fleet) {
	p := w.Planets[f.Target]
	if p == nil || distance(w, f.SystemID, p.SystemID) > SpyRange(w.Empires[f.OwnerID].Tech.Sensors) || f.Ships[ShipScout] == 0 {
		w.emit(Event{Type: "spy_failed", EmpireID: f.OwnerID, Target: f.Target, Detail: f.ID})
		return
	}
	intel := w.Empires[f.OwnerID].Tech.Sensors
	if p.OwnerID != "" {
		intel -= w.Empires[p.OwnerID].Tech.Sensors
	}
	r := buildReport(w, p, IntelTier(intel))
	w.Empires[f.OwnerID].Stats.SpyMissions++
	w.emit(Event{Type: "espionage", EmpireID: f.OwnerID, Target: p.ID, Other: p.OwnerID,
		Detail: fmt.Sprintf("tier=%d owner=%s", r.Tier, p.OwnerID), Report: r})
	if p.OwnerID == "" {
		return
	}
	if seededRNG(w, "spy", f.ID, p.ID).IntN(100) < DetectionChance(intel) {
		w.Empires[f.OwnerID].Stats.Detected++
		w.emit(Event{Type: "spy_detected", EmpireID: p.OwnerID, Target: p.ID, Other: f.OwnerID, Detail: "scout fleet " + f.ID})
	}
}

func buildReport(w *World, p *Planet, tier int) *SpyReport {
	r := &SpyReport{Planet: p.ID, System: p.SystemID, Owner: p.OwnerID, ObservedTurn: w.Turn, Tier: tier, Confidence: tierConfidence[tier]}
	ships := Ships{}
	for k, n := range p.Ships {
		ships[k] += n
	}
	if p.OwnerID != "" {
		for _, id := range fleetIDs(w) {
			if f := w.Fleets[id]; f.OwnerID == p.OwnerID && f.SystemID == p.SystemID {
				for k, n := range f.Ships {
					ships[k] += n
				}
			}
		}
	}
	r.Activity = activity(p, ships)
	if tier == 0 {
		return r
	}
	b := p.Buildings
	if tier == 1 {
		res := p.Resources
		r.ResourceBands = map[string]string{"metal": resourceBand(res.Metal), "crystal": resourceBand(res.Crystal), "deuterium": resourceBand(res.Deuterium)}
		r.BuildingBands = map[string]string{}
		for _, k := range BuildingKinds() {
			r.BuildingBands[k] = buildingBand(buildingLevel(b, k))
		}
		r.FleetSize = SizeBand(ships.Count())
		return r
	}
	res := p.Resources
	r.Resources, r.Buildings = &res, &b
	r.Ships = Ships{}
	for k, n := range ships {
		if tier == 2 {
			n = (n + 2) / 5 * 5
		}
		if n > 0 {
			r.Ships[k] = n
		}
	}
	if tier == 3 && p.OwnerID != "" {
		t := w.Empires[p.OwnerID].Tech
		r.Tech = &t
		for _, id := range fleetIDs(w) { // fleets that left this system on a route still being flown
			if f := w.Fleets[id]; f.OwnerID == p.OwnerID && len(f.Route) > 1 && f.Route[0] == p.SystemID {
				r.Outgoing = append(r.Outgoing, fmt.Sprintf("%s -> %s", f.ID, f.Route[len(f.Route)-1]))
			}
		}
		sort.Strings(r.Outgoing)
	}
	return r
}

func activity(p *Planet, ships Ships) string {
	switch {
	case p.OwnerID == "":
		return "unclaimed"
	case p.ShipyardQueue != nil && hasCombatShip(ships):
		return "armed, shipyard active"
	case p.ShipyardQueue != nil:
		return "shipyard active"
	case hasCombatShip(ships):
		return "armed"
	case p.Construction != nil:
		return "construction"
	}
	return "quiet"
}

package agent

import (
	"sort"

	"github.com/KakkoiDev/agame/world"
)

// Autopilot is the zero-download decision layer. It keeps every universe
// playable immediately and is deliberately replaceable by classifier/model
// providers without changing the engine.
func Autopilot(w *world.World, empireID string) Decision {
	e := w.Empires[empireID]
	if e == nil || e.Eliminated {
		return Decision{}
	}
	planets := ownedPlanets(w, empireID)
	if len(planets) == 0 {
		return Decision{Statement: "No sovereign planet; preserving exile assets."}
	}
	p := planets[0]

	// Existing idle fleets act before the economy layer.
	for _, f := range ownedFleets(w, empireID) {
		if len(f.Route) == 0 && (f.Ships["frigate"] > 0 || f.Ships["cruiser"] > 0) {
			if target := enemyPlanet(w, empireID); target != "" {
				return Decision{Orders: []world.Order{{Type: "attack", Actor: f.ID, Target: target}}, Statement: "Fleet dispatched against a rival world."}
			}
		}
	}

	// Turn a homeworld garrison into a fleet once enough combat ships exist.
	combat := world.Ships{}
	if p.Ships["frigate"] > 0 {
		combat["frigate"] = p.Ships["frigate"]
	}
	if p.Ships["cruiser"] > 0 {
		combat["cruiser"] = p.Ships["cruiser"]
	}
	if len(combat) > 0 && w.Turn%6 == 5 {
		return Decision{Orders: []world.Order{{Type: "form_fleet", Actor: p.ID, Params: map[string]any{"ships": combat}}}, Statement: "Combat ships organized into an expeditionary fleet."}
	}

	// One economic order per turn avoids cross-order overspend while the
	// engine still validates all submissions against the same snapshot.
	if p.Construction == nil {
		buildings := []string{"metal_mine", "crystal_mine", "deuterium_extractor", "infrastructure", "research_lab", "shipyard", "defense_grid"}
		target := buildings[(w.Turn+empireOrdinal(empireID))%len(buildings)]
		return Decision{Orders: []world.Order{{Type: "construct", Actor: p.ID, Target: target}}, Statement: "Investing in planetary infrastructure."}
	}
	if e.Research == nil {
		tech := []string{"industry", "propulsion", "weapons", "shields", "sensors", "colonization"}
		target := tech[(w.Turn+empireOrdinal(empireID))%len(tech)]
		return Decision{Orders: []world.Order{{Type: "research", Actor: p.ID, Target: target}}, Statement: "Research program selected for the month."}
	}
	if p.ShipyardQueue == nil {
		return Decision{Orders: []world.Order{{Type: "build_ships", Actor: p.ID, Target: "frigate", Params: map[string]any{"quantity": 1}}}, Statement: "Shipyard ordered to produce a frigate."}
	}
	return Decision{Statement: "Queues are occupied; conserving resources."}
}

func ownedPlanets(w *world.World, eid string) []*world.Planet {
	var out []*world.Planet
	for _, p := range w.Planets {
		if p.OwnerID == eid {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func ownedFleets(w *world.World, eid string) []*world.Fleet {
	var out []*world.Fleet
	for _, f := range w.Fleets {
		if f.OwnerID == eid {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func enemyPlanet(w *world.World, eid string) string {
	ids := make([]string, 0, len(w.Planets))
	for id, p := range w.Planets {
		if p.OwnerID != "" && p.OwnerID != eid {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return ""
	}
	return ids[0]
}

func empireOrdinal(id string) int {
	n := 0
	for _, r := range id {
		if r >= '0' && r <= '9' {
			n = n*10 + int(r-'0')
		}
	}
	return n
}

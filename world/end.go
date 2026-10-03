package world

import "sort"

// CanonicalTurnLimit is the canonical benchmark horizon: 600 monthly turns,
// 50 years (spec/game.md, End conditions; D30).
const CanonicalTurnLimit = 600

// End reasons (D30, D58).
const (
	EndTurnLimit    = "turn_limit"
	EndLastStanding = "last_empire_standing"
	EndNoSovereign  = "no_sovereignty_possible"
	EndObserverStop = "observer_stop"
)

// End describes why and when a run ended.
type End struct {
	Turn      int      `json:"turn"`
	Reason    string   `json:"reason"`
	Survivors []string `json:"survivors"`
}

// CheckEnd reports whether S(t) is terminal under D30: the turn limit is
// reached (limit <= 0 means the canonical 600), one non-eliminated empire
// remains, or none does (no empire can establish or contest sovereignty any
// more; a planetless empire without a viable Colony Ark is already
// eliminated, D29). It never ends a run because one empire dominates.
func CheckEnd(w *World, limit int) *End {
	if limit <= 0 {
		limit = CanonicalTurnLimit
	}
	var alive []string
	for _, id := range sortedKeys(w.Empires) {
		if !w.Empires[id].Eliminated {
			alive = append(alive, id)
		}
	}
	switch {
	case len(alive) == 0:
		return &End{Turn: w.Turn, Reason: EndNoSovereign, Survivors: alive}
	case len(alive) == 1 && len(w.Empires) > 1:
		return &End{Turn: w.Turn, Reason: EndLastStanding, Survivors: alive}
	case w.Turn >= limit:
		return &End{Turn: w.Turn, Reason: EndTurnLimit, Survivors: alive}
	}
	return nil
}

// Standing is one empire's raw measurements at a point in the run plus its
// rank (spec/benchmark.md, Metrics; D58). Score is a documented
// experiment-specific aggregate shown next to, never instead of, the raw
// numbers.
type Standing struct {
	Rank       int       `json:"rank"`
	Empire     string    `json:"empire"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	Alliance   string    `json:"alliance,omitempty"`
	Survived   int       `json:"turns_survived"`
	Planets    int       `json:"planets"`
	Homeworld  bool      `json:"homeworld_held"`
	Production Resources `json:"production"`
	Stored     Resources `json:"stored"`
	FleetValue int       `json:"fleet_value"`
	TechLevels int       `json:"tech_levels"`
	Stats      Stats     `json:"stats"`
	Score      int       `json:"score"`
}

// Standings ranks every empire. Surviving empires rank above eliminated
// ones, longer survival ranks higher, then Score, then empire id. Score =
// 100 x planets + per-turn production (metal+crystal+deuterium) + fleet
// value / 100 + 10 x technology levels, where fleet value is the full
// resource cost of every ship the empire owns, docked or in fleets.
func Standings(w *World) []Standing {
	var out []Standing
	for _, id := range sortedKeys(w.Empires) {
		e := w.Empires[id]
		s := Standing{Empire: id, Name: e.Name, Status: EmpireStatus(e), Alliance: e.AllianceID, Stats: e.Stats, Survived: w.Turn}
		if e.Eliminated && e.Stats.EliminatedTurn > 0 {
			s.Survived = e.Stats.EliminatedTurn
		}
		for _, pid := range sortedKeys(w.Planets) {
			p := w.Planets[pid]
			if p.OwnerID != id {
				continue
			}
			s.Planets++
			s.Homeworld = s.Homeworld || pid == e.HomeworldID
			s.Production = s.Production.Add(Production(w, p))
			s.Stored = s.Stored.Add(p.Resources)
			s.FleetValue += shipsValue(p.Ships)
		}
		for _, fid := range fleetIDs(w) {
			if f := w.Fleets[fid]; f.OwnerID == id {
				s.FleetValue += shipsValue(f.Ships)
			}
		}
		t := e.Tech
		s.TechLevels = t.Industry + t.Propulsion + t.Weapons + t.Shields + t.Sensors + t.Colonization
		pr := s.Production
		s.Score = 100*s.Planets + pr.Metal + pr.Crystal + pr.Deuterium + s.FleetValue/100 + 10*s.TechLevels
		out = append(out, s)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if (a.Status == "eliminated") != (b.Status == "eliminated") {
			return b.Status == "eliminated"
		}
		if a.Survived != b.Survived {
			return a.Survived > b.Survived
		}
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		return a.Empire < b.Empire
	})
	for i := range out {
		out[i].Rank = i + 1
	}
	return out
}

// EmpireStatus names an empire's sovereignty state.
func EmpireStatus(e *Empire) string {
	switch {
	case e.Eliminated:
		return "eliminated"
	case e.Exile:
		return "exile"
	}
	return "sovereign"
}

func shipsValue(s Ships) int {
	v := 0
	for k, n := range s {
		c := ShipSpecs[k].Cost
		v += n * (c.Metal + c.Crystal + c.Deuterium)
	}
	return v
}

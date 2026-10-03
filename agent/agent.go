package agent

import (
	"context"
	"sort"

	"github.com/KakkoiDev/agame/world"
)

// Observation is one ruler's legal view of the frozen S(t). Visibility rules
// (spec/decisions.md D48):
//   - the system graph, every planet's id/system/slot/owner and the homeworld
//     flag are public;
//   - the empire's own planets and fleets are shown in full;
//   - foreign fleets are seen only in systems where the empire owns a planet
//     or has a fleet, and only as owner plus a size band (exact composition
//     is espionage territory);
//   - debris is seen only in those same systems.
type Observation struct {
	Turn          int               `json:"turn"`
	Empire        *world.Empire     `json:"empire"`
	Planets       []*world.Planet   `json:"planets"`
	Fleets        []*world.Fleet    `json:"fleets"`
	Messages      []world.Message   `json:"messages"`
	Systems       []SystemView      `json:"systems"`
	OtherPlanets  []PlanetView      `json:"other_planets"`
	ForeignFleets []FleetSighting   `json:"foreign_fleets"`
	Orders        []world.OrderSpec `json:"orders"`
}

// SystemView is a node of the public hyperspace graph.
type SystemView struct {
	ID        string           `json:"id"`
	Neighbors []string         `json:"neighbors"`
	Planets   []string         `json:"planets"`
	Debris    *world.Resources `json:"debris,omitempty"`
}

// PlanetView is the public part of a planet the empire does not own.
type PlanetView struct {
	ID        string `json:"id"`
	SystemID  string `json:"system"`
	Slot      int    `json:"slot"`
	OwnerID   string `json:"owner,omitempty"`
	Homeworld bool   `json:"homeworld,omitempty"`
}

// FleetSighting is a foreign fleet seen in a system where the empire is present.
type FleetSighting struct {
	ID       string `json:"id"`
	OwnerID  string `json:"owner"`
	SystemID string `json:"system"`
	Size     string `json:"size"`
}

// SizeBand coarsens a ship count the way low-tier espionage reports do.
func SizeBand(n int) string {
	switch {
	case n < 5:
		return "1-4"
	case n < 20:
		return "5-19"
	case n < 50:
		return "20-49"
	}
	return "50+"
}

type Decision struct {
	Orders    []world.Order `json:"orders"`
	Statement string        `json:"statement"`
}
type Agent interface {
	Decide(context.Context, Observation) (Decision, error)
}

// Observe returns a deep copy sorted by ID: agents run sequentially, so they
// must neither see map-order noise nor be able to mutate the frozen S(t).
func Observe(w *world.World, eid string) Observation {
	o := Observation{Turn: w.Turn}
	e := w.Empires[eid]
	if e == nil {
		return o
	}
	c := *e
	c.Research = copyQueue(e.Research)
	o.Empire = &c

	present := map[string]bool{}
	for _, id := range sortedIDs(w.Planets) {
		p := w.Planets[id]
		if p.OwnerID == eid {
			c := *p
			c.Ships = copyShips(p.Ships)
			c.Construction = copyQueue(p.Construction)
			c.ShipyardQueue = copyQueue(p.ShipyardQueue)
			o.Planets = append(o.Planets, &c)
			present[p.SystemID] = true
			continue
		}
		o.OtherPlanets = append(o.OtherPlanets, PlanetView{ID: p.ID, SystemID: p.SystemID, Slot: p.Slot, OwnerID: p.OwnerID, Homeworld: p.Homeworld})
	}
	for _, id := range sortedIDs(w.Fleets) {
		f := w.Fleets[id]
		if f.OwnerID == eid {
			c := *f
			c.Ships = copyShips(f.Ships)
			c.Route = append([]string(nil), f.Route...)
			o.Fleets = append(o.Fleets, &c)
			present[f.SystemID] = true
		}
	}
	for _, id := range sortedIDs(w.Fleets) {
		f := w.Fleets[id]
		if f.OwnerID != eid && present[f.SystemID] {
			o.ForeignFleets = append(o.ForeignFleets, FleetSighting{ID: f.ID, OwnerID: f.OwnerID, SystemID: f.SystemID, Size: SizeBand(f.Ships.Count())})
		}
	}
	for _, id := range sortedIDs(w.Systems) {
		s := w.Systems[id]
		v := SystemView{ID: s.ID, Neighbors: sortedCopy(s.Neighbors), Planets: sortedCopy(s.Planets)}
		if present[s.ID] && s.Debris != (world.Resources{}) {
			d := s.Debris
			v.Debris = &d
		}
		o.Systems = append(o.Systems, v)
	}
	for _, m := range w.Messages {
		if m.To == eid && m.Turn <= w.Turn {
			o.Messages = append(o.Messages, m)
		}
	}
	o.Orders = world.OrderSpecs()
	return o
}

func sortedIDs[V any](m map[string]V) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
func sortedCopy(s []string) []string {
	c := append([]string(nil), s...)
	sort.Strings(c)
	return c
}
func copyQueue(q *world.Queue) *world.Queue {
	if q == nil {
		return nil
	}
	c := *q
	return &c
}
func copyShips(s world.Ships) world.Ships {
	c := world.Ships{}
	for k, n := range s {
		c[k] = n
	}
	return c
}

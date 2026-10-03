package agent

import (
	"context"

	"github.com/KakkoiDev/agame/world"
)

// AutopilotAgent runs the deterministic Autopilot as an ordinary Agent. It
// sees only the ruler's Observation: the world it reasons about is rebuilt
// from that observation, so the autopilot can never use hidden information.
type AutopilotAgent struct{}

func (AutopilotAgent) Decide(_ context.Context, o Observation) (Decision, error) {
	if o.Empire == nil {
		return Decision{}, nil
	}
	return Autopilot(WorldFromObservation(o), o.Empire.ID), nil
}

// WorldFromObservation builds the partial world a ruler knows: the public
// graph and planet ownership, alliance membership, its own planets, fleets
// and empire in full, and foreign fleets it has sighted (without their
// composition). Foreign empires are known only by id. The result is a fresh
// copy; mutating it never touches the observation.
func WorldFromObservation(o Observation) *world.World {
	w := &world.World{Turn: o.Turn, Systems: map[string]*world.System{}, Planets: map[string]*world.Planet{},
		Empires: map[string]*world.Empire{}, Fleets: map[string]*world.Fleet{}, Alliances: map[string]*world.Alliance{}}
	if o.Empire == nil {
		return w
	}
	me := *o.Empire
	me.Research = copyQueue(o.Empire.Research)
	w.Empires[me.ID] = &me
	stub := func(id string) {
		if id != "" && w.Empires[id] == nil {
			w.Empires[id] = &world.Empire{ID: id}
		}
	}
	for _, r := range o.Rulers {
		if r.ID != me.ID {
			w.Empires[r.ID] = &world.Empire{ID: r.ID, Name: r.Name, Exile: r.Status == "exile", Eliminated: r.Status == "eliminated", AllianceID: r.Alliance}
		}
	}
	for _, s := range o.Systems {
		x := &world.System{ID: s.ID, Neighbors: sortedCopy(s.Neighbors), Planets: sortedCopy(s.Planets)}
		if s.Debris != nil {
			x.Debris = *s.Debris
		}
		w.Systems[s.ID] = x
	}
	for _, p := range o.Planets {
		c := *p
		c.Ships = copyShips(p.Ships)
		c.Construction = copyQueue(p.Construction)
		c.ShipyardQueue = copyQueue(p.ShipyardQueue)
		w.Planets[p.ID] = &c
	}
	for _, p := range o.OtherPlanets {
		w.Planets[p.ID] = &world.Planet{ID: p.ID, SystemID: p.SystemID, Slot: p.Slot, OwnerID: p.OwnerID, Homeworld: p.Homeworld, Ships: world.Ships{}}
		stub(p.OwnerID)
	}
	for _, f := range o.Fleets {
		c := *f
		c.Ships = copyShips(f.Ships)
		c.Route = append([]string(nil), f.Route...)
		w.Fleets[f.ID] = &c
	}
	for _, f := range o.ForeignFleets {
		w.Fleets[f.ID] = &world.Fleet{ID: f.ID, OwnerID: f.OwnerID, SystemID: f.SystemID, Ships: world.Ships{}}
		stub(f.OwnerID)
	}
	for _, a := range o.Alliances {
		w.Alliances[a.ID] = &world.Alliance{ID: a.ID, Name: a.Name, Members: sortedCopy(a.Members), Invited: sortedCopy(a.Invited)}
		for _, m := range a.Members {
			stub(m)
			w.Empires[m].AllianceID = a.ID
		}
	}
	for _, id := range o.Invitations {
		if a := w.Alliances[id]; a != nil && !contains(a.Invited, me.ID) {
			a.Invited = append(a.Invited, me.ID)
		}
	}
	return w
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

// Name identifies the agent in run records.
func (AutopilotAgent) Name() string { return "autopilot" }

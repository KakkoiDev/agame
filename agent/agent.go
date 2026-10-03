package agent

import (
	"context"
	"github.com/KakkoiDev/agame/world"
	"sort"
)

type Observation struct {
	Turn     int             `json:"turn"`
	Empire   *world.Empire   `json:"empire"`
	Planets  []*world.Planet `json:"planets"`
	Fleets   []*world.Fleet  `json:"fleets"`
	Messages []world.Message `json:"messages"`
}
type Decision struct {
	Orders    []world.Order `json:"orders"`
	Statement string        `json:"statement"`
}
type Agent interface {
	Decide(context.Context, Observation) (Decision, error)
}

// Observe returns a deep copy sorted by ID: agents run sequentially, so they must neither see map-order noise nor be able to mutate the frozen S(t).
func Observe(w *world.World, eid string) Observation {
	o := Observation{Turn: w.Turn}
	if e := w.Empires[eid]; e != nil {
		c := *e
		c.Research = copyQueue(e.Research)
		o.Empire = &c
	}
	for _, p := range w.Planets {
		if p.OwnerID == eid {
			c := *p
			c.Ships = copyShips(p.Ships)
			c.Construction = copyQueue(p.Construction)
			c.ShipyardQueue = copyQueue(p.ShipyardQueue)
			o.Planets = append(o.Planets, &c)
		}
	}
	sort.Slice(o.Planets, func(i, j int) bool { return o.Planets[i].ID < o.Planets[j].ID })
	for _, f := range w.Fleets {
		if f.OwnerID == eid {
			c := *f
			c.Ships = copyShips(f.Ships)
			c.Route = append([]string(nil), f.Route...)
			o.Fleets = append(o.Fleets, &c)
		}
	}
	sort.Slice(o.Fleets, func(i, j int) bool { return o.Fleets[i].ID < o.Fleets[j].ID })
	for _, m := range w.Messages {
		if m.To == eid && m.Turn <= w.Turn {
			o.Messages = append(o.Messages, m)
		}
	}
	return o
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

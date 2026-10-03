package agent

import (
	"context"
	"sort"
	"strings"

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
//
// Diplomacy and feedback follow spec/diplomacy.md timing (D11, D51): messages
// sent during turn t-1 are delivered at turn t, and the observation carries
// the events of turn t-1 the empire is entitled to see (its own actions,
// battles it fought, espionage reports it gathered, public alliance and
// sovereignty changes) plus its orders the engine rejected, with reasons.
type Observation struct {
	Turn          int               `json:"turn"`
	Empire        *world.Empire     `json:"empire"`
	Planets       []*world.Planet   `json:"planets"`
	Fleets        []*world.Fleet    `json:"fleets"`
	Messages      []world.Message   `json:"messages"`
	Systems       []SystemView      `json:"systems"`
	OtherPlanets  []PlanetView      `json:"other_planets"`
	ForeignFleets []FleetSighting   `json:"foreign_fleets"`
	Rulers        []RulerView       `json:"rulers"`
	Alliances     []AllianceView    `json:"alliances,omitempty"`
	Invitations   []string          `json:"invitations,omitempty"`
	Hostilities   map[string]int    `json:"hostilities,omitempty"`
	Events        []world.Event     `json:"events,omitempty"`
	Rejected      []world.Rejection `json:"rejected,omitempty"`
	Orders        []world.OrderSpec `json:"orders"`
}

// RulerView is the public face of an empire: its name and whether it is
// sovereign, in exile or eliminated (planet ownership makes this public).
type RulerView struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Alliance string `json:"alliance,omitempty"`
}

// Status names an empire's sovereignty state.
func Status(e *world.Empire) string { return world.EmpireStatus(e) }

// AllianceView is the public part of an alliance: who belongs to it.
// Pending invitations are shown only to the alliance's members.
type AllianceView struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Members []string `json:"members"`
	Invited []string `json:"invited,omitempty"`
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
func SizeBand(n int) string { return world.SizeBand(n) }

type Decision struct {
	Orders    []world.Order `json:"orders"`
	Statement string        `json:"statement"`
	// Raw is the model's unparsed output, kept for the decision record.
	Raw string `json:"-"`
}
type Agent interface {
	Decide(context.Context, Observation) (Decision, error)
}

// Repairer is an Agent that can be asked to correct an invalid decision
// (spec/agents.md, Failure handling): prev is its last attempt (with Raw
// output when it had one) and problem says what was wrong.
type Repairer interface {
	Repair(ctx context.Context, o Observation, prev Decision, problem string) (Decision, error)
}

// Reflector is an Agent that accepts the optional reflection phase offered
// after major events (spec/agents.md, Reflection triggers). It sees the new
// observation and the triggers, may update its durable memory and returns a
// concise note for the record. It cannot submit orders.
type Reflector interface {
	Reflect(ctx context.Context, o Observation, triggers []string) (string, error)
}

// Configured is an Agent that can describe its inference configuration for
// the run record (spec/benchmark.md, Reproducible universe).
type Configured interface {
	Config() map[string]any
}

// MalformedError is returned when model output is not a decision envelope.
// Raw is the output, so it can be recorded and fed back for repair.
type MalformedError struct {
	Raw string
	Err error
}

func (e *MalformedError) Error() string { return "malformed decision: " + e.Err.Error() }
func (e *MalformedError) Unwrap() error { return e.Err }

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
	c.Rejected = nil
	c.Stats.Detected = 0 // the spy is not told it was detected
	o.Empire = &c
	for _, r := range e.Rejected {
		r.Params = copyParams(r.Params)
		o.Rejected = append(o.Rejected, r)
	}

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
		if m.Turn == w.Turn && received(m, eid) {
			m.Recipients = append([]string(nil), m.Recipients...)
			o.Messages = append(o.Messages, m)
		}
	}
	for _, id := range sortedIDs(w.Empires) {
		x := w.Empires[id]
		o.Rulers = append(o.Rulers, RulerView{ID: x.ID, Name: x.Name, Status: Status(x), Alliance: x.AllianceID})
	}
	for _, id := range world.AllianceIDs(w) {
		a := w.Alliances[id]
		v := AllianceView{ID: a.ID, Name: a.Name, Members: sortedCopy(a.Members)}
		if e.AllianceID == a.ID {
			v.Invited = sortedCopy(a.Invited)
		}
		for _, x := range a.Invited {
			if x == eid {
				o.Invitations = append(o.Invitations, a.ID)
			}
		}
		o.Alliances = append(o.Alliances, v)
	}
	for k, turn := range w.Hostilities {
		if a, b, ok := strings.Cut(k, "|"); ok && (a == eid || b == eid) {
			if o.Hostilities == nil {
				o.Hostilities = map[string]int{}
			}
			other := a
			if a == eid {
				other = b
			}
			o.Hostilities[other] = turn
		}
	}
	for _, ev := range w.Events {
		if ev.Turn == w.Turn-1 && world.EventVisibleTo(ev, eid) {
			if ev.Report != nil {
				r := *ev.Report
				ev.Report = &r
			}
			o.Events = append(o.Events, ev)
		}
	}
	o.Orders = world.OrderSpecs()
	return o
}

// received reports whether m was addressed to eid. Messages written before
// group addressing existed carry only To.
func received(m world.Message, eid string) bool {
	if len(m.Recipients) == 0 {
		return m.To == eid
	}
	for _, r := range m.Recipients {
		if r == eid {
			return true
		}
	}
	return false
}

func copyParams(p map[string]any) map[string]any {
	if p == nil {
		return nil
	}
	c := make(map[string]any, len(p))
	for k, v := range p {
		c[k] = v
	}
	return c
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

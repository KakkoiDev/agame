package world

import (
	"fmt"
	"sort"
)

type Order struct {
	EmpireID, Type, Actor, Target string
	Params                        map[string]any
}

// TurnResult is the outcome of one resolved turn. Submitted is the complete
// input (per empire, in submission order) so a run can be replayed from the
// initial state; StateHash is the hash of S(t+1).
type TurnResult struct {
	Turn      int                `json:"turn"`
	Submitted map[string][]Order `json:"submitted,omitempty"`
	Accepted  []Order            `json:"accepted"`
	Rejected  []Rejection        `json:"rejected"`
	Events    []Event            `json:"events"`
	StateHash string             `json:"state_hash"`
}

// ResolveTurn closes the decision barrier: it validates every submitted order
// against the same frozen S(t), applies the valid ones in empire-ID and then
// submission order, runs the turn phases and produces S(t+1).
func ResolveTurn(w *World, submitted map[string][]Order) (TurnResult, error) {
	if w == nil {
		return TurnResult{}, fmt.Errorf("nil world")
	}
	// S(t) carries only the previous turn's events and the messages due now;
	// both were delivered with the observations, and the append-only turn log
	// keeps the history (D56).
	w.Events = nil
	var pending []Message
	for _, m := range w.Messages {
		if m.Turn > w.Turn {
			pending = append(pending, m)
		}
	}
	w.Messages = pending
	snap := cloneWorld(w)
	res := TurnResult{Turn: w.Turn, Submitted: submitted}
	firstEvent := len(w.Events)
	ids := make([]string, 0, len(submitted))
	for id := range submitted {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, e := range w.Empires {
		e.Rejected = nil
	}
	reject := func(submitter string, o Order, err error) {
		r := Rejection{Order: o, Reason: err.Error()}
		res.Rejected = append(res.Rejected, r)
		if e := w.Empires[submitter]; e != nil {
			e.Rejected = append(e.Rejected, r)
		}
	}
	var valid []Order
	for _, eid := range ids {
		for _, o := range submitted[eid] {
			switch {
			case o.EmpireID != eid:
				reject(eid, o, fmt.Errorf("order empire %q does not match submitter %q", o.EmpireID, eid))
			case o.Type == "":
				reject(eid, o, fmt.Errorf("missing order type"))
			default:
				if err := validateOrder(snap, o); err != nil {
					reject(eid, o, err)
				} else {
					valid = append(valid, o)
				}
			}
		}
	}
	// Every validation above saw the same snapshot. Orders only touch their
	// own empire's assets, so re-validating against the live world only
	// catches same-empire conflicts (double-spent resources, a second queue or
	// mission for the same planet/fleet) and never another empire's orders.
	for _, o := range valid {
		if err := validateOrder(w, o); err != nil {
			reject(o.EmpireID, o, err)
			continue
		}
		if err := applyOrder(w, o); err != nil {
			reject(o.EmpireID, o, err)
			continue
		}
		res.Accepted = append(res.Accepted, o)
	}
	advanceFleets(w)
	progressQueues(w)
	resolveArrivals(w)
	produce(w)
	updateSovereignty(w)
	w.Turn++
	res.Events = append(res.Events, w.Events[firstEvent:]...)
	res.StateHash = StateHash(w)
	return res, nil
}

// cloneWorld copies what validation reads and application mutates in place.
func cloneWorld(w *World) *World {
	c := *w
	c.Planets = map[string]*Planet{}
	for id, p := range w.Planets {
		x := *p
		c.Planets[id] = &x
	}
	c.Empires = map[string]*Empire{}
	for id, e := range w.Empires {
		x := *e
		c.Empires[id] = &x
	}
	c.Fleets = map[string]*Fleet{}
	for id, f := range w.Fleets {
		x := *f
		c.Fleets[id] = &x
	}
	c.Alliances = map[string]*Alliance{}
	for id, a := range w.Alliances {
		x := *a
		x.Members = append([]string(nil), a.Members...)
		x.Invited = append([]string(nil), a.Invited...)
		c.Alliances[id] = &x
	}
	return &c
}

// routeFuel is the deuterium a fleet pays to cross edges route edges:
// sum(ship fuel/edge) x edges, -5% per Propulsion level, rounded up.
func routeFuel(ships Ships, edges, propulsion int) int {
	fuel := 0
	for k, n := range ships {
		fuel += ShipSpecs[k].Fuel * n
	}
	fuel *= max(0, edges)
	discount := max(0, 100-5*propulsion)
	return (fuel*discount + 99) / 100
}

func cargoCapacity(ships Ships) int {
	room := 0
	for k, n := range ships {
		room += ShipSpecs[k].Cargo * n
	}
	return room
}

// FuelAvailable reports whether fleet f can pay the fuel to reach the target
// system or planet from where it is, under the same rules launch applies.
func FuelAvailable(w *World, f *Fleet, target string) bool {
	if p := w.Planets[target]; p != nil {
		target = p.SystemID
	}
	route := shortest(w, f.SystemID, target)
	if len(route) == 0 || w.Empires[f.OwnerID] == nil {
		return false
	}
	fuel := routeFuel(f.Ships, len(route)-1, w.Empires[f.OwnerID].Tech.Propulsion)
	if fuel == 0 || f.Cargo.Deuterium >= fuel {
		return true
	}
	src := ownedPlanetAt(w, f.OwnerID, f.SystemID)
	return src != nil && src.Resources.Deuterium >= fuel
}

func shortest(w *World, a, b string) []string {
	if a == b {
		return []string{a}
	}
	q := []string{a}
	prev := map[string]string{a: ""}
	for len(q) > 0 {
		c := q[0]
		q = q[1:]
		for _, n := range w.Systems[c].Neighbors {
			if _, ok := prev[n]; ok {
				continue
			}
			prev[n] = c
			if n == b {
				path := []string{b}
				for x := c; x != ""; x = prev[x] {
					path = append(path, x)
				}
				for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
					path[i], path[j] = path[j], path[i]
				}
				return path
			}
			q = append(q, n)
		}
	}
	return nil
}
func advanceFleets(w *World) {
	ids := fleetIDs(w)
	for _, id := range ids {
		f := w.Fleets[id]
		if len(f.Route) < 2 {
			continue
		}
		speed := 1 + w.Empires[f.OwnerID].Tech.Propulsion/3
		for i := 0; i < speed && f.RouteIndex < len(f.Route)-1; i++ {
			f.RouteIndex++
			f.SystemID = f.Route[f.RouteIndex]
		}
	}
}
func resolveArrivals(w *World) {
	captured := map[string]bool{}
	var recyclers []*Fleet
	for _, id := range fleetIDs(w) {
		f := w.Fleets[id]
		if len(f.Route) == 0 || f.RouteIndex != len(f.Route)-1 {
			continue
		}
		prev := ""
		if len(f.Route) >= 2 {
			prev = f.Route[len(f.Route)-2]
		}
		mission := f.Mission
		f.Route = nil
		f.RouteIndex = 0
		f.Mission = ""
		if f.Ships.Count() == 0 { // destroyed by an earlier battle this turn
			continue
		}
		switch mission {
		case OrderMove:
			w.emit(Event{Type: "fleet_arrived", EmpireID: f.OwnerID, Target: f.ID, Detail: "at " + f.SystemID})
		case OrderTransport:
			if w.Planets[f.Target] != nil {
				deliver(w, f)
			} else {
				w.emit(Event{Type: "fleet_arrived", EmpireID: f.OwnerID, Target: f.ID, Detail: "at " + f.SystemID + " with cargo"})
			}
		case OrderColonize:
			colonize(w, f)
		case OrderAttack:
			if t := w.Fleets[f.Target]; t != nil {
				fleetCombat(w, f, t, prev)
			} else if w.Planets[f.Target] != nil {
				combat(w, f, prev, captured)
			} else {
				w.emit(Event{Type: "attack_target_lost", EmpireID: f.OwnerID, Target: f.Target, Detail: f.ID + " found nothing to attack"})
			}
		case OrderSpy:
			spy(w, f)
		case OrderRecycle:
			recyclers = append(recyclers, f)
		}
	}
	recycle(w, recyclers)
	pruneEmptyFleets(w)
}

// deliver unloads a transport at its target planet. Cargo is physical: it goes
// to whoever owns the planet on arrival, which may be another empire (trade or
// aid, spec/game.md "Resource transfer and trade", D22). Nothing is unloaded on
// an unowned planet.
func deliver(w *World, f *Fleet) {
	p := w.Planets[f.Target]
	if p == nil || p.OwnerID == "" || p.SystemID != f.SystemID {
		w.emit(Event{Type: "transport_failed", EmpireID: f.OwnerID, Target: f.Target, Detail: f.ID + " found no recipient"})
		return
	}
	c := f.Cargo
	p.Resources = p.Resources.Add(c)
	f.Cargo = Resources{}
	amount := c.Metal + c.Crystal + c.Deuterium
	detail := fmt.Sprintf("metal=%d crystal=%d deuterium=%d", c.Metal, c.Crystal, c.Deuterium)
	if p.OwnerID == f.OwnerID {
		w.emit(Event{Type: "cargo_delivered", EmpireID: f.OwnerID, Target: p.ID, Detail: detail})
		return
	}
	w.Empires[f.OwnerID].Stats.ResourcesSent += amount
	w.Empires[p.OwnerID].Stats.ResourcesReceived += amount
	w.emit(Event{Type: "transfer", EmpireID: f.OwnerID, Target: p.ID, Other: p.OwnerID, Detail: detail})
}

func progressQueues(w *World) {
	for _, id := range sortedKeys(w.Planets) {
		p := w.Planets[id]
		if p.OwnerID == "" {
			continue
		}
		if q := p.Construction; q != nil {
			q.Progress += max(1, p.Buildings.Infrastructure)
			if q.Progress >= q.Required {
				incBuilding(&p.Buildings, q.Kind)
				p.Construction = nil
				w.emit(Event{Type: "construction_complete", EmpireID: p.OwnerID, Target: p.ID, Detail: fmt.Sprintf("%s L%d", q.Kind, q.Level)})
			}
		}
		if q := p.ShipyardQueue; q != nil {
			q.Progress += max(1, p.Buildings.Shipyard)
			if q.Progress >= q.Required {
				p.Ships[q.Kind] += q.Quantity
				p.ShipyardQueue = nil
				w.Empires[p.OwnerID].Stats.ShipsBuilt += q.Quantity
				w.emit(Event{Type: "ships_built", EmpireID: p.OwnerID, Target: p.ID, Detail: fmt.Sprintf("%d %s", q.Quantity, q.Kind)})
			}
		}
	}
	for _, id := range sortedKeys(w.Empires) {
		e := w.Empires[id]
		if e.Research == nil {
			continue
		}
		pts, owned := 0, 0
		for _, p := range w.Planets {
			if p.OwnerID == e.ID {
				pts += p.Buildings.ResearchLab
				owned++
			}
		}
		if owned == 0 { // research needs a planet; a planetless empire's queue waits
			continue
		}
		e.Research.Progress += max(1, pts)
		if e.Research.Progress >= e.Research.Required {
			incTech(&e.Tech, e.Research.Kind)
			w.emit(Event{Type: "research_complete", EmpireID: e.ID, Detail: fmt.Sprintf("%s L%d", e.Research.Kind, e.Research.Level)})
			e.Research = nil
		}
	}
}

// produce pays every owned planet its yield and records one production
// event per empire.
func produce(w *World) {
	total := map[string]Resources{}
	for _, id := range sortedKeys(w.Planets) {
		p := w.Planets[id]
		if p.OwnerID == "" {
			continue
		}
		pr := Production(w, p)
		p.Resources = p.Resources.Add(pr)
		total[p.OwnerID] = total[p.OwnerID].Add(pr)
	}
	for _, id := range sortedKeys(total) {
		t := total[id]
		w.emit(Event{Type: "production", EmpireID: id, Detail: fmt.Sprintf("metal=%d crystal=%d deuterium=%d", t.Metal, t.Crystal, t.Deuterium)})
	}
}

// Production is what an owned planet yields per turn (spec/game.md, Economy).
func Production(w *World, p *Planet) Resources {
	e := w.Empires[p.OwnerID]
	if e == nil {
		return Resources{}
	}
	mul := 100 + 10*e.Tech.Industry
	return Resources{30 * p.Buildings.MetalMine * mul / 100, 20 * p.Buildings.CrystalMine * mul / 100, 12 * p.Buildings.DeuteriumExtractor * mul / 100}
}

func colonize(w *World, f *Fleet) {
	p := w.Planets[f.Target]
	e := w.Empires[f.OwnerID]
	if p == nil || p.OwnerID != "" || f.Ships[ShipColonyArk] < 1 || planetCount(w, e.ID) >= 1+e.Tech.Colonization {
		w.emit(Event{Type: "colonize_failed", EmpireID: f.OwnerID, Target: f.Target, Detail: f.ID})
		return
	}
	f.Ships[ShipColonyArk]--
	if f.Ships[ShipColonyArk] == 0 {
		delete(f.Ships, ShipColonyArk)
	}
	p.OwnerID = e.ID
	p.Buildings.Infrastructure = 1
	if f.Ships.Count() == 0 { // the fleet is about to be pruned: its cargo lands with the colonists
		p.Resources = p.Resources.Add(f.Cargo)
		f.Cargo = Resources{}
	}
	e.Stats.Colonies++
	w.emit(Event{Type: "colonized", EmpireID: e.ID, Target: p.ID})
}

// recycle resolves every recycle mission that ended this turn. Recyclers in
// the same system share its debris in proportion to their capacity, the
// remainder going one unit at a time in a seeded order (spec/game.md, Debris
// and recycling). Capacity is recycler cargo, bounded by free cargo room.
func recycle(w *World, fleets []*Fleet) {
	bySystem := map[string][]*Fleet{}
	for _, f := range fleets {
		bySystem[f.SystemID] = append(bySystem[f.SystemID], f)
	}
	for _, sid := range sortedKeys(bySystem) {
		s := w.Systems[sid]
		fs := bySystem[sid]
		caps := make([]int, len(fs))
		total := 0
		for i, f := range fs {
			load := f.Cargo.Metal + f.Cargo.Crystal + f.Cargo.Deuterium
			caps[i] = max(0, min(f.Ships[ShipRecycler]*ShipSpecs[ShipRecycler].Cargo, cargoCapacity(f.Ships)-load))
			total += caps[i]
		}
		debris := s.Debris.Metal + s.Debris.Crystal
		share := append([]int(nil), caps...)
		if total > debris {
			given := 0
			for i := range share {
				share[i] = debris * caps[i] / total
				given += share[i]
			}
			order := seededOrder(w, "recycle|"+sid, len(fs))
			for k := 0; given < debris; k = (k + 1) % len(order) {
				if i := order[k]; share[i] < caps[i] {
					share[i]++
					given++
				}
			}
		}
		for _, i := range seededOrder(w, "recycle-take|"+sid, len(fs)) {
			f := fs[i]
			m := min(share[i], s.Debris.Metal)
			c := min(share[i]-m, s.Debris.Crystal)
			s.Debris.Metal -= m
			s.Debris.Crystal -= c
			f.Cargo.Metal += m
			f.Cargo.Crystal += c
			w.Empires[f.OwnerID].Stats.DebrisCollected += m + c
			w.emit(Event{Type: "debris_collected", EmpireID: f.OwnerID, Target: sid, Detail: fmt.Sprintf("%s metal=%d crystal=%d", f.ID, m, c)})
		}
	}
}

// updateSovereignty applies spec/game.md "Elimination and government in
// exile" and D29: a planetless empire survives only with a viable Colony Ark,
// i.e. one already committed to a paid route, or an idle one that can pay
// (from carried deuterium) the fuel to reach some unowned planet it could
// colonize. Transitions are events; an eliminated empire leaves its alliance.
func updateSovereignty(w *World) {
	for _, id := range sortedKeys(w.Empires) {
		e := w.Empires[id]
		wasExile, wasEliminated := e.Exile, e.Eliminated
		if planetCount(w, e.ID) > 0 {
			e.Exile = false
			e.Eliminated = false
		} else {
			viable := false
			for _, fid := range fleetIDs(w) {
				if f := w.Fleets[fid]; f.OwnerID == e.ID && f.Ships[ShipColonyArk] > 0 && viableArk(w, f) {
					viable = true
					break
				}
			}
			e.Exile = viable
			e.Eliminated = !viable
		}
		switch {
		case e.Eliminated && !wasEliminated:
			e.Stats.EliminatedTurn = w.Turn + 1 // the first turn the empire is gone
			w.emit(Event{Type: "eliminated", EmpireID: e.ID})
			if e.AllianceID != "" {
				leaveAlliance(w, e, "eliminated")
			}
		case e.Exile && !wasExile:
			w.emit(Event{Type: "exiled", EmpireID: e.ID})
		case wasExile && !e.Exile && !e.Eliminated:
			w.emit(Event{Type: "restored", EmpireID: e.ID})
		}
	}
}

func viableArk(w *World, f *Fleet) bool {
	if len(f.Route) > 0 {
		return true
	}
	for _, id := range systemIDs(w) {
		for _, pid := range w.Systems[id].Planets {
			if p := w.Planets[pid]; p != nil && p.OwnerID == "" && FuelAvailable(w, f, id) {
				return true
			}
		}
	}
	return false
}
func planetCount(w *World, e string) int {
	n := 0
	for _, p := range w.Planets {
		if p.OwnerID == e {
			n++
		}
	}
	return n
}
func ownedPlanetAt(w *World, e, s string) *Planet {
	sys := w.Systems[s]
	if sys == nil {
		return nil
	}
	for _, id := range sys.Planets {
		if p := w.Planets[id]; p != nil && p.OwnerID == e {
			return p
		}
	}
	return nil
} // slot order, not map order: the paying planet must be deterministic
func fleetIDs(w *World) []string {
	a := make([]string, 0, len(w.Fleets))
	for id := range w.Fleets {
		a = append(a, id)
	}
	sort.Strings(a)
	return a
}
func intParam(o Order, k string, d int) int {
	if v, ok := o.Params[k].(float64); ok {
		return int(v)
	}
	if v, ok := o.Params[k].(int); ok {
		return v
	}
	return d
}
func stringParam(o Order, k string) string { v, _ := o.Params[k].(string); return v }
func boolParam(o Order, k string) bool     { v, _ := o.Params[k].(bool); return v }
func shipMapParam(o Order) Ships {
	out := Ships{}
	if m, ok := o.Params["ships"].(map[string]int); ok {
		for k, v := range m {
			out[k] = v
		}
	}
	if m, ok := o.Params["ships"].(map[string]any); ok {
		for k, v := range m {
			if n, ok := v.(float64); ok {
				out[k] = int(n)
			}
		}
	}
	return out
}
func resourceParam(o Order) Resources {
	return Resources{intParam(o, "metal", 0), intParam(o, "crystal", 0), intParam(o, "deuterium", 0)}
}
func takeShips(have, need Ships) bool {
	for k, n := range need {
		if n < 0 || have[k] < n {
			return false
		}
	}
	for k, n := range need {
		have[k] -= n
	}
	return true
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

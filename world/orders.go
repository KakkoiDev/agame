package world

import (
	"fmt"
	"sort"
	"strings"
)

// Limits on diplomatic free text so one ruler cannot flood the others'
// observations (spec/agents.md, Small-model budgets).
const (
	MaxMessageBytes      = 2000
	MaxAllianceNameBytes = 64
	BroadcastAddress     = "all"
)

// Rejection is an order the engine refused, with the reason in plain words.
type Rejection struct {
	Order
	Reason string `json:"reason"`
}

// ValidateOrder checks o against w without changing anything. It applies
// exactly the checks ResolveTurn applies to every submitted order against the
// frozen S(t), so a harness can tell a ruler which orders would be rejected
// and why before the decision barrier closes.
func ValidateOrder(w *World, o Order) error {
	if w == nil {
		return fmt.Errorf("no world")
	}
	return validateOrder(w, o)
}

func validateOrder(w *World, o Order) error {
	e := w.Empires[o.EmpireID]
	if e == nil {
		return fmt.Errorf("unknown empire %q", o.EmpireID)
	}
	if e.Eliminated {
		return fmt.Errorf("empire %s is eliminated", e.ID)
	}
	switch o.Type {
	case OrderConstruct:
		p, err := ownPlanet(w, o.EmpireID, o.Actor)
		if err != nil {
			return err
		}
		if q := p.Construction; q != nil {
			return fmt.Errorf("planet %s is already constructing %s L%d (cancel it first)", p.ID, q.Kind, q.Level)
		}
		base, ok := BuildingBase[o.Target]
		if !ok {
			return fmt.Errorf("unknown building %q (valid: %s)", o.Target, strings.Join(BuildingKinds(), ", "))
		}
		return afford(p, scale(base, buildingLevel(p.Buildings, o.Target)+1))
	case OrderResearch:
		if q := e.Research; q != nil {
			return fmt.Errorf("research queue busy with %s L%d (cancel it first)", q.Kind, q.Level)
		}
		base, ok := TechBase[o.Target]
		if !ok {
			return fmt.Errorf("unknown technology %q (valid: %s)", o.Target, strings.Join(TechKinds(), ", "))
		}
		p, err := ownPlanet(w, o.EmpireID, o.Actor)
		if err != nil {
			return err
		}
		return afford(p, scale(base, techLevel(e.Tech, o.Target)+1))
	case OrderBuildShips:
		p, err := ownPlanet(w, o.EmpireID, o.Actor)
		if err != nil {
			return err
		}
		if q := p.ShipyardQueue; q != nil {
			return fmt.Errorf("planet %s shipyard is busy with %d %s (cancel it first)", p.ID, q.Quantity, q.Kind)
		}
		if _, ok := ShipSpecs[o.Target]; !ok {
			return fmt.Errorf("unknown ship class %q (valid: %s)", o.Target, strings.Join(ShipKinds(), ", "))
		}
		n := intParam(o, "quantity", 1)
		cost, err := shipCost(o.Target, n)
		if err != nil {
			return fmt.Errorf("quantity must be 1..%d, got %d", MaxShipBatch, n)
		}
		if o.Target == ShipColonyArk && e.Tech.Colonization < 1 {
			return fmt.Errorf("colony_ark requires colonization level 1")
		}
		return afford(p, cost)
	case OrderFormFleet:
		p, err := ownPlanet(w, o.EmpireID, o.Actor)
		if err != nil {
			return err
		}
		return checkShips(shipMapParam(o), p.Ships, "docked at "+p.ID)
	case OrderSplitFleet:
		f, err := idleFleet(w, o)
		if err != nil {
			return err
		}
		ships := shipMapParam(o)
		if err := checkShips(ships, f.Ships, "in fleet "+f.ID); err != nil {
			return err
		}
		rest := Ships{}
		for k, n := range f.Ships {
			if n-ships[k] > 0 {
				rest[k] = n - ships[k]
			}
		}
		if rest.Count() == 0 {
			return fmt.Errorf("split must leave at least one ship in fleet %s", f.ID)
		}
		if load := f.Cargo.Metal + f.Cargo.Crystal + f.Cargo.Deuterium; load > cargoCapacity(rest) {
			return fmt.Errorf("the ships left in fleet %s cannot hold its %d cargo", f.ID, load)
		}
	case OrderCancel:
		p, err := ownPlanet(w, o.EmpireID, o.Actor)
		if err != nil {
			return err
		}
		switch o.Target {
		case QueueConstruction:
			if p.Construction == nil {
				return fmt.Errorf("planet %s has no construction queue", p.ID)
			}
		case QueueShipyard:
			if p.ShipyardQueue == nil {
				return fmt.Errorf("planet %s has no shipyard queue", p.ID)
			}
		case QueueResearch:
			if e.Research == nil {
				return fmt.Errorf("no research queue")
			}
		default:
			return fmt.Errorf("cancel target must be construction, shipyard or research, got %q", o.Target)
		}
	case OrderMove, OrderAttack, OrderSpy, OrderTransport, OrderRecycle, OrderColonize:
		_, err := planLaunch(w, o)
		return err
	case OrderMessage:
		_, err := messageRecipients(w, o)
		return err
	case OrderAllianceCreate:
		if e.AllianceID != "" {
			return fmt.Errorf("already a member of alliance %s (leave it first)", e.AllianceID)
		}
		if n := stringParam(o, "name"); len(n) > MaxAllianceNameBytes {
			return fmt.Errorf("alliance name longer than %d bytes", MaxAllianceNameBytes)
		}
		for _, id := range stringsParam(o, "invite") {
			if err := invitable(w, o.EmpireID, id); err != nil {
				return err
			}
		}
	case OrderAllianceInvite:
		a := w.Alliances[e.AllianceID]
		if a == nil {
			return fmt.Errorf("not a member of any alliance")
		}
		if err := invitable(w, o.EmpireID, o.Target); err != nil {
			return err
		}
		if contains(a.Members, o.Target) {
			return fmt.Errorf("%s is already a member of %s", o.Target, a.ID)
		}
		if contains(a.Invited, o.Target) {
			return fmt.Errorf("%s is already invited to %s", o.Target, a.ID)
		}
	case OrderAllianceJoin:
		a := w.Alliances[o.Target]
		if a == nil {
			return fmt.Errorf("unknown alliance %q", o.Target)
		}
		if e.AllianceID != "" {
			return fmt.Errorf("already a member of alliance %s (leave it on an earlier turn first)", e.AllianceID)
		}
		if !contains(a.Invited, o.EmpireID) {
			return fmt.Errorf("not invited to alliance %s; a member must send alliance_invite first", a.ID)
		}
	case OrderAllianceLeave:
		if e.AllianceID == "" {
			return fmt.Errorf("not a member of any alliance")
		}
		if o.Target != "" && o.Target != e.AllianceID {
			return fmt.Errorf("not a member of alliance %q", o.Target)
		}
	default:
		return fmt.Errorf("unsupported order type %q", o.Type)
	}
	return nil
}

func applyOrder(w *World, o Order) error {
	e := w.Empires[o.EmpireID]
	switch o.Type {
	case OrderConstruct:
		p := w.Planets[o.Actor]
		level := buildingLevel(p.Buildings, o.Target) + 1
		cost := scale(BuildingBase[o.Target], level)
		p.Resources = p.Resources.Sub(cost)
		p.Construction = &Queue{Kind: o.Target, Level: level, Required: work(cost), Paid: cost}
		w.emit(Event{Type: "queued", EmpireID: o.EmpireID, Target: p.ID, Detail: fmt.Sprintf("construction %s L%d", o.Target, level)})
	case OrderResearch:
		p := w.Planets[o.Actor]
		level := techLevel(e.Tech, o.Target) + 1
		cost := scale(TechBase[o.Target], level)
		p.Resources = p.Resources.Sub(cost)
		e.Research = &Queue{Kind: o.Target, Level: level, Required: work(cost), Paid: cost}
		w.emit(Event{Type: "queued", EmpireID: o.EmpireID, Target: p.ID, Detail: fmt.Sprintf("research %s L%d", o.Target, level)})
	case OrderBuildShips:
		p := w.Planets[o.Actor]
		n := intParam(o, "quantity", 1)
		cost, _ := shipCost(o.Target, n)
		p.Resources = p.Resources.Sub(cost)
		p.ShipyardQueue = &Queue{Kind: o.Target, Quantity: n, Required: work(cost), Paid: cost}
		w.emit(Event{Type: "queued", EmpireID: o.EmpireID, Target: p.ID, Detail: fmt.Sprintf("shipyard %d %s", n, o.Target)})
	case OrderFormFleet:
		p := w.Planets[o.Actor]
		ships := shipMapParam(o)
		if !takeShips(p.Ships, ships) {
			return fmt.Errorf("ships unavailable")
		}
		f := w.newFleet(o.EmpireID, p.SystemID, ships)
		w.emit(Event{Type: "fleet_formed", EmpireID: o.EmpireID, Target: f.ID, Detail: "at " + p.ID + " " + shipsString(ships)})
	case OrderSplitFleet:
		f := w.Fleets[o.Actor]
		ships := shipMapParam(o)
		if !takeShips(f.Ships, ships) {
			return fmt.Errorf("ships unavailable")
		}
		for k, n := range f.Ships {
			if n == 0 {
				delete(f.Ships, k)
			}
		}
		nf := w.newFleet(o.EmpireID, f.SystemID, ships)
		w.emit(Event{Type: "fleet_split", EmpireID: o.EmpireID, Target: nf.ID, Detail: "from " + f.ID + " " + shipsString(ships)})
	case OrderCancel:
		p := w.Planets[o.Actor]
		var q *Queue
		switch o.Target {
		case QueueConstruction:
			q, p.Construction = p.Construction, nil
		case QueueShipyard:
			q, p.ShipyardQueue = p.ShipyardQueue, nil
		case QueueResearch:
			q, e.Research = e.Research, nil
		}
		refund := Resources{q.Paid.Metal / 2, q.Paid.Crystal / 2, q.Paid.Deuterium / 2}
		p.Resources = p.Resources.Add(refund)
		w.emit(Event{Type: "queue_cancelled", EmpireID: o.EmpireID, Target: p.ID,
			Detail: fmt.Sprintf("%s %s refund=%d/%d/%d", o.Target, q.Kind, refund.Metal, refund.Crystal, refund.Deuterium)})
	case OrderMove, OrderAttack, OrderSpy, OrderTransport, OrderRecycle, OrderColonize:
		plan, err := planLaunch(w, o)
		if err != nil {
			return err
		}
		applyLaunch(w, o, plan)
	case OrderMessage:
		rcpt, err := messageRecipients(w, o)
		if err != nil {
			return err
		}
		w.Messages = append(w.Messages, Message{Turn: w.Turn + 1, From: o.EmpireID, To: o.Target, Recipients: rcpt, Body: stringParam(o, "body"), Major: boolParam(o, "major")})
		e.Stats.MessagesSent++
		w.emit(Event{Type: "message_sent", EmpireID: o.EmpireID, Target: o.Target, Detail: "to " + strings.Join(rcpt, ",") + ": " + stringParam(o, "body")})
	case OrderAllianceCreate:
		createAlliance(w, e, stringParam(o, "name"), stringsParam(o, "invite"))
	case OrderAllianceInvite:
		a := w.Alliances[e.AllianceID]
		a.Invited = sortedSet(append(append([]string(nil), a.Invited...), o.Target))
		w.emit(Event{Type: "alliance_invited", EmpireID: o.EmpireID, Target: a.ID, Other: o.Target})
	case OrderAllianceJoin:
		a := w.Alliances[o.Target]
		a.Invited = without(a.Invited, o.EmpireID)
		a.Members = sortedSet(append(append([]string(nil), a.Members...), o.EmpireID))
		e.AllianceID = a.ID
		e.Stats.AlliancesJoined++
		w.emit(Event{Type: "alliance_joined", EmpireID: o.EmpireID, Target: a.ID})
	case OrderAllianceLeave:
		leaveAlliance(w, e, "left")
	}
	return nil
}

func (w *World) newFleet(owner, system string, ships Ships) *Fleet {
	id := fmt.Sprintf("f%06d", w.NextFleet)
	w.NextFleet++
	f := &Fleet{ID: id, OwnerID: owner, SystemID: system, Ships: ships}
	w.Fleets[id] = f
	return f
}

// emit appends an event stamped with the current turn.
func (w *World) emit(e Event) {
	e.Turn = w.Turn
	w.Events = append(w.Events, e)
}

func ownPlanet(w *World, eid, id string) (*Planet, error) {
	p := w.Planets[id]
	if p == nil {
		return nil, fmt.Errorf("unknown planet %q", id)
	}
	if p.OwnerID != eid {
		return nil, fmt.Errorf("planet %s is not yours", id)
	}
	return p, nil
}

func afford(p *Planet, cost Resources) error {
	if !p.Resources.Enough(cost) {
		r := p.Resources
		return fmt.Errorf("planet %s has %d/%d/%d metal/crystal/deuterium, needs %d/%d/%d", p.ID, r.Metal, r.Crystal, r.Deuterium, cost.Metal, cost.Crystal, cost.Deuterium)
	}
	return nil
}

func checkShips(want, have Ships, where string) error {
	if len(want) == 0 {
		return fmt.Errorf("params.ships must name at least one ship class with a count")
	}
	for _, k := range sortedKinds(want) {
		n := want[k]
		if _, ok := ShipSpecs[k]; !ok {
			return fmt.Errorf("unknown ship class %q", k)
		}
		if n < 1 {
			return fmt.Errorf("ship count for %s must be >= 1", k)
		}
		if have[k] < n {
			return fmt.Errorf("only %d %s %s, asked for %d", have[k], k, where, n)
		}
	}
	return nil
}

func idleFleet(w *World, o Order) (*Fleet, error) {
	f := w.Fleets[o.Actor]
	if f == nil || f.OwnerID != o.EmpireID {
		return nil, fmt.Errorf("fleet %q is not one of your fleets", o.Actor)
	}
	if len(f.Route) > 0 {
		if f.Mission != "" && f.RouteIndex == 0 {
			return nil, fmt.Errorf("fleet %s already has a %s mission this turn", f.ID, f.Mission)
		}
		return nil, fmt.Errorf("fleet %s is in transit (%s to %s)", f.ID, f.Mission, f.Target)
	}
	if f.Ships.Count() == 0 {
		return nil, fmt.Errorf("fleet %s has no ships", f.ID)
	}
	return f, nil
}

// launchPlan is everything a fleet mission needs, computed without side
// effects so validation and application share one code path.
type launchPlan struct {
	route      []string
	cargo      Resources
	fromPlanet Resources
	fromCargo  int
	source     *Planet
	victim     string
}

// planLaunch checks a fleet mission. Fuel is paid at departure (spec/game.md,
// Fleets and movement) by the empire's own planet in the fleet's system when
// that planet can cover fuel and any cargo; otherwise by deuterium the fleet
// already carries as cargo (D49), so a fleet stranded away from its planets
// can still leave. Cargo itself is always loaded from the planet.
func planLaunch(w *World, o Order) (launchPlan, error) {
	var plan launchPlan
	f, err := idleFleet(w, o)
	if err != nil {
		return plan, err
	}
	dest := ""
	switch o.Type {
	case OrderMove:
		if w.Systems[o.Target] == nil {
			return plan, fmt.Errorf("unknown system %q", o.Target)
		}
		if o.Target == f.SystemID {
			return plan, fmt.Errorf("fleet %s is already in %s", f.ID, o.Target)
		}
		dest = o.Target
	case OrderAttack:
		if p := w.Planets[o.Target]; p != nil {
			if p.OwnerID == "" {
				return plan, fmt.Errorf("planet %s is unowned; use colonize", p.ID)
			}
			if p.OwnerID == o.EmpireID {
				return plan, fmt.Errorf("planet %s is your own", p.ID)
			}
			dest, plan.victim = p.SystemID, p.OwnerID
		} else if t := w.Fleets[o.Target]; t != nil {
			if t.OwnerID == o.EmpireID {
				return plan, fmt.Errorf("fleet %s is your own", t.ID)
			}
			dest, plan.victim = t.SystemID, t.OwnerID
		} else {
			return plan, fmt.Errorf("attack target %q is neither a planet nor a fleet", o.Target)
		}
	case OrderSpy:
		if f.Ships[ShipScout] == 0 {
			return plan, fmt.Errorf("spying needs a scout in fleet %s", f.ID)
		}
		p := w.Planets[o.Target]
		if p == nil {
			return plan, fmt.Errorf("unknown planet %q", o.Target)
		}
		if p.OwnerID == o.EmpireID {
			return plan, fmt.Errorf("planet %s is your own", p.ID)
		}
		if p.SystemID != f.SystemID && !adjacent(w, f.SystemID, p.SystemID) {
			return plan, fmt.Errorf("planet %s is not in or adjacent to %s", p.ID, f.SystemID)
		}
		dest = f.SystemID // scouts observe from where they are (D53)
	case OrderTransport:
		// A planet target unloads on arrival; a system target only ferries
		// the cargo there (e.g. fuel for a stranded fleet) (D55).
		if p := w.Planets[o.Target]; p != nil {
			if p.OwnerID == "" {
				return plan, fmt.Errorf("planet %s is unowned; nobody can receive cargo there", p.ID)
			}
			dest = p.SystemID
		} else if w.Systems[o.Target] != nil {
			dest = o.Target
		} else {
			return plan, fmt.Errorf("unknown planet or system %q", o.Target)
		}
		plan.cargo = resourceParam(o)
		c := plan.cargo
		room := cargoCapacity(f.Ships) - (f.Cargo.Metal + f.Cargo.Crystal + f.Cargo.Deuterium)
		if c.Metal < 0 || c.Crystal < 0 || c.Deuterium < 0 {
			return plan, fmt.Errorf("cargo amounts must be >= 0")
		}
		if c.Metal > room || c.Crystal > room || c.Deuterium > room || c.Metal+c.Crystal+c.Deuterium > room {
			return plan, fmt.Errorf("fleet %s has room for %d cargo", f.ID, max(0, room))
		}
	case OrderRecycle:
		if w.Systems[o.Target] == nil {
			return plan, fmt.Errorf("unknown system %q", o.Target)
		}
		if f.Ships[ShipRecycler] == 0 {
			return plan, fmt.Errorf("recycling needs a recycler in fleet %s", f.ID)
		}
		dest = o.Target
	case OrderColonize:
		if f.Ships[ShipColonyArk] == 0 {
			return plan, fmt.Errorf("colonizing needs a colony_ark in fleet %s", f.ID)
		}
		p := w.Planets[o.Target]
		if p == nil {
			return plan, fmt.Errorf("unknown planet %q", o.Target)
		}
		if p.OwnerID != "" {
			return plan, fmt.Errorf("planet %s is already owned by %s", p.ID, p.OwnerID)
		}
		e := w.Empires[o.EmpireID]
		if n := planetCount(w, e.ID); n >= 1+e.Tech.Colonization {
			return plan, fmt.Errorf("at the sustainable colony limit (%d planets, colonization %d allows %d)", n, e.Tech.Colonization, 1+e.Tech.Colonization)
		}
		dest = p.SystemID
	}
	plan.route = shortest(w, f.SystemID, dest)
	if len(plan.route) == 0 {
		return plan, fmt.Errorf("no route from %s to %s", f.SystemID, dest)
	}
	fuel := routeFuel(f.Ships, len(plan.route)-1, w.Empires[o.EmpireID].Tech.Propulsion)
	plan.source = ownedPlanetAt(w, o.EmpireID, f.SystemID)
	plan.fromPlanet = plan.cargo
	plan.fromPlanet.Deuterium += fuel
	if plan.source == nil || !plan.source.Resources.Enough(plan.fromPlanet) {
		if f.Cargo.Deuterium < fuel {
			return plan, fmt.Errorf("needs %d deuterium fuel; no own planet in %s can pay it and the fleet carries %d", fuel, f.SystemID, f.Cargo.Deuterium)
		}
		plan.fromPlanet, plan.fromCargo = plan.cargo, fuel
	}
	if plan.fromPlanet != (Resources{}) { // a free launch (e.g. colonizing in place) needs no paying planet
		if plan.source == nil {
			return plan, fmt.Errorf("cargo must be loaded from an own planet in %s", f.SystemID)
		}
		if err := afford(plan.source, plan.fromPlanet); err != nil {
			return plan, err
		}
	}
	return plan, nil
}

func applyLaunch(w *World, o Order, plan launchPlan) {
	f := w.Fleets[o.Actor]
	if plan.fromPlanet != (Resources{}) {
		plan.source.Resources = plan.source.Resources.Sub(plan.fromPlanet)
	}
	f.Cargo.Deuterium -= plan.fromCargo
	f.Cargo = f.Cargo.Add(plan.cargo)
	f.Route = plan.route
	f.RouteIndex = 0
	f.Mission = o.Type
	f.Target = o.Target
	w.emit(Event{Type: "fleet_departed", EmpireID: o.EmpireID, Target: f.ID,
		Detail: fmt.Sprintf("%s %s via %s", o.Type, o.Target, strings.Join(plan.route, ">"))})
	if o.Type == OrderAttack {
		breachIfAllied(w, o.EmpireID, plan.victim, "attack on "+o.Target)
	}
}

// messageRecipients resolves a message address: an empire id, an alliance id
// (its members when sent) or "all" (every non-eliminated empire). The sender
// and eliminated empires never receive.
func messageRecipients(w *World, o Order) ([]string, error) {
	body := stringParam(o, "body")
	if strings.TrimSpace(body) == "" {
		return nil, fmt.Errorf("params.body must be a non-empty string")
	}
	if len(body) > MaxMessageBytes {
		return nil, fmt.Errorf("message body longer than %d bytes", MaxMessageBytes)
	}
	var cand []string
	switch {
	case o.Target == BroadcastAddress:
		cand = sortedKeys(w.Empires)
	case w.Alliances[o.Target] != nil:
		cand = w.Alliances[o.Target].Members
	case w.Empires[o.Target] != nil:
		if o.Target == o.EmpireID {
			return nil, fmt.Errorf("cannot message yourself")
		}
		cand = []string{o.Target}
	default:
		return nil, fmt.Errorf("unknown recipient %q (use an empire id, an alliance id or %q)", o.Target, BroadcastAddress)
	}
	var out []string
	for _, id := range cand {
		if id != o.EmpireID && !w.Empires[id].Eliminated {
			out = append(out, id)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no living recipient at %q", o.Target)
	}
	return out, nil
}

func stringsParam(o Order, k string) []string {
	switch v := o.Params[k].(type) {
	case []string:
		return append([]string(nil), v...)
	case []any:
		var out []string
		for _, x := range v {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		if v != "" {
			return []string{v}
		}
	}
	return nil
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

func without(xs []string, x string) []string {
	var out []string
	for _, y := range xs {
		if y != x {
			out = append(out, y)
		}
	}
	return out
}

func sortedSet(xs []string) []string {
	sort.Strings(xs)
	var out []string
	for i, x := range xs {
		if i == 0 || x != xs[i-1] {
			out = append(out, x)
		}
	}
	return out
}

func shipsString(s Ships) string {
	parts := make([]string, 0, len(s))
	for _, k := range sortedKinds(s) {
		parts = append(parts, fmt.Sprintf("%s=%d", k, s[k]))
	}
	return strings.Join(parts, ",")
}

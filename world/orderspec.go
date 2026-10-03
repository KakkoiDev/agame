package world

import (
	"sort"
	"strings"
)

// OrderSpec describes one legal order type and the shape of its fields, so
// rulers can build orders without guessing (spec/agents.md, Order schema).
type OrderSpec struct {
	Type   string            `json:"type"`
	Actor  string            `json:"actor"`
	Target string            `json:"target,omitempty"`
	Params map[string]string `json:"params,omitempty"`
	Note   string            `json:"note,omitempty"`
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// BuildingKinds, TechKinds and ShipKinds list the valid order targets in a
// stable order.
func BuildingKinds() []string { return sortedKeys(BuildingBase) }
func TechKinds() []string     { return sortedKeys(TechBase) }
func ShipKinds() []string     { return sortedKeys(ShipSpecs) }

// OrderSpecs returns the order types the engine accepts, in a fixed order.
// The result is freshly allocated on every call.
func OrderSpecs() []OrderSpec {
	join := func(xs []string) string { return strings.Join(xs, "|") }
	buildings, techs, ships := join(BuildingKinds()), join(TechKinds()), join(ShipKinds())
	return []OrderSpec{
		{Type: OrderConstruct, Actor: "own planet id", Target: buildings, Note: "pays next level from the planet; one construction queue per planet"},
		{Type: OrderResearch, Actor: "own planet id (pays)", Target: techs, Note: "one research queue per empire"},
		{Type: OrderBuildShips, Actor: "own planet id", Target: ships, Params: map[string]string{"quantity": "int >= 1"}, Note: "one shipyard queue per planet; colony_ark needs colonization 1"},
		{Type: OrderCancel, Actor: "own planet id (receives refund)", Target: "construction|shipyard|research", Note: "refunds 50% of the paid cost; progress is lost"},
		{Type: OrderFormFleet, Actor: "own planet id", Params: map[string]string{"ships": "{ship class: int >= 1} docked at the planet"}, Note: "creates an idle fleet in the planet's system"},
		{Type: OrderSplitFleet, Actor: "own idle fleet id", Params: map[string]string{"ships": "{ship class: int >= 1} to move into a new fleet"}, Note: "must leave a ship behind; cargo stays with the original fleet"},
		{Type: OrderMove, Actor: "own idle fleet id", Target: "system id", Note: "fuel (deuterium) is paid at departure by an own planet in the fleet's system, else from deuterium the fleet carries"},
		{Type: OrderAttack, Actor: "own idle fleet id", Target: "foreign planet id or foreign fleet id", Note: "capture needs a surviving frigate or cruiser; attacking an ally breaks your alliance"},
		{Type: OrderSpy, Actor: "own idle fleet id with a scout", Target: "planet id in or adjacent to the fleet's system", Note: "the report arrives next turn; the owner may detect you"},
		{Type: OrderTransport, Actor: "own idle fleet id", Target: "owned planet id (yours or another empire's) or system id", Params: map[string]string{"metal": "int >= 0", "crystal": "int >= 0", "deuterium": "int >= 0"}, Note: "cargo is loaded from an own planet in the fleet's system and unloaded to the planet's owner on arrival"},
		{Type: OrderRecycle, Actor: "own idle fleet id with a recycler", Target: "system id", Note: "collects debris up to recycler capacity"},
		{Type: OrderColonize, Actor: "own idle fleet id with a colony_ark", Target: "unowned planet id", Note: "planets allowed = 1 + colonization level"},
		{Type: OrderMessage, Actor: "", Target: "empire id, alliance id or all", Params: map[string]string{"body": "string", "major": "bool"}, Note: "delivered next turn"},
		{Type: OrderAllianceCreate, Actor: "", Params: map[string]string{"name": "string", "invite": "[empire id]"}, Note: "you become its first member; one alliance per empire"},
		{Type: OrderAllianceInvite, Actor: "", Target: "empire id", Note: "invites into your alliance; they may join from next turn"},
		{Type: OrderAllianceJoin, Actor: "", Target: "alliance id", Note: "needs an invitation; leave your current alliance on an earlier turn"},
		{Type: OrderAllianceLeave, Actor: "", Target: "your alliance id (optional)"},
	}
}

package world

type Resources struct{ Metal, Crystal, Deuterium int }

func (r Resources) Add(x Resources) Resources {
	return Resources{r.Metal + x.Metal, r.Crystal + x.Crystal, r.Deuterium + x.Deuterium}
}
func (r Resources) Enough(x Resources) bool {
	return r.Metal >= x.Metal && r.Crystal >= x.Crystal && r.Deuterium >= x.Deuterium
}
func (r Resources) Sub(x Resources) Resources {
	return Resources{r.Metal - x.Metal, r.Crystal - x.Crystal, r.Deuterium - x.Deuterium}
}

type Buildings struct {
	MetalMine, CrystalMine, DeuteriumExtractor         int
	Infrastructure, ResearchLab, Shipyard, DefenseGrid int
}
type Tech struct{ Industry, Propulsion, Weapons, Shields, Sensors, Colonization int }
type Queue struct {
	Kind                                string
	Level, Quantity, Progress, Required int
	Paid                                Resources
}
type Ships map[string]int
type Planet struct {
	ID, SystemID, OwnerID       string
	Slot                        int
	Homeworld                   bool
	Resources                   Resources
	Buildings                   Buildings
	Construction, ShipyardQueue *Queue
	Ships                       Ships
}
type System struct {
	ID                 string
	Neighbors, Planets []string
	Debris             Resources
}
type Fleet struct {
	ID, OwnerID, SystemID string
	Ships                 Ships
	Cargo                 Resources
	Route                 []string
	RouteIndex            int
	Mission, Target       string
}
type Empire struct {
	ID, Name, HomeworldID string
	Eliminated, Exile     bool
	Tech                  Tech
	Research              *Queue
	AllianceID            string
	// Rejected holds this empire's orders the engine rejected on the most
	// recently resolved turn, with reasons, so the ruler can learn from them.
	Rejected []Rejection `json:",omitempty"`
	Stats    Stats
}

// Stats are raw engine-recorded measurements (spec/benchmark.md, Metrics).
type Stats struct {
	ShipsBuilt, ShipsLost, ShipsDestroyed int
	Battles, Captures, PlanetsLost        int
	Colonies, SpyMissions, Detected       int
	MessagesSent, AlliancesJoined         int
	Breaches                              int
	ResourcesSent, ResourcesReceived      int
	DebrisCollected                       int
	// EliminatedTurn is the turn the empire was eliminated, 0 while alive.
	EliminatedTurn int
}

// Alliance is an engine-recognised alliance (spec/game.md, Diplomacy
// mechanics). Membership is public; Invited lists empires that may join.
type Alliance struct {
	ID, Name, Founder string
	Founded           int
	Members, Invited  []string
}

// Message is a diplomatic message. To is the address the sender used (an
// empire id, an alliance id or "all"); Recipients are the empires it was
// resolved to when sent. It is delivered on Turn (spec/diplomacy.md).
type Message struct {
	Turn           int
	From, To, Body string
	Recipients     []string `json:",omitempty"`
	Major          bool
}
type Event struct {
	Turn     int    `json:"turn"`
	Type     string `json:"type,omitempty"`
	EmpireID string `json:"empire_id,omitempty"`
	Target   string `json:"target,omitempty"`
	// Other is the counterpart empire (defender, recipient, victim...).
	Other  string     `json:"other,omitempty"`
	Detail string     `json:"detail,omitempty"`
	Report *SpyReport `json:"report,omitempty"`
}
type World struct {
	Seed      int64
	Turn      int
	Systems   map[string]*System
	Planets   map[string]*Planet
	Empires   map[string]*Empire
	Fleets    map[string]*Fleet
	Alliances map[string]*Alliance `json:",omitempty"`
	// Hostilities maps an empire pair "a|b" (a < b) to the last turn they
	// fought. It is a record of combat, not a declared state (D52).
	Hostilities  map[string]int `json:",omitempty"`
	Messages     []Message
	Events       []Event
	NextFleet    int
	NextAlliance int `json:",omitempty"`
}

// Order types (spec/agents.md). Fleet missions reuse the fleet order names.
const (
	OrderConstruct  = "construct"
	OrderResearch   = "research"
	OrderBuildShips = "build_ships"
	OrderFormFleet  = "form_fleet"
	OrderMove       = "move"
	OrderAttack     = "attack"
	OrderSpy        = "spy"
	OrderTransport  = "transport"
	OrderRecycle    = "recycle"
	OrderColonize   = "colonize"
	OrderMessage    = "message"

	OrderSplitFleet     = "split_fleet"
	OrderCancel         = "cancel"
	OrderAllianceCreate = "alliance_create"
	OrderAllianceInvite = "alliance_invite"
	OrderAllianceJoin   = "alliance_join"
	OrderAllianceLeave  = "alliance_leave"
)

// Queue names accepted by the cancel order.
const (
	QueueConstruction = "construction"
	QueueShipyard     = "shipyard"
	QueueResearch     = "research"
)

// Ship classes (spec/game.md, Ships).
const (
	ShipScout     = "scout"
	ShipTransport = "transport"
	ShipColonyArk = "colony_ark"
	ShipFrigate   = "frigate"
	ShipCruiser   = "cruiser"
	ShipRecycler  = "recycler"
)

// Count returns the total number of ships.
func (s Ships) Count() int {
	n := 0
	for _, c := range s {
		n += c
	}
	return n
}

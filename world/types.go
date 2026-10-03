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
}
type Message struct {
	Turn           int
	From, To, Body string
	Major          bool
}
type Event struct {
	Turn     int    `json:"turn"`
	Type     string `json:"type,omitempty"`
	EmpireID string `json:"empire_id,omitempty"`
	Target   string `json:"target,omitempty"`
	Detail   string `json:"detail,omitempty"`
}
type World struct {
	Seed      int64
	Turn      int
	Systems   map[string]*System
	Planets   map[string]*Planet
	Empires   map[string]*Empire
	Fleets    map[string]*Fleet
	Messages  []Message
	Events    []Event
	NextFleet int
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

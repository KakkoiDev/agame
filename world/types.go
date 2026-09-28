package world

type Resources struct {
	Metal, Crystal, Deuterium int
}

type Buildings struct {
	MetalMine, CrystalMine, DeuteriumExtractor int
	Infrastructure, ResearchLab, Shipyard      int
	DefenseGrid                                int
}

type Planet struct {
	ID        string
	SystemID  string
	Slot      int
	OwnerID   string
	Homeworld bool
	Resources Resources
	Buildings Buildings
}

type System struct {
	ID        string
	Neighbors []string
	Planets   []string
}

type Empire struct {
	ID          string
	Name        string
	HomeworldID string
	Eliminated  bool
}

type World struct {
	Seed     int64
	Turn     int
	Systems  map[string]*System
	Planets  map[string]*Planet
	Empires  map[string]*Empire
}

package world

import (
	"fmt"
	"math/rand/v2"
	"sort"
)

const (
	CanonicalSystems = 32
	PlanetsPerSystem = 4
	CanonicalEmpires = 8
)

func Generate(seed int64, empireNames []string) (*World, error) {
	if len(empireNames) != CanonicalEmpires {
		return nil, fmt.Errorf("canonical universe requires %d empires, got %d", CanonicalEmpires, len(empireNames))
	}
	w := &World{Seed: seed, Systems: map[string]*System{}, Planets: map[string]*Planet{}, Empires: map[string]*Empire{}}
	for i := 0; i < CanonicalSystems; i++ {
		id := fmt.Sprintf("s%02d", i)
		w.Systems[id] = &System{ID: id}
		for slot := 1; slot <= PlanetsPerSystem; slot++ {
			pid := fmt.Sprintf("%s-p%d", id, slot)
			w.Systems[id].Planets = append(w.Systems[id].Planets, pid)
			w.Planets[pid] = &Planet{ID: pid, SystemID: id, Slot: slot}
		}
	}
	rng := rand.New(rand.NewPCG(uint64(seed), uint64(seed)^0x9e3779b97f4a7c15))
	buildConnectedGraph(w, rng)
	homes := selectHomes(w, rng, CanonicalEmpires)
	for i, name := range empireNames {
		eid := fmt.Sprintf("e%02d", i)
		p := w.Planets[fmt.Sprintf("%s-p1", homes[i])]
		p.OwnerID, p.Homeworld = eid, true
		p.Resources = Resources{Metal: 500, Crystal: 300, Deuterium: 150}
		p.Buildings = Buildings{1, 1, 1, 1, 1, 1, 1}
		w.Empires[eid] = &Empire{ID: eid, Name: name, HomeworldID: p.ID}
	}
	return w, nil
}

func buildConnectedGraph(w *World, rng *rand.Rand) {
	ids := systemIDs(w)
	perm := append([]string(nil), ids...)
	rng.Shuffle(len(perm), func(i, j int) { perm[i], perm[j] = perm[j], perm[i] })
	for i := 1; i < len(perm); i++ {
		j := rng.IntN(i)
		connect(w, perm[i], perm[j])
	}
	// Add routes until average degree is 3. The tree above guarantees connectivity.
	targetEdges := CanonicalSystems * 3 / 2
	for edgeCount(w) < targetEdges {
		a, b := ids[rng.IntN(len(ids))], ids[rng.IntN(len(ids))]
		if a != b && !adjacent(w, a, b) {
			connect(w, a, b)
		}
	}
	for _, s := range w.Systems {
		sort.Strings(s.Neighbors)
	}
}

func selectHomes(w *World, rng *rand.Rand, n int) []string {
	ids := systemIDs(w)
	first := ids[rng.IntN(len(ids))]
	selected := []string{first}
	used := map[string]bool{first: true}
	for len(selected) < n {
		best, bestDistance := "", -1
		for _, candidate := range ids {
			if used[candidate] { continue }
			min := CanonicalSystems + 1
			for _, home := range selected {
				if d := distance(w, candidate, home); d < min { min = d }
			}
			if min > bestDistance {
				best, bestDistance = candidate, min
			}
		}
		selected = append(selected, best)
		used[best] = true
	}
	sort.Strings(selected)
	return selected
}

func distance(w *World, from, to string) int {
	if from == to { return 0 }
	q := []string{from}
	dist := map[string]int{from: 0}
	for len(q) > 0 {
		cur := q[0]; q = q[1:]
		for _, next := range w.Systems[cur].Neighbors {
			if _, ok := dist[next]; ok { continue }
			dist[next] = dist[cur] + 1
			if next == to { return dist[next] }
			q = append(q, next)
		}
	}
	return CanonicalSystems + 1
}

func systemIDs(w *World) []string {
	ids := make([]string, 0, len(w.Systems))
	for id := range w.Systems { ids = append(ids, id) }
	sort.Strings(ids)
	return ids
}

func connect(w *World, a, b string) {
	w.Systems[a].Neighbors = append(w.Systems[a].Neighbors, b)
	w.Systems[b].Neighbors = append(w.Systems[b].Neighbors, a)
}
func adjacent(w *World, a, b string) bool {
	for _, x := range w.Systems[a].Neighbors { if x == b { return true } }
	return false
}
func edgeCount(w *World) int {
	n := 0
	for _, s := range w.Systems { n += len(s.Neighbors) }
	return n / 2
}

package world

import (
	"errors"
	"sort"
)

type Order struct {
	EmpireID string
	Type     string
	Actor    string
	Target   string
}

type TurnResult struct {
	Turn     int
	Accepted []Order
	Rejected []Order
}

// ResolveTurn is a barrier: validation observes the same pre-turn world for every empire.
// Later implementation phases add concrete order effects after this collection/validation step.
func ResolveTurn(w *World, submitted map[string][]Order) (TurnResult, error) {
	if w == nil { return TurnResult{}, errors.New("nil world") }
	ids := make([]string, 0, len(submitted))
	for id := range submitted { ids = append(ids, id) }
	sort.Strings(ids)

	result := TurnResult{Turn: w.Turn}
	for _, empireID := range ids {
		if _, ok := w.Empires[empireID]; !ok {
			result.Rejected = append(result.Rejected, submitted[empireID]...)
			continue
		}
		for _, order := range submitted[empireID] {
			if order.EmpireID != empireID || order.Type == "" {
				result.Rejected = append(result.Rejected, order)
				continue
			}
			result.Accepted = append(result.Accepted, order)
		}
	}
	w.Turn++
	return result, nil
}

package engine

import (
	"context"
	"fmt"
	"github.com/KakkoiDev/agame/agent"
	"github.com/KakkoiDev/agame/world"
	"sort"
)

type Runner struct {
	World  *world.World
	Agents map[string]agent.Agent
}

func (r *Runner) Turn(ctx context.Context) (world.TurnResult, error) {
	ids := make([]string, 0, len(r.World.Empires))
	for id, e := range r.World.Empires {
		if !e.Eliminated {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	submitted := map[string][]world.Order{}
	for _, id := range ids {
		a := r.Agents[id]
		if a == nil {
			continue
		}
		d, err := a.Decide(ctx, agent.Observe(r.World, id))
		if err != nil {
			continue
		}
		for i := range d.Orders {
			d.Orders[i].EmpireID = id
		}
		submitted[id] = d.Orders
	}
	res, err := world.ResolveTurn(r.World, submitted)
	if err != nil {
		return res, fmt.Errorf("resolve: %w", err)
	}
	return res, nil
}

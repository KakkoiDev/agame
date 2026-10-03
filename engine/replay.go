package engine

import (
	"encoding/json"
	"fmt"

	"github.com/KakkoiDev/agame/agent"
	"github.com/KakkoiDev/agame/world"
)

// Replay rebuilds a run from its initial state and turn log without invoking
// any agent (spec/benchmark.md, Replay): every logged turn is re-resolved
// from its submitted orders and must reproduce the logged events and state
// hash. initial is not modified. It returns the final world.
func Replay(initial *world.World, turns []world.TurnResult) (*world.World, error) {
	w, err := Clone(initial)
	if err != nil {
		return nil, err
	}
	for i, t := range turns {
		if t.Turn != w.Turn {
			return w, fmt.Errorf("log entry %d is turn %d, world is at turn %d", i, t.Turn, w.Turn)
		}
		res, err := world.ResolveTurn(w, t.Submitted)
		if err != nil {
			return w, err
		}
		if t.StateHash != "" && res.StateHash != t.StateHash {
			return w, fmt.Errorf("turn %d: state hash %s, log says %s", t.Turn, res.StateHash, t.StateHash)
		}
		a, _ := json.Marshal(res.Events)
		b, _ := json.Marshal(t.Events)
		if string(a) != string(b) {
			return w, fmt.Errorf("turn %d: events differ from the log", t.Turn)
		}
	}
	return w, nil
}

// Clone deep-copies a world through its canonical JSON encoding.
func Clone(w *world.World) (*world.World, error) {
	if w == nil {
		return nil, fmt.Errorf("nil world")
	}
	b, err := json.Marshal(w)
	if err != nil {
		return nil, err
	}
	var c world.World
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// ObservationAt replays the log up to the start of turn and returns what
// empire eid observed then: the ruler's historical knowledge, as opposed to
// today's world truth. Its agent.PromptHash matches the decision record of
// that turn.
func ObservationAt(initial *world.World, turns []world.TurnResult, turn int, eid string) (agent.Observation, error) {
	if turn < 0 || turn > len(turns) {
		return agent.Observation{}, fmt.Errorf("turn %d is outside the log (0..%d)", turn, len(turns))
	}
	w, err := Replay(initial, turns[:turn])
	if err != nil {
		return agent.Observation{}, err
	}
	if w.Empires[eid] == nil {
		return agent.Observation{}, fmt.Errorf("unknown empire %q", eid)
	}
	return agent.Observe(w, eid), nil
}

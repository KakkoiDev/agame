//go:build js && wasm

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"syscall/js"

	"github.com/KakkoiDev/agame/agent"
	"github.com/KakkoiDev/agame/engine"
	"github.com/KakkoiDev/agame/world"
)

var names = []string{"Cassian", "Malrec", "Aya", "Kael", "Iona", "Talos", "Nara", "Orion"}

func main() {
	api := js.Global().Get("Object").New()
	api.Set("newUniverse", js.FuncOf(newUniverse))
	api.Set("advanceTurn", js.FuncOf(advanceTurn))
	api.Set("autoTurn", js.FuncOf(autoTurn))
	api.Set("summary", js.FuncOf(summary))
	js.Global().Set("AGameWASM", api)
	select {}
}

func newUniverse(_ js.Value, args []js.Value) any {
	seed := int64(1)
	if len(args) > 0 {
		seed = int64(args[0].Int())
	}
	w, err := world.Generate(seed, names)
	if err != nil {
		return fail(err)
	}
	return encode(w)
}

// autoTurn plays one turn with every ruler on the deterministic autopilot,
// through engine.Runner: the same code path as `agame run`.
func autoTurn(_ js.Value, args []js.Value) any {
	w, err := decodeWorld(args)
	if err != nil {
		return fail(err)
	}
	agents := map[string]agent.Agent{}
	for id := range w.Empires {
		agents[id] = agent.AutopilotAgent{}
	}
	r := &engine.Runner{World: w, Agents: agents, Budget: engine.Budget{TurnLimit: world.CanonicalTurnLimit}}
	rec, err := r.Step(context.Background())
	if err != nil {
		return fail(err)
	}
	statements := map[string]string{}
	for _, d := range rec.Decisions {
		statements[d.Empire] = d.Statement
	}
	payload := struct {
		World      *world.World            `json:"world"`
		Result     world.TurnResult        `json:"result"`
		Statements map[string]string       `json:"statements"`
		Decisions  []engine.DecisionRecord `json:"decisions"`
		End        *world.End              `json:"end,omitempty"`
	}{w, rec.Result, statements, rec.Decisions, rec.End}
	return encode(payload)
}

func advanceTurn(_ js.Value, args []js.Value) any {
	w, err := decodeWorld(args)
	if err != nil {
		return fail(err)
	}
	orders := map[string][]world.Order{}
	if len(args) > 1 && args[1].Type() == js.TypeString && args[1].String() != "" {
		if err := json.Unmarshal([]byte(args[1].String()), &orders); err != nil {
			return fail(err)
		}
	}
	result, err := world.ResolveTurn(w, orders)
	if err != nil {
		return fail(err)
	}
	payload := struct {
		World  *world.World     `json:"world"`
		Result world.TurnResult `json:"result"`
	}{w, result}
	return encode(payload)
}

// summary is the observer view of a world: per-empire standings plus the
// public diplomatic state.
func summary(_ js.Value, args []js.Value) any {
	w, err := decodeWorld(args)
	if err != nil {
		return fail(err)
	}
	type empire struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Planets    int    `json:"planets"`
		Exile      bool   `json:"exile"`
		Eliminated bool   `json:"eliminated"`
		Fleets     int    `json:"fleets"`
		Metal      int    `json:"metal"`
		Crystal    int    `json:"crystal"`
		Deuterium  int    `json:"deuterium"`
		Tech       int    `json:"tech"`
		Alliance   string `json:"alliance,omitempty"`
		Rank       int    `json:"rank"`
		Score      int    `json:"score"`
		FleetValue int    `json:"fleet_value"`
	}
	type alliance struct {
		ID      string   `json:"id"`
		Name    string   `json:"name"`
		Members []string `json:"members"`
	}
	type war struct {
		A    string `json:"a"`
		B    string `json:"b"`
		Last int    `json:"last"`
	}
	out := struct {
		Seed      int64      `json:"seed"`
		Turn      int        `json:"turn"`
		Empires   []empire   `json:"empires"`
		Alliances []alliance `json:"alliances"`
		Wars      []war      `json:"wars"`
		End       *world.End `json:"end,omitempty"`
	}{Seed: w.Seed, Turn: w.Turn, End: world.CheckEnd(w, world.CanonicalTurnLimit)}
	standing := map[string]world.Standing{}
	for _, st := range world.Standings(w) {
		standing[st.Empire] = st
	}
	ids := make([]string, 0, len(w.Empires))
	for id := range w.Empires {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		e := w.Empires[id]
		st := standing[id]
		x := empire{ID: e.ID, Name: e.Name, Exile: e.Exile, Eliminated: e.Eliminated, Tech: st.TechLevels, Planets: st.Planets,
			Metal: st.Stored.Metal, Crystal: st.Stored.Crystal, Deuterium: st.Stored.Deuterium, Alliance: e.AllianceID, Rank: st.Rank, Score: st.Score, FleetValue: st.FleetValue}
		for _, f := range w.Fleets {
			if f.OwnerID == e.ID {
				x.Fleets++
			}
		}
		out.Empires = append(out.Empires, x)
	}
	for _, id := range world.AllianceIDs(w) {
		a := w.Alliances[id]
		out.Alliances = append(out.Alliances, alliance{ID: a.ID, Name: a.Name, Members: append([]string(nil), a.Members...)})
	}
	keys := make([]string, 0, len(w.Hostilities))
	for k := range w.Hostilities {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		a, b, _ := strings.Cut(k, "|")
		out.Wars = append(out.Wars, war{A: a, B: b, Last: w.Hostilities[k]})
	}
	return encode(out)
}

func decodeWorld(args []js.Value) (*world.World, error) {
	if len(args) == 0 || args[0].Type() != js.TypeString {
		return nil, fmt.Errorf("world JSON required")
	}
	var w world.World
	if err := json.Unmarshal([]byte(args[0].String()), &w); err != nil {
		return nil, err
	}
	return &w, nil
}
func encode(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return fail(err)
	}
	return map[string]any{"ok": true, "json": string(b)}
}
func fail(err error) any { return map[string]any{"ok": false, "error": err.Error()} }

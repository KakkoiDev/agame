//go:build js && wasm

package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"syscall/js"

	"github.com/KakkoiDev/agame/agent"
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

func autoTurn(_ js.Value, args []js.Value) any {
	w, err := decodeWorld(args)
	if err != nil {
		return fail(err)
	}
	submitted := map[string][]world.Order{}
	statements := map[string]string{}
	for id := range w.Empires {
		d := agent.Autopilot(w, id)
		for i := range d.Orders {
			d.Orders[i].EmpireID = id
		}
		submitted[id] = d.Orders
		statements[id] = d.Statement
	}
	result, err := world.ResolveTurn(w, submitted)
	if err != nil {
		return fail(err)
	}
	payload := struct {
		World      *world.World      `json:"world"`
		Result     world.TurnResult  `json:"result"`
		Statements map[string]string `json:"statements"`
	}{w, result, statements}
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
	}
	out := struct {
		Seed    int64    `json:"seed"`
		Turn    int      `json:"turn"`
		Empires []empire `json:"empires"`
	}{Seed: w.Seed, Turn: w.Turn}
	ids := make([]string, 0, len(w.Empires))
	for id := range w.Empires {\n\t\tids = append(ids, id)\n\t}
	sort.Strings(ids)
	for _, id := range ids {
		e := w.Empires[id]
		x := empire{ID: e.ID, Name: e.Name, Exile: e.Exile, Eliminated: e.Eliminated, Tech: e.Tech.Industry+e.Tech.Propulsion+e.Tech.Weapons+e.Tech.Shields+e.Tech.Sensors+e.Tech.Colonization}
		for _, p := range w.Planets {
			if p.OwnerID == e.ID {
				x.Planets++
				x.Metal += p.Resources.Metal\n\t\t\t\tx.Crystal += p.Resources.Crystal\n\t\t\t\tx.Deuterium += p.Resources.Deuterium
			}
		}
		for _, f := range w.Fleets {\n\t\t\tif f.OwnerID == e.ID {\n\t\t\t\tx.Fleets++\n\t\t\t}\n\t\t}
		out.Empires = append(out.Empires, x)
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

//go:build js && wasm

package main

import (
	"encoding/json"
	"fmt"
	"syscall/js"

	"github.com/KakkoiDev/agame/world"
)

var names = []string{"Cassian", "Malrec", "Aya", "Kael", "Iona", "Talos", "Nara", "Orion"}

func main() {
	api := js.Global().Get("Object").New()
	api.Set("newUniverse", js.FuncOf(newUniverse))
	api.Set("advanceTurn", js.FuncOf(advanceTurn))
	api.Set("summary", js.FuncOf(summary))
	js.Global().Set("AGameWASM", api)
	select {}
}

func newUniverse(_ js.Value, args []js.Value) any {
	seed := int64(1)
	if len(args) > 0 { seed = int64(args[0].Int()) }
	w, err := world.Generate(seed, names)
	if err != nil { return fail(err) }
	return encode(w)
}

func advanceTurn(_ js.Value, args []js.Value) any {
	w, err := decodeWorld(args)
	if err != nil { return fail(err) }
	orders := map[string][]world.Order{}
	if len(args) > 1 && args[1].Type() == js.TypeString && args[1].String() != "" {
		if err := json.Unmarshal([]byte(args[1].String()), &orders); err != nil { return fail(err) }
	}
	result, err := world.ResolveTurn(w, orders)
	if err != nil { return fail(err) }
	payload := struct { World *world.World `json:"world"`; Result world.TurnResult `json:"result"` }{w, result}
	return encode(payload)
}

func summary(_ js.Value, args []js.Value) any {
	w, err := decodeWorld(args)
	if err != nil { return fail(err) }
	type empire struct { ID string `json:"id"`; Name string `json:"name"`; Planets int `json:"planets"`; Exile bool `json:"exile"`; Eliminated bool `json:"eliminated"` }
	out := struct { Seed int64 `json:"seed"`; Turn int `json:"turn"`; Empires []empire `json:"empires"` }{Seed:w.Seed, Turn:w.Turn}
	for _, e := range w.Empires {
		x := empire{ID:e.ID, Name:e.Name, Exile:e.Exile, Eliminated:e.Eliminated}
		for _, p := range w.Planets { if p.OwnerID == e.ID { x.Planets++ } }
		out.Empires = append(out.Empires, x)
	}
	return encode(out)
}

func decodeWorld(args []js.Value) (*world.World, error) {
	if len(args) == 0 || args[0].Type() != js.TypeString { return nil, fmt.Errorf("world JSON required") }
	var w world.World
	if err := json.Unmarshal([]byte(args[0].String()), &w); err != nil { return nil, err }
	return &w, nil
}
func encode(v any) any { b, err := json.Marshal(v); if err != nil { return fail(err) }; return map[string]any{"ok":true, "json":string(b)} }
func fail(err error) any { return map[string]any{"ok":false, "error":err.Error()} }

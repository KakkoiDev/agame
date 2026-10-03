package world

import (
	"reflect"
	"testing"
)

func TestResolveTurnDoesNotDependOnSubmissionMapOrder(t *testing.T) {
	a, _ := Generate(1, names)
	b, _ := Generate(1, names)
	orders1 := map[string][]Order{
		"e01": {{EmpireID: "e01", Type: "message", Target: "e00", Params: map[string]any{"body": "hello"}}},
		"e00": {{EmpireID: "e00", Type: "message", Target: "e01", Params: map[string]any{"body": "hello"}}},
	}
	orders2 := map[string][]Order{"e00": orders1["e00"], "e01": orders1["e01"]}
	r1, _ := ResolveTurn(a, orders1)
	r2, _ := ResolveTurn(b, orders2)
	if len(r1.Accepted) != 2 || len(r2.Accepted) != 2 {
		t.Fatal("orders unexpectedly rejected")
	}
	for i := range r1.Accepted {
		if !reflect.DeepEqual(r1.Accepted[i], r2.Accepted[i]) {
			t.Fatal("resolution ordering depends on map iteration")
		}
	}
	if a.Turn != 1 || b.Turn != 1 {
		t.Fatal("turn did not advance exactly once")
	}
}

func TestRejectsSpoofedEmpire(t *testing.T) {
	w, _ := Generate(1, names)
	r, _ := ResolveTurn(w, map[string][]Order{"e00": {{EmpireID: "e01", Type: "move"}}})
	if len(r.Accepted) != 0 || len(r.Rejected) != 1 {
		t.Fatal("spoofed order accepted")
	}
}

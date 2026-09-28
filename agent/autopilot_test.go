package agent

import (
	"testing"

	"github.com/KakkoiDev/agame/world"
)

func TestAutopilotProducesEngineSafeTurn(t *testing.T) {
	w, err := world.Generate(7, []string{"A","B","C","D","E","F","G","H"})
	if err != nil { t.Fatal(err) }
	submitted := map[string][]world.Order{}
	for id := range w.Empires {
		d := Autopilot(w,id)
		for i := range d.Orders { d.Orders[i].EmpireID=id }
		submitted[id]=d.Orders
	}
	res, err := world.ResolveTurn(w,submitted)
	if err != nil { t.Fatal(err) }
	if w.Turn != 1 { t.Fatalf("turn=%d",w.Turn) }
	if len(res.Accepted)==0 { t.Fatal("autopilot produced no accepted actions") }
}

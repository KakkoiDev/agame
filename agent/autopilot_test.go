package agent

import (
	"testing"

	"github.com/KakkoiDev/agame/world"
)

func TestAutopilotProducesEngineSafeTurn(t *testing.T) {
	w,err:=world.Generate(7,[]string{"A","B","C","D","E","F","G","H"});if err!=nil{t.Fatal(err)}
	submitted:=map[string][]world.Order{};for id:=range w.Empires{d:=Autopilot(w,id);for i:=range d.Orders{d.Orders[i].EmpireID=id};submitted[id]=d.Orders}
	res,err:=world.ResolveTurn(w,submitted);if err!=nil{t.Fatal(err)};if w.Turn!=1{t.Fatalf("turn=%d",w.Turn)};if len(res.Accepted)==0{t.Fatal("autopilot produced no accepted actions")}
}

func TestAutopilotChangesStrategicStateAcross57Turns(t *testing.T){
	w,err:=world.Generate(57,[]string{"A","B","C","D","E","F","G","H"});if err!=nil{t.Fatal(err)}
	initialOwned:=ownedCount(w);initialTech:=techTotal(w);initialFleets:=len(w.Fleets)
	accepted:=0
	for turn:=0;turn<57;turn++{submitted:=map[string][]world.Order{};for id:=range w.Empires{d:=Autopilot(w,id);for i:=range d.Orders{d.Orders[i].EmpireID=id};submitted[id]=d.Orders};res,err:=world.ResolveTurn(w,submitted);if err!=nil{t.Fatal(err)};accepted+=len(res.Accepted)}
	if accepted<40{t.Fatalf("only %d accepted orders in 57 turns",accepted)}
	if ownedCount(w)<=initialOwned{e:=w.Empires["e00"];p:=w.Planets[e.HomeworldID];t.Fatalf("no expansion after 57 turns: planets=%d colonization=%d resources=%+v ships=%+v shipq=%+v fleets=%d",ownedCount(w),e.Tech.Colonization,p.Resources,p.Ships,p.ShipyardQueue,len(w.Fleets))}
	if techTotal(w)<=initialTech{t.Fatalf("no research after 57 turns: tech=%d",techTotal(w))}
	if len(w.Fleets)<=initialFleets{t.Fatalf("no persistent fleet activity after 57 turns: fleets=%d",len(w.Fleets))}
}

func ownedCount(w *world.World)int{n:=0;for _,p:=range w.Planets{if p.OwnerID!=""{n++}};return n}
func techTotal(w *world.World)int{n:=0;for _,e:=range w.Empires{n+=e.Tech.Industry+e.Tech.Propulsion+e.Tech.Weapons+e.Tech.Shields+e.Tech.Sensors+e.Tech.Colonization};return n}

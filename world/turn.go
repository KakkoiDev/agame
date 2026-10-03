package world

import("fmt";"sort")
type Order struct{EmpireID,Type,Actor,Target string;Params map[string]any}
type TurnResult struct{Turn int `json:"turn"`;Accepted []Order `json:"accepted"`;Rejected []Order `json:"rejected"`;Events []Event `json:"events"`}
func ResolveTurn(w *World,submitted map[string][]Order)(TurnResult,error){
 if w==nil{return TurnResult{},fmt.Errorf("nil world")}
 snap:=cloneWorld(w);res:=TurnResult{Turn:w.Turn};firstEvent:=len(w.Events);ids:=make([]string,0,len(submitted));for id:=range submitted{ids=append(ids,id)};sort.Strings(ids);var valid []Order
 for _,eid:=range ids{for _,o:=range submitted[eid]{if o.EmpireID!=eid||w.Empires[eid]==nil||o.Type==""{res.Rejected=append(res.Rejected,o);continue};if err:=validateOrder(snap,o);err!=nil{res.Rejected=append(res.Rejected,o);continue};valid=append(valid,o)}}
 // deterministic phase ordering; every validation above saw the same snapshot. Orders only touch their own empire's assets, so re-validating against the
 // live world only catches same-empire conflicts (double-spent resources, a second queue/mission for the same planet/fleet) and never another empire's orders.
 for _,o:=range valid{if err:=validateOrder(w,o);err!=nil{res.Rejected=append(res.Rejected,o);continue};if err:=applyOrder(w,o);err!=nil{res.Rejected=append(res.Rejected,o);continue};res.Accepted=append(res.Accepted,o)}
 advanceFleets(w);progressQueues(w);resolveArrivals(w);produce(w);updateSovereignty(w);w.Turn++
 res.Events=append(res.Events,w.Events[firstEvent:]...);return res,nil
}
func cloneWorld(w *World)*World{ // validation only needs immutable ownership/resources; JSON-free explicit copy is cheap enough.
 c:=*w;c.Planets=map[string]*Planet{};for id,p:=range w.Planets{x:=*p;c.Planets[id]=&x};c.Empires=map[string]*Empire{};for id,e:=range w.Empires{x:=*e;c.Empires[id]=&x};c.Fleets=map[string]*Fleet{};for id,f:=range w.Fleets{x:=*f;c.Fleets[id]=&x};return &c
}
func validateOrder(w *World,o Order)error{switch o.Type{
 case"construct":p:=w.Planets[o.Actor];if p==nil||p.OwnerID!=o.EmpireID||p.Construction!=nil{return fmt.Errorf("planet unavailable")};base,ok:=BuildingBase[o.Target];if !ok{return fmt.Errorf("building")};cost:=scale(base,buildingLevel(p.Buildings,o.Target)+1);if !p.Resources.Enough(cost){return fmt.Errorf("resources")}
 case"research":e:=w.Empires[o.EmpireID];if e.Research!=nil{return fmt.Errorf("research busy")};base,ok:=TechBase[o.Target];if !ok{return fmt.Errorf("tech")};p:=w.Planets[o.Actor];if p==nil||p.OwnerID!=o.EmpireID{return fmt.Errorf("payer")};if !p.Resources.Enough(scale(base,techLevel(e.Tech,o.Target)+1)){return fmt.Errorf("resources")}
 case"build_ships":p:=w.Planets[o.Actor];if p==nil||p.OwnerID!=o.EmpireID||p.ShipyardQueue!=nil{return fmt.Errorf("shipyard")};n:=intParam(o,"quantity",1);cost,err:=shipCost(o.Target,n);if err!=nil||!p.Resources.Enough(cost){return fmt.Errorf("ships")};if o.Target=="colony_ark"&&w.Empires[o.EmpireID].Tech.Colonization<1{return fmt.Errorf("colonization")}
 case"form_fleet":p:=w.Planets[o.Actor];if p==nil||p.OwnerID!=o.EmpireID{return fmt.Errorf("planet")};ships:=shipMapParam(o);if len(ships)==0{return fmt.Errorf("no ships")};for k,n:=range ships{if _,ok:=ShipSpecs[k];!ok||n<1||p.Ships[k]<n{return fmt.Errorf("ships")}}
 case"move","attack","spy","transport","recycle","colonize":f:=w.Fleets[o.Actor];if f==nil||f.OwnerID!=o.EmpireID||len(f.Route)>0{return fmt.Errorf("fleet")}
 case"message":if w.Empires[o.Target]==nil{return fmt.Errorf("recipient")}
 default:return fmt.Errorf("unsupported")
 };return nil}
func applyOrder(w *World,o Order)error{switch o.Type{
 case"construct":p:=w.Planets[o.Actor];cost:=scale(BuildingBase[o.Target],buildingLevel(p.Buildings,o.Target)+1);p.Resources=p.Resources.Sub(cost);p.Construction=&Queue{Kind:o.Target,Level:buildingLevel(p.Buildings,o.Target)+1,Required:work(cost),Paid:cost}
 case"research":e:=w.Empires[o.EmpireID];p:=w.Planets[o.Actor];cost:=scale(TechBase[o.Target],techLevel(e.Tech,o.Target)+1);p.Resources=p.Resources.Sub(cost);e.Research=&Queue{Kind:o.Target,Level:techLevel(e.Tech,o.Target)+1,Required:work(cost),Paid:cost}
 case"build_ships":p:=w.Planets[o.Actor];n:=intParam(o,"quantity",1);cost,_:=shipCost(o.Target,n);p.Resources=p.Resources.Sub(cost);p.ShipyardQueue=&Queue{Kind:o.Target,Quantity:n,Required:work(cost),Paid:cost}
 case"form_fleet":p:=w.Planets[o.Actor];ships:=shipMapParam(o);if !takeShips(p.Ships,ships){return fmt.Errorf("ships unavailable")};id:=fmt.Sprintf("f%06d",w.NextFleet);w.NextFleet++;w.Fleets[id]=&Fleet{ID:id,OwnerID:o.EmpireID,SystemID:p.SystemID,Ships:ships}
 case"move","attack","spy","recycle","colonize":return launch(w,o)
 case"transport":return launch(w,o)
 case"message":w.Messages=append(w.Messages,Message{Turn:w.Turn+1,From:o.EmpireID,To:o.Target,Body:stringParam(o,"body"),Major:boolParam(o,"major")})
 };return nil}
func launch(w *World,o Order)error{f:=w.Fleets[o.Actor];targetSystem:=o.Target;if p:=w.Planets[o.Target];p!=nil{targetSystem=p.SystemID};route:=shortest(w,f.SystemID,targetSystem);if len(route)<2&&f.SystemID!=targetSystem{return fmt.Errorf("route")};fuel:=0;for k,n:=range f.Ships{fuel+=ShipSpecs[k].Fuel*n};fuel*=max(0,len(route)-1);discount:=100-5*w.Empires[o.EmpireID].Tech.Propulsion;if discount<0{discount=0};fuel=(fuel*discount+99)/100;source:=ownedPlanetAt(w,o.EmpireID,f.SystemID);if source==nil{return fmt.Errorf("fuel")}
 // every check happens before any mutation so a rejected launch has no side effects.
 var cargo Resources;if o.Type=="transport"{cargo=resourceParam(o);room:=0;for k,n:=range f.Ships{room+=ShipSpecs[k].Cargo*n};room-=f.Cargo.Metal+f.Cargo.Crystal+f.Cargo.Deuterium;if cargo.Metal<0||cargo.Crystal<0||cargo.Deuterium<0||cargo.Metal>room||cargo.Crystal>room||cargo.Deuterium>room||cargo.Metal+cargo.Crystal+cargo.Deuterium>room{return fmt.Errorf("cargo")}};need:=cargo;need.Deuterium+=fuel;if !source.Resources.Enough(need){return fmt.Errorf("fuel/cargo")}
 source.Resources=source.Resources.Sub(need);f.Cargo=f.Cargo.Add(cargo);f.Route=route;f.RouteIndex=0;f.Mission=o.Type;f.Target=o.Target;return nil}
func shortest(w *World,a,b string)[]string{if a==b{return []string{a}};q:=[]string{a};prev:=map[string]string{a:""};for len(q)>0{c:=q[0];q=q[1:];for _,n:=range w.Systems[c].Neighbors{if _,ok:=prev[n];ok{continue};prev[n]=c;if n==b{path:=[]string{b};for x:=c;x!="";x=prev[x]{path=append(path,x)};for i,j:=0,len(path)-1;i<j;i,j=i+1,j-1{path[i],path[j]=path[j],path[i]};return path};q=append(q,n)}};return nil}
func advanceFleets(w *World){ids:=fleetIDs(w);for _,id:=range ids{f:=w.Fleets[id];if len(f.Route)<2{continue};speed:=1+w.Empires[f.OwnerID].Tech.Propulsion/3;for i:=0;i<speed&&f.RouteIndex<len(f.Route)-1;i++{f.RouteIndex++;f.SystemID=f.Route[f.RouteIndex]}}}
func resolveArrivals(w *World){for _,id:=range fleetIDs(w){f:=w.Fleets[id];if len(f.Route)>0&&f.RouteIndex==len(f.Route)-1{mission:=f.Mission;f.Route=nil;f.RouteIndex=0;switch mission{case"transport":if p:=w.Planets[f.Target];p!=nil&&p.OwnerID==f.OwnerID{p.Resources=p.Resources.Add(f.Cargo);f.Cargo=Resources{}};case"colonize":colonize(w,f);case"attack":combat(w,f);case"spy":spy(w,f);case"recycle":recycle(w,f)};f.Mission=""}}}
func progressQueues(w *World){for _,p:=range w.Planets{if p.OwnerID==""{continue};if q:=p.Construction;q!=nil{q.Progress+=max(1,p.Buildings.Infrastructure);if q.Progress>=q.Required{incBuilding(&p.Buildings,q.Kind);p.Construction=nil}};if q:=p.ShipyardQueue;q!=nil{q.Progress+=max(1,p.Buildings.Shipyard);if q.Progress>=q.Required{p.Ships[q.Kind]+=q.Quantity;p.ShipyardQueue=nil}}};for _,e:=range w.Empires{if e.Research==nil{continue};pts:=0;for _,p:=range w.Planets{if p.OwnerID==e.ID{pts+=p.Buildings.ResearchLab}};e.Research.Progress+=max(1,pts);if e.Research.Progress>=e.Research.Required{incTech(&e.Tech,e.Research.Kind);e.Research=nil}}}
func produce(w *World){for _,p:=range w.Planets{if p.OwnerID==""{continue};e:=w.Empires[p.OwnerID];mul:=100+10*e.Tech.Industry;p.Resources.Metal+=30*p.Buildings.MetalMine*mul/100;p.Resources.Crystal+=20*p.Buildings.CrystalMine*mul/100;p.Resources.Deuterium+=12*p.Buildings.DeuteriumExtractor*mul/100}}
func colonize(w *World,f *Fleet){p:=w.Planets[f.Target];e:=w.Empires[f.OwnerID];if p==nil||p.OwnerID!=""||f.Ships["colony_ark"]<1||planetCount(w,e.ID)>=1+e.Tech.Colonization{return};f.Ships["colony_ark"]--;p.OwnerID=e.ID;p.Buildings.Infrastructure=1;w.Events=append(w.Events,Event{Turn:w.Turn,Type:"colonized",EmpireID:e.ID,Target:p.ID})}
func combat(w *World,a *Fleet){p:=w.Planets[a.Target];if p==nil||p.OwnerID==""||p.OwnerID==a.OwnerID{return};def:=p.OwnerID;attack:=combatPower(a.Ships,w.Empires[a.OwnerID].Tech);defense:=20*p.Buildings.DefenseGrid*(100+10*w.Empires[def].Tech.Shields)/100;for _,f:=range w.Fleets{if f.OwnerID==def&&f.SystemID==p.SystemID&&len(f.Route)==0{defense+=combatPower(f.Ships,w.Empires[def].Tech)}};if attack>defense&&a.Ships["frigate"]+a.Ships["cruiser"]>0{old:=p.OwnerID;p.OwnerID=a.OwnerID;p.Resources=Resources{p.Resources.Metal/2,p.Resources.Crystal/2,p.Resources.Deuterium/2};p.Construction=nil;p.ShipyardQueue=nil;w.Events=append(w.Events,Event{Turn:w.Turn,Type:"captured",EmpireID:a.OwnerID,Target:p.ID,Detail:old})}else{ // explainable aggregate loss model; deterministic.
 for k:=range a.Ships{a.Ships[k]=a.Ships[k]/2};w.Events=append(w.Events,Event{Turn:w.Turn,Type:"attack_repulsed",EmpireID:a.OwnerID,Target:p.ID})}}
func combatPower(s Ships,t Tech)int{v:=0;for k,n:=range s{v+=ShipSpecs[k].Attack*n};return v*(100+10*t.Weapons)/100}
func spy(w *World,f *Fleet){p:=w.Planets[f.Target];if p==nil{return};delta:=w.Empires[f.OwnerID].Tech.Sensors;if p.OwnerID!=""{delta-=w.Empires[p.OwnerID].Tech.Sensors};tier:=0;if delta>=-1{tier=1};if delta>=1{tier=2};if delta>=3{tier=3};w.Events=append(w.Events,Event{Turn:w.Turn,Type:"espionage",EmpireID:f.OwnerID,Target:p.ID,Detail:fmt.Sprintf("tier=%d owner=%s",tier,p.OwnerID)})}
func recycle(w *World,f *Fleet){s:=w.Systems[f.SystemID];cap:=0;for k,n:=range f.Ships{cap+=ShipSpecs[k].Cargo*n};take:=min(cap,s.Debris.Metal+s.Debris.Crystal);m:=min(take,s.Debris.Metal);f.Cargo.Metal+=m;s.Debris.Metal-=m;take-=m;c:=min(take,s.Debris.Crystal);f.Cargo.Crystal+=c;s.Debris.Crystal-=c}
func updateSovereignty(w *World){for _,e:=range w.Empires{if planetCount(w,e.ID)>0{e.Exile=false;e.Eliminated=false;continue};viable:=false;for _,f:=range w.Fleets{if f.OwnerID==e.ID&&f.Ships["colony_ark"]>0{viable=true}};e.Exile=viable;e.Eliminated=!viable}}
func planetCount(w *World,e string)int{n:=0;for _,p:=range w.Planets{if p.OwnerID==e{n++}};return n}
func ownedPlanetAt(w *World,e,s string)*Planet{for _,p:=range w.Planets{if p.OwnerID==e&&p.SystemID==s{return p}};return nil}
func fleetIDs(w *World)[]string{a:=make([]string,0,len(w.Fleets));for id:=range w.Fleets{a=append(a,id)};sort.Strings(a);return a}
func intParam(o Order,k string,d int)int{if v,ok:=o.Params[k].(float64);ok{return int(v)};if v,ok:=o.Params[k].(int);ok{return v};return d}
func stringParam(o Order,k string)string{v,_:=o.Params[k].(string);return v}
func boolParam(o Order,k string)bool{v,_:=o.Params[k].(bool);return v}
func shipMapParam(o Order)Ships{out:=Ships{};if m,ok:=o.Params["ships"].(map[string]int);ok{for k,v:=range m{out[k]=v}};if m,ok:=o.Params["ships"].(map[string]any);ok{for k,v:=range m{if n,ok:=v.(float64);ok{out[k]=int(n)}}};return out}
func resourceParam(o Order)Resources{return Resources{intParam(o,"metal",0),intParam(o,"crystal",0),intParam(o,"deuterium",0)}}
func takeShips(have,need Ships)bool{for k,n:=range need{if n<0||have[k]<n{return false}};for k,n:=range need{have[k]-=n};return true}
func max(a,b int)int{if a>b{return a};return b};func min(a,b int)int{if a<b{return a};return b}

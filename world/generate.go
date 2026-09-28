package world

import("fmt";"math/rand/v2";"sort")
const(CanonicalSystems=32;PlanetsPerSystem=4;CanonicalEmpires=8)
func Generate(seed int64,names []string)(*World,error){
 if len(names)!=CanonicalEmpires{return nil,fmt.Errorf("canonical universe requires %d empires, got %d",CanonicalEmpires,len(names))}
 w:=&World{Seed:seed,Systems:map[string]*System{},Planets:map[string]*Planet{},Empires:map[string]*Empire{},Fleets:map[string]*Fleet{}}
 for i:=0;i<CanonicalSystems;i++{id:=fmt.Sprintf("s%02d",i);w.Systems[id]=&System{ID:id};for slot:=1;slot<=PlanetsPerSystem;slot++{pid:=fmt.Sprintf("%s-p%d",id,slot);w.Systems[id].Planets=append(w.Systems[id].Planets,pid);w.Planets[pid]=&Planet{ID:pid,SystemID:id,Slot:slot,Ships:Ships{}}}}
 rng:=rand.New(rand.NewPCG(uint64(seed),uint64(seed)^0x9e3779b97f4a7c15));buildConnectedGraph(w,rng);homes:=selectHomes(w,rng,CanonicalEmpires)
 for i,name:=range names{eid:=fmt.Sprintf("e%02d",i);p:=w.Planets[fmt.Sprintf("%s-p1",homes[i])];p.OwnerID=eid;p.Homeworld=true;p.Resources=Resources{500,300,150};p.Buildings=Buildings{1,1,1,1,1,1,1};p.Ships=Ships{"scout":1,"transport":1,"frigate":2};w.Empires[eid]=&Empire{ID:eid,Name:name,HomeworldID:p.ID}}
 return w,nil
}
func buildConnectedGraph(w *World,rng *rand.Rand){ids:=systemIDs(w);perm:=append([]string(nil),ids...);rng.Shuffle(len(perm),func(i,j int){perm[i],perm[j]=perm[j],perm[i]});for i:=1;i<len(perm);i++{connect(w,perm[i],perm[rng.IntN(i)])};for edgeCount(w)<CanonicalSystems*3/2{a,b:=ids[rng.IntN(len(ids))],ids[rng.IntN(len(ids))];if a!=b&&!adjacent(w,a,b){connect(w,a,b)}};for _,s:=range w.Systems{sort.Strings(s.Neighbors)}}
func selectHomes(w *World,rng *rand.Rand,n int)[]string{ids:=systemIDs(w);first:=ids[rng.IntN(len(ids))];sel:=[]string{first};used:=map[string]bool{first:true};for len(sel)<n{best:="";bd:=-1;for _,c:=range ids{if used[c]{continue};m:=CanonicalSystems+1;for _,h:=range sel{if d:=distance(w,c,h);d<m{m=d}};if m>bd{best,bd=c,m}};sel=append(sel,best);used[best]=true};sort.Strings(sel);return sel}
func distance(w *World,a,b string)int{if a==b{return 0};q:=[]string{a};d:=map[string]int{a:0};for len(q)>0{c:=q[0];q=q[1:];for _,n:=range w.Systems[c].Neighbors{if _,ok:=d[n];ok{continue};d[n]=d[c]+1;if n==b{return d[n]};q=append(q,n)}};return CanonicalSystems+1}
func systemIDs(w *World)[]string{ids:=make([]string,0,len(w.Systems));for id:=range w.Systems{ids=append(ids,id)};sort.Strings(ids);return ids}
func connect(w *World,a,b string){w.Systems[a].Neighbors=append(w.Systems[a].Neighbors,b);w.Systems[b].Neighbors=append(w.Systems[b].Neighbors,a)}
func adjacent(w *World,a,b string)bool{for _,x:=range w.Systems[a].Neighbors{if x==b{return true}};return false}
func edgeCount(w *World)int{n:=0;for _,s:=range w.Systems{n+=len(s.Neighbors)};return n/2}

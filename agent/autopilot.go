package agent

import (
	"fmt"
	"sort"

	"github.com/KakkoiDev/agame/world"
)

// Autopilot is a deterministic, zero-download ruler. It deliberately plays a
// complete expansion/economy/war loop so the game remains meaningful without
// optional local models.
func Autopilot(w *world.World, eid string) Decision {
	e:=w.Empires[eid]; if e==nil||e.Eliminated{return Decision{}}
	ps:=ownedPlanets(w,eid)
	if len(ps)==0{return exileDecision(w,eid)}
	p:=bestPlanet(ps)

	// Fleets already in space get a purpose first.
	for _,f:=range ownedFleets(w,eid){
		if len(f.Route)>0{continue}
		if f.Ships["colony_ark"]>0 {
			if target:=nearestEmptyPlanet(w,f.SystemID);target!=""&&canLaunch(w,eid,f,target){
				return one("colonize",f.ID,target,nil,"Colonization fleet dispatched to "+target+".")
			}
		}
		if f.Ships["frigate"]+f.Ships["cruiser"]>0 && hasOwnedPlanetAt(w,eid,f.SystemID) {
			if target:=nearestEnemyPlanet(w,eid,f.SystemID);target!=""{
				return one("attack",f.ID,target,nil,"Expeditionary fleet attacking "+target+".")
			}
		}
	}

	// Expansion is the strategic priority once Colonization is known.
	if e.Tech.Colonization>0 && len(ps) < 1+e.Tech.Colonization {
		if p.Ships["colony_ark"]>0 {
			return one("form_fleet",p.ID,"",map[string]any{"ships":map[string]any{"colony_ark":float64(1)}},"Launching a colony expedition.")
		}
		if p.ShipyardQueue==nil && enough(p.Resources,world.ShipSpecs["colony_ark"].Cost) {
			return one("build_ships",p.ID,"colony_ark",map[string]any{"quantity":1},"Building a Colony Ark for expansion.")
		}
		return Decision{Statement:"Reserving resources for a Colony Ark."}
	}

	// Get Colonization early instead of wandering through technologies.
	if e.Tech.Colonization==0 && e.Research==nil && enough(p.Resources,world.TechBase["colonization"]) {
		return one("research",p.ID,"colonization",nil,"Researching Colonization to open new worlds.")
	}

	// Build an actual military rather than endlessly rotating building names.
	if p.ShipyardQueue==nil {
		if enough(p.Resources,world.ShipSpecs["cruiser"].Cost) && e.Tech.Weapons>0 {
			return one("build_ships",p.ID,"cruiser",map[string]any{"quantity":1},"Shipyard laying down a cruiser.")
		}
		if enough(p.Resources,world.ShipSpecs["frigate"].Cost) {
			return one("build_ships",p.ID,"frigate",map[string]any{"quantity":1},"Shipyard producing a frigate.")
		}
	}
	combat:=map[string]any{}
	if p.Ships["frigate"]>=2{combat["frigate"]=float64(p.Ships["frigate"])}
	if p.Ships["cruiser"]>0{combat["cruiser"]=float64(p.Ships["cruiser"])}
	if len(combat)>0 {
		return one("form_fleet",p.ID,"",map[string]any{"ships":combat},"Combat group launched from "+p.ID+".")
	}

	// Keep one construction queue working, biased toward resource production.
	if p.Construction==nil {
		for _,k:=range []string{"metal_mine","crystal_mine","deuterium_extractor","infrastructure","shipyard","research_lab","defense_grid"} {
			cost:=buildingCost(p,k)
			if enough(p.Resources,cost){return one("construct",p.ID,k,nil,"Upgrading "+human(k)+" on "+p.ID+".")}
		}
	}

	if e.Research==nil {
		for _,k:=range []string{"industry","propulsion","weapons","shields","sensors"} {
			cost:=techCost(e,k)
			if enough(p.Resources,cost){return one("research",p.ID,k,nil,"Research program: "+human(k)+".")}
		}
	}
	return Decision{Statement:"Conserving resources while active queues progress."}
}

func one(kind,actor,target string,params map[string]any,statement string)Decision{return Decision{Orders:[]world.Order{{Type:kind,Actor:actor,Target:target,Params:params}},Statement:statement}}
func enough(a,b world.Resources)bool{return a.Metal>=b.Metal&&a.Crystal>=b.Crystal&&a.Deuterium>=b.Deuterium}
func doubled(base world.Resources,n int)world.Resources{m:=1;for i:=0;i<n;i++{m*=2};return world.Resources{Metal:base.Metal*m,Crystal:base.Crystal*m,Deuterium:base.Deuterium*m}}
func buildingCost(p *world.Planet,k string)world.Resources{level:=0;switch k{case"metal_mine":level=p.Buildings.MetalMine;case"crystal_mine":level=p.Buildings.CrystalMine;case"deuterium_extractor":level=p.Buildings.DeuteriumExtractor;case"infrastructure":level=p.Buildings.Infrastructure;case"research_lab":level=p.Buildings.ResearchLab;case"shipyard":level=p.Buildings.Shipyard;case"defense_grid":level=p.Buildings.DefenseGrid};return doubled(world.BuildingBase[k],level)}
func techCost(e *world.Empire,k string)world.Resources{level:=0;switch k{case"industry":level=e.Tech.Industry;case"propulsion":level=e.Tech.Propulsion;case"weapons":level=e.Tech.Weapons;case"shields":level=e.Tech.Shields;case"sensors":level=e.Tech.Sensors;case"colonization":level=e.Tech.Colonization};return doubled(world.TechBase[k],level)}
func human(s string)string{m:=map[string]string{"metal_mine":"Metal Mine","crystal_mine":"Crystal Mine","deuterium_extractor":"Deuterium Extractor","research_lab":"Research Lab","defense_grid":"Defense Grid"};if x:=m[s];x!=""{return x};return s}
func bestPlanet(ps []*world.Planet)*world.Planet{sort.Slice(ps,func(i,j int)bool{ai:=ps[i].Resources.Metal+ps[i].Resources.Crystal+ps[i].Resources.Deuterium;aj:=ps[j].Resources.Metal+ps[j].Resources.Crystal+ps[j].Resources.Deuterium;if ai==aj{return ps[i].ID<ps[j].ID};return ai>aj});return ps[0]}
func ownedPlanets(w *world.World,eid string)[]*world.Planet{var out []*world.Planet;for _,p:=range w.Planets{if p.OwnerID==eid{out=append(out,p)}};sort.Slice(out,func(i,j int)bool{return out[i].ID<out[j].ID});return out}
func ownedFleets(w *world.World,eid string)[]*world.Fleet{var out []*world.Fleet;for _,f:=range w.Fleets{if f.OwnerID==eid{out=append(out,f)}};sort.Slice(out,func(i,j int)bool{return out[i].ID<out[j].ID});return out}
func hasOwnedPlanetAt(w *world.World,eid,system string)bool{for _,p:=range w.Planets{if p.OwnerID==eid&&p.SystemID==system{return true}};return false}
func nearestEmptyPlanet(w *world.World,from string)string{return nearestPlanet(w,from,func(p *world.Planet)bool{return p.OwnerID==""})}
func nearestEnemyPlanet(w *world.World,eid,from string)string{return nearestPlanet(w,from,func(p *world.Planet)bool{return p.OwnerID!=""&&p.OwnerID!=eid})}
func nearestPlanet(w *world.World,from string,ok func(*world.Planet)bool)string{q:=[]string{from};seen:=map[string]bool{from:true};for len(q)>0{s:=q[0];q=q[1:];ids:=append([]string(nil),w.Systems[s].Planets...);sort.Strings(ids);for _,id:=range ids{if ok(w.Planets[id]){return id}};ns:=append([]string(nil),w.Systems[s].Neighbors...);sort.Strings(ns);for _,n:=range ns{if !seen[n]{seen[n]=true;q=append(q,n)}}};return ""}
func exileDecision(w *world.World,eid string)Decision{for _,f:=range ownedFleets(w,eid){if len(f.Route)==0&&f.Ships["colony_ark"]>0{if t:=nearestEmptyPlanet(w,f.SystemID);t!=""&&canLaunch(w,eid,f,t){return one("colonize",f.ID,t,nil,"Exile government attempting recolonization.")}}};return Decision{Statement:"Government in exile has no immediate recovery route."}}
func describeOrder(o world.Order)string{if o.Target!=""{return fmt.Sprintf("%s %s → %s",o.Type,o.Actor,o.Target)};return fmt.Sprintf("%s %s",o.Type,o.Actor)}
// canLaunch: fuel is paid by an owned planet in the fleet's system, so a fleet elsewhere can only act in place; ordering anything else is always rejected.
func canLaunch(w *world.World,eid string,f *world.Fleet,target string)bool{return hasOwnedPlanetAt(w,eid,f.SystemID)||w.Planets[target].SystemID==f.SystemID}

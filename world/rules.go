package world

import "fmt"

var BuildingBase = map[string]Resources{
	"metal_mine": {60,15,0}, "crystal_mine": {48,24,0}, "deuterium_extractor": {36,30,0},
	"infrastructure": {80,40,0}, "research_lab": {60,90,30}, "shipyard": {100,60,20}, "defense_grid": {120,80,20},
}
var TechBase = map[string]Resources{
	"industry": {150,100,20}, "propulsion": {100,150,50}, "weapons": {100,180,40},
	"shields": {120,160,40}, "sensors": {80,200,60}, "colonization": {200,250,120},
}
type ShipSpec struct{ Cost Resources; Hull, Attack, Cargo, Fuel int }
var ShipSpecs = map[string]ShipSpec{
	"scout": {Cost: Resources{20,40,20}, Hull:20, Attack:5, Cargo:0, Fuel:2}, "transport": {Cost: Resources{60,30,30}, Hull:60, Attack:5, Cargo:250, Fuel:4},
	"colony_ark": {Cost: Resources{200,150,100}, Hull:150, Attack:10, Cargo:100, Fuel:8}, "frigate": {Cost: Resources{100,50,30}, Hull:100, Attack:40, Cargo:10, Fuel:5},
	"cruiser": {Cost: Resources{240,120,80}, Hull:260, Attack:110, Cargo:20, Fuel:10}, "recycler": {Cost: Resources{80,60,40}, Hull:80, Attack:5, Cargo:200, Fuel:5},
}
func scale(base Resources, level int) Resources {
	m:=1
	for i:=1;i<level;i++ { m*=2 }
	return Resources{base.Metal*m,base.Crystal*m,base.Deuterium*m}
}
func work(r Resources) int { n:=r.Metal+r.Crystal+r.Deuterium; return (n+99)/100 }
func buildingLevel(b Buildings, k string) int {
	switch k { case "metal_mine":return b.MetalMine;case "crystal_mine":return b.CrystalMine;case "deuterium_extractor":return b.DeuteriumExtractor;case "infrastructure":return b.Infrastructure;case "research_lab":return b.ResearchLab;case "shipyard":return b.Shipyard;case "defense_grid":return b.DefenseGrid }; return -1
}
func incBuilding(b *Buildings,k string){ switch k{case"metal_mine":b.MetalMine++;case"crystal_mine":b.CrystalMine++;case"deuterium_extractor":b.DeuteriumExtractor++;case"infrastructure":b.Infrastructure++;case"research_lab":b.ResearchLab++;case"shipyard":b.Shipyard++;case"defense_grid":b.DefenseGrid++}}
func techLevel(t Tech,k string)int{switch k{case"industry":return t.Industry;case"propulsion":return t.Propulsion;case"weapons":return t.Weapons;case"shields":return t.Shields;case"sensors":return t.Sensors;case"colonization":return t.Colonization};return -1}
func incTech(t *Tech,k string){switch k{case"industry":t.Industry++;case"propulsion":t.Propulsion++;case"weapons":t.Weapons++;case"shields":t.Shields++;case"sensors":t.Sensors++;case"colonization":t.Colonization++}}
// MaxShipBatch bounds one shipyard batch so cost arithmetic can never overflow int (e.g. 1<<62 scouts cost 0 after wraparound).
const MaxShipBatch=1<<20
func shipCost(kind string,n int)(Resources,error){s,ok:=ShipSpecs[kind];if !ok||n<1||n>MaxShipBatch{return Resources{},fmt.Errorf("invalid ship batch")};return Resources{s.Cost.Metal*n,s.Cost.Crystal*n,s.Cost.Deuterium*n},nil}

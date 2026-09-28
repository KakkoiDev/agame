package agent

import("context";"github.com/KakkoiDev/agame/world")
type Observation struct{Turn int `json:"turn"`;Empire *world.Empire `json:"empire"`;Planets []*world.Planet `json:"planets"`;Fleets []*world.Fleet `json:"fleets"`;Messages []world.Message `json:"messages"`}
type Decision struct{Orders []world.Order `json:"orders"`;Statement string `json:"statement"`}
type Agent interface{Decide(context.Context,Observation)(Decision,error)}
func Observe(w *world.World,eid string)Observation{o:=Observation{Turn:w.Turn,Empire:w.Empires[eid]};for _,p:=range w.Planets{if p.OwnerID==eid{o.Planets=append(o.Planets,p)}};for _,f:=range w.Fleets{if f.OwnerID==eid{o.Fleets=append(o.Fleets,f)}};for _,m:=range w.Messages{if m.To==eid&&m.Turn<=w.Turn{o.Messages=append(o.Messages,m)}};return o}

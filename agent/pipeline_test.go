package agent

import (
	"context"
	"testing"

	"github.com/KakkoiDev/agame/world"
)

type captureSink struct{ texts []string }
func (s *captureSink) Write(_ context.Context,_ WriteIntent,text string)error{s.texts=append(s.texts,text);return nil}

func TestRulerSeparatesDecisionFromWriting(t *testing.T){
	sink:=&captureSink{}
	decision:=ScriptedDecision(func(context.Context,Observation)(Decision,error){
		return Decision{
			Orders:[]world.Order{{Type:"research",Actor:"p1",Target:"sensors"}},
			Writes:[]WriteIntent{{Kind:"message",Target:"diplomacy/e01.md",Instruction:"Reject demand"}},
		},nil
	})
	r:=Ruler{Decision:decision,Writer:StaticWriter("We decline."),Sink:sink}
	d,err:=r.Decide(context.Background(),Observation{})
	if err!=nil{t.Fatal(err)}
	if len(d.Orders)!=1{t.Fatal("decision provider did not control gameplay")}
	if len(sink.texts)!=1||sink.texts[0]!="We decline."{t.Fatal("writer was not isolated to prose")}
}

func TestRulerDoesNotRequireWriter(t *testing.T){
	decision:=ScriptedDecision(func(context.Context,Observation)(Decision,error){return Decision{Orders:[]world.Order{{Type:"move"}}},nil})
	d,err:=(Ruler{Decision:decision}).Decide(context.Background(),Observation{})
	if err!=nil||len(d.Orders)!=1{t.Fatal("classifier-only ruler must work")}
}

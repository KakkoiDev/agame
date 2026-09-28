package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/KakkoiDev/agame/world"
)

// ClassifierHTTP is a model-neutral structured decision adapter.
// A local service may implement it with GLiNER/GLiNER2.5-Decide, another
// zero-shot classifier, or any model that can fill the requested schema.
type ClassifierHTTP struct {
	Endpoint string
	Model    string
	Client   *http.Client
}

type classifierRequest struct {
	Model       string         `json:"model,omitempty"`
	Task        string         `json:"task"`
	Input       Observation    `json:"input"`
	Schema      DecisionSchema `json:"schema"`
}

type DecisionSchema struct {
	OrderTypes  []string `json:"order_types"`
	WriteKinds  []string `json:"write_kinds"`
	Postures    []string `json:"postures"`
}

func DefaultDecisionSchema() DecisionSchema {
	return DecisionSchema{
		OrderTypes: []string{"construct","build_ships","research","form_fleet","move","attack","colonize","spy","transport","recycle","message"},
		WriteKinds: []string{"message","report","memory","plan","reflection","propaganda"},
		Postures: []string{"passive","defensive","opportunistic","aggressive","cooperate","negotiate","threaten","deceive","betray","ignore"},
	}
}

func (c ClassifierHTTP) Decide(ctx context.Context, o Observation) (Decision, error) {
	payload := classifierRequest{Model:c.Model, Task:"agame_decision", Input:o, Schema:DefaultDecisionSchema()}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(c.Endpoint,"/")+"/classify", bytes.NewReader(b))
	if err != nil { return Decision{}, err }
	req.Header.Set("Content-Type","application/json")
	client:=c.Client; if client==nil { client=http.DefaultClient }
	resp,err:=client.Do(req); if err!=nil{return Decision{},err}; defer resp.Body.Close()
	if resp.StatusCode/100!=2{return Decision{},fmt.Errorf("classifier HTTP %s",resp.Status)}
	var d Decision
	if err:=json.NewDecoder(resp.Body).Decode(&d);err!=nil{return Decision{},err}
	return d,nil
}

// ScriptedDecision keeps the engine/model boundary testable without inference.
type ScriptedDecision func(context.Context, Observation) (Decision, error)
func (f ScriptedDecision) Decide(ctx context.Context,o Observation)(Decision,error){return f(ctx,o)}

type StaticWriter string
func (s StaticWriter) Generate(context.Context,TextRequest)(string,error){return string(s),nil}

func MessageOrder(empire,target,body string) world.Order {
	return world.Order{EmpireID:empire,Type:"message",Target:target,Params:map[string]any{"body":body}}
}

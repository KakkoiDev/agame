package agent

import (
	"context"

	"github.com/KakkoiDev/agame/world"
)

type Observation struct {
	Turn     int              `json:"turn"`
	Empire   *world.Empire    `json:"empire"`
	Planets  []*world.Planet  `json:"planets"`
	Fleets   []*world.Fleet   `json:"fleets"`
	Messages []world.Message  `json:"messages"`
}

type Decision struct {
	Orders    []world.Order `json:"orders"`
	Statement string        `json:"statement,omitempty"`
	Writes    []WriteIntent `json:"writes,omitempty"`
}

type WriteIntent struct {
	Kind       string         `json:"kind"`
	Target     string         `json:"target,omitempty"`
	Instruction string        `json:"instruction"`
	Facts      map[string]any `json:"facts,omitempty"`
	MaxWords   int            `json:"max_words,omitempty"`
}

type DecisionProvider interface {
	Decide(context.Context, Observation) (Decision, error)
}

type TextRequest struct {
	Kind        string         `json:"kind"`
	Instruction string         `json:"instruction"`
	Facts       map[string]any `json:"facts,omitempty"`
	MaxWords    int            `json:"max_words,omitempty"`
}

type TextGenerator interface {
	Generate(context.Context, TextRequest) (string, error)
}

type TextSink interface {
	Write(context.Context, WriteIntent, string) error
}

type Agent interface {
	Decide(context.Context, Observation) (Decision, error)
}

type Ruler struct {
	Decision DecisionProvider
	Writer   TextGenerator
	Sink     TextSink
}

func (r Ruler) Decide(ctx context.Context, o Observation) (Decision, error) {
	d, err := r.Decision.Decide(ctx, o)
	if err != nil {
		return Decision{}, err
	}
	if r.Writer == nil || r.Sink == nil {
		return d, nil
	}
	for _, intent := range d.Writes {
		text, err := r.Writer.Generate(ctx, TextRequest{
			Kind: intent.Kind, Instruction: intent.Instruction,
			Facts: intent.Facts, MaxWords: intent.MaxWords,
		})
		if err != nil {
			continue
		}
		if err := r.Sink.Write(ctx, intent, text); err != nil {
			continue
		}
	}
	return d, nil
}

func Observe(w *world.World, eid string) Observation {
	o := Observation{Turn: w.Turn, Empire: w.Empires[eid]}
	for _, p := range w.Planets {
		if p.OwnerID == eid { o.Planets = append(o.Planets, p) }
	}
	for _, f := range w.Fleets {
		if f.OwnerID == eid { o.Fleets = append(o.Fleets, f) }
	}
	for _, m := range w.Messages {
		if m.To == eid && m.Turn <= w.Turn { o.Messages = append(o.Messages, m) }
	}
	return o
}

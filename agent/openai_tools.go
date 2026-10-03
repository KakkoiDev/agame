package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ToolProtocol explains the tool loop to the model: the JSON action protocol
// by default, or OpenAI function tools when native is set.
func ToolProtocol(t *Tools, native bool) string {
	b := t.Budget()
	var s strings.Builder
	s.WriteString("Before answering you may consult your Jikko memory: notes, plans, intelligence and alliance documents. Nothing is read for you; the tree below lists what you can open.\n")
	if native {
		s.WriteString("Use the provided functions (jikko_tree, jikko_read, jikko_read_many, jikko_search, jikko_mentions, jikko_create, jikko_update, jikko_retract, game_inspect")
		if !t.Reflection() {
			s.WriteString(", game_submit")
		}
		s.WriteString(").\n")
	} else {
		s.WriteString(`Reply with exactly one JSON object per message. A tool call is {"tool":"<name>", ...arguments}. Tools:` + "\n")
	}
	fmt.Fprintf(&s, `jikko.tree {"path":"dir/"} lists documents you can read (path optional)
jikko.read {"path":"..."}; jikko.read_many {"paths":[...]} (at most %d)
jikko.search {"query":"..."}; jikko.mentions {} finds documents mentioning you or your alliance
jikko.create {"path":"...","content":"...","type":"document|task"}: a document is durable memory, a task is a current plan
jikko.update {"path":"...","expected_revision":N,"content":"..."}: tasks may be rewritten; documents only take appended corrections that say what changed and when
jikko.retract {"path":"...","expected_revision":N,"reason":"..."} leaves an auditable tombstone
game.inspect {"ref":"id"} shows an object you can observe
`, b.ReadMany)
	fmt.Fprintf(&s, "Each reply that calls tools uses one of your %d rounds. Your writes take effect after every ruler has decided.\n", b.Rounds)
	if t.Reflection() {
		s.WriteString(`Finish with {"note":"..."}: a concise note of what you concluded (it is not an order).`)
	} else if native {
		s.WriteString(`Finish by calling game_submit {"orders":[...],"statement":"..."} or by replying with the decision JSON.`)
	} else {
		s.WriteString(`Finish with {"tool":"game.submit","orders":[...],"statement":"..."} or the plain decision object.`)
	}
	return s.String()
}

// Action is a parsed model reply: a tool call or a final answer.
type Action struct {
	Call     *ToolCall
	Decision *Decision
}

// ParseAction decodes one reply of the JSON action protocol, tolerating the
// code fences and preambles small models add. A final answer is a
// game.submit call, a plain decision object or a reflection {"note":...}.
func ParseAction(content string) (Action, error) {
	s := strings.TrimSpace(content)
	var m map[string]json.RawMessage
	if json.Unmarshal([]byte(s), &m) != nil {
		i, j := strings.Index(s, "{"), strings.LastIndex(s, "}")
		if i < 0 || j <= i || json.Unmarshal([]byte(s[i:j+1]), &m) != nil {
			return Action{}, fmt.Errorf("reply is not one JSON object")
		}
		s = s[i : j+1]
	}
	if _, ok := m["tool"]; ok {
		var c ToolCall
		if err := json.Unmarshal([]byte(s), &c); err != nil {
			return Action{}, fmt.Errorf("tool call arguments: %v", err)
		}
		if c.Tool == ToolSubmit {
			return Action{Decision: &Decision{Orders: c.Orders, Statement: c.Statement}}, nil
		}
		return Action{Call: &c}, nil
	}
	_, orders := m["orders"]
	_, statement := m["statement"]
	if orders || statement {
		var d Decision
		if err := json.Unmarshal([]byte(s), &d); err != nil {
			return Action{}, fmt.Errorf("decision: %v", err)
		}
		return Action{Decision: &d}, nil
	}
	if raw, ok := m["note"]; ok {
		var note string
		if err := json.Unmarshal(raw, &note); err != nil {
			return Action{}, fmt.Errorf("note must be a string")
		}
		return Action{Decision: &Decision{Statement: note}}, nil
	}
	return Action{}, fmt.Errorf(`reply is neither a tool call ({"tool":...}) nor a decision ({"orders":[...],"statement":"..."})`)
}

// DecideTools runs the tool loop: the model may call Jikko and game tools
// for up to the session's rounds, then submits its decision.
func (a OpenAICompatible) DecideTools(ctx context.Context, o Observation, t *Tools) (Decision, error) {
	sys := SystemPrompt(o) + "\n" + ToolProtocol(t, a.NativeTools)
	user := Prompt(o) + "\n" + t.Context()
	t.CountInput(sys + user)
	return a.toolLoop(ctx, t, []map[string]any{{"role": "system", "content": sys}, {"role": "user", "content": user}})
}

// RepairTools continues the same conversation with the problem found; the
// session's remaining tool budget still applies.
func (a OpenAICompatible) RepairTools(ctx context.Context, o Observation, t *Tools, prev Decision, problem string) (Decision, error) {
	msgs := append([]map[string]any(nil), prev.Transcript...)
	if len(msgs) == 0 {
		sys := SystemPrompt(o) + "\n" + ToolProtocol(t, a.NativeTools)
		user := Prompt(o) + "\n" + t.Context()
		t.CountInput(sys + user)
		msgs = []map[string]any{{"role": "system", "content": sys}, {"role": "user", "content": user}, {"role": "assistant", "content": prev.Raw}}
	}
	p := RepairPrompt(problem)
	t.CountInput(p)
	return a.toolLoop(ctx, t, append(msgs, map[string]any{"role": "user", "content": p}))
}

// ReflectTools runs the reflection phase with its own 4-round tool session;
// the model may write memory and finishes with a note.
func (a OpenAICompatible) ReflectTools(ctx context.Context, o Observation, t *Tools, triggers []string) (string, error) {
	a.MaxTokens = ReflectionMaxTokens
	sys := ReflectionPrompt(triggers) + "\n" + ToolProtocol(t, a.NativeTools)
	user := Prompt(o) + "\n" + t.Context()
	t.CountInput(sys + user)
	d, err := a.toolLoop(ctx, t, []map[string]any{{"role": "system", "content": sys}, {"role": "user", "content": user}})
	return strings.TrimSpace(d.Statement), err
}

func (a OpenAICompatible) toolLoop(ctx context.Context, t *Tools, msgs []map[string]any) (Decision, error) {
	var tools []any
	if a.NativeTools {
		tools = NativeToolSpecs(t.Reflection())
	}
	late := 0
	for {
		r, err := a.send(ctx, msgs, tools)
		if err != nil {
			return Decision{Transcript: msgs}, err
		}
		raw := r.Content
		if len(r.ToolCalls) > 0 {
			b, _ := json.Marshal(r.ToolCalls)
			raw = strings.TrimSpace(raw + "\n" + string(b))
		}
		t.CountOutput(raw)
		msgs = append(msgs, r.message())
		var calls []ToolCall
		var ids []string
		if len(r.ToolCalls) > 0 {
			for _, nc := range r.ToolCalls {
				var c ToolCall
				if args := strings.TrimSpace(nc.Function.Arguments); args != "" && args != "null" {
					if err := json.Unmarshal([]byte(args), &c); err != nil {
						c = ToolCall{Path: "(unparsable arguments)"}
					}
				}
				c.Tool = toolName(nc.Function.Name)
				if c.Tool == ToolSubmit && !t.Reflection() {
					return Decision{Orders: c.Orders, Statement: c.Statement, Raw: raw, Transcript: msgs}, nil
				}
				calls, ids = append(calls, c), append(ids, nc.ID)
			}
		} else {
			act, err := ParseAction(r.Content)
			switch {
			case err != nil && t.Reflection():
				return Decision{Statement: r.Content, Raw: raw, Transcript: msgs}, nil
			case err != nil:
				return Decision{Raw: raw, Transcript: msgs}, &MalformedError{Raw: raw, Err: err}
			case act.Decision != nil:
				d := *act.Decision
				d.Raw, d.Transcript = raw, msgs
				return d, nil
			}
			calls = []ToolCall{*act.Call}
		}
		if t.Exhausted() {
			if late++; late > 1 {
				return Decision{Raw: raw, Transcript: msgs}, &MalformedError{Raw: raw, Err: fmt.Errorf("kept calling tools after the tool budget was exhausted")}
			}
		}
		results := t.Round(calls)
		for i, res := range results {
			if ids != nil {
				msgs = append(msgs, map[string]any{"role": "tool", "tool_call_id": ids[i], "content": res})
				continue
			}
			p := "Result of " + calls[i].Tool + ": "
			t.CountInput(p)
			msgs = append(msgs, map[string]any{"role": "user", "content": p + res})
		}
	}
}

// toolName maps a native function name (jikko_read_many) to the canonical
// tool name (jikko.read_many).
func toolName(fn string) string {
	if strings.Contains(fn, ".") {
		return fn
	}
	return strings.Replace(fn, "_", ".", 1)
}

// NativeToolSpecs are the tools as OpenAI function definitions.
func NativeToolSpecs(reflection bool) []any {
	str := map[string]any{"type": "string"}
	obj := func(props map[string]any, required ...string) map[string]any {
		o := map[string]any{"type": "object", "properties": props}
		if len(required) > 0 {
			o["required"] = required
		}
		return o
	}
	order := map[string]any{"type": "object", "properties": map[string]any{"type": str, "actor": str, "target": str, "params": map[string]any{"type": "object"}}, "required": []string{"type"}}
	defs := []struct {
		name, desc string
		params     map[string]any
	}{
		{ToolTree, "List the Jikko documents you can read, optionally under a directory.", obj(map[string]any{"path": str})},
		{ToolRead, "Read one Jikko document.", obj(map[string]any{"path": str}, "path")},
		{ToolReadMany, "Read several Jikko documents.", obj(map[string]any{"paths": map[string]any{"type": "array", "items": str}}, "paths")},
		{ToolSearch, "Search the documents you can read.", obj(map[string]any{"query": str}, "query")},
		{ToolMentions, "Documents that mention you or your alliance.", obj(map[string]any{})},
		{ToolCreate, "Create a document (durable memory) or task (current plan).", obj(map[string]any{"path": str, "content": str, "type": map[string]any{"type": "string", "enum": []string{"document", "task"}}}, "path", "content")},
		{ToolUpdate, "Update a document: tasks may be rewritten, documents only take appended corrections.", obj(map[string]any{"path": str, "expected_revision": map[string]any{"type": "integer"}, "content": str}, "path", "expected_revision", "content")},
		{ToolRetract, "Retract a document, leaving an auditable tombstone.", obj(map[string]any{"path": str, "expected_revision": map[string]any{"type": "integer"}, "reason": str}, "path", "expected_revision", "reason")},
		{ToolInspect, "Show an object you can observe by id.", obj(map[string]any{"ref": str}, "ref")},
	}
	if !reflection {
		defs = append(defs, struct {
			name, desc string
			params     map[string]any
		}{ToolSubmit, "Submit this turn's orders and a concise statement. Ends the turn.", obj(map[string]any{"orders": map[string]any{"type": "array", "items": order}, "statement": str}, "orders")})
	}
	out := make([]any, len(defs))
	for i, d := range defs {
		out[i] = map[string]any{"type": "function", "function": map[string]any{"name": strings.Replace(d.name, ".", "_", 1), "description": d.desc, "parameters": d.params}}
	}
	return out
}

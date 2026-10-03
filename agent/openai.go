package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type OpenAICompatible struct {
	Endpoint, APIKey, Model string
	Client                  *http.Client
	// MaxTokens caps the decision output; 0 means the canonical 2,000
	// (spec/agents.md, Small-model budgets).
	MaxTokens int
	// NativeTools offers the Jikko and game tools as OpenAI function tools
	// instead of the JSON action protocol (D69). Both drive the same loop.
	NativeTools bool
}

// DefaultMaxTokens is the canonical final-decision output budget.
const DefaultMaxTokens = 2000

func (a OpenAICompatible) Decide(ctx context.Context, o Observation) (Decision, error) {
	return a.complete(ctx, []map[string]any{
		{"role": "system", "content": SystemPrompt(o)},
		{"role": "user", "content": Prompt(o)},
	})
}

// Repair re-asks the model with its previous answer and the problem found,
// keeping the original system prompt and observation.
func (a OpenAICompatible) Repair(ctx context.Context, o Observation, prev Decision, problem string) (Decision, error) {
	raw := prev.Raw
	if raw == "" {
		b, _ := json.Marshal(prev)
		raw = string(b)
	}
	return a.complete(ctx, []map[string]any{
		{"role": "system", "content": SystemPrompt(o)},
		{"role": "user", "content": Prompt(o)},
		{"role": "assistant", "content": raw},
		{"role": "user", "content": RepairPrompt(problem)},
	})
}

// Temperature is the sampling temperature of every request.
const Temperature = 0.3

// ReflectionMaxTokens bounds a reflection note.
const ReflectionMaxTokens = 400

// Reflect asks the model for a short durable memory note about the events
// that triggered reflection. The answer is free text; no orders are read.
func (a OpenAICompatible) Reflect(ctx context.Context, o Observation, triggers []string) (string, error) {
	a.MaxTokens = ReflectionMaxTokens
	content, err := a.chat(ctx, []map[string]any{
		{"role": "system", "content": ReflectionPrompt(triggers)},
		{"role": "user", "content": Prompt(o)},
	})
	return strings.TrimSpace(content), err
}

// ReflectionPrompt asks for a memory note, not orders.
func ReflectionPrompt(triggers []string) string {
	return "Reflection phase after: " + strings.Join(triggers, ", ") + ". You cannot give orders now. " +
		"Write a concise note (at most 120 words) of what changed, what you now believe and what you want to remember. " +
		"Do not rewrite earlier observations; state corrections explicitly."
}

// Config describes the inference settings for the run record.
func (a OpenAICompatible) Config() map[string]any {
	mt := a.MaxTokens
	if mt <= 0 {
		mt = DefaultMaxTokens
	}
	protocol := "json"
	if a.NativeTools {
		protocol = "native"
	}
	return map[string]any{"model": a.Model, "temperature": Temperature, "max_tokens": mt, "reflection_max_tokens": ReflectionMaxTokens, "prompt_version": PromptVersion, "tool_protocol": protocol}
}

// RepairPrompt is the message that feeds a validation problem back.
func RepairPrompt(problem string) string {
	return "Your previous answer could not be used: " + problem + "\nReply again with the corrected JSON decision only."
}

func (a OpenAICompatible) complete(ctx context.Context, messages []map[string]any) (Decision, error) {
	content, err := a.chat(ctx, messages)
	if err != nil {
		return Decision{}, err
	}
	d, err := ParseDecision(content)
	if err != nil {
		return Decision{Raw: content}, &MalformedError{Raw: content, Err: err}
	}
	d.Raw = content
	return d, nil
}

// chat sends one chat-completion request and returns the reply text.
func (a OpenAICompatible) chat(ctx context.Context, messages []map[string]any) (string, error) {
	r, err := a.send(ctx, messages, nil)
	return r.Content, err
}

// nativeCall is one OpenAI tool call in a reply.
type nativeCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// chatReply is one assistant reply.
type chatReply struct {
	Content   string
	ToolCalls []nativeCall
}

// message is the reply as a conversation message, to send back next round.
func (r chatReply) message() map[string]any {
	m := map[string]any{"role": "assistant", "content": r.Content}
	if len(r.ToolCalls) > 0 {
		m["tool_calls"] = r.ToolCalls
	}
	return m
}

// send posts one chat-completion request, offering tools when given.
func (a OpenAICompatible) send(ctx context.Context, messages []map[string]any, tools []any) (chatReply, error) {
	maxTokens := a.MaxTokens
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
	}
	body := map[string]any{"model": a.Model, "temperature": Temperature, "max_tokens": maxTokens, "messages": messages}
	if len(tools) > 0 {
		body["tools"] = tools
	}
	b, err := json.Marshal(body)
	if err != nil {
		return chatReply{}, err
	}
	url := strings.TrimRight(a.Endpoint, "/") + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	if err != nil {
		return chatReply{}, fmt.Errorf("model request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if a.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+a.APIKey)
	}
	c := a.Client
	if c == nil {
		c = http.DefaultClient
	}
	resp, err := c.Do(req)
	if err != nil {
		return chatReply{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return chatReply{}, fmt.Errorf("model HTTP %s", resp.Status)
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content   *string      `json:"content"`
				ToolCalls []nativeCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, maxModelResponse)).Decode(&out); err != nil || len(out.Choices) == 0 {
		return chatReply{}, fmt.Errorf("invalid model response")
	}
	m := out.Choices[0].Message
	r := chatReply{ToolCalls: m.ToolCalls}
	if m.Content != nil {
		r.Content = *m.Content
	}
	return r, nil
}

const maxModelResponse = 8 << 20

// ParseDecision decodes a model's decision envelope, tolerating the Markdown code fences and short preambles small models commonly add around the JSON object.
func ParseDecision(content string) (Decision, error) {
	var d Decision
	s := strings.TrimSpace(content)
	if err := json.Unmarshal([]byte(s), &d); err == nil {
		return d, nil
	}
	if i, j := strings.Index(s, "{"), strings.LastIndex(s, "}"); i >= 0 && j > i {
		var x Decision
		if err := json.Unmarshal([]byte(s[i:j+1]), &x); err == nil {
			return x, nil
		}
	}
	return Decision{}, fmt.Errorf("model output is not a decision JSON object")
}

// Name identifies the agent and model in run records.
func (a OpenAICompatible) Name() string { return "openai-compatible:" + a.Model }

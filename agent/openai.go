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
}

// DefaultMaxTokens is the canonical final-decision output budget.
const DefaultMaxTokens = 2000

func (a OpenAICompatible) Decide(ctx context.Context, o Observation) (Decision, error) {
	return a.complete(ctx, []map[string]string{
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
	return a.complete(ctx, []map[string]string{
		{"role": "system", "content": SystemPrompt(o)},
		{"role": "user", "content": Prompt(o)},
		{"role": "assistant", "content": raw},
		{"role": "user", "content": RepairPrompt(problem)},
	})
}

// RepairPrompt is the message that feeds a validation problem back.
func RepairPrompt(problem string) string {
	return "Your previous answer could not be used: " + problem + "\nReply again with the corrected JSON decision only."
}

func (a OpenAICompatible) complete(ctx context.Context, messages []map[string]string) (Decision, error) {
	maxTokens := a.MaxTokens
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
	}
	body := map[string]any{"model": a.Model, "temperature": 0.3, "max_tokens": maxTokens, "messages": messages}
	b, err := json.Marshal(body)
	if err != nil {
		return Decision{}, err
	}
	url := strings.TrimRight(a.Endpoint, "/") + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	if err != nil {
		return Decision{}, fmt.Errorf("model request: %w", err)
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
		return Decision{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return Decision{}, fmt.Errorf("model HTTP %s", resp.Status)
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, maxModelResponse)).Decode(&out); err != nil || len(out.Choices) == 0 {
		return Decision{}, fmt.Errorf("invalid model response")
	}
	content := out.Choices[0].Message.Content
	d, err := ParseDecision(content)
	if err != nil {
		return Decision{Raw: content}, &MalformedError{Raw: content, Err: err}
	}
	d.Raw = content
	return d, nil
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

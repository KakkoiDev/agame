package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type OpenAICompatible struct {
	Endpoint, APIKey, Model string
	Client                  *http.Client
	Temperature             float64
}

func (a OpenAICompatible) Generate(ctx context.Context, req TextRequest) (string, error) {
	b, _ := json.Marshal(req)
	content, err := a.complete(ctx,
		"Write only the requested in-world text. Do not make gameplay decisions or invent facts.",
		string(b))
	return content, err
}

func (a OpenAICompatible) Decide(ctx context.Context, o Observation) (Decision, error) {
	obs, _ := json.Marshal(o)
	content, err := a.complete(ctx,
		`Return JSON only: {"orders":[],"writes":[],"statement":""}. Choose legal AGame orders and optional writing intents. Never invent IDs.`,
		string(obs))
	if err != nil { return Decision{}, err }
	var d Decision
	if err := json.Unmarshal([]byte(content), &d); err != nil { return Decision{}, err }
	return d, nil
}

func (a OpenAICompatible) complete(ctx context.Context, system, user string) (string, error) {
	body := map[string]any{"model": a.Model, "temperature": a.Temperature,
		"messages": []map[string]string{{"role":"system","content":system},{"role":"user","content":user}}}
	b, _ := json.Marshal(body)
	url := strings.TrimRight(a.Endpoint, "/") + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	if err != nil { return "", err }
	req.Header.Set("Content-Type", "application/json")
	if a.APIKey != "" { req.Header.Set("Authorization", "Bearer "+a.APIKey) }
	c := a.Client; if c == nil { c = http.DefaultClient }
	resp, err := c.Do(req); if err != nil { return "", err }
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 { return "", fmt.Errorf("model HTTP %s", resp.Status) }
	var out struct{ Choices []struct{ Message struct{ Content string `json:"content"` } `json:"message"` } `json:"choices"` }
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || len(out.Choices)==0 { return "", fmt.Errorf("invalid model response") }
	return out.Choices[0].Message.Content, nil
}

package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/KakkoiDev/agame/world"
)

func chatServer(t *testing.T, status int, content string, check func(*http.Request, map[string]any)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		var body map[string]any
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		if check != nil {
			check(r, body)
		}
		rw.WriteHeader(status)
		resp, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": content}}}})
		_, _ = rw.Write(resp)
	}))
}

func TestOpenAIDecideParsesOrdersAndSendsRequest(t *testing.T) {
	srv := chatServer(t, 200, `{"orders":[{"type":"construct","actor":"s00-p1","target":"metal_mine"}],"statement":"grow"}`,
		func(r *http.Request, body map[string]any) {
			if r.URL.Path != "/v1/chat/completions" || r.Method != http.MethodPost {
				t.Errorf("request %s %s", r.Method, r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer k" {
				t.Errorf("auth header %q", r.Header.Get("Authorization"))
			}
			if body["model"] != "m" {
				t.Errorf("model %v", body["model"])
			}
			msgs, _ := body["messages"].([]any)
			if len(msgs) != 2 {
				t.Errorf("messages %v", msgs)
			}
		})
	defer srv.Close()
	a := OpenAICompatible{Endpoint: srv.URL + "/", APIKey: "k", Model: "m"}
	d, err := a.Decide(context.Background(), Observe(genWorld(t, 1), "e00"))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Orders) != 1 || d.Orders[0].Type != "construct" || d.Orders[0].Actor != "s00-p1" || d.Statement != "grow" {
		t.Fatalf("decision=%+v", d)
	}
}

func TestOpenAIDecideToleratesMarkdownFences(t *testing.T) {
	// Small local models very often wrap JSON in a ```json fence or add a
	// short preamble despite being told not to.
	for _, content := range []string{
		"```json\n{\"orders\":[],\"statement\":\"ok\"}\n```",
		"```\n{\"orders\":[],\"statement\":\"ok\"}\n```",
		"Here is my decision:\n{\"orders\":[],\"statement\":\"ok\"}",
	} {
		srv := chatServer(t, 200, content, nil)
		d, err := OpenAICompatible{Endpoint: srv.URL}.Decide(context.Background(), Observation{})
		srv.Close()
		if err != nil || d.Statement != "ok" {
			t.Errorf("content %q: decision=%+v err=%v", content, d, err)
		}
	}
}

func TestOpenAIDecideErrors(t *testing.T) {
	cases := map[string]*httptest.Server{
		"http 500": chatServer(t, 500, "", nil),
		"not json": chatServer(t, 200, "I refuse.", nil),
		"no choices": httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			_, _ = rw.Write([]byte(`{"choices":[]}`))
		})),
		"garbage body": httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			_, _ = rw.Write([]byte(`<html>`))
		})),
	}
	for name, srv := range cases {
		_, err := OpenAICompatible{Endpoint: srv.URL}.Decide(context.Background(), Observation{})
		srv.Close()
		if err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestOpenAIDecideBadEndpointReturnsError(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Decide panicked on a malformed endpoint: %v", r)
		}
	}()
	if _, err := (OpenAICompatible{Endpoint: "http://bad\x7f host"}).Decide(context.Background(), Observation{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestOpenAIDecideHonoursContext(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) { <-block }))
	defer srv.Close()
	defer close(block)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := OpenAICompatible{Endpoint: srv.URL}.Decide(ctx, Observation{})
	if err == nil || !strings.Contains(err.Error(), "context deadline") {
		t.Fatalf("err=%v", err)
	}
}

func TestParseDecision(t *testing.T) {
	d, err := ParseDecision("Sure!\n```json\n{\"orders\":[{\"type\":\"message\",\"target\":\"e01\",\"params\":{\"body\":\"{peace}\"}}],\"statement\":\"talk\"}\n```\nGood luck.")
	if err != nil || len(d.Orders) != 1 || d.Orders[0].Params["body"] != "{peace}" {
		t.Fatalf("d=%+v err=%v", d, err)
	}
	for _, bad := range []string{"", "{", "no json", `{"orders":"nope"}`} {
		if _, err := ParseDecision(bad); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
}

func TestPromptIsCompactCompleteAndDeterministic(t *testing.T) {
	w := genWorld(t, 3)
	w.Fleets["f1"] = &world.Fleet{ID: "f1", OwnerID: "e00", SystemID: "s00", Ships: world.Ships{"scout": 1}}
	w.Messages = []world.Message{{Turn: 0, From: "e01", To: "e00", Body: "peace?"}}
	o := Observe(w, "e00")
	sys, user := SystemPrompt(o), Prompt(o)
	if sys != SystemPrompt(Observe(w, "e00")) || user != Prompt(Observe(w, "e00")) {
		t.Fatal("prompt not deterministic")
	}
	for _, s := range world.OrderSpecs() {
		if !strings.Contains(sys, "\n"+s.Type+": ") {
			t.Fatalf("system prompt lacks order %s:\n%s", s.Type, sys)
		}
	}
	h := w.Planets[w.Empires["e00"].HomeworldID]
	hs := w.Systems[h.SystemID]
	line := hs.ID + ": " + strings.Join(hs.Neighbors, " ") + " | "
	for _, want := range []string{line, h.ID + "=e00*", `"ID":"f1"`, `"ID":"` + h.ID + `"`, "peace?", "-=unowned"} {
		if !strings.Contains(user, want) {
			t.Fatalf("prompt lacks %q:\n%s", want, user)
		}
	}
	for _, s := range w.Systems {
		if !strings.Contains(user, "\n"+s.ID+": ") {
			t.Fatalf("graph line missing for %s", s.ID)
		}
	}
	// Rough budget guard: ~4 chars/token, the 8k-token initial context
	// must leave room for rules, identity and the Jikko tree.
	if n := len(sys) + len(user); n > 12000 {
		t.Fatalf("prompt is %d chars", n)
	}
	if Prompt(Observation{Turn: 2}) == "" {
		t.Fatal("empty prompt for a missing empire")
	}
}

func TestOpenAIRequestCarriesPrompt(t *testing.T) {
	w := genWorld(t, 1)
	o := Observe(w, "e00")
	srv := chatServer(t, 200, `{"orders":[],"statement":"wait"}`, func(_ *http.Request, body map[string]any) {
		msgs, _ := body["messages"].([]any)
		if len(msgs) != 2 {
			t.Errorf("messages %v", msgs)
			return
		}
		sys, _ := msgs[0].(map[string]any)["content"].(string)
		user, _ := msgs[1].(map[string]any)["content"].(string)
		if sys != SystemPrompt(o) || user != Prompt(o) {
			t.Errorf("request does not carry the rendered prompt")
		}
	})
	defer srv.Close()
	if _, err := (OpenAICompatible{Endpoint: srv.URL, Model: "m"}).Decide(context.Background(), o); err != nil {
		t.Fatal(err)
	}
}

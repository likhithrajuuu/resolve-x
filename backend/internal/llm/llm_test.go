package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var tools = []Tool{{Name: "list_services", Description: "d", Schema: map[string]any{"type": "object", "properties": map[string]any{"window": map[string]any{"type": "string"}}, "additionalProperties": false}}}

func serve(t *testing.T, handler func(body map[string]any, r *http.Request) string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(handler(body, r)))
	}))
}

func TestAnthropicToolRoundTrip(t *testing.T) {
	n := 0
	srv := serve(t, func(b map[string]any, r *http.Request) string {
		n++
		if r.Header.Get("x-api-key") != "k" || r.Header.Get("anthropic-version") == "" || r.URL.Path != "/v1/messages" {
			t.Errorf("bad request: %v %s", r.Header, r.URL.Path)
		}
		if n == 1 {
			if _, ok := b["tools"].([]any)[0].(map[string]any)["input_schema"]; !ok {
				t.Error("tool schema must be sent as input_schema")
			}
			return `{"content":[{"type":"text","text":"checking"},{"type":"tool_use","id":"tu1","name":"list_services","input":{"window":"1h"}}]}`
		}
		last := b["messages"].([]any)
		blocks := last[len(last)-1].(map[string]any)["content"].([]any)
		if blocks[0].(map[string]any)["type"] != "tool_result" || blocks[0].(map[string]any)["tool_use_id"] != "tu1" {
			t.Errorf("tool result not echoed: %v", blocks)
		}
		return `{"content":[{"type":"text","text":"done"}]}`
	})
	defer srv.Close()
	p, _ := New(Config{Provider: "anthropic", APIKey: "k", BaseURL: srv.URL}, nil)
	r1, err := p.Generate(context.Background(), Request{System: "s", Tools: tools, Messages: []Message{{Role: "user", Text: "hi"}}})
	if err != nil || len(r1.Calls) != 1 || r1.Calls[0].Name != "list_services" || r1.Text != "checking" {
		t.Fatalf("%+v %v", r1, err)
	}
	r2, err := p.Generate(context.Background(), Request{Tools: tools, Messages: []Message{{Role: "user", Text: "hi"},
		{Role: "assistant", Text: r1.Text, Calls: r1.Calls}, {Role: "tool", Results: []Result{{CallID: "tu1", Name: "list_services", Content: "[]"}}}}})
	if err != nil || r2.Text != "done" {
		t.Fatalf("%+v %v", r2, err)
	}
}

func TestOpenAIResponsesChainsWithPreviousResponseID(t *testing.T) {
	n := 0
	srv := serve(t, func(b map[string]any, r *http.Request) string {
		n++
		if r.Header.Get("Authorization") != "Bearer k" || r.URL.Path != "/v1/responses" {
			t.Errorf("bad request %s", r.URL.Path)
		}
		if n == 1 {
			if b["previous_response_id"] != nil {
				t.Error("first call must not chain")
			}
			return `{"id":"resp_1","output":[{"type":"function_call","call_id":"c1","name":"list_services","arguments":"{\"window\":\"1h\"}"}]}`
		}
		if b["previous_response_id"] != "resp_1" {
			t.Errorf("second call must chain, got %v", b["previous_response_id"])
		}
		in := b["input"].([]any)
		if len(in) != 1 || in[0].(map[string]any)["type"] != "function_call_output" || in[0].(map[string]any)["call_id"] != "c1" {
			t.Errorf("only the tool output should be sent: %v", in)
		}
		return `{"id":"resp_2","output":[{"type":"message","content":[{"type":"output_text","text":"all good"}]}]}`
	})
	defer srv.Close()
	p, _ := New(Config{Provider: "openai", Model: "gpt-5-codex", APIKey: "k", BaseURL: srv.URL}, nil)
	r1, err := p.Generate(context.Background(), Request{Tools: tools, Messages: []Message{{Role: "user", Text: "hi"}}})
	if err != nil || len(r1.Calls) != 1 || r1.State != "resp_1" || string(r1.Calls[0].Args) != `{"window":"1h"}` {
		t.Fatalf("%+v %v", r1, err)
	}
	r2, err := p.Generate(context.Background(), Request{State: r1.State, Tools: tools, Messages: []Message{{Role: "user", Text: "hi"},
		{Role: "assistant", Calls: r1.Calls}, {Role: "tool", Results: []Result{{CallID: "c1", Content: "[]"}}}}})
	if err != nil || r2.Text != "all good" {
		t.Fatalf("%+v %v", r2, err)
	}
}

func TestGeminiFunctionCalling(t *testing.T) {
	n := 0
	srv := serve(t, func(b map[string]any, r *http.Request) string {
		n++
		if r.Header.Get("x-goog-api-key") != "k" || !strings.HasSuffix(r.URL.Path, "/models/gemini-x:generateContent") || r.URL.RawQuery != "" {
			t.Errorf("bad request %s ?%s (key must be a header)", r.URL.Path, r.URL.RawQuery)
		}
		decl := b["tools"].([]any)[0].(map[string]any)["functionDeclarations"].([]any)[0].(map[string]any)
		if _, bad := decl["parameters"].(map[string]any)["additionalProperties"]; bad {
			t.Error("additionalProperties must be stripped for Gemini")
		}
		if n == 1 {
			return `{"candidates":[{"content":{"parts":[{"functionCall":{"name":"list_services","args":{"window":"1h"}}}]}}]}`
		}
		contents := b["contents"].([]any)
		fr := contents[len(contents)-1].(map[string]any)["parts"].([]any)[0].(map[string]any)["functionResponse"].(map[string]any)
		if fr["name"] != "list_services" {
			t.Errorf("functionResponse: %v", fr)
		}
		return `{"candidates":[{"content":{"parts":[{"text":"fine"}]}}]}`
	})
	defer srv.Close()
	p, _ := New(Config{Provider: "gemini", Model: "gemini-x", APIKey: "k", BaseURL: srv.URL}, nil)
	r1, err := p.Generate(context.Background(), Request{Tools: tools, Messages: []Message{{Role: "user", Text: "hi"}}})
	if err != nil || len(r1.Calls) != 1 {
		t.Fatalf("%+v %v", r1, err)
	}
	r2, err := p.Generate(context.Background(), Request{Tools: tools, Messages: []Message{{Role: "user", Text: "hi"},
		{Role: "assistant", Calls: r1.Calls}, {Role: "tool", Results: []Result{{CallID: "call_0", Name: "list_services", Content: "[]"}}}}})
	if err != nil || r2.Text != "fine" {
		t.Fatalf("%+v %v", r2, err)
	}
}

func TestChatCompletionsCompatible(t *testing.T) {
	n := 0
	srv := serve(t, func(b map[string]any, r *http.Request) string {
		n++
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path %s", r.URL.Path)
		}
		if n == 1 {
			return `{"choices":[{"message":{"content":"","tool_calls":[{"id":"t1","function":{"name":"list_services","arguments":"{}"}}]}}]}`
		}
		msgs := b["messages"].([]any)
		if msgs[len(msgs)-1].(map[string]any)["role"] != "tool" {
			t.Errorf("tool message missing: %v", msgs)
		}
		return `{"choices":[{"message":{"content":"ok"}}]}`
	})
	defer srv.Close()
	p, _ := New(Config{Provider: "openai-compatible", Model: "llama", BaseURL: srv.URL + "/v1"}, nil) // no key: Ollama style
	r1, err := p.Generate(context.Background(), Request{Tools: tools, Messages: []Message{{Role: "user", Text: "hi"}}})
	if err != nil || len(r1.Calls) != 1 {
		t.Fatalf("%+v %v", r1, err)
	}
	r2, err := p.Generate(context.Background(), Request{Tools: tools, Messages: []Message{{Role: "user", Text: "hi"}, {Role: "assistant", Calls: r1.Calls},
		{Role: "tool", Results: []Result{{CallID: "t1", Content: "x"}}}}})
	if err != nil || r2.Text != "ok" {
		t.Fatalf("%+v %v", r2, err)
	}
}

func TestErrorsDoNotLeakKeysAndAreHelpful(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid x-api-key"}}`))
	}))
	defer srv.Close()
	p, _ := New(Config{Provider: "anthropic", APIKey: "sk-secret-123", BaseURL: srv.URL}, nil)
	_, err := p.Generate(context.Background(), Request{Messages: []Message{{Role: "user", Text: "hi"}}})
	if err == nil || strings.Contains(err.Error(), "sk-secret-123") || !strings.Contains(err.Error(), "check the API key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigValidation(t *testing.T) {
	if _, err := New(Config{Provider: "nope", Model: "m"}, nil); err == nil {
		t.Error("unknown provider accepted")
	}
	if _, err := New(Config{Provider: "openai-compatible", Model: "m"}, nil); err == nil {
		t.Error("compatible without base URL accepted")
	}
	if p, err := New(Config{Provider: "gemini"}, nil); err != nil || p == nil {
		t.Errorf("default model should apply: %v", err)
	}
}

func TestSafeClientBlocksPrivateAddresses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close() // listens on 127.0.0.1
	if _, err := SafeClient(false, 0).Get(srv.URL); err == nil {
		t.Error("loopback must be blocked by default")
	}
	if resp, err := SafeClient(true, 0).Get(srv.URL); err != nil {
		t.Errorf("allowPrivate should connect: %v", err)
	} else {
		resp.Body.Close()
	}
	if ValidateBaseURL("http://example.com", false) == nil || ValidateBaseURL("https://user:pw@example.com", false) == nil || ValidateBaseURL("https://example.com/v1", false) != nil {
		t.Error("base URL validation wrong")
	}
}

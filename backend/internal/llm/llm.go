// Package llm gives Resolve-X one interface over several model providers, so
// the chat assistant and root-cause refinement work with whichever one an
// organization configures. Providers are stateless HTTP clients; credentials
// are supplied per call by the caller.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type Tool struct {
	Name        string
	Description string
	Schema      map[string]any // JSON Schema for the arguments object
}

type Call struct {
	ID   string
	Name string
	Args json.RawMessage
}

type Result struct {
	CallID  string
	Name    string
	Content string
	IsError bool
}

// Message roles: "user", "assistant" (text and/or tool calls), "tool" (results).
type Message struct {
	Role    string
	Text    string
	Calls   []Call
	Results []Result
}

type Request struct {
	System    string
	Messages  []Message
	Tools     []Tool
	MaxTokens int
	// State lets a provider continue a server-side conversation between the
	// steps of one agent loop (used by the OpenAI Responses API).
	State string
}

type Response struct {
	Text  string
	Calls []Call
	State string
}

type Provider interface {
	Generate(ctx context.Context, req Request) (Response, error)
}

type Config struct {
	Provider string // anthropic | openai | gemini | openai-compatible
	Model    string
	BaseURL  string // optional override (proxy, gateway, self-hosted)
	APIKey   string
}

type Info struct {
	ID           string   `json:"id"`
	Label        string   `json:"label"`
	DefaultModel string   `json:"defaultModel"`
	Models       []string `json:"models"`
	NeedsBaseURL bool     `json:"needsBaseUrl"`
	KeyOptional  bool     `json:"keyOptional"`
	Help         string   `json:"help"`
}

// Catalog lists what the UI offers. Model lists are suggestions only: the
// field is free text because providers ship new models constantly.
var Catalog = []Info{
	{ID: "anthropic", Label: "Anthropic Claude", DefaultModel: "claude-sonnet-5-5", Models: []string{"claude-sonnet-5-5", "claude-opus-5-5", "claude-haiku-5-5"},
		Help: "Uses the Messages API with tool use."},
	{ID: "openai", Label: "OpenAI (GPT and Codex models)", DefaultModel: "gpt-5", Models: []string{"gpt-5", "gpt-5-codex", "gpt-4.1", "o4-mini"},
		Help: "Uses the Responses API, which also serves the Codex models. Requests are stored by OpenAI per its retention policy."},
	{ID: "gemini", Label: "Google Gemini", DefaultModel: "gemini-2.5-pro", Models: []string{"gemini-2.5-pro", "gemini-2.5-flash"},
		Help: "Uses generateContent with function calling."},
	{ID: "openai-compatible", Label: "OpenAI-compatible endpoint", DefaultModel: "", Models: []string{}, NeedsBaseURL: true, KeyOptional: true,
		Help: "Any server speaking Chat Completions with tools: Ollama, vLLM, LM Studio, OpenRouter, Azure, LiteLLM…"},
}

var ErrNotConfigured = errors.New("no AI provider configured")

func New(cfg Config, hc *http.Client) (Provider, error) {
	if hc == nil {
		hc = http.DefaultClient
	}
	if strings.TrimSpace(cfg.Model) == "" {
		for _, i := range Catalog {
			if i.ID == cfg.Provider {
				cfg.Model = i.DefaultModel
			}
		}
	}
	if cfg.Model == "" {
		return nil, errors.New("a model name is required")
	}
	switch cfg.Provider {
	case "anthropic":
		return &anthropic{cfg: cfg, hc: hc}, nil
	case "openai":
		return &openaiResponses{cfg: cfg, hc: hc}, nil
	case "gemini":
		return &gemini{cfg: cfg, hc: hc}, nil
	case "openai-compatible":
		if cfg.BaseURL == "" {
			return nil, errors.New("openai-compatible needs a base URL")
		}
		return &chatCompletions{cfg: cfg, hc: hc}, nil
	}
	return nil, fmt.Errorf("unknown provider %q", cfg.Provider)
}

// httpError turns a failed provider response into a short, key-free error.
func httpError(provider string, status int, body []byte) error {
	msg := strings.TrimSpace(string(body))
	var env struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &env) == nil && env.Error.Message != "" {
		msg = env.Error.Message
	}
	if len(msg) > 300 {
		msg = msg[:300] + "…"
	}
	hint := ""
	switch status {
	case 401, 403:
		hint = " (check the API key and its permissions)"
	case 404:
		hint = " (check the model name and base URL)"
	case 429:
		hint = " (rate limited or out of quota)"
	}
	return fmt.Errorf("%s API returned HTTP %d%s: %s", provider, status, hint, msg)
}

func baseURL(cfg Config, def string) string {
	return strings.TrimRight(orDefault(cfg.BaseURL, def), "/")
}

func orDefault(v, d string) string {
	if v != "" {
		return v
	}
	return d
}

func maxTokens(r Request) int {
	if r.MaxTokens > 0 {
		return r.MaxTokens
	}
	return 2048
}

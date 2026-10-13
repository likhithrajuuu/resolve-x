package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/resolvex/resolve-x/backend/internal/aiconfig"
	"github.com/resolvex/resolve-x/backend/internal/authn"
	"github.com/resolvex/resolve-x/backend/internal/llm"
)

type API struct {
	Store        *aiconfig.Store
	Backend      *Backend
	Secret       []byte
	LLMClient    *http.Client // SSRF-safe client used for user-supplied URLs
	AllowPrivate bool
	limiters     sync.Map // tenant -> *rate.Limiter
	Revoker      authn.Revoker
}

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/ai/settings", a.getSettings)
	mux.HandleFunc("PUT /api/v1/ai/settings", a.putSettings)
	mux.HandleFunc("DELETE /api/v1/ai/settings", a.deleteSettings)
	mux.HandleFunc("POST /api/v1/ai/test", a.test)
	mux.HandleFunc("POST /api/v1/ai/chat", a.chat)
	return authn.RequireWith(a.Secret, a.Revoker, mux)
}

func claims(r *http.Request) *authn.Claims { return authn.FromContext(r.Context()) }

func isAdmin(c *authn.Claims) bool { return c.Role == "owner" || c.Role == "admin" }

func (a *API) provider(ctx context.Context, tenant string) (llm.Provider, aiconfig.Settings, error) {
	cfg, set, err := a.Store.Resolve(ctx, tenant)
	if err != nil {
		return nil, set, err
	}
	p, err := llm.New(cfg, a.LLMClient)
	return p, set, err
}

type settingsView struct {
	aiconfig.Settings
	Configured bool       `json:"configured"`
	Providers  []llm.Info `json:"providers"`
	CanEdit    bool       `json:"canEdit"`
}

func (a *API) getSettings(w http.ResponseWriter, r *http.Request) {
	_, set, err := a.Store.Resolve(r.Context(), claims(r).Tenant)
	if err != nil && !errors.Is(err, llm.ErrNotConfigured) {
		authn.WriteError(w, 500, "internal", "could not load settings")
		return
	}
	authn.WriteJSON(w, 200, settingsView{Settings: set, Configured: err == nil, Providers: llm.Catalog, CanEdit: isAdmin(claims(r))})
}

func (a *API) putSettings(w http.ResponseWriter, r *http.Request) {
	c := claims(r)
	if !isAdmin(c) {
		authn.WriteError(w, 403, "forbidden", "only owners and admins can change AI settings")
		return
	}
	var in struct{ Provider, Model, BaseURL, APIKey string }
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		authn.WriteError(w, 400, "invalid_request", "invalid JSON")
		return
	}
	in.APIKey, in.Model, in.BaseURL = strings.TrimSpace(in.APIKey), strings.TrimSpace(in.Model), strings.TrimSpace(in.BaseURL)
	var info *llm.Info
	for i := range llm.Catalog {
		if llm.Catalog[i].ID == in.Provider {
			info = &llm.Catalog[i]
		}
	}
	if info == nil {
		authn.WriteError(w, 400, "invalid_request", "unknown provider")
		return
	}
	if in.BaseURL != "" && !info.NeedsBaseURL && in.Provider != "openai" && in.Provider != "anthropic" && in.Provider != "gemini" {
		in.BaseURL = ""
	}
	if err := llm.ValidateBaseURL(in.BaseURL, a.AllowPrivate); err != nil {
		authn.WriteError(w, 400, "invalid_request", err.Error())
		return
	}
	if info.NeedsBaseURL && in.BaseURL == "" {
		authn.WriteError(w, 400, "invalid_request", "a base URL is required for this provider")
		return
	}
	if in.Model == "" {
		in.Model = info.DefaultModel
	}
	if in.Model == "" || len(in.Model) > 120 || len(in.APIKey) > 400 {
		authn.WriteError(w, 400, "invalid_request", "a model name is required")
		return
	}
	// A key is required the first time, and whenever the provider changes.
	_, cur, err := a.Store.Resolve(r.Context(), c.Tenant)
	hasKeyForProvider := err == nil && cur.Source == "tenant" && cur.Provider == in.Provider && cur.KeyHint != ""
	if in.APIKey == "" && !hasKeyForProvider && !info.KeyOptional {
		authn.WriteError(w, 400, "invalid_request", "an API key is required")
		return
	}
	if err := a.Store.Put(r.Context(), c.Tenant, c.Email, llm.Config{Provider: in.Provider, Model: in.Model, BaseURL: in.BaseURL, APIKey: in.APIKey}); err != nil {
		authn.WriteError(w, 500, "internal", "could not save settings")
		return
	}
	a.getSettings(w, r)
}

func (a *API) deleteSettings(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(claims(r)) {
		authn.WriteError(w, 403, "forbidden", "only owners and admins can change AI settings")
		return
	}
	if err := a.Store.Delete(r.Context(), claims(r).Tenant); err != nil {
		authn.WriteError(w, 500, "internal", "could not remove settings")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// test sends a tiny prompt so a bad key or model is caught at setup time.
func (a *API) test(w http.ResponseWriter, r *http.Request) {
	p, set, err := a.provider(r.Context(), claims(r).Tenant)
	if err != nil {
		authn.WriteJSON(w, 200, map[string]any{"ok": false, "error": notConfiguredMsg(err)})
		return
	}
	if !a.allow(claims(r).Tenant) {
		authn.WriteError(w, 429, "rate_limited", "too many requests; try again in a moment")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	started := time.Now()
	resp, err := p.Generate(ctx, llm.Request{System: "Reply with the single word OK.", Messages: []llm.Message{{Role: "user", Text: "ping"}}, MaxTokens: 16})
	if err != nil {
		authn.WriteJSON(w, 200, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	authn.WriteJSON(w, 200, map[string]any{"ok": true, "provider": set.Provider, "model": set.Model, "reply": resp.Text[:min(len(resp.Text), 40)], "ms": time.Since(started).Milliseconds()})
}

func notConfiguredMsg(err error) string {
	if errors.Is(err, llm.ErrNotConfigured) {
		return "No AI provider is configured. Add one in Settings."
	}
	return err.Error()
}

func (a *API) allow(tenant string) bool {
	v, _ := a.limiters.LoadOrStore(tenant, rate.NewLimiter(rate.Every(6*time.Second), 5)) // ~10/min sustained, burst 5
	return v.(*rate.Limiter).Allow()
}

func (a *API) chat(w http.ResponseWriter, r *http.Request) {
	c := claims(r)
	var in struct {
		Messages   []Turn `json:"messages"`
		IncidentID string `json:"incidentId"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 128<<10)
	if json.NewDecoder(r.Body).Decode(&in) != nil || len(in.Messages) == 0 || in.Messages[len(in.Messages)-1].Role != "user" {
		authn.WriteError(w, 400, "invalid_request", "send the conversation with a final user message")
		return
	}
	if len(in.Messages) > 24 {
		in.Messages = in.Messages[len(in.Messages)-24:]
	}
	for i := range in.Messages {
		in.Messages[i].Text = in.Messages[i].Text[:min(len(in.Messages[i].Text), 4000)]
	}
	p, _, err := a.provider(r.Context(), c.Tenant)
	if errors.Is(err, llm.ErrNotConfigured) {
		authn.WriteError(w, 409, "not_configured", "No AI provider is configured. Add one in Settings.")
		return
	}
	if err != nil {
		authn.WriteError(w, 500, "internal", err.Error())
		return
	}
	if !a.allow(c.Tenant) {
		authn.WriteError(w, 429, "rate_limited", "You're asking quickly. Wait a few seconds and try again.")
		return
	}
	extra := ""
	if idRe.MatchString(in.IncidentID) {
		extra = "The user is currently viewing incident " + in.IncidentID + "; fetch it with get_incident if the question is about it."
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	ans, err := Chat(r.Context(), p, a.Backend, token, in.Messages, extra)
	if err != nil {
		// The provider's own message (key, quota, model) is what the user needs; it never contains the key.
		authn.WriteJSON(w, 502, map[string]any{"error": "provider_error", "message": err.Error(), "steps": ans.Steps})
		return
	}
	authn.WriteJSON(w, 200, ans)
}

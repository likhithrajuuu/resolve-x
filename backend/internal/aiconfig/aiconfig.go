// Package aiconfig stores and resolves which LLM an organization uses.
// A tenant's own configuration wins; otherwise server-wide environment
// variables (AI_PROVIDER, ANTHROPIC_API_KEY, OPENAI_API_KEY, GEMINI_API_KEY)
// act as a fallback for self-hosted installs.
package aiconfig

import (
	"context"
	"errors"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/resolvex/resolve-x/backend/internal/llm"
	"github.com/resolvex/resolve-x/backend/internal/secrets"
)

type Store struct {
	DB  *pgxpool.Pool
	Box *secrets.Box
}

// Settings is the safe-to-display view; it never contains the key.
type Settings struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	BaseURL  string `json:"baseUrl"`
	KeyHint  string `json:"keyHint"`
	Source   string `json:"source"` // tenant | env | none
}

func envConfig() (llm.Config, bool) {
	p := os.Getenv("AI_PROVIDER")
	keys := map[string]string{"anthropic": "ANTHROPIC_API_KEY", "openai": "OPENAI_API_KEY", "gemini": "GEMINI_API_KEY"}
	if p == "" { // infer from whichever key is present
		for id, env := range keys {
			if os.Getenv(env) != "" {
				p = id
				break
			}
		}
	}
	if p == "" {
		return llm.Config{}, false
	}
	key := os.Getenv(keys[p])
	if p == "openai-compatible" {
		key = os.Getenv("AI_API_KEY")
	}
	if key == "" && p != "openai-compatible" {
		return llm.Config{}, false
	}
	return llm.Config{Provider: p, Model: os.Getenv("AI_MODEL"), BaseURL: os.Getenv("AI_BASE_URL"), APIKey: key}, true
}

// Resolve returns the usable configuration for a tenant, or llm.ErrNotConfigured.
func (s *Store) Resolve(ctx context.Context, tenant string) (llm.Config, Settings, error) {
	var set Settings
	var cipher string
	err := s.DB.QueryRow(ctx, `SELECT provider, model, base_url, key_cipher, key_hint FROM ai_settings WHERE tenant_id=$1`, tenant).
		Scan(&set.Provider, &set.Model, &set.BaseURL, &cipher, &set.KeyHint)
	if err == nil {
		key := ""
		if cipher != "" {
			if key, err = s.Box.Open(cipher); err != nil {
				return llm.Config{}, set, err
			}
		}
		set.Source = "tenant"
		return llm.Config{Provider: set.Provider, Model: set.Model, BaseURL: set.BaseURL, APIKey: key}, set, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return llm.Config{}, set, err
	}
	if c, ok := envConfig(); ok {
		return c, Settings{Provider: c.Provider, Model: c.Model, BaseURL: c.BaseURL, KeyHint: hint(c.APIKey), Source: "env"}, nil
	}
	return llm.Config{}, Settings{Source: "none"}, llm.ErrNotConfigured
}

func hint(k string) string {
	if len(k) <= 8 {
		return ""
	}
	return k[len(k)-4:]
}

// Put saves settings. An empty apiKey keeps the existing key.
func (s *Store) Put(ctx context.Context, tenant, actor string, c llm.Config) error {
	cipher, h := "", ""
	if c.APIKey != "" {
		var err error
		if cipher, err = s.Box.Seal(c.APIKey); err != nil {
			return err
		}
		h = hint(c.APIKey)
	}
	_, err := s.DB.Exec(ctx, `
		INSERT INTO ai_settings (tenant_id, provider, model, base_url, key_cipher, key_hint, updated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (tenant_id) DO UPDATE SET provider=EXCLUDED.provider, model=EXCLUDED.model, base_url=EXCLUDED.base_url,
			key_cipher = CASE WHEN EXCLUDED.key_cipher = '' AND ai_settings.provider = EXCLUDED.provider THEN ai_settings.key_cipher ELSE EXCLUDED.key_cipher END,
			key_hint   = CASE WHEN EXCLUDED.key_cipher = '' AND ai_settings.provider = EXCLUDED.provider THEN ai_settings.key_hint ELSE EXCLUDED.key_hint END,
			updated_by=EXCLUDED.updated_by, updated_at=now()`,
		tenant, c.Provider, c.Model, c.BaseURL, cipher, h, actor)
	return err
}

func (s *Store) Delete(ctx context.Context, tenant string) error {
	_, err := s.DB.Exec(ctx, `DELETE FROM ai_settings WHERE tenant_id=$1`, tenant)
	return err
}

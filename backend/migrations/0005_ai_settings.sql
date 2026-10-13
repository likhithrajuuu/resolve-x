-- Per-tenant LLM provider settings. The API key is encrypted (AES-256-GCM);
-- key_hint holds only the last four characters for display.
CREATE TABLE IF NOT EXISTS ai_settings (
    tenant_id  TEXT PRIMARY KEY REFERENCES tenants(id),
    provider   TEXT NOT NULL CHECK (provider IN ('anthropic', 'openai', 'gemini', 'openai-compatible')),
    model      TEXT NOT NULL,
    base_url   TEXT NOT NULL DEFAULT '',
    key_cipher TEXT NOT NULL DEFAULT '',
    key_hint   TEXT NOT NULL DEFAULT '',
    updated_by TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

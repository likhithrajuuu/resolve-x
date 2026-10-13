CREATE TABLE IF NOT EXISTS notification_channels (
    id            TEXT PRIMARY KEY,
    tenant_id     TEXT NOT NULL REFERENCES tenants(id),
    kind          TEXT NOT NULL CHECK (kind IN ('slack', 'webhook')),
    name          TEXT NOT NULL,
    url_cipher    TEXT NOT NULL,          -- webhook URLs are credentials (Slack URLs embed a secret)
    url_hint      TEXT NOT NULL,          -- scheme://host only, for display
    secret_cipher TEXT NOT NULL DEFAULT '',  -- HMAC signing secret (generic webhooks)
    enabled       BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS channels_tenant ON notification_channels(tenant_id);

CREATE TABLE IF NOT EXISTS notification_log (
    id          BIGSERIAL PRIMARY KEY,
    tenant_id   TEXT NOT NULL,
    channel_id  TEXT NOT NULL,
    event       TEXT NOT NULL,
    incident_id TEXT NOT NULL DEFAULT '',
    ok          BOOLEAN NOT NULL,
    error       TEXT NOT NULL DEFAULT '',
    at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS notification_log_tenant ON notification_log(tenant_id, at DESC);

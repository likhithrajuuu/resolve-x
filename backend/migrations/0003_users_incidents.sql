-- Users and memberships (api-gateway.md, Data).
CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS memberships (
    user_id   TEXT NOT NULL REFERENCES users(id),
    tenant_id TEXT NOT NULL REFERENCES tenants(id),
    role      TEXT NOT NULL CHECK (role IN ('owner', 'admin', 'member')),
    PRIMARY KEY (user_id, tenant_id)
);

-- Incident service (incident-service.md).
CREATE TABLE IF NOT EXISTS incidents (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL REFERENCES tenants(id),
    title       TEXT NOT NULL,
    service     TEXT NOT NULL DEFAULT '',
    severity    TEXT NOT NULL CHECK (severity IN ('SEV-1', 'SEV-2', 'SEV-3')),
    status      TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'investigating', 'identified', 'resolved')),
    source      TEXT NOT NULL DEFAULT 'manual',
    description TEXT NOT NULL DEFAULT '',
    fingerprint TEXT,                         -- dedup key for alerts / detector
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS incidents_tenant ON incidents(tenant_id, created_at DESC);
-- At most one unresolved incident per fingerprint.
CREATE UNIQUE INDEX IF NOT EXISTS incidents_open_fingerprint
    ON incidents(tenant_id, fingerprint) WHERE fingerprint IS NOT NULL AND status <> 'resolved';

CREATE TABLE IF NOT EXISTS incident_timeline (
    id          BIGSERIAL PRIMARY KEY,
    incident_id TEXT NOT NULL REFERENCES incidents(id),
    tenant_id   TEXT NOT NULL,
    at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    kind        TEXT NOT NULL,
    summary     TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS timeline_incident ON incident_timeline(incident_id, at);

CREATE TABLE IF NOT EXISTS analyses (
    id             TEXT PRIMARY KEY,          -- = analysis.completed eventId, makes consumption idempotent
    incident_id    TEXT NOT NULL REFERENCES incidents(id),
    tenant_id      TEXT NOT NULL,
    root_cause     TEXT NOT NULL,
    confidence     INT  NOT NULL,
    evidence       JSONB NOT NULL,
    recommendation JSONB NOT NULL,
    analyzer       TEXT NOT NULL,             -- 'heuristic' or an LLM model id
    approval       TEXT NOT NULL DEFAULT 'pending' CHECK (approval IN ('pending', 'approved', 'rejected', 'none')),
    decided_by     TEXT,
    decided_at     TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS analyses_incident ON analyses(incident_id, created_at DESC);

CREATE TABLE IF NOT EXISTS deployments (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL REFERENCES tenants(id),
    service     TEXT NOT NULL,
    version     TEXT NOT NULL,
    environment TEXT NOT NULL DEFAULT '',
    at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS deployments_lookup ON deployments(tenant_id, service, at DESC);

-- Transactional outbox: state change and event are written in one transaction;
-- a relay publishes rows to Kafka (data-flow.md, Delivery Rules).
CREATE TABLE IF NOT EXISTS outbox (
    id           BIGSERIAL PRIMARY KEY,
    topic        TEXT NOT NULL,
    key          TEXT NOT NULL,
    envelope     JSONB NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS outbox_unpublished ON outbox(id) WHERE published_at IS NULL;

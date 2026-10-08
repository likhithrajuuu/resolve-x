-- Gateway-owned tenancy data (api-gateway.md, Data).

CREATE TABLE IF NOT EXISTS tenants (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    rate_per_sec INT  NOT NULL DEFAULT 1000,   -- ingestion requests/sec per instance
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS projects (
    id         TEXT PRIMARY KEY,
    tenant_id  TEXT NOT NULL REFERENCES tenants(id),
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);

CREATE TABLE IF NOT EXISTS environments (
    id         TEXT PRIMARY KEY,
    tenant_id  TEXT NOT NULL REFERENCES tenants(id),
    project_id TEXT NOT NULL REFERENCES projects(id),
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, name)
);

-- Keys are stored hashed (SHA-256) and shown once at creation.
CREATE TABLE IF NOT EXISTS api_keys (
    id             TEXT PRIMARY KEY,
    tenant_id      TEXT NOT NULL REFERENCES tenants(id),
    project_id     TEXT NOT NULL REFERENCES projects(id),
    environment_id TEXT NOT NULL REFERENCES environments(id),
    key_hash       TEXT NOT NULL UNIQUE,
    key_prefix     TEXT NOT NULL,              -- first chars, for display only
    status         TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at     TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS api_keys_project ON api_keys(project_id);

CREATE TABLE IF NOT EXISTS audit_log (
    id         BIGSERIAL PRIMARY KEY,
    tenant_id  TEXT NOT NULL,
    actor      TEXT NOT NULL,
    action     TEXT NOT NULL,
    target     TEXT,
    at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

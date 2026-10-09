CREATE TABLE IF NOT EXISTS dashboards (
    id         TEXT PRIMARY KEY,
    tenant_id  TEXT NOT NULL REFERENCES tenants(id),
    name       TEXT NOT NULL,
    widgets    JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS dashboards_tenant ON dashboards(tenant_id);

CREATE TABLE IF NOT EXISTS alert_rules (
    id             TEXT PRIMARY KEY,
    tenant_id      TEXT NOT NULL REFERENCES tenants(id),
    name           TEXT NOT NULL,
    kind           TEXT NOT NULL CHECK (kind IN ('error_rate', 'latency_p95', 'metric')),
    service        TEXT NOT NULL DEFAULT '',
    metric         TEXT NOT NULL DEFAULT '',
    agg            TEXT NOT NULL DEFAULT 'avg',
    op             TEXT NOT NULL DEFAULT '>' CHECK (op IN ('>', '<')),
    threshold      DOUBLE PRECISION NOT NULL,
    window_minutes INT  NOT NULL DEFAULT 5 CHECK (window_minutes BETWEEN 1 AND 1440),
    severity       TEXT NOT NULL DEFAULT 'SEV-2' CHECK (severity IN ('SEV-1', 'SEV-2', 'SEV-3')),
    enabled        BOOLEAN NOT NULL DEFAULT true,
    last_state     TEXT NOT NULL DEFAULT 'unknown',
    last_value     DOUBLE PRECISION,
    last_eval_at   TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS alert_rules_tenant ON alert_rules(tenant_id);

CREATE TABLE IF NOT EXISTS slos (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL REFERENCES tenants(id),
    name        TEXT NOT NULL,
    service     TEXT NOT NULL,
    kind        TEXT NOT NULL CHECK (kind IN ('availability', 'latency')),
    objective   DOUBLE PRECISION NOT NULL CHECK (objective > 0 AND objective < 100),
    latency_ms  DOUBLE PRECISION,
    window_days INT NOT NULL DEFAULT 7 CHECK (window_days BETWEEN 1 AND 30),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS slos_tenant ON slos(tenant_id);

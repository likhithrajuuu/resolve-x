-- Read-only account for services that only query telemetry (query, incident,
-- analysis). Only the processor needs the read/write user.
CREATE USER IF NOT EXISTS resolvex_ro IDENTIFIED WITH sha256_password BY 'resolvex_ro';
GRANT SELECT ON default.* TO resolvex_ro;

-- LOCAL DEVELOPMENT ONLY. Never apply to a shared environment.
-- Dev ingestion key:  rx_dev_local_key
INSERT INTO tenants (id, name, rate_per_sec) VALUES ('tnt_dev', 'Dev Tenant', 100000) ON CONFLICT DO NOTHING;
INSERT INTO projects (id, tenant_id, name) VALUES ('prj_dev', 'tnt_dev', 'demo') ON CONFLICT DO NOTHING;
INSERT INTO environments (id, tenant_id, project_id, name) VALUES ('env_dev', 'tnt_dev', 'prj_dev', 'production') ON CONFLICT DO NOTHING;
INSERT INTO api_keys (id, tenant_id, project_id, environment_id, key_hash, key_prefix)
VALUES ('key_dev', 'tnt_dev', 'prj_dev', 'env_dev', 'cdea89b30b7818bfa58f31aa154e42cdf8c091debcb73dce446dc5436c64a12e', 'rx_dev_') ON CONFLICT DO NOTHING;

CREATE INDEX IF NOT EXISTS idx_repository_objects_digest ON repository_objects (digest);

CREATE TABLE usage_events (
    id          UUID        PRIMARY KEY,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    tenant_id   UUID        NOT NULL REFERENCES tenants(id),
    registry_id UUID        REFERENCES registries(id),
    repo_id     UUID        REFERENCES repositories(id),
    metric      TEXT        NOT NULL CHECK (metric IN ('storage-bytes', 'push-op-count', 'pull-op-count')),
    value       BIGINT      NOT NULL
);

CREATE INDEX ON usage_events (tenant_id, created_at);

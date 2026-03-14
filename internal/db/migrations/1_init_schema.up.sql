-- extensions
CREATE EXTENSION citext;

-- tenants
CREATE TABLE tenants (
  id UUID PRIMARY KEY,
  name TEXT NOT NULL,
  onboarded BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE UNIQUE INDEX unique_tenants_name ON tenants(name);

-- users
CREATE TABLE users (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL
    REFERENCES tenants(id),
  sub TEXT NOT NULL
);
CREATE UNIQUE INDEX unique_users_sub ON users(sub);

-- registries
CREATE TABLE registries (
  id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL
    REFERENCES tenants(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  cached_size_bytes BIGINT NOT NULL DEFAULT 0 CHECK (cached_size_bytes >= 0),
  cached_size_updated_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX unique_registry_name ON registries (name);
CREATE INDEX idx_registry_tenant_id ON registries (tenant_id);

-- repositories
CREATE TABLE repositories (
  id UUID PRIMARY KEY,
  registry_id UUID NOT NULL
    REFERENCES registries(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_pushed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (name <> ''),
  UNIQUE (registry_id, name)
);
CREATE INDEX idx_repositories_registry_id ON repositories (registry_id, id);

-- api keys
CREATE TYPE api_key_permission AS ENUM ('read', 'write', 'admin');

CREATE TABLE api_keys (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL
    REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  secret_encrypted TEXT NOT NULL,
  prefix TEXT NOT NULL,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  last_used_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX unique_api_key_user_id_name ON api_keys (user_id, name);
CREATE UNIQUE INDEX unique_api_key_prefix ON api_keys (prefix);
CREATE INDEX idx_api_key_user ON api_keys (user_id);

CREATE TABLE api_key_scopes (
  id UUID PRIMARY KEY,
  api_key_id UUID NOT NULL
    REFERENCES api_keys(id) ON DELETE CASCADE,
  registry_id UUID NOT NULL
    REFERENCES registries(id) ON DELETE CASCADE,
  repository_id UUID
    REFERENCES repositories(id) ON DELETE CASCADE,
  permission api_key_permission NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_api_key_scopes_api_key_id ON api_key_scopes (api_key_id);
CREATE INDEX idx_api_key_scopes_registry_repo_id ON api_key_scopes (registry_id, repository_id);
CREATE UNIQUE INDEX unique_api_key_registry_scope
  ON api_key_scopes (api_key_id, registry_id)
  WHERE repository_id IS NULL;
CREATE UNIQUE INDEX unique_api_key_repository_scope
  ON api_key_scopes (api_key_id, registry_id, repository_id)
  WHERE repository_id IS NOT NULL;

-- objects: unified store for blobs, manifests, and manifest indexes
CREATE TABLE objects (
  digest TEXT PRIMARY KEY,
  size_bytes BIGINT NOT NULL CHECK (size_bytes >= 0),
  type TEXT NOT NULL,
  content_type TEXT NOT NULL DEFAULT '',
  storage TEXT NOT NULL DEFAULT 'r2',
  body BYTEA,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  existence_checked_at TIMESTAMPTZ,
  CHECK (digest ~ '^sha256:[a-f0-9]{64}$'),
  CHECK (storage IN ('db', 'r2')),
  CHECK (type IN ('blob', 'manifest', 'manifest_index'))
);

-- graph: directed relationships between objects
CREATE TABLE graph (
  parent_digest TEXT NOT NULL REFERENCES objects(digest) ON DELETE CASCADE,
  child_digest TEXT NOT NULL REFERENCES objects(digest) ON DELETE CASCADE,
  position INT NOT NULL DEFAULT 0,
  is_subject BOOLEAN NOT NULL DEFAULT FALSE,
  PRIMARY KEY (parent_digest, child_digest)
);
CREATE INDEX idx_graph_child ON graph (child_digest);

-- tags: named references within a repository
CREATE TABLE tags (
  repository_id UUID NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  digest TEXT NOT NULL REFERENCES objects(digest) ON DELETE CASCADE,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (repository_id, name),
  CHECK (name <> ''),
  CHECK (digest ~ '^sha256:[a-f0-9]{64}$')
);
CREATE INDEX idx_tags_digest ON tags (digest);

-- repository_objects: objects directly associated with a repository (for storage accounting)
CREATE TABLE repository_objects (
  repository_id UUID NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
  digest TEXT NOT NULL REFERENCES objects(digest) ON DELETE CASCADE,
  PRIMARY KEY (repository_id, digest)
);
CREATE INDEX IF NOT EXISTS idx_repository_objects_digest ON repository_objects (digest);

CREATE TABLE usage_events (
  id UUID PRIMARY KEY,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  registry_id UUID REFERENCES registries(id),
  repo_id UUID REFERENCES repositories(id),
  digest TEXT CHECK (digest ~ '^sha256:[a-f0-9]{64}$'),
  metric TEXT NOT NULL CHECK (metric IN ('storage-bytes', 'push-op-count', 'pull-op-count')),
  value BIGINT NOT NULL
);
CREATE INDEX idx_usage_events_tenant_created_at ON usage_events (tenant_id, created_at);
CREATE INDEX idx_usage_events_tenant_metric_created_at
  ON usage_events (tenant_id, metric, created_at)
  INCLUDE (value);

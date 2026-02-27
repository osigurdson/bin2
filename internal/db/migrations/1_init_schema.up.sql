-- extensions
CREATE EXTENSION citext;

-- users
CREATE TABLE users (
  id UUID PRIMARY KEY,
  sub TEXT NOT NULL,
  email citext NOT NULL
);
CREATE UNIQUE INDEX unique_users_sub ON users(sub);
CREATE UNIQUE INDEX unique_users_email ON users(email);

-- organizations
CREATE TABLE organizations (
  id UUID PRIMARY KEY,
  workos_org_id TEXT UNIQUE,        -- NULL for personal orgs
  slug TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE org_members (
  org_id  UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role    TEXT NOT NULL DEFAULT 'member',   -- 'owner' | 'member'
  created_at TIMESTAMPTZ DEFAULT NOW(),
  PRIMARY KEY (org_id, user_id)
);
CREATE INDEX idx_org_members_user_id ON org_members (user_id);


-- registries
CREATE TABLE registries (
  id UUID PRIMARY KEY,
  org_id UUID NOT NULL
    REFERENCES organizations(id) ON DELETE CASCADE,
  name TEXT NOT NULL
);
CREATE UNIQUE INDEX unique_registry_nam ON registries (name);
CREATE INDEX idx_registry_org_id ON registries (org_id);

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

CREATE INDEX idx_repositories_registry_id
  ON repositories (registry_id, name);

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

-- blob GC index
CREATE TABLE blobs (
  digest TEXT PRIMARY KEY,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (digest ~ '^sha256:[a-f0-9]{64}$')
);

CREATE TABLE manifest_refs (
  repository_id UUID NOT NULL
    REFERENCES repositories(id) ON DELETE CASCADE,
  reference TEXT NOT NULL,
  manifest_digest TEXT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (repository_id, reference),
  CHECK (reference <> ''),
  CHECK (manifest_digest ~ '^sha256:[a-f0-9]{64}$')
);

CREATE INDEX idx_manifest_refs_manifest
  ON manifest_refs (repository_id, manifest_digest);

CREATE TABLE manifest_blob_refs (
  repository_id UUID NOT NULL
    REFERENCES repositories(id) ON DELETE CASCADE,
  manifest_digest TEXT NOT NULL,
  blob_digest TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (repository_id, manifest_digest, blob_digest),
  CHECK (manifest_digest ~ '^sha256:[a-f0-9]{64}$'),
  CHECK (blob_digest ~ '^sha256:[a-f0-9]{64}$')
);

CREATE INDEX idx_manifest_blob_refs_blob
  ON manifest_blob_refs (blob_digest);

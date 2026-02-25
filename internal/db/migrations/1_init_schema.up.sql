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

-- api keys
CREATE TABLE api_keys (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL
    REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  key_hash TEXT NOT NULL,
  prefix TEXT NOT NULL,
  created_at TIMESTAMPTZ DEFAULT NOW(),
 last_used_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX unique_api_key_key_hash ON api_keys (key_hash);
CREATE UNIQUE INDEX unique_api_key_user_id_name ON api_keys (user_id, name);
CREATE INDEX idx_api_key_user ON api_keys (user_id);

-- registries
CREATE TABLE registries (
  id UUID PRIMARY KEY,
  org_id UUID NOT NULL
    REFERENCES organizations(id) ON DELETE CASCADE,
  name TEXT NOT NULL
);
CREATE UNIQUE INDEX unique_registry_nam ON registries (name);
CREATE INDEX idx_registry_org_id ON registries (org_id);

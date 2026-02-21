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

-- stores
CREATE TABLE registries (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL
    REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL
);
CREATE UNIQUE INDEX unique_registry_nam ON registries (name);
CREATE INDEX idx_registry_user_id ON registries (user_id);

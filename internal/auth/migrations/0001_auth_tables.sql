-- +goose Up
CREATE TABLE auth_users (
  slug             TEXT PRIMARY KEY,
  display_name     TEXT NOT NULL,
  active           INTEGER NOT NULL,
  password_allowed INTEGER NOT NULL,
  passkey_allowed  INTEGER NOT NULL
);

CREATE TABLE auth_user_roles (
  user_slug TEXT NOT NULL,
  role_slug TEXT NOT NULL,
  PRIMARY KEY (user_slug, role_slug)
);

CREATE TABLE auth_passwords (
  user_slug      TEXT PRIMARY KEY,
  argon2id_hash  TEXT NOT NULL,
  set_at         INTEGER NOT NULL,
  set_by         TEXT NOT NULL
);

CREATE TABLE auth_passkeys (
  credential_id BLOB    PRIMARY KEY,
  user_slug     TEXT    NOT NULL,
  public_key    BLOB    NOT NULL,
  sign_count    INTEGER NOT NULL,
  attestation   BLOB,
  label         TEXT,
  registered_at INTEGER NOT NULL,
  last_used_at  INTEGER
);
CREATE INDEX auth_passkeys_user ON auth_passkeys(user_slug);

CREATE TABLE auth_tokens (
  token_id      TEXT PRIMARY KEY,
  user_slug     TEXT NOT NULL,
  hash_b64      TEXT NOT NULL,
  scope_blob    BLOB NOT NULL,
  label         TEXT NOT NULL,
  issued_at     INTEGER NOT NULL,
  issued_by     TEXT NOT NULL,
  expires_at    INTEGER,
  revoked_at    INTEGER,
  last_used_at  INTEGER
);
CREATE INDEX auth_tokens_user ON auth_tokens(user_slug);

CREATE TABLE auth_sessions (
  session_id      TEXT PRIMARY KEY,
  user_slug       TEXT NOT NULL,
  auth_method     TEXT NOT NULL DEFAULT '',
  refresh_hash    TEXT NOT NULL,
  issued_at       INTEGER NOT NULL,
  refresh_ttl_at  INTEGER NOT NULL,
  refresh_idle_at INTEGER NOT NULL,
  user_agent      TEXT,
  remote_ip       TEXT
);
CREATE INDEX auth_sessions_user ON auth_sessions(user_slug);

CREATE TABLE auth_enrollment_tokens (
  token_hash    TEXT PRIMARY KEY,
  user_slug     TEXT NOT NULL,
  intent        TEXT NOT NULL,
  expires_at    INTEGER NOT NULL,
  consumed_at   INTEGER
);

CREATE TABLE auth_attempts (
  bucket       TEXT NOT NULL,
  attempted_at INTEGER NOT NULL,
  succeeded    INTEGER NOT NULL
);
CREATE INDEX auth_attempts_bucket ON auth_attempts(bucket, attempted_at);

-- +goose Down
DROP TABLE IF EXISTS auth_attempts;
DROP TABLE IF EXISTS auth_enrollment_tokens;
DROP TABLE IF EXISTS auth_sessions;
DROP TABLE IF EXISTS auth_tokens;
DROP TABLE IF EXISTS auth_passkeys;
DROP TABLE IF EXISTS auth_passwords;
DROP TABLE IF EXISTS auth_user_roles;
DROP TABLE IF EXISTS auth_users;

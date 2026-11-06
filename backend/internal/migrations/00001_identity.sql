-- +goose Up
CREATE TABLE owners (
    id uuid PRIMARY KEY,
    singleton boolean NOT NULL DEFAULT true UNIQUE CHECK (singleton),
    username text NOT NULL CHECK (char_length(username) BETWEEN 1 AND 120 AND username ~ '[^[:space:]]'),
    password_hash text NOT NULL CHECK (char_length(password_hash) > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    credentials_changed_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX owners_username_unique ON owners (lower(username));

CREATE TABLE sessions (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL REFERENCES owners(id),
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    csrf_token_hash bytea NOT NULL CHECK (octet_length(csrf_token_hash) = 32),
    created_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    CHECK (expires_at > created_at)
);
CREATE INDEX sessions_owner_id_idx ON sessions(owner_id);

-- Forward-only: deliberately no Down section. Restore/rollback requires its own plan.

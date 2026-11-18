-- +goose Up
CREATE TABLE friends (
    id uuid PRIMARY KEY,
    site_name text NOT NULL CHECK (char_length(btrim(site_name)) BETWEEN 1 AND 80),
    site_url text NOT NULL CHECK (char_length(site_url) BETWEEN 9 AND 2048 AND site_url LIKE 'https://%'),
    introduction text NOT NULL CHECK (char_length(btrim(introduction)) BETWEEN 1 AND 300),
    icon_url text NOT NULL DEFAULT '' CHECK (char_length(icon_url)<=2048 AND (icon_url='' OR icon_url LIKE 'https://%')),
    published boolean NOT NULL DEFAULT false,
    position integer NOT NULL CHECK (position>=0),
    version bigint NOT NULL DEFAULT 1 CHECK (version>0),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX friends_directory_idx ON friends(position,id) WHERE published;
CREATE TABLE friend_requests (
    id uuid PRIMARY KEY,
    site_name text NOT NULL CHECK (char_length(btrim(site_name)) BETWEEN 1 AND 80),
    site_url text NOT NULL CHECK (char_length(site_url) BETWEEN 9 AND 2048 AND site_url LIKE 'https://%'),
    introduction text NOT NULL CHECK (char_length(btrim(introduction)) BETWEEN 1 AND 300),
    icon_url text NOT NULL DEFAULT '' CHECK (char_length(icon_url)<=2048 AND (icon_url='' OR icon_url LIKE 'https://%')),
    email text NOT NULL DEFAULT '' CHECK (char_length(email)<=254),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected')),
    friend_id uuid REFERENCES friends(id),
    version bigint NOT NULL DEFAULT 1 CHECK (version>0),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    moderated_at timestamptz,
    CHECK ((status='approved')=(friend_id IS NOT NULL))
);
CREATE INDEX friend_requests_queue_idx ON friend_requests(status,created_at DESC,id DESC);
CREATE INDEX friend_requests_rate_idx ON friend_requests(created_at);

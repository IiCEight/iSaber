-- +goose Up
CREATE TABLE import_jobs (
    id uuid PRIMARY KEY,
    article_id uuid REFERENCES articles(id),
    expected_version bigint CHECK (expected_version > 0),
    upload_storage_key text NOT NULL CHECK (char_length(upload_storage_key) > 0),
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'processing', 'succeeded', 'failed')),
    result_revision_id uuid,
    issues jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(issues) = 'array'),
    created_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,
    FOREIGN KEY (article_id, result_revision_id) REFERENCES revisions(article_id, id),
    CHECK (expected_version IS NULL OR article_id IS NOT NULL),
    CHECK ((status IN ('queued', 'processing') AND finished_at IS NULL) OR
           (status IN ('succeeded', 'failed') AND finished_at IS NOT NULL)),
    CHECK (status <> 'succeeded' OR (article_id IS NOT NULL AND result_revision_id IS NOT NULL)),
    CHECK (result_revision_id IS NULL OR article_id IS NOT NULL)
);
CREATE INDEX import_jobs_article_id_idx ON import_jobs(article_id);
CREATE INDEX import_jobs_pending_idx ON import_jobs(created_at, id) WHERE status IN ('queued', 'processing');

CREATE TABLE comments (
    id uuid PRIMARY KEY,
    article_id uuid NOT NULL REFERENCES articles(id),
    display_name text NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 80 AND display_name ~ '[^[:space:]]'),
    body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 5000 AND body ~ '[^[:space:]]'),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'removed')),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    moderated_at timestamptz,
    CHECK (status = 'pending' OR moderated_at IS NOT NULL)
);
CREATE INDEX comments_article_idx ON comments(article_id, created_at, id);
CREATE INDEX comments_moderation_idx ON comments(status, created_at DESC, id DESC);

CREATE TABLE idempotency_records (
    scope text NOT NULL CHECK (char_length(scope) > 0),
    key uuid NOT NULL,
    request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
    state text NOT NULL CHECK (state IN ('in_progress', 'completed')),
    response_status integer CHECK (response_status BETWEEN 200 AND 599),
    response_body jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (scope, key),
    CHECK (expires_at > created_at),
    CHECK ((state = 'in_progress' AND response_status IS NULL AND response_body IS NULL) OR
           (state = 'completed' AND response_status IS NOT NULL AND response_body IS NOT NULL))
);
CREATE INDEX idempotency_expiry_idx ON idempotency_records(expires_at);

-- Forward-only migration; future changes must preserve rolling-release compatibility.

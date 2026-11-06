-- +goose Up
CREATE TABLE categories (
    id uuid PRIMARY KEY,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120 AND name ~ '[^[:space:]]'),
    slug text NOT NULL UNIQUE CHECK (char_length(slug) <= 160 AND slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0)
);
CREATE TABLE tags (LIKE categories INCLUDING ALL);

CREATE TABLE articles (
    id uuid PRIMARY KEY,
    slug text UNIQUE CHECK (char_length(slug) <= 160 AND slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    visibility text NOT NULL DEFAULT 'private' CHECK (visibility IN ('public', 'private')),
    comments_enabled boolean NOT NULL DEFAULT true,
    draft_revision_id uuid,
    published_revision_id uuid,
    published_at timestamptz,
    updated_at timestamptz,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    modified_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((published_at IS NULL AND updated_at IS NULL) OR (published_at IS NOT NULL AND updated_at IS NOT NULL)),
    CHECK (published_revision_id IS NULL OR (slug IS NOT NULL AND published_at IS NOT NULL))
);

CREATE TABLE revisions (
    id uuid PRIMARY KEY,
    article_id uuid NOT NULL REFERENCES articles(id),
    markdown text NOT NULL CHECK (octet_length(markdown) <= 5242880),
    title text CHECK (char_length(title) <= 300),
    summary text CHECK (char_length(summary) <= 2000),
    language text CHECK (language IN ('en', 'zh')),
    written_at date,
    category_id uuid REFERENCES categories(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    first_published_at timestamptz,
    restored_from_revision_id uuid,
    UNIQUE (article_id, id),
    FOREIGN KEY (article_id, restored_from_revision_id) REFERENCES revisions(article_id, id)
);
ALTER TABLE articles ADD CONSTRAINT article_draft_ownership
    FOREIGN KEY (id, draft_revision_id) REFERENCES revisions(article_id, id);
ALTER TABLE articles ADD CONSTRAINT article_published_ownership
    FOREIGN KEY (id, published_revision_id) REFERENCES revisions(article_id, id);

CREATE TABLE revision_tags (
    revision_id uuid NOT NULL REFERENCES revisions(id),
    tag_id uuid NOT NULL REFERENCES tags(id),
    PRIMARY KEY (revision_id, tag_id)
);
CREATE INDEX revision_tags_tag_id_idx ON revision_tags(tag_id);

CREATE TABLE assets (
    id uuid PRIMARY KEY,
    article_id uuid NOT NULL REFERENCES articles(id),
    storage_key text NOT NULL UNIQUE CHECK (char_length(storage_key) > 0),
    media_type text NOT NULL CHECK (media_type IN ('image/png', 'image/jpeg', 'image/gif', 'image/webp')),
    byte_size bigint NOT NULL CHECK (byte_size > 0 AND byte_size <= 10485760),
    checksum bytea NOT NULL CHECK (octet_length(checksum) = 32),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (article_id, id)
);
CREATE TABLE revision_assets (
    article_id uuid NOT NULL REFERENCES articles(id),
    revision_id uuid NOT NULL,
    asset_id uuid NOT NULL,
    original_path text NOT NULL,
    PRIMARY KEY (revision_id, asset_id),
    FOREIGN KEY (article_id, revision_id) REFERENCES revisions(article_id, id),
    FOREIGN KEY (article_id, asset_id) REFERENCES assets(article_id, id)
);
CREATE INDEX revision_assets_asset_id_idx ON revision_assets(asset_id);
CREATE INDEX articles_modified_idx ON articles(modified_at DESC, id DESC);
CREATE INDEX articles_public_idx ON articles(published_at DESC, id DESC)
    WHERE visibility = 'public' AND published_revision_id IS NOT NULL;
CREATE INDEX revisions_history_idx ON revisions(article_id, first_published_at DESC, id DESC)
    WHERE first_published_at IS NOT NULL;
CREATE INDEX revisions_category_id_idx ON revisions(category_id);

-- Snapshot immutability, locked slugs, and public-date transitions are transactional
-- service rules, not implied by these ownership constraints. No Down migration.

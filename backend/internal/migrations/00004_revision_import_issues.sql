-- +goose Up
-- Keep unresolved imported metadata attached to its immutable snapshot so that
-- later preview/publication cannot accidentally forget an import validation error.
ALTER TABLE revisions ADD COLUMN source_issues jsonb NOT NULL DEFAULT '[]'::jsonb
    CHECK (jsonb_typeof(source_issues) = 'array');
ALTER TABLE import_jobs ADD COLUMN filename text NOT NULL DEFAULT 'article.md';

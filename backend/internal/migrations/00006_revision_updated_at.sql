-- +goose Up
ALTER TABLE revisions ADD COLUMN updated_at timestamptz;

-- +goose Up
-- Global guest-submission limits inspect the recent committed comment window.
CREATE INDEX comments_created_at_idx ON comments(created_at);

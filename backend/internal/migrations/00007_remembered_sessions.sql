-- +goose Up
ALTER TABLE sessions ADD COLUMN remember_me boolean NOT NULL DEFAULT false;

-- Existing sessions keep their original expiry and idle policy.
-- Forward-only: deliberately no Down section.

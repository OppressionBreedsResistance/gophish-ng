-- +goose Up
ALTER TABLE results ADD COLUMN attachment_opened boolean default 0;

-- +goose Down
-- SQLite does not support DROP COLUMN natively

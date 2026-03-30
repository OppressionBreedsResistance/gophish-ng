-- +goose Up
ALTER TABLE campaigns ADD COLUMN host_attachment boolean default 0;

-- +goose Down
-- SQLite does not support dropping columns

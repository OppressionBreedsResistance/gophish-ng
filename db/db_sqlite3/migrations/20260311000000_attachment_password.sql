-- +goose Up
ALTER TABLE attachments ADD COLUMN password varchar(255) DEFAULT '';

-- +goose Down
-- SQLite does not support DROP COLUMN natively

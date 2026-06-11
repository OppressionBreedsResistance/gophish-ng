-- +goose Up
ALTER TABLE targets ADD COLUMN phone varchar(255) DEFAULT '';
ALTER TABLE results ADD COLUMN phone varchar(255) DEFAULT '';
ALTER TABLE campaigns ADD COLUMN campaign_type varchar(255) DEFAULT 'email';

-- +goose Down
-- SQLite does not support dropping columns

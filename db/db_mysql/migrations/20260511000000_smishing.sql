-- +goose Up
ALTER TABLE targets ADD COLUMN phone varchar(255) DEFAULT '';
ALTER TABLE results ADD COLUMN phone varchar(255) DEFAULT '';
ALTER TABLE campaigns ADD COLUMN campaign_type varchar(255) DEFAULT 'email';

-- +goose Down
ALTER TABLE targets DROP COLUMN phone;
ALTER TABLE results DROP COLUMN phone;
ALTER TABLE campaigns DROP COLUMN campaign_type;

-- +goose Up
ALTER TABLE attachments ADD COLUMN password varchar(255) DEFAULT '';

-- +goose Down
ALTER TABLE attachments DROP COLUMN password;

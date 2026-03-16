-- +goose Up
ALTER TABLE `results` ADD COLUMN attachment_opened boolean default 0;

-- +goose Down
ALTER TABLE `results` DROP COLUMN attachment_opened;

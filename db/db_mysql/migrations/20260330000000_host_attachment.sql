-- +goose Up
ALTER TABLE campaigns ADD COLUMN host_attachment boolean default 0;

-- +goose Down
ALTER TABLE campaigns DROP COLUMN host_attachment;

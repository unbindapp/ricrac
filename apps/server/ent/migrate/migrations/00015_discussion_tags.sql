-- +goose Up
-- +goose StatementBegin
ALTER TABLE discussions
    ADD COLUMN thumbnail_url TEXT,
    ADD COLUMN tags          TEXT[] NOT NULL DEFAULT '{}';
CREATE INDEX discussions_tags ON discussions USING GIN (tags);
-- +goose StatementEnd

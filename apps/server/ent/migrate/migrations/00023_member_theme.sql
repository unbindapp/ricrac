-- +goose Up
-- +goose StatementBegin
ALTER TABLE workspace_members ADD COLUMN theme TEXT;
-- +goose StatementEnd

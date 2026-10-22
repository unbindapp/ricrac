-- +goose Up
-- +goose StatementBegin
ALTER TABLE workspaces ADD COLUMN theme TEXT NOT NULL DEFAULT 'default';
-- +goose StatementEnd

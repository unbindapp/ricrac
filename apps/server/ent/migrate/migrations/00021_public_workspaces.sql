-- +goose Up
-- +goose StatementBegin
ALTER TABLE workspaces ADD COLUMN public BOOLEAN NOT NULL DEFAULT false;
-- +goose StatementEnd

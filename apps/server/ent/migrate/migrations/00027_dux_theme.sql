-- +goose Up
-- +goose StatementBegin
UPDATE workspaces SET theme = 'dux' WHERE theme = 'dank';
UPDATE workspace_members SET theme = 'dux' WHERE theme = 'dank';
-- +goose StatementEnd

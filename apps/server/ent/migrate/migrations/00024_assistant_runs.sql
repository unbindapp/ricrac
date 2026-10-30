-- +goose Up
-- +goose StatementBegin
CREATE TABLE assistant_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_id UUID NOT NULL,
    cost_usd DOUBLE PRECISION NOT NULL,
    turns INTEGER NOT NULL,
    outcome TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX assistant_runs_created_at ON assistant_runs (created_at);
CREATE INDEX assistant_runs_session_id ON assistant_runs (session_id);
ALTER TABLE workspaces ADD COLUMN assistant_enabled BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE workspace_members ADD COLUMN assistant BOOLEAN NOT NULL DEFAULT false;
-- +goose StatementEnd

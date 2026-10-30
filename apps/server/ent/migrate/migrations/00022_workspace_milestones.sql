-- +goose Up
-- +goose StatementBegin
ALTER TABLE project_milestones ADD COLUMN workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE;
UPDATE project_milestones m SET workspace_id = p.workspace_id FROM projects p WHERE p.id = m.project_id;
ALTER TABLE project_milestones ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE project_milestones ALTER COLUMN project_id DROP NOT NULL;
ALTER TABLE project_milestones ADD COLUMN closed_at TIMESTAMPTZ;
CREATE INDEX project_milestones_workspace ON project_milestones(workspace_id) WHERE project_id IS NULL;

CREATE TABLE github_milestone_links (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    milestone_id      UUID NOT NULL REFERENCES project_milestones(id) ON DELETE CASCADE,
    repo_sync_id      UUID NOT NULL REFERENCES github_repo_syncs(id) ON DELETE CASCADE,
    github_id         BIGINT NOT NULL,
    github_number     INT NOT NULL,
    github_url        TEXT NOT NULL,
    github_updated_at TIMESTAMPTZ NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (repo_sync_id, github_id),
    UNIQUE (milestone_id, repo_sync_id)
);

ALTER TABLE github_repo_syncs ADD COLUMN milestones_synced_at TIMESTAMPTZ;
ALTER TABLE teams ADD COLUMN github_open_issues BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE teams ADD COLUMN github_sync_fields BOOLEAN NOT NULL DEFAULT false;
UPDATE teams t SET github_open_issues = true, github_sync_fields = true
WHERE EXISTS (SELECT 1 FROM github_repo_syncs s WHERE s.team_id = t.id AND s.link_back);
ALTER TABLE github_outbox ADD COLUMN milestone_id UUID REFERENCES project_milestones(id) ON DELETE SET NULL;
-- +goose StatementEnd

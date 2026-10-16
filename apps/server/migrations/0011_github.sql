-- GitHub: the app's installations in workspaces, repositories whose issues sync with a team,
-- the links between Ricrac's issues and comments and GitHub's, and the changes waiting to be sent
-- to GitHub.

-- Lowercase. Maps GitHub authors and assignees to people.
ALTER TABLE users ADD COLUMN github_login TEXT UNIQUE;

-- Where pull requests move issues; null means the team's first started / completed status.
ALTER TABLE teams
    ADD COLUMN github_pr_opened_state_id UUID REFERENCES workflow_states(id) ON DELETE SET NULL,
    ADD COLUMN github_pr_merged_state_id UUID REFERENCES workflow_states(id) ON DELETE SET NULL;

CREATE TABLE github_installations (
    -- GitHub's installation id.
    id            BIGINT PRIMARY KEY,
    workspace_id  UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    account_login TEXT NOT NULL,
    -- User or Organization
    account_type  TEXT NOT NULL,
    installer_id  UUID REFERENCES users(id) ON DELETE SET NULL,
    suspended_at  TIMESTAMPTZ,
    -- The last webhook event, like pull_request.opened.
    last_event    TEXT,
    last_event_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX github_installations_workspace ON github_installations(workspace_id);

CREATE TABLE github_repo_syncs (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id     UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    installation_id  BIGINT NOT NULL REFERENCES github_installations(id) ON DELETE CASCADE,
    -- GitHub's repository id; the name follows renames.
    repo_id          BIGINT NOT NULL UNIQUE,
    full_name        TEXT NOT NULL,
    team_id          UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    project_id       UUID REFERENCES projects(id) ON DELETE SET NULL,
    -- Importing the repository's existing issues: pending, running, completed or failed; null
    -- when none was asked for.
    backfill_status  TEXT,
    backfill_closed  BOOLEAN NOT NULL DEFAULT false,
    backfill_page    INT NOT NULL DEFAULT 1,
    issues_created   INT NOT NULL DEFAULT 0,
    issues_updated   INT NOT NULL DEFAULT 0,
    comments_created INT NOT NULL DEFAULT 0,
    skipped          INT NOT NULL DEFAULT 0,
    last_error       TEXT,
    last_synced_at   TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX github_repo_syncs_workspace ON github_repo_syncs(workspace_id);

CREATE TABLE github_issue_links (
    issue_id          UUID PRIMARY KEY REFERENCES issues(id) ON DELETE CASCADE,
    repo_sync_id      UUID NOT NULL REFERENCES github_repo_syncs(id) ON DELETE CASCADE,
    github_id         BIGINT NOT NULL UNIQUE,
    github_number     INT NOT NULL,
    github_node_id    TEXT NOT NULL,
    github_url        TEXT NOT NULL,
    -- When GitHub last changed the issue, to ignore events older than what Ricrac has.
    github_updated_at TIMESTAMPTZ NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX github_issue_links_repo_sync ON github_issue_links(repo_sync_id);

CREATE TABLE github_comment_links (
    comment_id UUID PRIMARY KEY REFERENCES comments(id) ON DELETE CASCADE,
    issue_id   UUID NOT NULL REFERENCES github_issue_links(issue_id) ON DELETE CASCADE,
    github_id  BIGINT NOT NULL UNIQUE,
    -- The app posted it for Ricrac, so edits and deletions in Ricrac go to GitHub.
    posted     BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Changes made in Ricrac to linked issues and their comments, queued in the change's transaction
-- and sent by a job.
CREATE TABLE github_outbox (
    id                BIGSERIAL PRIMARY KEY,
    repo_sync_id      UUID NOT NULL REFERENCES github_repo_syncs(id) ON DELETE CASCADE,
    github_number     INT NOT NULL,
    issue_id          UUID REFERENCES issues(id) ON DELETE CASCADE,
    comment_id        UUID REFERENCES comments(id) ON DELETE SET NULL,
    github_comment_id BIGINT,
    -- issue, comment_created, comment_edited or comment_deleted
    kind              TEXT NOT NULL,
    -- For issue: the history entry's changes.
    changes           JSONB NOT NULL DEFAULT '{}',
    attempts          INT NOT NULL DEFAULT 0,
    next_attempt_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX github_outbox_due ON github_outbox(next_attempt_at);

-- People, workspaces and access ------------------------------------------------------------

CREATE TABLE users (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    google_subject TEXT UNIQUE,
    email          TEXT NOT NULL UNIQUE,
    name           TEXT NOT NULL,
    display_name   TEXT NOT NULL,
    avatar_url     TEXT,
    timezone       TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE workspaces (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                   TEXT NOT NULL,
    url_key                TEXT NOT NULL UNIQUE,
    logo_url               TEXT,
    -- Teams' estimate, cycle and automation settings start from these.
    default_team_settings  JSONB NOT NULL DEFAULT '{}',
    project_update_reminder_day TEXT,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE workspace_members (
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- owner, admin, member or guest
    role         TEXT NOT NULL,
    active       BOOLEAN NOT NULL DEFAULT true,
    joined_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, user_id)
);
CREATE INDEX workspace_members_user ON workspace_members(user_id);

CREATE TABLE invitations (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    email        TEXT NOT NULL,
    role         TEXT NOT NULL,
    team_ids     UUID[] NOT NULL DEFAULT '{}',
    inviter_id   UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at   TIMESTAMPTZ NOT NULL,
    UNIQUE (workspace_id, email)
);
CREATE INDEX invitations_email ON invitations(email);

CREATE TABLE api_keys (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label        TEXT NOT NULL,
    key_hash     TEXT NOT NULL UNIQUE,
    key_hint     TEXT NOT NULL,
    scopes       TEXT[] NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ
);

-- OAuth 2.1. Clients come from dynamic registration, from client ID metadata documents (id is
-- the document's URL) or are created by a user in the dashboard (owner_id set). Secrets, codes
-- and tokens are stored as SHA-256 hashes.
CREATE TABLE oauth_clients (
    id             TEXT PRIMARY KEY,
    secret_hash    TEXT,
    name           TEXT NOT NULL,
    description    TEXT,
    homepage_url   TEXT,
    redirect_uris  TEXT[] NOT NULL,
    owner_id       UUID REFERENCES users(id) ON DELETE CASCADE,
    workspace_id   UUID REFERENCES workspaces(id) ON DELETE CASCADE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    fetched_at     TIMESTAMPTZ
);

CREATE TABLE oauth_grants (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id    TEXT NOT NULL REFERENCES oauth_clients(id) ON DELETE CASCADE,
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    scopes       TEXT[] NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX oauth_grants_user ON oauth_grants(user_id);

CREATE TABLE oauth_codes (
    code_hash      TEXT PRIMARY KEY,
    client_id      TEXT NOT NULL REFERENCES oauth_clients(id) ON DELETE CASCADE,
    user_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id   UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    scopes         TEXT[] NOT NULL,
    redirect_uri   TEXT NOT NULL,
    code_challenge TEXT,
    resource       TEXT,
    expires_at     TIMESTAMPTZ NOT NULL
);

CREATE TABLE oauth_tokens (
    token_hash TEXT PRIMARY KEY,
    kind       TEXT NOT NULL,
    grant_id   UUID NOT NULL REFERENCES oauth_grants(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX oauth_tokens_grant ON oauth_tokens(grant_id);

-- Teams and workflow ------------------------------------------------------------------------

CREATE TABLE teams (
    id                        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id              UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    parent_id                 UUID REFERENCES teams(id) ON DELETE SET NULL,
    key                       TEXT NOT NULL,
    name                      TEXT NOT NULL,
    description               TEXT,
    icon                      TEXT,
    color                     TEXT,
    private                   BOOLEAN NOT NULL DEFAULT false,
    timezone                  TEXT NOT NULL DEFAULT 'UTC',
    issue_count               INTEGER NOT NULL DEFAULT 0,
    -- Estimates: notUsed, exponential, fibonacci, linear or tShirt.
    estimate_type             TEXT NOT NULL DEFAULT 'notUsed',
    estimate_allow_zero       BOOLEAN NOT NULL DEFAULT false,
    estimate_extended         BOOLEAN NOT NULL DEFAULT false,
    default_estimate          DOUBLE PRECISION,
    -- Cycles.
    cycles_enabled            BOOLEAN NOT NULL DEFAULT false,
    cycle_duration_weeks      INTEGER NOT NULL DEFAULT 2,
    cycle_cooldown_weeks      INTEGER NOT NULL DEFAULT 0,
    cycle_start_day           INTEGER NOT NULL DEFAULT 1,
    upcoming_cycle_count      INTEGER NOT NULL DEFAULT 2,
    cycle_auto_add_started    BOOLEAN NOT NULL DEFAULT true,
    cycle_auto_add_completed  BOOLEAN NOT NULL DEFAULT true,
    cycle_rollover            BOOLEAN NOT NULL DEFAULT true,
    -- Triage.
    triage_enabled            BOOLEAN NOT NULL DEFAULT false,
    triage_action             TEXT NOT NULL DEFAULT 'none',
    triage_responsible_ids    UUID[] NOT NULL DEFAULT '{}',
    triage_rotation_weeks     INTEGER,
    triage_rotation_start     DATE,
    require_priority_to_leave_triage BOOLEAN NOT NULL DEFAULT false,
    -- Automations: months, null when off.
    auto_archive_months       INTEGER DEFAULT 6,
    auto_close_months         INTEGER,
    auto_close_state_id       UUID,
    auto_close_parent_issues  BOOLEAN NOT NULL DEFAULT false,
    auto_close_child_issues   BOOLEAN NOT NULL DEFAULT false,
    default_issue_state_id    UUID,
    default_template_member_id UUID,
    default_template_non_member_id UUID,
    default_project_template_id UUID,
    archived_at               TIMESTAMPTZ,
    created_at                TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX teams_key ON teams(workspace_id, upper(key)) WHERE archived_at IS NULL;

CREATE TABLE team_members (
    team_id   UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    owner     BOOLEAN NOT NULL DEFAULT false,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (team_id, user_id)
);
CREATE INDEX team_members_user ON team_members(user_id);

CREATE TABLE workflow_states (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id     UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    -- triage, backlog, unstarted, started, completed or canceled
    type        TEXT NOT NULL,
    color       TEXT NOT NULL,
    description TEXT,
    position    DOUBLE PRECISION NOT NULL,
    archived_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX workflow_states_team ON workflow_states(team_id);

ALTER TABLE teams ADD FOREIGN KEY (auto_close_state_id) REFERENCES workflow_states(id) ON DELETE SET NULL;
ALTER TABLE teams ADD FOREIGN KEY (default_issue_state_id) REFERENCES workflow_states(id) ON DELETE SET NULL;

-- Labels: workspace labels have no team. Groups hold child labels, of which an issue gets one
-- (single select) or any number (multi select).
CREATE TABLE issue_labels (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    team_id      UUID REFERENCES teams(id) ON DELETE CASCADE,
    parent_id    UUID REFERENCES issue_labels(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    description  TEXT,
    color        TEXT NOT NULL,
    is_group     BOOLEAN NOT NULL DEFAULT false,
    group_type   TEXT NOT NULL DEFAULT 'singleSelect',
    creator_id   UUID REFERENCES users(id) ON DELETE SET NULL,
    retired_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX issue_labels_workspace ON issue_labels(workspace_id);

-- Projects and initiatives ------------------------------------------------------------------

CREATE TABLE project_statuses (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    -- backlog, planned, started, paused, completed or canceled
    type         TEXT NOT NULL,
    color        TEXT NOT NULL,
    description  TEXT,
    position     DOUBLE PRECISION NOT NULL,
    archived_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE project_labels (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    parent_id    UUID REFERENCES project_labels(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    description  TEXT,
    color        TEXT NOT NULL,
    is_group     BOOLEAN NOT NULL DEFAULT false,
    creator_id   UUID REFERENCES users(id) ON DELETE SET NULL,
    retired_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE projects (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id          UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    slug_id               TEXT NOT NULL UNIQUE,
    name                  TEXT NOT NULL,
    summary               TEXT,
    content               TEXT NOT NULL DEFAULT '',
    icon                  TEXT,
    color                 TEXT,
    status_id             UUID NOT NULL REFERENCES project_statuses(id),
    priority              INTEGER NOT NULL DEFAULT 0,
    lead_id               UUID REFERENCES users(id) ON DELETE SET NULL,
    lead_team_id          UUID REFERENCES teams(id) ON DELETE SET NULL,
    creator_id            UUID REFERENCES users(id) ON DELETE SET NULL,
    start_date            DATE,
    start_date_resolution TEXT,
    target_date           DATE,
    target_date_resolution TEXT,
    sort_order            DOUBLE PRECISION NOT NULL DEFAULT 0,
    health                TEXT,
    started_at            TIMESTAMPTZ,
    completed_at          TIMESTAMPTZ,
    canceled_at           TIMESTAMPTZ,
    archived_at           TIMESTAMPTZ,
    trashed_at            TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    search                TSVECTOR GENERATED ALWAYS AS (
        setweight(to_tsvector('simple', coalesce(name, '')), 'A') ||
        setweight(to_tsvector('english', coalesce(summary, '') || ' ' || content), 'B')
    ) STORED
);
CREATE INDEX projects_workspace ON projects(workspace_id);
CREATE INDEX projects_search ON projects USING GIN (search);

CREATE TABLE project_teams (
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    team_id    UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    PRIMARY KEY (project_id, team_id)
);
CREATE INDEX project_teams_team ON project_teams(team_id);

CREATE TABLE project_members (
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (project_id, user_id)
);

CREATE TABLE project_label_links (
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    label_id   UUID NOT NULL REFERENCES project_labels(id) ON DELETE CASCADE,
    PRIMARY KEY (project_id, label_id)
);

CREATE TABLE project_subscribers (
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (project_id, user_id)
);

-- Dependencies between projects: `project_id` blocks `related_project_id`, or they're related.
CREATE TABLE project_relations (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id         UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    related_project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    type               TEXT NOT NULL,
    creator_id         UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, related_project_id, type)
);

CREATE TABLE project_milestones (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT,
    target_date DATE,
    sort_order  DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX project_milestones_project ON project_milestones(project_id);

CREATE TABLE initiatives (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id           UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    slug_id                TEXT NOT NULL UNIQUE,
    parent_id              UUID REFERENCES initiatives(id) ON DELETE SET NULL,
    name                   TEXT NOT NULL,
    summary                TEXT,
    content                TEXT NOT NULL DEFAULT '',
    icon                   TEXT,
    color                  TEXT,
    -- planned, active or completed
    status                 TEXT NOT NULL DEFAULT 'planned',
    owner_id               UUID REFERENCES users(id) ON DELETE SET NULL,
    creator_id             UUID REFERENCES users(id) ON DELETE SET NULL,
    target_date            DATE,
    target_date_resolution TEXT,
    health                 TEXT,
    sort_order             DOUBLE PRECISION NOT NULL DEFAULT 0,
    started_at             TIMESTAMPTZ,
    completed_at           TIMESTAMPTZ,
    archived_at            TIMESTAMPTZ,
    trashed_at             TIMESTAMPTZ,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE initiative_projects (
    initiative_id UUID NOT NULL REFERENCES initiatives(id) ON DELETE CASCADE,
    project_id    UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    sort_order    DOUBLE PRECISION NOT NULL DEFAULT 0,
    PRIMARY KEY (initiative_id, project_id)
);

-- Status updates on projects and initiatives, with their health.
CREATE TABLE status_updates (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id  UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    project_id    UUID REFERENCES projects(id) ON DELETE CASCADE,
    initiative_id UUID REFERENCES initiatives(id) ON DELETE CASCADE,
    user_id       UUID REFERENCES users(id) ON DELETE SET NULL,
    body          TEXT NOT NULL,
    health        TEXT NOT NULL,
    edited_at     TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((project_id IS NULL) <> (initiative_id IS NULL))
);
CREATE INDEX status_updates_project ON status_updates(project_id);
CREATE INDEX status_updates_initiative ON status_updates(initiative_id);

-- External links (resources) on projects and initiatives.
CREATE TABLE entity_links (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id  UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    project_id    UUID REFERENCES projects(id) ON DELETE CASCADE,
    initiative_id UUID REFERENCES initiatives(id) ON DELETE CASCADE,
    url           TEXT NOT NULL,
    label         TEXT NOT NULL,
    sort_order    DOUBLE PRECISION NOT NULL DEFAULT 0,
    creator_id    UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Cycles ------------------------------------------------------------------------------------

CREATE TABLE cycles (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id      UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    number       INTEGER NOT NULL,
    name         TEXT,
    description  TEXT,
    starts_at    TIMESTAMPTZ NOT NULL,
    ends_at      TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    -- Issue counts per day while the cycle runs: [{date, scope, completed, started}].
    history      JSONB NOT NULL DEFAULT '[]',
    archived_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (team_id, number)
);

-- Releases ----------------------------------------------------------------------------------

CREATE TABLE release_pipelines (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id  UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    slug_id       TEXT NOT NULL UNIQUE,
    name          TEXT NOT NULL,
    -- continuous or scheduled
    type          TEXT NOT NULL DEFAULT 'continuous',
    is_production BOOLEAN NOT NULL DEFAULT false,
    archived_at   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE release_pipeline_teams (
    pipeline_id UUID NOT NULL REFERENCES release_pipelines(id) ON DELETE CASCADE,
    team_id     UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    PRIMARY KEY (pipeline_id, team_id)
);

CREATE TABLE release_stages (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_id UUID NOT NULL REFERENCES release_pipelines(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    -- planned, started, completed or canceled
    type        TEXT NOT NULL,
    color       TEXT NOT NULL,
    position    DOUBLE PRECISION NOT NULL,
    archived_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE releases (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    pipeline_id  UUID NOT NULL REFERENCES release_pipelines(id) ON DELETE CASCADE,
    stage_id     UUID REFERENCES release_stages(id) ON DELETE SET NULL,
    slug_id      TEXT NOT NULL UNIQUE,
    name         TEXT NOT NULL,
    version      TEXT,
    description  TEXT,
    commit_sha   TEXT,
    start_date   DATE,
    target_date  DATE,
    started_at   TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    canceled_at  TIMESTAMPTZ,
    creator_id   UUID REFERENCES users(id) ON DELETE SET NULL,
    archived_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX releases_pipeline ON releases(pipeline_id);

CREATE TABLE release_notes (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    pipeline_id  UUID NOT NULL REFERENCES release_pipelines(id) ON DELETE CASCADE,
    slug_id      TEXT NOT NULL UNIQUE,
    title        TEXT NOT NULL,
    content      TEXT NOT NULL DEFAULT '',
    release_ids  UUID[] NOT NULL DEFAULT '{}',
    creator_id   UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Issues ------------------------------------------------------------------------------------

CREATE TABLE issues (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id         UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    team_id              UUID NOT NULL REFERENCES teams(id),
    number               INTEGER NOT NULL,
    previous_identifiers TEXT[] NOT NULL DEFAULT '{}',
    title                TEXT NOT NULL,
    description          TEXT NOT NULL DEFAULT '',
    state_id             UUID NOT NULL REFERENCES workflow_states(id),
    -- 0 none, 1 urgent, 2 high, 3 medium, 4 low
    priority             INTEGER NOT NULL DEFAULT 0,
    estimate             DOUBLE PRECISION,
    assignee_id          UUID REFERENCES users(id) ON DELETE SET NULL,
    creator_id           UUID REFERENCES users(id) ON DELETE SET NULL,
    project_id           UUID REFERENCES projects(id) ON DELETE SET NULL,
    milestone_id         UUID REFERENCES project_milestones(id) ON DELETE SET NULL,
    cycle_id             UUID REFERENCES cycles(id) ON DELETE SET NULL,
    parent_id            UUID REFERENCES issues(id) ON DELETE SET NULL,
    due_date             DATE,
    sort_order           DOUBLE PRECISION NOT NULL DEFAULT 0,
    sub_issue_sort_order DOUBLE PRECISION,
    sla_type             TEXT,
    sla_started_at       TIMESTAMPTZ,
    sla_breaches_at      TIMESTAMPTZ,
    snoozed_until        TIMESTAMPTZ,
    snoozed_by_id        UUID REFERENCES users(id) ON DELETE SET NULL,
    started_at           TIMESTAMPTZ,
    completed_at         TIMESTAMPTZ,
    canceled_at          TIMESTAMPTZ,
    started_triage_at    TIMESTAMPTZ,
    triaged_at           TIMESTAMPTZ,
    auto_closed_at       TIMESTAMPTZ,
    auto_archived_at     TIMESTAMPTZ,
    archived_at          TIMESTAMPTZ,
    trashed_at           TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    search               TSVECTOR GENERATED ALWAYS AS (
        setweight(to_tsvector('simple', coalesce(title, '')), 'A') ||
        setweight(to_tsvector('english', coalesce(description, '')), 'B')
    ) STORED,
    UNIQUE (team_id, number)
);
CREATE INDEX issues_workspace ON issues(workspace_id, updated_at DESC);
CREATE INDEX issues_team ON issues(team_id);
CREATE INDEX issues_assignee ON issues(assignee_id);
CREATE INDEX issues_project ON issues(project_id);
CREATE INDEX issues_cycle ON issues(cycle_id);
CREATE INDEX issues_parent ON issues(parent_id);
CREATE INDEX issues_state ON issues(state_id);
CREATE INDEX issues_search ON issues USING GIN (search);
CREATE INDEX issues_previous_identifiers ON issues USING GIN (previous_identifiers);

CREATE TABLE issue_label_links (
    issue_id UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    label_id UUID NOT NULL REFERENCES issue_labels(id) ON DELETE CASCADE,
    PRIMARY KEY (issue_id, label_id)
);
CREATE INDEX issue_label_links_label ON issue_label_links(label_id);

CREATE TABLE issue_subscribers (
    issue_id UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    user_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (issue_id, user_id)
);
CREATE INDEX issue_subscribers_user ON issue_subscribers(user_id);

-- Users without access to the issue's team who were given access to the issue itself.
CREATE TABLE issue_shares (
    issue_id   UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    sharer_id  UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (issue_id, user_id)
);

-- `issue_id` blocks `related_issue_id`, duplicates it, or is related to it.
CREATE TABLE issue_relations (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    issue_id         UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    related_issue_id UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    -- blocks, duplicate or related
    type             TEXT NOT NULL,
    creator_id       UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (issue_id, related_issue_id, type),
    CHECK (issue_id <> related_issue_id)
);
CREATE INDEX issue_relations_related ON issue_relations(related_issue_id);

CREATE TABLE issue_releases (
    issue_id   UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    release_id UUID NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
    PRIMARY KEY (issue_id, release_id)
);

-- What changed on an issue, by whom: {"field": {"from": .., "to": ..}, ...}.
CREATE TABLE issue_history (
    id         BIGSERIAL PRIMARY KEY,
    issue_id   UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    actor_id   UUID REFERENCES users(id) ON DELETE SET NULL,
    -- created, updated, archived, unarchived, trashed, restored, auto_closed, auto_archived, ...
    action     TEXT NOT NULL,
    changes    JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX issue_history_issue ON issue_history(issue_id, id);

CREATE TABLE attachments (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    issue_id     UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    url          TEXT NOT NULL,
    title        TEXT NOT NULL,
    subtitle     TEXT,
    icon_url     TEXT,
    metadata     JSONB NOT NULL DEFAULT '{}',
    source_type  TEXT,
    creator_id   UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (issue_id, url)
);

-- Uploaded files, referenced from Markdown or attachments by their URL.
CREATE TABLE files (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    uploader_id  UUID REFERENCES users(id) ON DELETE SET NULL,
    name         TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size         BIGINT NOT NULL DEFAULT 0,
    data         BYTEA,
    -- The upload URL's secret, while the file waits for its bytes.
    upload_hash  TEXT UNIQUE,
    upload_expires_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Documents ---------------------------------------------------------------------------------

CREATE TABLE documents (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id  UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    slug_id       TEXT NOT NULL UNIQUE,
    title         TEXT NOT NULL,
    content       TEXT NOT NULL DEFAULT '',
    icon          TEXT,
    color         TEXT,
    project_id    UUID REFERENCES projects(id) ON DELETE CASCADE,
    initiative_id UUID REFERENCES initiatives(id) ON DELETE CASCADE,
    team_id       UUID REFERENCES teams(id) ON DELETE CASCADE,
    issue_id      UUID REFERENCES issues(id) ON DELETE CASCADE,
    cycle_id      UUID REFERENCES cycles(id) ON DELETE CASCADE,
    release_id    UUID REFERENCES releases(id) ON DELETE CASCADE,
    creator_id    UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_by_id UUID REFERENCES users(id) ON DELETE SET NULL,
    sort_order    DOUBLE PRECISION NOT NULL DEFAULT 0,
    trashed_at    TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    search        TSVECTOR GENERATED ALWAYS AS (
        setweight(to_tsvector('simple', coalesce(title, '')), 'A') ||
        setweight(to_tsvector('english', content), 'B')
    ) STORED
);
CREATE INDEX documents_workspace ON documents(workspace_id);
CREATE INDEX documents_search ON documents USING GIN (search);

-- Earlier contents of a document, kept when it is rewritten.
CREATE TABLE document_versions (
    id          BIGSERIAL PRIMARY KEY,
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    content     TEXT NOT NULL,
    author_id   UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Comments and reactions --------------------------------------------------------------------

CREATE TABLE comments (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id       UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    issue_id           UUID REFERENCES issues(id) ON DELETE CASCADE,
    project_id         UUID REFERENCES projects(id) ON DELETE CASCADE,
    initiative_id      UUID REFERENCES initiatives(id) ON DELETE CASCADE,
    document_id        UUID REFERENCES documents(id) ON DELETE CASCADE,
    milestone_id       UUID REFERENCES project_milestones(id) ON DELETE CASCADE,
    status_update_id   UUID REFERENCES status_updates(id) ON DELETE CASCADE,
    parent_id          UUID REFERENCES comments(id) ON DELETE CASCADE,
    user_id            UUID REFERENCES users(id) ON DELETE SET NULL,
    body               TEXT NOT NULL,
    quoted_text        TEXT,
    edited_at          TIMESTAMPTZ,
    resolved_at        TIMESTAMPTZ,
    resolving_user_id  UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    search             TSVECTOR GENERATED ALWAYS AS (to_tsvector('english', body)) STORED
);
CREATE INDEX comments_issue ON comments(issue_id, created_at);
CREATE INDEX comments_project ON comments(project_id);
CREATE INDEX comments_document ON comments(document_id);
CREATE INDEX comments_parent ON comments(parent_id);
CREATE INDEX comments_search ON comments USING GIN (search);

CREATE TABLE reactions (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id     UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    emoji            TEXT NOT NULL,
    issue_id         UUID REFERENCES issues(id) ON DELETE CASCADE,
    comment_id       UUID REFERENCES comments(id) ON DELETE CASCADE,
    status_update_id UUID REFERENCES status_updates(id) ON DELETE CASCADE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX reactions_unique ON reactions(
    user_id, emoji,
    coalesce(issue_id, '00000000-0000-0000-0000-000000000000'),
    coalesce(comment_id, '00000000-0000-0000-0000-000000000000'),
    coalesce(status_update_id, '00000000-0000-0000-0000-000000000000')
);

-- Customers ---------------------------------------------------------------------------------

CREATE TABLE customer_statuses (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    color        TEXT NOT NULL,
    description  TEXT,
    position     DOUBLE PRECISION NOT NULL
);

CREATE TABLE customer_tiers (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    color        TEXT NOT NULL,
    description  TEXT,
    position     DOUBLE PRECISION NOT NULL
);

CREATE TABLE customers (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    slug_id      TEXT NOT NULL UNIQUE,
    name         TEXT NOT NULL,
    domains      TEXT[] NOT NULL DEFAULT '{}',
    external_ids TEXT[] NOT NULL DEFAULT '{}',
    logo_url     TEXT,
    owner_id     UUID REFERENCES users(id) ON DELETE SET NULL,
    status_id    UUID REFERENCES customer_statuses(id) ON DELETE SET NULL,
    tier_id      UUID REFERENCES customer_tiers(id) ON DELETE SET NULL,
    revenue      BIGINT,
    size         INTEGER,
    archived_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE customer_needs (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id   UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    customer_id    UUID REFERENCES customers(id) ON DELETE CASCADE,
    issue_id       UUID REFERENCES issues(id) ON DELETE CASCADE,
    project_id     UUID REFERENCES projects(id) ON DELETE CASCADE,
    body           TEXT,
    priority       INTEGER NOT NULL DEFAULT 0,
    attachment_url TEXT,
    creator_id     UUID REFERENCES users(id) ON DELETE SET NULL,
    archived_at    TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Views, favorites, templates ---------------------------------------------------------------

CREATE TABLE custom_views (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    team_id      UUID REFERENCES teams(id) ON DELETE CASCADE,
    owner_id     UUID REFERENCES users(id) ON DELETE CASCADE,
    slug_id      TEXT NOT NULL UNIQUE,
    name         TEXT NOT NULL,
    description  TEXT,
    icon         TEXT,
    color        TEXT,
    -- issue, project or initiative
    model        TEXT NOT NULL DEFAULT 'issue',
    filter       JSONB NOT NULL DEFAULT '{}',
    -- Grouping and ordering for whoever shows the view: {groupBy, orderBy, ...}.
    display      JSONB NOT NULL DEFAULT '{}',
    shared       BOOLEAN NOT NULL DEFAULT false,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE favorites (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id  UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- issue, project, initiative, cycle, custom_view, document, label, team, user or release
    type          TEXT NOT NULL,
    target_id     UUID NOT NULL,
    folder        TEXT,
    sort_order    DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, type, target_id)
);

CREATE TABLE templates (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    team_id      UUID REFERENCES teams(id) ON DELETE CASCADE,
    -- issue, project or document
    type         TEXT NOT NULL,
    name         TEXT NOT NULL,
    description  TEXT,
    -- What the template fills in: title, description, state, priority, labels, sub-issues, ...
    data         JSONB NOT NULL DEFAULT '{}',
    creator_id   UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Notifications -----------------------------------------------------------------------------

CREATE TABLE notifications (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id     UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    actor_id         UUID REFERENCES users(id) ON DELETE SET NULL,
    -- issueAssigned, issueMention, issueComment, issueStatusChanged, ...
    type             TEXT NOT NULL,
    issue_id         UUID REFERENCES issues(id) ON DELETE CASCADE,
    comment_id       UUID REFERENCES comments(id) ON DELETE CASCADE,
    project_id       UUID REFERENCES projects(id) ON DELETE CASCADE,
    initiative_id    UUID REFERENCES initiatives(id) ON DELETE CASCADE,
    document_id      UUID REFERENCES documents(id) ON DELETE CASCADE,
    status_update_id UUID REFERENCES status_updates(id) ON DELETE CASCADE,
    summary          TEXT NOT NULL,
    read_at          TIMESTAMPTZ,
    snoozed_until    TIMESTAMPTZ,
    archived_at      TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX notifications_user ON notifications(user_id, workspace_id, created_at DESC);

-- Webhooks ----------------------------------------------------------------------------------

CREATE TABLE webhooks (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id     UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    team_id          UUID REFERENCES teams(id) ON DELETE CASCADE,
    url              TEXT NOT NULL,
    label            TEXT,
    secret           TEXT NOT NULL,
    resource_types   TEXT[] NOT NULL,
    enabled          BOOLEAN NOT NULL DEFAULT true,
    creator_id       UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE webhook_deliveries (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id    UUID NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    payload       JSONB NOT NULL,
    attempts      INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    status_code   INTEGER,
    error         TEXT,
    delivered_at  TIMESTAMPTZ,
    failed_at     TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX webhook_deliveries_pending ON webhook_deliveries(next_attempt_at)
    WHERE delivered_at IS NULL AND failed_at IS NULL;

-- Audit log of administrative changes, for admins.
CREATE TABLE audit_entries (
    id           BIGSERIAL PRIMARY KEY,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    actor_id     UUID REFERENCES users(id) ON DELETE SET NULL,
    type         TEXT NOT NULL,
    metadata     JSONB NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX audit_entries_workspace ON audit_entries(workspace_id, id DESC);

-- +goose Up
-- +goose StatementBegin
-- The forum: discussions, the smaller threads inside them and who follows them. Comments and
-- notifications can be about either. A comment on a document can suggest text to replace the
-- passage it quotes.

CREATE TABLE discussions (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id     UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    slug_id          TEXT NOT NULL UNIQUE,
    title            TEXT NOT NULL,
    body             TEXT NOT NULL DEFAULT '',
    team_id          UUID REFERENCES teams(id) ON DELETE CASCADE,
    creator_id       UUID REFERENCES users(id) ON DELETE SET NULL,
    pinned_at        TIMESTAMPTZ,
    closed_at        TIMESTAMPTZ,
    trashed_at       TIMESTAMPTZ,
    last_activity_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    search           TSVECTOR GENERATED ALWAYS AS (
        setweight(to_tsvector('simple', coalesce(title, '')), 'A') ||
        setweight(to_tsvector('english', body), 'B')
    ) STORED
);
CREATE INDEX discussions_workspace ON discussions(workspace_id, last_activity_at DESC);
CREATE INDEX discussions_search ON discussions USING GIN (search);

-- bug, design, question, idea, task or general.
CREATE TABLE discussion_threads (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id      UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    discussion_id     UUID NOT NULL REFERENCES discussions(id) ON DELETE CASCADE,
    kind              TEXT NOT NULL DEFAULT 'general',
    title             TEXT NOT NULL,
    body              TEXT NOT NULL DEFAULT '',
    creator_id        UUID REFERENCES users(id) ON DELETE SET NULL,
    issue_id          UUID REFERENCES issues(id) ON DELETE SET NULL,
    resolved_at       TIMESTAMPTZ,
    resolving_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX discussion_threads_discussion ON discussion_threads(discussion_id, created_at);

CREATE TABLE discussion_subscribers (
    discussion_id UUID NOT NULL REFERENCES discussions(id) ON DELETE CASCADE,
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (discussion_id, user_id)
);

-- suggestion_status is pending, accepted or rejected when suggested_text is set.
ALTER TABLE comments
    ADD COLUMN discussion_id        UUID REFERENCES discussions(id) ON DELETE CASCADE,
    ADD COLUMN discussion_thread_id UUID REFERENCES discussion_threads(id) ON DELETE CASCADE,
    ADD COLUMN suggested_text       TEXT,
    ADD COLUMN suggestion_status    TEXT;
CREATE INDEX comments_discussion ON comments(discussion_id);
CREATE INDEX comments_discussion_thread ON comments(discussion_thread_id);

ALTER TABLE notifications
    ADD COLUMN discussion_id        UUID REFERENCES discussions(id) ON DELETE CASCADE,
    ADD COLUMN discussion_thread_id UUID REFERENCES discussion_threads(id) ON DELETE CASCADE;
-- +goose StatementEnd

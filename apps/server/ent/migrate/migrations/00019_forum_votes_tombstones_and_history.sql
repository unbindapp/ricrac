-- +goose Up
-- +goose StatementBegin
CREATE TABLE discussion_votes (
    discussion_id UUID NOT NULL REFERENCES discussions(id) ON DELETE CASCADE,
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (discussion_id, user_id)
);

-- Deleted forum messages stay, emptied, so their replies keep their place.
ALTER TABLE comments ADD COLUMN deleted_at TIMESTAMPTZ;

ALTER TABLE discussions ADD COLUMN edited_at TIMESTAMPTZ;

-- Earlier bodies of forum messages and discussion posts, each as it read from written_at on.
CREATE TABLE message_versions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id  UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    comment_id    UUID REFERENCES comments(id) ON DELETE CASCADE,
    discussion_id UUID REFERENCES discussions(id) ON DELETE CASCADE,
    body          TEXT NOT NULL,
    written_at    TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((comment_id IS NULL) <> (discussion_id IS NULL))
);
CREATE INDEX message_versions_comment ON message_versions(comment_id, written_at);
CREATE INDEX message_versions_discussion ON message_versions(discussion_id, written_at);
-- +goose StatementEnd

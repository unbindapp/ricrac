-- +goose Up
-- +goose StatementBegin
-- People's GitHub user access tokens, sealed with SECRET_KEY, so their comments post as them.
CREATE TABLE github_user_tokens (
    user_id            UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    access_token       TEXT NOT NULL,
    expires_at         TIMESTAMPTZ,
    refresh_token      TEXT,
    refresh_expires_at TIMESTAMPTZ
);

-- Who the app posted a comment as on GitHub; NULL when it posted as itself.
ALTER TABLE github_comment_links ADD COLUMN posted_as UUID REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE github_outbox ADD COLUMN act_as UUID REFERENCES users(id) ON DELETE SET NULL;
-- +goose StatementEnd

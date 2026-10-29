-- +goose Up
-- +goose StatementBegin
ALTER TABLE comments ADD COLUMN github_author TEXT;

ALTER TABLE github_repo_syncs ADD COLUMN link_back BOOLEAN NOT NULL DEFAULT false;

UPDATE comments
SET github_author = substring(body FROM '^\*\*@([A-Za-z0-9-]+) on GitHub\*\*'),
    body = regexp_replace(body, '^\*\*@[A-Za-z0-9-]+ on GitHub\*\*\n\n', '')
WHERE user_id IS NULL
  AND body ~ '^\*\*@[A-Za-z0-9-]+ on GitHub\*\*\n\n'
  AND id IN (SELECT comment_id FROM github_comment_links);
-- +goose StatementEnd


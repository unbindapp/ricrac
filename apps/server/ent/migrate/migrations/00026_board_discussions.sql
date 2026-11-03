-- +goose Up
-- +goose StatementBegin
ALTER TABLE board_issues RENAME TO board_cards;
ALTER INDEX board_issues_column RENAME TO board_cards_column;
ALTER INDEX board_issues_issue RENAME TO board_cards_issue;
ALTER TABLE board_cards ALTER COLUMN issue_id DROP NOT NULL;
ALTER TABLE board_cards ADD COLUMN discussion_id UUID REFERENCES discussions(id) ON DELETE CASCADE;
ALTER TABLE board_cards ADD CONSTRAINT board_cards_one_subject CHECK (num_nonnulls(issue_id, discussion_id) = 1);
ALTER TABLE board_cards ADD CONSTRAINT board_cards_board_id_discussion_id_key UNIQUE (board_id, discussion_id);
CREATE INDEX board_cards_discussion ON board_cards (discussion_id);
-- +goose StatementEnd

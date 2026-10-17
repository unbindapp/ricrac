-- +goose Up
-- +goose StatementBegin
-- Email: notification digests and the emails waiting to go out.

ALTER TABLE notifications ADD COLUMN emailed_at TIMESTAMPTZ;
-- Notifications from before email existed aren't sent.
UPDATE notifications SET emailed_at = created_at;
CREATE INDEX notifications_not_emailed ON notifications(created_at) WHERE emailed_at IS NULL;

ALTER TABLE users ADD COLUMN email_notifications BOOLEAN NOT NULL DEFAULT true;

CREATE TABLE emails (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recipient  TEXT NOT NULL,
    -- What the email says (mail.Content); it is rendered when sent.
    content    JSONB NOT NULL,
    attempts   INT NOT NULL DEFAULT 0,
    error      TEXT,
    send_after TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX emails_pending ON emails(send_after) WHERE sent_at IS NULL;
-- +goose StatementEnd

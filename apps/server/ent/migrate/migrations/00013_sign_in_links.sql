-- +goose Up
-- +goose StatementBegin
-- Emailed sign-in links, kept until they're used or expire.

CREATE TABLE sign_in_links (
    token_hash TEXT PRIMARY KEY,
    email      TEXT NOT NULL,
    return_to  TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX sign_in_links_email ON sign_in_links(email);
-- +goose StatementEnd

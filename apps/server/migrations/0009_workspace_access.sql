-- API keys and OAuth grants reach the listed workspaces, or every workspace their user belongs
-- to (now and later) when workspace_ids is NULL.
ALTER TABLE api_keys ADD COLUMN workspace_ids UUID[];
UPDATE api_keys SET workspace_ids = ARRAY[workspace_id];
ALTER TABLE api_keys DROP COLUMN workspace_id;

ALTER TABLE oauth_grants ADD COLUMN workspace_ids UUID[];
UPDATE oauth_grants SET workspace_ids = ARRAY[workspace_id];
ALTER TABLE oauth_grants DROP COLUMN workspace_id;

ALTER TABLE oauth_codes ADD COLUMN workspace_ids UUID[];
UPDATE oauth_codes SET workspace_ids = ARRAY[workspace_id];
ALTER TABLE oauth_codes DROP COLUMN workspace_id;

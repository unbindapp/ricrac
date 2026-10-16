-- Issue templates that create an issue on a schedule: {"every": 2, "unit": "week", "startsAt": ...}.
ALTER TABLE templates ADD COLUMN recurrence JSONB;
ALTER TABLE templates ADD COLUMN next_run_at TIMESTAMPTZ;
CREATE INDEX templates_next_run ON templates(next_run_at) WHERE next_run_at IS NOT NULL;

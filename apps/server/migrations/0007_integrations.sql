-- Short JSON forms for customers and releases, and indexes for their lookups.

CREATE FUNCTION customer_status_ref(id UUID) RETURNS JSONB LANGUAGE sql STABLE AS $$
    SELECT jsonb_build_object('id', s.id, 'name', s.name, 'color', s.color) FROM customer_statuses s WHERE s.id = $1
$$;

CREATE FUNCTION customer_tier_ref(id UUID) RETURNS JSONB LANGUAGE sql STABLE AS $$
    SELECT jsonb_build_object('id', t.id, 'name', t.name, 'color', t.color) FROM customer_tiers t WHERE t.id = $1
$$;

CREATE FUNCTION release_pipeline_ref(id UUID) RETURNS JSONB LANGUAGE sql STABLE AS $$
    SELECT jsonb_build_object('id', p.id, 'name', p.name, 'slugId', p.slug_id) FROM release_pipelines p WHERE p.id = $1
$$;

CREATE FUNCTION release_stage_ref(id UUID) RETURNS JSONB LANGUAGE sql STABLE AS $$
    SELECT jsonb_build_object('id', s.id, 'name', s.name, 'type', s.type, 'color', s.color)
    FROM release_stages s WHERE s.id = $1
$$;

CREATE INDEX customer_needs_customer ON customer_needs(customer_id);
CREATE INDEX customer_needs_issue ON customer_needs(issue_id);
CREATE INDEX customer_needs_project ON customer_needs(project_id);
CREATE INDEX release_stages_pipeline ON release_stages(pipeline_id);
CREATE INDEX issue_releases_release ON issue_releases(release_id);
CREATE INDEX webhook_deliveries_webhook ON webhook_deliveries(webhook_id, created_at DESC);

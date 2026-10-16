-- Short JSON forms of things other things point at, so every response shows them the same way.
-- Each answers NULL for a NULL id.

CREATE FUNCTION user_ref(id UUID) RETURNS JSONB LANGUAGE sql STABLE AS $$
    SELECT jsonb_build_object('id', u.id, 'name', u.name, 'displayName', u.display_name, 'email', u.email)
    FROM users u WHERE u.id = $1
$$;

CREATE FUNCTION team_ref(id UUID) RETURNS JSONB LANGUAGE sql STABLE AS $$
    SELECT jsonb_build_object('id', t.id, 'key', t.key, 'name', t.name) FROM teams t WHERE t.id = $1
$$;

CREATE FUNCTION state_ref(id UUID) RETURNS JSONB LANGUAGE sql STABLE AS $$
    SELECT jsonb_build_object('id', s.id, 'name', s.name, 'type', s.type, 'color', s.color)
    FROM workflow_states s WHERE s.id = $1
$$;

CREATE FUNCTION issue_identifier(id UUID) RETURNS TEXT LANGUAGE sql STABLE AS $$
    SELECT t.key || '-' || i.number FROM issues i JOIN teams t ON t.id = i.team_id WHERE i.id = $1
$$;

CREATE FUNCTION issue_ref(id UUID) RETURNS JSONB LANGUAGE sql STABLE AS $$
    SELECT jsonb_build_object(
        'id', i.id, 'identifier', t.key || '-' || i.number, 'title', i.title, 'state', state_ref(i.state_id)
    )
    FROM issues i JOIN teams t ON t.id = i.team_id WHERE i.id = $1
$$;

CREATE FUNCTION label_ref(id UUID) RETURNS JSONB LANGUAGE sql STABLE AS $$
    SELECT jsonb_build_object(
        'id', l.id, 'name', l.name, 'color', l.color,
        'parent', (SELECT p.name FROM issue_labels p WHERE p.id = l.parent_id)
    )
    FROM issue_labels l WHERE l.id = $1
$$;

CREATE FUNCTION issue_label_refs(issue UUID) RETURNS JSONB LANGUAGE sql STABLE AS $$
    SELECT coalesce(jsonb_agg(label_ref(l.label_id) ORDER BY lb.name), '[]')
    FROM issue_label_links l JOIN issue_labels lb ON lb.id = l.label_id WHERE l.issue_id = $1
$$;

CREATE FUNCTION project_ref(id UUID) RETURNS JSONB LANGUAGE sql STABLE AS $$
    SELECT jsonb_build_object('id', p.id, 'name', p.name, 'slugId', p.slug_id) FROM projects p WHERE p.id = $1
$$;

CREATE FUNCTION project_status_ref(id UUID) RETURNS JSONB LANGUAGE sql STABLE AS $$
    SELECT jsonb_build_object('id', s.id, 'name', s.name, 'type', s.type, 'color', s.color)
    FROM project_statuses s WHERE s.id = $1
$$;

CREATE FUNCTION project_label_ref(id UUID) RETURNS JSONB LANGUAGE sql STABLE AS $$
    SELECT jsonb_build_object(
        'id', l.id, 'name', l.name, 'color', l.color,
        'parent', (SELECT p.name FROM project_labels p WHERE p.id = l.parent_id)
    )
    FROM project_labels l WHERE l.id = $1
$$;

CREATE FUNCTION milestone_ref(id UUID) RETURNS JSONB LANGUAGE sql STABLE AS $$
    SELECT jsonb_build_object('id', m.id, 'name', m.name, 'targetDate', m.target_date)
    FROM project_milestones m WHERE m.id = $1
$$;

CREATE FUNCTION cycle_ref(id UUID) RETURNS JSONB LANGUAGE sql STABLE AS $$
    SELECT jsonb_build_object('id', c.id, 'number', c.number, 'name', c.name, 'startsAt', c.starts_at, 'endsAt', c.ends_at)
    FROM cycles c WHERE c.id = $1
$$;

CREATE FUNCTION initiative_ref(id UUID) RETURNS JSONB LANGUAGE sql STABLE AS $$
    SELECT jsonb_build_object('id', i.id, 'name', i.name, 'slugId', i.slug_id) FROM initiatives i WHERE i.id = $1
$$;

CREATE FUNCTION document_ref(id UUID) RETURNS JSONB LANGUAGE sql STABLE AS $$
    SELECT jsonb_build_object('id', d.id, 'title', d.title, 'slugId', d.slug_id) FROM documents d WHERE d.id = $1
$$;

CREATE FUNCTION release_ref(id UUID) RETURNS JSONB LANGUAGE sql STABLE AS $$
    SELECT jsonb_build_object('id', r.id, 'name', r.name, 'version', r.version, 'slugId', r.slug_id)
    FROM releases r WHERE r.id = $1
$$;

CREATE FUNCTION customer_ref(id UUID) RETURNS JSONB LANGUAGE sql STABLE AS $$
    SELECT jsonb_build_object('id', c.id, 'name', c.name, 'slugId', c.slug_id) FROM customers c WHERE c.id = $1
$$;

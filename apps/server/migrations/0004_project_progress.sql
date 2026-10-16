-- URL slugs and issue progress of projects, milestones and initiatives.

-- The readable part of a project or initiative URL: lowercase, runs of other characters as '-'.
CREATE FUNCTION entity_slug(name TEXT) RETURNS TEXT LANGUAGE sql IMMUTABLE AS $$
    SELECT trim(both '-' from left(trim(both '-' from regexp_replace(lower($1), '[^a-z0-9]+', '-', 'g')), 60))
$$;

-- Issue counts of a project (or a milestone) and its progress: completed / (all - canceled).
CREATE FUNCTION project_issue_stats(project UUID, milestone UUID) RETURNS JSONB LANGUAGE sql STABLE AS $$
    SELECT jsonb_build_object(
        'issueCount', count(*),
        'completedIssueCount', count(*) FILTER (WHERE s.type = 'completed'),
        'canceledIssueCount', count(*) FILTER (WHERE s.type = 'canceled'),
        'progress', coalesce(round(
            (count(*) FILTER (WHERE s.type = 'completed'))::numeric
                / nullif(count(*) FILTER (WHERE s.type <> 'canceled'), 0), 4)::float8, 0)
    )
    FROM issues i JOIN workflow_states s ON s.id = i.state_id
    WHERE i.trashed_at IS NULL AND (i.project_id = $1 OR i.milestone_id = $2)
$$;

-- The average progress of an initiative's projects.
CREATE FUNCTION initiative_progress(initiative UUID) RETURNS DOUBLE PRECISION LANGUAGE sql STABLE AS $$
    SELECT coalesce(round(avg((project_issue_stats(p.id, NULL)->>'progress')::numeric), 4)::float8, 0)
    FROM initiative_projects ip JOIN projects p ON p.id = ip.project_id
    WHERE ip.initiative_id = $1 AND p.trashed_at IS NULL
$$;

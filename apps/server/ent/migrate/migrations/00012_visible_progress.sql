-- +goose Up
-- +goose StatementBegin
-- Progress counts only the issues the reader can see: issues of private teams they aren't in, or
-- of teams a guest isn't in, are left out.

-- The teams whose issues the user can see in the workspace, as Ctx.TeamIDs decides.
CREATE FUNCTION visible_teams(viewer UUID, workspace UUID) RETURNS UUID[] LANGUAGE sql STABLE AS $$
    SELECT coalesce(array_agg(t.id), '{}') FROM teams t
    JOIN workspace_members m ON m.workspace_id = t.workspace_id AND m.user_id = $1 AND m.active
    WHERE t.workspace_id = $2 AND (
        (m.role <> 'guest' AND NOT t.private)
        OR EXISTS (SELECT 1 FROM team_members tm WHERE tm.team_id = t.id AND tm.user_id = $1))
$$;

CREATE FUNCTION project_issue_stats(project UUID, milestone UUID, viewer UUID) RETURNS JSONB LANGUAGE sql STABLE AS $$
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
    AND i.team_id = ANY(visible_teams($3, i.workspace_id))
$$;

-- The average progress of the initiative's projects the user can see.
CREATE FUNCTION initiative_progress(initiative UUID, viewer UUID) RETURNS DOUBLE PRECISION LANGUAGE sql STABLE AS $$
    SELECT coalesce(round(avg((project_issue_stats(p.id, NULL, $2)->>'progress')::numeric), 4)::float8, 0)
    FROM initiative_projects ip JOIN projects p ON p.id = ip.project_id
    WHERE ip.initiative_id = $1 AND p.trashed_at IS NULL
    AND EXISTS (SELECT 1 FROM project_teams pt WHERE pt.project_id = p.id
        AND pt.team_id = ANY(visible_teams($2, p.workspace_id)))
$$;
-- +goose StatementEnd

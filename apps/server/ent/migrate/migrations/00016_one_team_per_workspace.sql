-- +goose Up
-- +goose StatementBegin
-- A workspace is one team. The teams of a workspace merge into one that keeps the keys, names and
-- ids of the others, so that what agents pass (DES, Design, DES-12, a team's id) keeps resolving.
ALTER TABLE teams
    ADD COLUMN previous_keys   TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN previous_names  TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN merged_team_ids UUID[] NOT NULL DEFAULT '{}';
ALTER TABLE workflow_states ADD COLUMN merged_ids UUID[] NOT NULL DEFAULT '{}';
ALTER TABLE issue_labels ADD COLUMN merged_ids UUID[] NOT NULL DEFAULT '{}';

-- The key of a workspace's team, from its URL key as seed_service.WorkspaceTeamKey makes it.
CREATE FUNCTION workspace_team_key(url_key TEXT) RETURNS TEXT LANGUAGE sql IMMUTABLE AS $$
    WITH words AS (
        SELECT w, n FROM unnest(string_to_array($1, '-')) WITH ORDINALITY AS u(w, n) WHERE w <> ''
    ), joined AS (
        SELECT CASE WHEN count(*) > 1 THEN left(string_agg(left(w, 1), '' ORDER BY n), 5)
                    ELSE left(min(w), 3) END AS k
        FROM words
    )
    SELECT coalesce(nullif(regexp_replace(upper(k), '^[0-9]+', ''), ''), 'TEAM') FROM joined
$$;

-- Merges every team of the workspace into one (made if there is none) and returns it. The team
-- kept is an unarchived one with the most issues, else the oldest. Running it again changes
-- nothing but adding members who joined since.
CREATE FUNCTION unite_workspace_teams(ws UUID) RETURNS UUID LANGUAGE plpgsql AS $fn$
DECLARE
    survivor  UUID;
    merged    RECORD;
    row_      RECORD;
    target    UUID;
    old_ids   UUID[] := '{}';
    new_ids   UUID[] := '{}';
    base      INT;
    moved     INT;
    last_pos  FLOAT8;
BEGIN
    SELECT t.id INTO survivor FROM teams t WHERE t.workspace_id = ws
    ORDER BY t.archived_at IS NOT NULL, (SELECT count(*) FROM issues i WHERE i.team_id = t.id) DESC, t.created_at, t.id
    LIMIT 1;

    IF survivor IS NULL THEN
        INSERT INTO teams (workspace_id, name, key)
        SELECT w.id, w.name, workspace_team_key(w.url_key) FROM workspaces w WHERE w.id = ws
        RETURNING id INTO survivor;
        INSERT INTO workflow_states (team_id, name, type, color, position) VALUES
            (survivor, 'Backlog', 'backlog', '#bec2c8', 0), (survivor, 'Todo', 'unstarted', '#e2e2e2', 1),
            (survivor, 'In Progress', 'started', '#f2c94c', 2), (survivor, 'In Review', 'started', '#4cb782', 3),
            (survivor, 'Done', 'completed', '#5e6ad2', 4), (survivor, 'Canceled', 'canceled', '#95a2b3', 5),
            (survivor, 'Duplicate', 'canceled', '#95a2b3', 6);
        UPDATE teams SET
            default_issue_state_id = (SELECT id FROM workflow_states WHERE team_id = survivor AND name = 'Backlog'),
            auto_close_state_id = (SELECT id FROM workflow_states WHERE team_id = survivor AND name = 'Canceled')
        WHERE id = survivor;
    END IF;

    FOR merged IN SELECT * FROM teams WHERE workspace_id = ws AND id <> survivor ORDER BY created_at, id LOOP
        -- Statuses of the same type and name become the survivor's; the rest move after its own.
        SELECT coalesce(max(position), 0) INTO last_pos FROM workflow_states WHERE team_id = survivor;
        FOR row_ IN SELECT * FROM workflow_states WHERE team_id = merged.id ORDER BY position, created_at, id LOOP
            SELECT s.id INTO target FROM workflow_states s
            WHERE s.team_id = survivor AND s.type = row_.type AND lower(s.name) = lower(row_.name)
            ORDER BY s.archived_at IS NOT NULL, s.position LIMIT 1;
            IF target IS NULL THEN
                last_pos := last_pos + 1;
                UPDATE workflow_states SET team_id = survivor, position = last_pos WHERE id = row_.id;
                CONTINUE;
            END IF;
            UPDATE issues SET state_id = target WHERE state_id = row_.id;
            UPDATE workflow_states SET merged_ids = merged_ids || row_.id || row_.merged_ids WHERE id = target;
            old_ids := old_ids || row_.id;
            new_ids := new_ids || target;
        END LOOP;

        -- Labels of the same name (in the same group) become the survivor's; the rest move. Groups
        -- first, so that their labels find their group's new id.
        FOR row_ IN SELECT * FROM issue_labels WHERE team_id = merged.id ORDER BY parent_id IS NOT NULL, created_at, id LOOP
            SELECT l.parent_id INTO target FROM issue_labels l WHERE l.id = row_.id;
            row_.parent_id := target;
            SELECT s.id INTO target FROM issue_labels s
            WHERE s.team_id = survivor AND s.is_group = row_.is_group AND lower(s.name) = lower(row_.name)
              AND s.parent_id IS NOT DISTINCT FROM row_.parent_id
            ORDER BY s.retired_at IS NOT NULL, s.created_at LIMIT 1;
            IF target IS NULL THEN
                UPDATE issue_labels SET team_id = survivor WHERE id = row_.id;
                CONTINUE;
            END IF;
            INSERT INTO issue_label_links (issue_id, label_id)
            SELECT issue_id, target FROM issue_label_links WHERE label_id = row_.id ON CONFLICT DO NOTHING;
            DELETE FROM issue_label_links WHERE label_id = row_.id;
            UPDATE issue_labels SET parent_id = target WHERE parent_id = row_.id;
            UPDATE issue_labels SET merged_ids = merged_ids || row_.id || row_.merged_ids WHERE id = target;
            DELETE FROM favorites f WHERE f.type = 'label' AND f.target_id = row_.id
                AND EXISTS (SELECT 1 FROM favorites g WHERE g.user_id = f.user_id AND g.type = 'label' AND g.target_id = target);
            UPDATE favorites SET target_id = target WHERE type = 'label' AND target_id = row_.id;
            DELETE FROM issue_labels WHERE id = row_.id;
            old_ids := old_ids || row_.id;
            new_ids := new_ids || target;
        END LOOP;

        -- Cycles move, numbered after the survivor's.
        SELECT coalesce(max(number), 0) INTO base FROM cycles WHERE team_id = survivor;
        UPDATE cycles c SET team_id = survivor, number = base + r.n
        FROM (SELECT id, row_number() OVER (ORDER BY number, id) AS n FROM cycles WHERE team_id = merged.id) r
        WHERE c.id = r.id;

        -- Issues move, numbered after the survivor's; their identifier here stays a previous one.
        SELECT greatest(coalesce(max(i.number), 0), t.issue_count) INTO base
        FROM teams t LEFT JOIN issues i ON i.team_id = t.id WHERE t.id = survivor GROUP BY t.issue_count;
        UPDATE issues i SET team_id = survivor, number = base + r.n,
            previous_identifiers = CASE WHEN upper(merged.key) || '-' || i.number = ANY(i.previous_identifiers)
                THEN i.previous_identifiers
                ELSE array_append(i.previous_identifiers, upper(merged.key) || '-' || i.number) END
        FROM (SELECT id, row_number() OVER (ORDER BY number, id) AS n FROM issues WHERE team_id = merged.id) r
        WHERE i.id = r.id;
        GET DIAGNOSTICS moved = ROW_COUNT;
        IF moved > 0 THEN
            UPDATE teams SET issue_count = base + moved WHERE id = survivor;
        END IF;

        UPDATE custom_views SET team_id = survivor WHERE team_id = merged.id;
        UPDATE discussions SET team_id = survivor WHERE team_id = merged.id;
        UPDATE documents SET team_id = survivor WHERE team_id = merged.id;
        UPDATE github_repo_syncs SET team_id = survivor WHERE team_id = merged.id;
        UPDATE templates SET team_id = survivor WHERE team_id = merged.id;
        UPDATE webhooks SET team_id = survivor WHERE team_id = merged.id;
        UPDATE projects SET lead_team_id = survivor WHERE lead_team_id = merged.id;
        UPDATE teams SET parent_id = survivor WHERE parent_id = merged.id;

        INSERT INTO team_members (team_id, user_id, owner, joined_at)
        SELECT survivor, user_id, owner, joined_at FROM team_members WHERE team_id = merged.id
        ON CONFLICT (team_id, user_id) DO UPDATE SET owner = team_members.owner OR excluded.owner;
        INSERT INTO project_teams (project_id, team_id)
        SELECT project_id, survivor FROM project_teams WHERE team_id = merged.id ON CONFLICT DO NOTHING;
        INSERT INTO release_pipeline_teams (pipeline_id, team_id)
        SELECT pipeline_id, survivor FROM release_pipeline_teams WHERE team_id = merged.id ON CONFLICT DO NOTHING;
        DELETE FROM favorites f WHERE f.type = 'team' AND f.target_id = merged.id
            AND EXISTS (SELECT 1 FROM favorites g WHERE g.user_id = f.user_id AND g.type = 'team' AND g.target_id = survivor);
        UPDATE favorites SET target_id = survivor WHERE type = 'team' AND target_id = merged.id;
        UPDATE invitations SET team_ids = ARRAY(
            SELECT DISTINCT unnest(array_replace(team_ids, merged.id, survivor)))
        WHERE merged.id = ANY(team_ids);

        UPDATE teams t SET
            previous_keys = ARRAY(
                SELECT k FROM unnest(t.previous_keys || upper(merged.key) || merged.previous_keys) WITH ORDINALITY AS u(k, n)
                WHERE k <> upper(t.key) GROUP BY k ORDER BY min(n)),
            previous_names = ARRAY(
                SELECT min(k) FROM unnest(t.previous_names || merged.name || merged.previous_names) WITH ORDINALITY AS u(k, n)
                WHERE lower(k) <> lower(t.name) GROUP BY lower(k) ORDER BY min(n)),
            merged_team_ids = t.merged_team_ids || merged.id || merged.merged_team_ids
        WHERE t.id = survivor;
        old_ids := old_ids || merged.id;
        new_ids := new_ids || survivor;
        DELETE FROM teams WHERE id = merged.id;
    END LOOP;

    -- Filters of views and templates name statuses, labels and teams by id.
    FOR n IN 1 .. coalesce(array_length(old_ids, 1), 0) LOOP
        UPDATE custom_views SET
            filter = replace(filter::text, old_ids[n]::text, new_ids[n]::text)::jsonb,
            display = replace(display::text, old_ids[n]::text, new_ids[n]::text)::jsonb
        WHERE workspace_id = ws AND (filter::text LIKE '%' || old_ids[n] || '%' OR display::text LIKE '%' || old_ids[n] || '%');
        UPDATE templates SET data = replace(data::text, old_ids[n]::text, new_ids[n]::text)::jsonb
        WHERE workspace_id = ws AND data::text LIKE '%' || old_ids[n] || '%';
    END LOOP;

    UPDATE teams SET private = false, parent_id = NULL, archived_at = NULL
    WHERE id = survivor AND (private OR parent_id IS NOT NULL OR archived_at IS NOT NULL);
    INSERT INTO team_members (team_id, user_id)
    SELECT survivor, m.user_id FROM workspace_members m WHERE m.workspace_id = ws AND m.active AND m.role <> 'guest'
    ON CONFLICT DO NOTHING;
    RETURN survivor;
END
$fn$;

SELECT unite_workspace_teams(id) FROM workspaces;
-- +goose StatementEnd

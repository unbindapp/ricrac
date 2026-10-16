package ops

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"ricrac/app"
)

// The workspace, its members and invitations, and the audit log.

func workspaceOps() []*Op {
	return []*Op{
		accountOp("list_workspaces", app.Read, []string{"GET /workspaces"},
			"List the workspaces this token reaches: name, URL key, link and your role. When there are several, pass "+
				"the URL key as `workspace` to the other operations.",
			listWorkspaces).returns(ResultOf[WorkspaceList]()),
		op("get_workspace", app.Read, []string{"GET /workspace"},
			"The workspace: its name, URL key and link, your role, the teams you can see (key, name, private) and "+
				"how many members and issues it has.",
			getWorkspace).returns(ResultOf[Workspace]()),
		op("update_workspace", app.AdminAccess, []string{"PATCH /workspace"},
			"Rename the workspace, change its URL key (2 to 32 lowercase letters, digits and dashes; links with the old "+
				"key stop working) or its logo. Admins only. Omitted fields stay as they are; null clears the logo.",
			updateWorkspace).returns(ResultOf[Workspace]()),
		op("list_users", app.Read, []string{"GET /users"},
			"List the workspace's members with their role, teams and whether they are you. Narrow with query (name, "+
				"display name or email) or team; removed members are left out unless includeInactive is set.",
			listUsers).returns(ResultOf[PageOf[WorkspaceUser]]()),
		op("get_user", app.Read, []string{"GET /users/{id}"},
			"Read a member by id, name, display name, email or \"me\": role, teams, timezone and how many open issues "+
				"are assigned to them. Removed members can be read by id.",
			getUser).returns(ResultOf[WorkspaceUserDetail]()),
		op("update_user", app.Write, []string{"PATCH /users/{id}"},
			"Change a member. Anyone may change their own name, displayName and timezone. Changing a role (owner, admin, "+
				"member, guest) needs an admin; only owners grant or take away owner, and the last owner stays one. Omitted "+
				"fields stay as they are; null clears the timezone.",
			updateUser).returns(ResultOf[WorkspaceUserDetail]()),
		op("remove_user", app.AdminAccess, []string{"DELETE /users/{id}"},
			"Remove a member from the workspace (admins only): they lose access and leave all teams; their issues, "+
				"comments and history stay. The last owner can't be removed. Invite them again to bring them back.",
			removeUser).returns(ResultOf[RemovedUser]()),
		op("list_invitations", app.Read, []string{"GET /invitations"},
			"List pending invitations to the workspace: email, role, teams, who invited and when they expire.",
			listInvitations).returns(ResultOf[PageOf[Invitation]]()),
		op("invite_user", app.Write, []string{"POST /invitations"},
			"Invite someone by email. role is member (default), admin (only admins invite admins) or guest; teams "+
				"(keys, names or ids) are the teams they join, required for guests, who only see those teams. The "+
				"invitation expires in 14 days; inviting the same email again renews it. They accept it in the dashboard.",
			inviteUser).returns(ResultOf[Invitation]()),
		op("delete_invitation", app.Write, []string{"DELETE /invitations/{id}"},
			"Withdraw a pending invitation by id. Admins can withdraw any; members the ones they sent.",
			deleteInvitation).returns(ResultOf[Deleted]()),
		op("get_audit_log", app.AdminAccess, []string{"GET /audit-log"},
			"The workspace's audit log, newest first (admins only): role changes, removed members, invitations, "+
				"archived teams and workspace changes, with who did it. Filter with type, e.g. member.roleChanged.",
			getAuditLog).returns(ResultOf[PageOf[AuditEntry]]()),
	}
}

/* Workspace */

func listWorkspaces(c *app.Caller, _ NoArgs) (any, error) {
	reachable, err := c.Reachable()
	if err != nil {
		return nil, err
	}
	nodes := []any{}
	for _, w := range reachable {
		nodes = append(nodes, M{
			"id": w.ID, "name": w.Name, "urlKey": w.URLKey, "url": c.State.Config.PublicURL + "/" + w.URLKey, "role": w.Role,
		})
	}
	return M{"nodes": nodes}, nil
}

func workspaceValue(c *app.Ctx, q app.Querier) (M, error) {
	teams, err := c.TeamIDs()
	if err != nil {
		return nil, err
	}
	return app.One[M](c, q,
		`SELECT jsonb_build_object(
			'id', w.id,
			'name', w.name,
			'urlKey', w.url_key,
			'logoUrl', w.logo_url,
			'url', $3 || '/' || w.url_key,
			'createdAt', w.created_at,
			'role', $4::text,
			'teams', (SELECT coalesce(jsonb_agg(jsonb_build_object('id', t.id, 'key', t.key, 'name', t.name,
				'private', t.private) ORDER BY t.name), '[]')
				FROM teams t WHERE t.id = ANY($2) AND t.archived_at IS NULL),
			'memberCount', (SELECT count(*) FROM workspace_members m WHERE m.workspace_id = w.id AND m.active),
			'issueCount', (SELECT count(*) FROM issues i WHERE i.workspace_id = w.id AND i.team_id = ANY($2)
				AND i.trashed_at IS NULL))
		 FROM workspaces w WHERE w.id = $1`, c.Workspace, teams, c.PublicURL(), c.Role.String())
}

func getWorkspace(c *app.Ctx, _ NoArgs) (any, error) {
	return workspaceValue(c, c.DB())
}

type UpdateWorkspaceArgs struct {
	Name    *string          `json:"name" doc:"1 to 80 characters."`
	URLKey  *string          `json:"urlKey" doc:"2 to 32 lowercase letters, digits and dashes; must not be taken by another workspace."`
	LogoURL Nullable[string] `json:"logoUrl" doc:"Image URL; null removes the logo."`
}

func sameText(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func updateWorkspace(c *app.Ctx, args UpdateWorkspaceArgs) (any, error) {
	var value M
	err := inTx(c, func(tx pgx.Tx) error {
		type current struct {
			Name    string
			URLKey  string
			LogoURL *string
		}
		was, _, err := app.Row[current](c, tx, "SELECT name, url_key, logo_url FROM workspaces WHERE id = $1 FOR UPDATE", c.Workspace)
		if err != nil {
			return err
		}
		changes := M{}
		if args.Name != nil {
			name, err := nonEmpty(*args.Name, "name")
			if err != nil {
				return err
			}
			if utf8.RuneCountInString(name) > 80 {
				return app.BadRequest("The name is 1 to 80 characters")
			}
			if name != was.Name {
				changes["name"] = M{"from": was.Name, "to": name}
				if err := app.Exec(c, tx, "UPDATE workspaces SET name = $2 WHERE id = $1", c.Workspace, name); err != nil {
					return err
				}
			}
		}
		if args.URLKey != nil {
			key, err := CheckURLKey(*args.URLKey)
			if err != nil {
				return err
			}
			if key != was.URLKey {
				taken, err := app.One[bool](c, tx,
					"SELECT EXISTS (SELECT 1 FROM workspaces WHERE url_key = $1 AND id <> $2)", key, c.Workspace)
				if err != nil {
					return err
				}
				if taken {
					return app.Conflict("The URL key %q is taken", key)
				}
				changes["urlKey"] = M{"from": was.URLKey, "to": key}
				if err := app.Exec(c, tx, "UPDATE workspaces SET url_key = $2 WHERE id = $1", c.Workspace, key); err != nil {
					return err
				}
			}
		}
		if args.LogoURL.Set {
			logo := trimmed(args.LogoURL.Ptr())
			if !sameText(logo, was.LogoURL) {
				changes["logoUrl"] = M{"from": was.LogoURL, "to": logo}
				if err := app.Exec(c, tx, "UPDATE workspaces SET logo_url = $2 WHERE id = $1", c.Workspace, logo); err != nil {
					return err
				}
			}
		}
		if len(changes) > 0 {
			if err := app.Exec(c, tx, "UPDATE workspaces SET updated_at = now() WHERE id = $1", c.Workspace); err != nil {
				return err
			}
			if err := audit(c, tx, "workspace.updated", changes); err != nil {
				return err
			}
		}
		value, err = workspaceValue(c, tx)
		return err
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

/* Members */

// userJSON is the JSON of member u with membership m; $2 is the caller's visible teams, $3 the
// caller.
const userJSON = `jsonb_build_object(
	'id', u.id,
	'name', u.name,
	'displayName', u.display_name,
	'email', u.email,
	'avatarUrl', u.avatar_url,
	'timezone', u.timezone,
	'role', m.role,
	'active', m.active,
	'isMe', u.id = $3,
	'emailNotifications', CASE WHEN u.id = $3 THEN u.email_notifications END,
	'joinedAt', m.joined_at,
	'teams', (SELECT coalesce(jsonb_agg(team_ref(t.id) ORDER BY t.name), '[]')
		FROM team_members tm JOIN teams t ON t.id = tm.team_id
		WHERE tm.user_id = u.id AND t.workspace_id = m.workspace_id AND t.archived_at IS NULL AND t.id = ANY($2)))`

const memberFrom = "FROM users u JOIN workspace_members m ON m.user_id = u.id"

type ListUsersArgs struct {
	Query           *string `json:"query" doc:"Part of a name, display name or email."`
	IncludeInactive bool    `json:"includeInactive,omitempty" doc:"Also list removed members."`
	Team            *string `json:"team" doc:"Only members of this team (key, name or id)."`
	Page
}

func listUsers(c *app.Ctx, args ListUsersArgs) (any, error) {
	team, err := teamArg(c, args.Team)
	if err != nil {
		return nil, err
	}
	teams, err := c.TeamIDs()
	if err != nil {
		return nil, err
	}
	offset, err := args.offset()
	if err != nil {
		return nil, err
	}
	sql := fmt.Sprintf(`SELECT %s %s WHERE m.workspace_id = $1 AND ($4 OR m.active)
		 AND ($5::text IS NULL OR u.name ILIKE $5 OR u.display_name ILIKE $5 OR u.email ILIKE $5)
		 AND ($6::uuid IS NULL OR EXISTS (SELECT 1 FROM team_members tm WHERE tm.team_id = $6 AND tm.user_id = u.id))
		 ORDER BY lower(u.name), u.email LIMIT $7 OFFSET $8`, userJSON, memberFrom)
	nodes, err := app.All[any](c, c.DB(), sql, c.Workspace, teams, c.User, args.IncludeInactive, likePattern(args.Query),
		team, args.limit()+1, offset)
	if err != nil {
		return nil, err
	}
	return args.wrap(nodes)
}

// findMember is an active member by id, name, email or "me", or a removed member by id.
func findMember(c *app.Ctx, value string) (uuid.UUID, error) {
	found, err := resolveUser(c, value)
	if err == nil {
		return found, nil
	}
	id, ok := asUUID(value)
	if !ok {
		return found, err
	}
	removed, queryErr := app.One[bool](c, c.DB(),
		"SELECT EXISTS (SELECT 1 FROM workspace_members WHERE workspace_id = $1 AND user_id = $2)", c.Workspace, id)
	if queryErr != nil {
		return uuid.Nil, queryErr
	}
	if !removed {
		return found, err
	}
	return id, nil
}

func userValue(c *app.Ctx, q app.Querier, user uuid.UUID) (M, error) {
	teams, err := c.TeamIDs()
	if err != nil {
		return nil, err
	}
	sql := fmt.Sprintf(`SELECT %s || jsonb_build_object(
			'openIssueCount', (SELECT count(*) FROM issues i JOIN workflow_states s ON s.id = i.state_id
				WHERE i.workspace_id = m.workspace_id AND i.assignee_id = u.id AND i.trashed_at IS NULL
				AND i.archived_at IS NULL AND s.type NOT IN ('completed', 'canceled')))
		 %s WHERE m.workspace_id = $1 AND u.id = $4`, userJSON, memberFrom)
	value, found, err := app.Maybe[M](c, q, sql, c.Workspace, teams, c.User, user)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, app.NotFound("No such member")
	}
	return value, nil
}

func getUser(c *app.Ctx, args IDArgs) (any, error) {
	user, err := findMember(c, args.ID)
	if err != nil {
		return nil, err
	}
	return userValue(c, c.DB(), user)
}

type UpdateUserArgs struct {
	ID                 string           `json:"id" doc:"The member: id, name, email or \"me\"."`
	Role               *string          `json:"role" doc:"owner, admin, member or guest. Needs an admin; owner changes need an owner."`
	Name               *string          `json:"name" doc:"Your full name."`
	DisplayName        *string          `json:"displayName" doc:"Your short handle, used for @mentions."`
	Timezone           Nullable[string] `json:"timezone" doc:"Your IANA timezone, like Europe/Berlin; null clears it."`
	EmailNotifications *bool            `json:"emailNotifications" doc:"Whether you get unread notifications by email."`
}

func activeOwners(c *app.Ctx, q app.Querier) (int64, error) {
	return app.One[int64](c, q,
		"SELECT count(*) FROM workspace_members WHERE workspace_id = $1 AND role = 'owner' AND active", c.Workspace)
}

func roleOf(c *app.Ctx, q app.Querier, user uuid.UUID) (app.Role, error) {
	name, err := app.One[string](c, q,
		"SELECT role FROM workspace_members WHERE workspace_id = $1 AND user_id = $2 FOR UPDATE", c.Workspace, user)
	if err != nil {
		return app.Guest, err
	}
	role, ok := app.ParseRole(name)
	if !ok {
		return app.Guest, app.BadRequest("Unknown role")
	}
	return role, nil
}

func updateUser(c *app.Ctx, args UpdateUserArgs) (any, error) {
	user, err := resolveUser(c, args.ID)
	if err != nil {
		return nil, err
	}
	profileChange := args.Name != nil || args.DisplayName != nil || args.Timezone.Set || args.EmailNotifications != nil
	if profileChange && user != c.User {
		return nil, app.Forbidden("You can only change your own name, display name, timezone and email notifications")
	}
	var value M
	err = inTx(c, func(tx pgx.Tx) error {
		if args.Role != nil {
			if err := changeRole(c, tx, user, *args.Role); err != nil {
				return err
			}
		}
		if args.Name != nil {
			name, err := nonEmpty(*args.Name, "name")
			if err != nil {
				return err
			}
			if err := app.Exec(c, tx, "UPDATE users SET name = $2 WHERE id = $1", user, name); err != nil {
				return err
			}
		}
		if args.DisplayName != nil {
			displayName, err := nonEmpty(strings.TrimLeft(*args.DisplayName, "@"), "displayName")
			if err != nil {
				return err
			}
			if strings.IndexFunc(displayName, unicode.IsSpace) >= 0 {
				return app.BadRequest("A display name is one word, used for @mentions, like \"ann\"")
			}
			if err := app.Exec(c, tx, "UPDATE users SET display_name = $2 WHERE id = $1", user, displayName); err != nil {
				return err
			}
		}
		if args.Timezone.Set {
			timezone := trimmed(args.Timezone.Ptr())
			if timezone != nil {
				if err := checkTimezone(c, tx, *timezone); err != nil {
					return err
				}
			}
			if err := app.Exec(c, tx, "UPDATE users SET timezone = $2 WHERE id = $1", user, timezone); err != nil {
				return err
			}
		}
		if args.EmailNotifications != nil {
			if err := app.Exec(c, tx, "UPDATE users SET email_notifications = $2 WHERE id = $1", user, *args.EmailNotifications); err != nil {
				return err
			}
		}
		value, err = userValue(c, tx, user)
		return err
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

func changeRole(c *app.Ctx, tx pgx.Tx, user uuid.UUID, value string) error {
	if err := c.Require(app.AdminAccess); err != nil {
		return err
	}
	role, ok := app.ParseRole(strings.TrimSpace(value))
	if !ok {
		return app.BadRequest("role is owner, admin, member or guest")
	}
	old, err := roleOf(c, tx, user)
	if err != nil {
		return err
	}
	if (role == app.Owner || old == app.Owner) && role != old && c.Role != app.Owner {
		return app.Forbidden("Only owners can make someone an owner or take it away")
	}
	if old == app.Owner && role != app.Owner {
		owners, err := activeOwners(c, tx)
		if err != nil {
			return err
		}
		if owners <= 1 {
			return app.BadRequest("The workspace needs an owner; make someone else owner first")
		}
	}
	if role == old {
		return nil
	}
	if err := app.Exec(c, tx, "UPDATE workspace_members SET role = $3 WHERE workspace_id = $1 AND user_id = $2",
		c.Workspace, user, role.String()); err != nil {
		return err
	}
	return audit(c, tx, "member.roleChanged", M{"user": user, "from": old.String(), "to": role.String()})
}

// checkTimezone accepts IANA timezone names Postgres knows, like Europe/Berlin or UTC.
func checkTimezone(c *app.Ctx, q app.Querier, zone string) error {
	known, err := app.One[bool](c, q, "SELECT EXISTS (SELECT 1 FROM pg_timezone_names WHERE name = $1)", zone)
	if err != nil {
		return err
	}
	if !known {
		return app.BadRequest("%q isn't a timezone; use an IANA name like Europe/Berlin or UTC", zone)
	}
	return nil
}

func removeUser(c *app.Ctx, args IDArgs) (any, error) {
	user, err := resolveUser(c, args.ID)
	if err != nil {
		return nil, err
	}
	err = inTx(c, func(tx pgx.Tx) error {
		role, err := roleOf(c, tx, user)
		if err != nil {
			return err
		}
		if role == app.Owner {
			if c.Role != app.Owner {
				return app.Forbidden("Only owners can remove an owner")
			}
			owners, err := activeOwners(c, tx)
			if err != nil {
				return err
			}
			if owners <= 1 {
				return app.BadRequest("The last owner can't be removed; make someone else owner first")
			}
		}
		if err := app.Exec(c, tx, "UPDATE workspace_members SET active = false WHERE workspace_id = $1 AND user_id = $2",
			c.Workspace, user); err != nil {
			return err
		}
		if err := app.Exec(c, tx,
			"DELETE FROM team_members tm USING teams t WHERE t.id = tm.team_id AND t.workspace_id = $1 AND tm.user_id = $2",
			c.Workspace, user); err != nil {
			return err
		}
		if err := app.Exec(c, tx,
			`UPDATE teams SET triage_responsible_ids = array_remove(triage_responsible_ids, $2)
			 WHERE workspace_id = $1 AND $2 = ANY(triage_responsible_ids)`, c.Workspace, user); err != nil {
			return err
		}
		email, err := app.One[string](c, tx, "SELECT email FROM users WHERE id = $1", user)
		if err != nil {
			return err
		}
		return audit(c, tx, "member.removed", M{"user": user, "email": email, "role": role.String()})
	})
	if err != nil {
		return nil, err
	}
	return M{"removed": user}, nil
}

/* Invitations */

const invitationJSON = `jsonb_build_object(
	'id', i.id,
	'email', i.email,
	'role', i.role,
	'teams', (SELECT coalesce(jsonb_agg(team_ref(t.id) ORDER BY t.name), '[]') FROM teams t WHERE t.id = ANY(i.team_ids)),
	'inviter', user_ref(i.inviter_id),
	'createdAt', i.created_at,
	'expiresAt', i.expires_at)`

type ListInvitationsArgs struct {
	Page
}

func listInvitations(c *app.Ctx, args ListInvitationsArgs) (any, error) {
	if err := c.RequireMember(); err != nil {
		return nil, err
	}
	offset, err := args.offset()
	if err != nil {
		return nil, err
	}
	nodes, err := app.All[any](c, c.DB(), fmt.Sprintf(
		`SELECT %s FROM invitations i WHERE i.workspace_id = $1 AND i.expires_at > now()
		 ORDER BY i.created_at DESC LIMIT $2 OFFSET $3`, invitationJSON), c.Workspace, args.limit()+1, offset)
	if err != nil {
		return nil, err
	}
	return args.wrap(nodes)
}

type InviteUserArgs struct {
	Email string   `json:"email" doc:"Their email address; they sign in with the Google account of this email."`
	Role  *string  `json:"role" doc:"member (default), admin (only admins invite admins) or guest."`
	Teams []string `json:"teams" doc:"Teams they join (keys, names or ids). Required for guests."`
}

func validEmail(email string) bool {
	local, domain, found := strings.Cut(email, "@")
	return found && local != "" && strings.Contains(domain, ".") && !strings.HasPrefix(domain, ".") &&
		!strings.HasSuffix(domain, ".") && strings.IndexFunc(email, unicode.IsSpace) < 0
}

func inviteUser(c *app.Ctx, args InviteUserArgs) (any, error) {
	if err := c.RequireMember(); err != nil {
		return nil, err
	}
	email := strings.ToLower(strings.TrimSpace(args.Email))
	if !validEmail(email) {
		return nil, app.BadRequest("%q isn't an email address", email)
	}
	role := app.Member
	switch name := strings.TrimSpace(orEmpty(args.Role)); {
	case args.Role == nil || name == "member":
	case name == "guest":
		role = app.Guest
	case name == "admin":
		if err := c.Require(app.AdminAccess); err != nil {
			return nil, app.Forbidden("Only admins can invite admins")
		}
		role = app.Admin
	default:
		return nil, app.BadRequest("role is member, admin or guest, not %q", name)
	}
	teams := []uuid.UUID{}
	for _, team := range args.Teams {
		id, err := resolveTeam(c, team)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(teams, id) {
			teams = append(teams, id)
		}
	}
	if role == app.Guest && len(teams) == 0 {
		return nil, app.BadRequest("Guests only see their teams; name at least one in teams")
	}
	var value M
	err := inTx(c, func(tx pgx.Tx) error {
		member, err := app.One[bool](c, tx,
			`SELECT EXISTS (SELECT 1 FROM users u JOIN workspace_members m ON m.user_id = u.id
				WHERE m.workspace_id = $1 AND m.active AND lower(u.email) = $2)`, c.Workspace, email)
		if err != nil {
			return err
		}
		if member {
			return app.Conflict("%s is already a member of this workspace", email)
		}
		id, err := app.One[uuid.UUID](c, tx,
			`INSERT INTO invitations (workspace_id, email, role, team_ids, inviter_id, expires_at)
			 VALUES ($1, $2, $3, $4, $5, now() + interval '14 days')
			 ON CONFLICT (workspace_id, email) DO UPDATE SET role = excluded.role, team_ids = excluded.team_ids,
				inviter_id = excluded.inviter_id, created_at = now(), expires_at = excluded.expires_at
			 RETURNING id`, c.Workspace, email, role.String(), teams, c.User)
		if err != nil {
			return err
		}
		if err := audit(c, tx, "invitation.created", M{"email": email, "role": role.String()}); err != nil {
			return err
		}
		if err := queueInvitationEmail(c, tx, email, role); err != nil {
			return err
		}
		value, err = app.One[M](c, tx, fmt.Sprintf("SELECT %s FROM invitations i WHERE i.id = $1", invitationJSON), id)
		return err
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

func deleteInvitation(c *app.Ctx, args IDArgs) (any, error) {
	if err := c.RequireMember(); err != nil {
		return nil, err
	}
	id, err := resolveByID(c, "invitations", "invitation", args.ID)
	if err != nil {
		return nil, err
	}
	inviter, err := app.One[*uuid.UUID](c, c.DB(), "SELECT inviter_id FROM invitations WHERE id = $1", id)
	if err != nil {
		return nil, err
	}
	if (inviter == nil || *inviter != c.User) && c.Role < app.Admin {
		return nil, app.Forbidden("Only admins can withdraw invitations someone else sent")
	}
	if err := app.Exec(c, c.DB(), "DELETE FROM invitations WHERE id = $1", id); err != nil {
		return nil, err
	}
	return M{"deleted": id}, nil
}

/* Audit log */

type AuditLogArgs struct {
	Type *string `json:"type" doc:"Only entries of this type, e.g. member.roleChanged, member.removed, invitation.created, team.archived, workspace.updated."`
	Page
}

func getAuditLog(c *app.Ctx, args AuditLogArgs) (any, error) {
	offset, err := args.offset()
	if err != nil {
		return nil, err
	}
	var kind *string
	if args.Type != nil {
		kind = new(strings.TrimSpace(*args.Type))
	}
	nodes, err := app.All[any](c, c.DB(),
		`SELECT jsonb_build_object('id', a.id, 'type', a.type, 'actor', user_ref(a.actor_id), 'metadata', a.metadata,
			'createdAt', a.created_at)
		 FROM audit_entries a WHERE a.workspace_id = $1 AND ($2::text IS NULL OR a.type = $2)
		 ORDER BY a.id DESC LIMIT $3 OFFSET $4`, c.Workspace, kind, args.limit()+1, offset)
	if err != nil {
		return nil, err
	}
	return args.wrap(nodes)
}

/* Shapes */

type WorkspaceList struct {
	Nodes []ReachableWorkspace `json:"nodes"`
}

type ReachableWorkspace struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	URLKey string `json:"urlKey"`
	URL    string `json:"url"`
	Role   string `json:"role" enum:"owner,admin,member,guest"`
}

type Workspace struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	URLKey      string          `json:"urlKey"`
	LogoURL     *string         `json:"logoUrl"`
	URL         string          `json:"url"`
	CreatedAt   string          `json:"createdAt"`
	Role        string          `json:"role" enum:"owner,admin,member,guest" doc:"Your role."`
	Teams       []WorkspaceTeam `json:"teams" doc:"The teams you can see."`
	MemberCount int             `json:"memberCount"`
	IssueCount  int             `json:"issueCount" doc:"Issues of the teams you can see, trashed ones left out."`
}

type WorkspaceTeam struct {
	ID      string `json:"id"`
	Key     string `json:"key"`
	Name    string `json:"name"`
	Private bool   `json:"private"`
}

type WorkspaceUser struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	DisplayName        string    `json:"displayName"`
	Email              string    `json:"email"`
	AvatarURL          *string   `json:"avatarUrl"`
	Timezone           *string   `json:"timezone"`
	Role               string    `json:"role" enum:"owner,admin,member,guest"`
	Active             bool      `json:"active" doc:"False once removed from the workspace."`
	IsMe               bool      `json:"isMe"`
	EmailNotifications *bool     `json:"emailNotifications" doc:"Only shown for yourself."`
	JoinedAt           string    `json:"joinedAt"`
	Teams              []TeamRef `json:"teams" doc:"Their teams that you can see."`
}

type WorkspaceUserDetail struct {
	WorkspaceUser
	OpenIssueCount int `json:"openIssueCount" doc:"Open issues assigned to them."`
}

type RemovedUser struct {
	Removed string `json:"removed"`
}

type Invitation struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role" enum:"admin,member,guest"`
	Teams     []TeamRef `json:"teams" doc:"The teams they join."`
	Inviter   *UserRef  `json:"inviter"`
	CreatedAt string    `json:"createdAt"`
	ExpiresAt string    `json:"expiresAt"`
}

type AuditEntry struct {
	ID        int      `json:"id"`
	Type      string   `json:"type" doc:"Like member.roleChanged, member.removed, invitation.created, team.archived or workspace.updated."`
	Actor     *UserRef `json:"actor"`
	Metadata  M        `json:"metadata"`
	CreatedAt string   `json:"createdAt"`
}

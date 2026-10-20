# Ricrac

The issue tracker for people and agents. Ricrac has teams, issues, workflows, projects,
initiatives, cycles and the rest. People work in it through the web app (sign in with Google):
lists and boards, an inbox, a command panel and keyboard shortcuts. Agents work in it through an
MCP server and a REST API, with the same workspaces, operations and permissions.

## Features

- **Workspaces** with members and roles (owner, admin, member, guest), email invitations and an
  audit log.
- **Email**: invitations, and digests of notifications still unread after a couple of minutes
  (each person can turn them off).
- **Teams** with keys (ENG), private teams, sub-teams, members, and settings: estimates
  (exponential, Fibonacci, linear, t-shirt), cycles, triage with responsibility rotations,
  auto-archive, auto-close, and auto-closing parents and sub-issues.
- **Workflow statuses** per team (triage, backlog, unstarted, started, completed, canceled).
- **Issues** with identifiers (ENG-123, still resolvable after a move to another team), Markdown
  descriptions with partial patches, status, priority, estimate, assignee, labels (with
  single- or multi-select groups), project and milestone, cycle, due date, SLA, snoozing,
  sub-issues, relations (blocks, blocked by, related, duplicate), link attachments and uploaded
  files, subscribers, sharing with people outside the team, history, archive and trash,
  changes to many issues at once, and imports from other trackers that keep issue numbers,
  dates, authors and comments.
- **Writing** in Markdown everywhere (descriptions, documents, comments, updates, release notes)
  with a toolbar, `/` commands, `@` mentions, `:` emoji, tables and code blocks with languages.
  Paste, drop or upload images, videos and files: images and videos embed (`![demo.mp4](url)`
  plays inline), and can be moved, aligned and resized; width and alignment ride in the URL
  fragment (`#width=480&align=center`), so the Markdown stays plain for agents.
- **Filtering** with a filter language (`and`/`or`, comparators, relation filters)
  and full-text search.
- **Comments** in threads with replies, resolving and reactions, on issues, projects,
  initiatives, documents, milestones and status updates. **@mentions** notify people.
- **Projects** across teams, with statuses, lead, members, labels, dates, priority, milestones,
  dependencies, links, status updates with health, and progress.
- **Initiatives** grouping projects, with sub-initiatives and status updates.
- **Cycles**, created and completed automatically, with rollover of unfinished issues and daily
  progress history.
- **Documents** attached to projects, initiatives, teams, issues, cycles or releases, with
  version history.
- **Custom views**, **favorites**, **templates** (issue templates with sub-issues, recurring
  issues on a schedule, project templates with milestones and issues, document templates), an **inbox** of notifications
  (read, archive, snooze), **customers** and customer requests, **releases** with pipelines,
  stages and release notes, and **webhooks** signed with HMAC-SHA256.
- **GitHub**, as a GitHub App admins install on their repositories: pull requests, branches and
  commits that mention ENG-123 are attached to the issue and move it (opened, then done when a
  pull request that fixes it merges or a fixing commit lands on the default branch), and
  repositories synced with a team keep their issues and comments in step both ways. See
  [docs/github-app.md](docs/github-app.md).

## Connecting agents

- **MCP** (streamable HTTP): `https://<host>/mcp`. Clients sign in with OAuth 2.1, using dynamic
  client registration or client ID metadata documents (Claude, ChatGPT, Claude Code, Codex,
  Cursor). The consent screen grants all your workspaces (including ones you join later) or the
  ones you pick; when a connection reaches several, every tool takes a `workspace` argument and
  `list_workspaces` lists them. The everyday operations are listed as tools; the rest are found
  with `find_operations` and run with `call_operation` (`/mcp?tools=all` lists every operation as
  a tool instead).
- **REST**: `https://<host>/v1`, with `Authorization: Bearer <token>`. The token is an OAuth
  access token or an API key (`rr_key_...`) made in the web app (Settings → API keys). The OpenAPI document is at
  `/v1/openapi.json` and an overview for agents at `/llms.txt`. Every operation is also
  `POST /v1/ops/<name>`.
- **OAuth apps** (for integrations such as Motile) are registered in the web app (Settings → OAuth apps): authorize at
  `/oauth/authorize`, exchange codes at `/oauth/token` (with the client secret, or PKCE without
  it), revoke at `/oauth/revoke`. Scopes are `read`, `write` and `admin`.
- **Webhooks** POST `{action, type, data, updatedFrom, url, ...}` with a `Ricrac-Signature`
  header: the hex HMAC-SHA256 of the body, keyed with the webhook's secret.

## Configuration

| Variable | |
| --- | --- |
| `DATABASE_URL` | Postgres connection string. Migrations run on start. |
| `PUBLIC_URL` | External base URL, e.g. `https://ricrac.example.com`. |
| `SECRET_KEY` | At least 32 characters; signs session cookies. |
| `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` | Sign-in with Google. The OAuth client's redirect URI is `<PUBLIC_URL>/api/auth/callback/google`. |
| `SMTP_URL`, `EMAIL_FROM` | Email for invitations and notification digests, e.g. `smtps://resend:<API key>@smtp.resend.com:465` and `Ricrac <notifications@example.com>`. Without them no emails are sent. |
| `GITHUB_APP_ID`, `GITHUB_APP_SLUG`, `GITHUB_APP_PRIVATE_KEY`, `GITHUB_WEBHOOK_SECRET`, `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET` | The GitHub App (see [docs/github-app.md](docs/github-app.md)): its id, its URL name (`github.com/apps/<slug>`), its private key (PEM; `\n` for line breaks works), the webhook secret, and its OAuth client id and secret. All or none; without them GitHub is off. |
| `GITHUB_API_URL`, `GITHUB_WEB_URL` | GitHub's API and web addresses, `https://api.github.com` and `https://github.com` by default (for GitHub Enterprise or tests). |
| `MAX_UPLOAD_MB` | Largest uploaded file, 25 by default. |
| `PORT` | 3000 by default. |

## Development

See AGENTS.md. In short: `docker compose up -d`, then `go run ./cmd/server` in `apps/server` and
`bun dev` in `apps/web`.

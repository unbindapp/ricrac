## What is this?

Ricrac is an issue tracker for people and agents. Agents use it through the MCP server (`/mcp`)
and the REST API (`/v1`); people sign in with Google to the web app, which runs on the same
operations. README.md covers features and configuration.

Two apps: `apps/server` is the Go server (net/http, pgx on Postgres) and `apps/web` the web app
(Vite, React, TanStack Router + Query, Tailwind, shadcn), embedded into the server binary at
build time. Production is the `Ricrac` project on Unbind: the `ricrac` service (this repo's
Dockerfile) and a `postgres` database service.

## Repo Structure:

### apps/server (Go, module `ricrac`, pgx on Postgres)

- `ops/` holds every operation of the tracker, defined once with `op(name, access, routes,
  description, handler)`. `ops.All()` feeds the MCP tools (`server/mcp.go`), the REST routes, the
  generic `POST /v1/ops/{name}` and the OpenAPI document (`server/rest.go`). MCP lists the everyday
  operations (`CORE` in `server/mcp.go`) as tools and reaches the rest through `find_operations`
  and `call_operation`, so a session doesn't start with every definition; `/mcp?tools=all` lists
  them all. A handler is `func(c *app.Ctx, args Args) (any, error)`; `Args` is a struct with
  camelCase `json` tags, and its `doc` tags (and `enum`) are what agents read (`ops/schema.go`
  turns it into a JSON Schema; unknown fields are refused, plain fields are required).
- `ops/resolve.go` turns what agents pass (names, keys, identifiers like ENG-123, "me", links)
  into ids and checks the caller may see the thing; errors list the valid options.
  `ops/filter.go` is the issue filter language, used by `list_issues` and custom views.
  `ops/events.go` writes issue history, notifications, webhook deliveries and the audit log inside
  the change's transaction. `ops/util.go` has dates and durations, paging, slugs and Markdown
  patches. `ops/seed.go` is what new workspaces and teams start with. `ops/email.go` queues emails
  (invitations, notification digests) in the change's transaction and sends them.
- `app/` is what everything shares: config from the environment, `State`, errors, signed keys, the
  DB helpers (`app.One`, `Maybe`, `All`, `Rows`, `Row`, `Exec`, the `app.SQL` builder) and the
  migrator (it keeps sqlx's `_sqlx_migrations` bookkeeping). `app/ctx.go` is the caller: `Caller`
  is the user, token scopes and the workspaces the token reaches (one, several or all);
  `Caller.Ctx` picks the one a call acts in (the `workspace` argument, links among the arguments,
  or the only one) and gives a `Ctx`: user, workspace, role (owner, admin, member, guest), scopes
  and the teams they can see (private teams only for their members; guests only their teams).
  Operations defined with `accountOp` run with the `Caller` instead.
- `migrations/0002_refs.sql` defines `user_ref`, `team_ref`, `state_ref`, `issue_ref`,
  `project_ref`, ... SQL functions: every response shows a referenced thing in the same short JSON
  form. Responses are built in SQL with `jsonb_build_object` (at most 100 arguments per call;
  join two with `||`).
- `server/oauth.go` is the OAuth 2.1 server (discovery, dynamic registration, client ID metadata
  documents, dashboard apps, PKCE, rotating refresh tokens, revocation) and the bearer check for
  `/v1` and `/mcp`, which also takes API keys (`rr_key_...`). Grants and keys reach the
  workspaces chosen when they were made, or all of the user's (`workspace_ids` NULL).
- `server/google.go` is sign-in with Google; `server/account.go` the dashboard's JSON API under
  `/api` (workspaces, invitations, API keys, OAuth apps, connected apps, OAuth consent).
- `server/jobs.go` runs the automations every minute (auto-archive, auto-close, trash, snoozes,
  due dates, cycles, recurring issues from templates, emails) and delivers webhooks.
- `server/files.go` serves uploads; `server/spa.go` serves the web build copied into
  `server/dist` (the committed `index.html` is a placeholder).
- `mail/` renders emails (`mail/templates`) and sends them over SMTP.

### apps/web (Vite, React, TanStack Router + Query + Form, Tailwind v4, shadcn on base-ui)

The app people work in: issues, projects, cycles, inbox, settings. TipTap edits Markdown,
dnd-kit drags, cmdk is the command panel.

- `src/routes` are the pages (file routes; `src/routeTree.gen.ts` is generated). Everything in a
  workspace lives under `/$workspace` (its URL key).
- `src/lib/server/client.ts` is the only way to the server: `call(workspace, "op_name", args)`
  runs an operation (`POST /v1/ops/{name}` with the session and `x-ricrac-workspace`), typed by
  `src/api/ops.gen.ts`; `account.*` is the dashboard API under `/api` (me, workspaces, keys, apps),
  typed by `src/api/account.gen.ts`. Both generated files come from `go test ./ops ./server` in
  apps/server; never edit them, regenerate them. Use the generated shapes (`Issue`,
  `IssueDetail`, ...) and `OpResult<"name">` instead of declaring your own.
- `src/lib/queries/*` wrap operations in TanStack Query options and mutations, one module per
  area; components never call the server directly. Queries and mutation hooks take the
  workspace id (`useWorkspace().workspace.id`), and their keys start with it
  (`[workspaceId, "teams", ...]`) so one invalidation refreshes a workspace; `callAll` reads
  every page of a small list. `teams.ts` and `workspaces.ts` show the shape.
- `/$workspace` (`routes/$workspace/_workspace/route.tsx`) loads `get_workspace` and provides
  `useWorkspace()` (workspace, role, `canEdit` for everyone but guests, `isAdmin` for owners and
  admins) and `useTeams()` (`list_teams`); route params use `workspace.urlKey`.
- Page structure comes only from `components/layout` (`PageContainer`, `PageHeader`,
  `PageToolbar`, `SettingsPage`, `SettingsSection`, `EmptyState`, `ErrorCard`, `ListSkeleton`,
  `SplitView`). Spacing: `gap-1.5` icon to label, `gap-2` controls in a row, `gap-3` cards,
  `gap-4` form fields, `gap-6` sections, `p-4` cards, rows `h-9`. Type: page title
  `text-xl font-semibold` via `PageHeader`, section title `text-base font-semibold`, body
  `text-sm`, secondary `text-sm text-muted-foreground`, captions and identifiers `text-xs`,
  identifiers `font-mono`. Buttons only from `components/ui/button`, sizes `sm|default|icon`.
  Icons lucide `size-4` (`size-3.5` in `text-xs`, `size-5` in headers). No arbitrary Tailwind
  values, no raw `<button>`/`<input>`. Every list has skeleton, error and empty states; every
  mutation failure toasts; icon-only buttons have `aria-label`. `bun run lint` enforces the
  mechanical parts.

## Conventions

- Output and arguments are camelCase. Timestamps are ISO 8601 UTC.
- `save_*` creates without `id` and updates with it. Optional fields left out stay as they are;
  `null` clears them (`Nullable[T]` fields).
- Lists take `limit` and `cursor` and answer `{ nodes, pageInfo: { hasNextPage, endCursor } }`
  through `Page`.
- pgx can't type a bare `$n` that is only used in `CASE`, `coalesce` or `jsonb_build_object`:
  cast it (`$2::uuid`, `$6::jsonb`).
- Deleting issues, projects, initiatives and documents moves them to the trash (`trashed_at`);
  the trash is emptied after 30 days.
- Every mutation runs in one transaction (`inTx`), records history (issues) and emits a webhook
  event (`emit`) with the entity's JSON.

## General Rules:

- Keep it simple. Do not overcomplicate things.
- Do not leave paragraphs of comments on top of the code. You should try to avoid them as much as possible with understandable function names and code. If they are necessary even then, make them concise. Remove such comments when you come by them in the codebase. Comments should always move with code, not be left behind.
- Use guard statement patterns in any code you write.
- Do not edit generated code: `go.sum`, `bun.lock`, `src/routeTree.gen.ts`, `src/api/*.gen.ts`.
  Never edit an applied migration in `migrations/`, add a new one.
- Do not write useless tests; tests should cover input/output behaviour.
- Reinvent the wheel but do not reinvent the car. If you are solving a simple problem do not
  introduce a library. If you are solving a complex but common problem, there is likely a
  modern library for it, if so, use it.
- Do not start editing code in response to a question. We'll tell you when to edit code.
- If we are missing a glaring issue when we ask you to do something, do not hesitate to
  point it out.
- Never commit or push code unless explicitly asked to do so.
- Never make a PR unless explicitly asked to do so.
- Do not insert yourself into our code, commits or PRs in any way. Our codebase is not your
  ad space.
- When a deploy changes required env vars, deploy the code first and change the variables
  after. Unbind restarts the running pod on every variable change.
- After you make code changes, run the checks (see below) and fix what they raise.

## Development

Needs Go 1.26, Bun and Docker (for Postgres).

    docker compose up -d                       # Postgres on localhost:5436
    cd apps/server && go run .                 # with the variables from .env.example exported
    cd apps/web && bun install && bun dev      # http://localhost:5173, proxies to the server

Checks (`go test` needs the compose Postgres, or `DATABASE_URL` pointing at another one; each
end-to-end test gets a throwaway database):

    cd apps/server && gofmt -l . && go vet ./... && go test ./...
    cd apps/web && bun run lint && bun run build && bun run test

The end-to-end tests live in `apps/server/e2e/`: each starts from `world(t)` (a workspace
"acme" owned by Ann with the team ENG) and calls operations with `call`, `fails` and
`tryCall`; `http_test.go` goes through the real router (REST, OAuth, MCP). Behaviour changes need
a test there.

Signing in needs Google; in development `go run . session ann@acme.test` (in apps/server, with the
variables exported) prints a session cookie for that user instead, and
`node scripts/seed.mjs "<cookie>"` (in apps/web) fills the server with a workspace "acme" to
click through.

For browser checks use headless Chromium through `playwright-core`, never the editor's preview tab.

## Commit Messages

A short imperative sentence describing the change, no prefixes:

    Refuse redirect URIs with fragments

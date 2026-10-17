## What is this?

Ricrac is an issue tracker for people and agents. Agents use it through the MCP server (`/mcp`)
and the REST API (`/v1`); people sign in with Google to the web app, which runs on the same
operations. README.md covers features and configuration.

Two apps: `apps/server` is the Go server (Huma on Chi, Ent on Postgres, Goose migrations) and `apps/web` the web app
(Vite, React, TanStack Router + Query, Tailwind, shadcn), embedded into the server binary at
build time. Production is the `Ricrac` project on Unbind: the `ricrac` service (this repo's
Dockerfile) and a `postgres` database service.

## Repo Structure:

### apps/server (Go, module `ricrac`)

Built like Zigzag (https://github.com/zigzagapp-org/zigzag, `apps/api`): handlers call services,
services call repositories, repositories use Ent.

- `cmd/server` is the binary (`session <email>` prints a session cookie for development);
  `config/` reads the environment.
- `ent/schema` maps every table (generated code in `ent/`, `make ent`). Join tables with two keys
  are edge schemas; columns with a database default are `Optional()`. `ent/migrate/migrations`
  holds the Goose migrations (`make migrate NAME=what_changed`, then write the SQL), applied on
  start by `internal/infrastructure/database`. Its first versions are the old sqlx migrations: a
  database the previous server migrated is adopted (its `_sqlx_migrations` checked and recorded
  for Goose, then left as it is).
- `internal/api/oapi` registers operations: a handler group calls
  `oapi.Register(reg, oapi.Operation{Name, Access, Routes, Description}, service.Method)` with
  `Method(ctx, actor *auth.Actor, input *XInput) (Model, error)` (`RegisterAccount` for operations
  that run with the `auth.Caller`). Each operation is `POST /v1/ops/{name}`, its REST routes on Huma,
  and an MCP tool (`internal/mcpserver`: the everyday ones in `core`, the rest through
  `find_operations` and `call_operation`, all with `/mcp?tools=all`). Input structs are the
  arguments: camelCase `json` tags, `doc` and `enum` tags are what agents read
  (`internal/common/args` turns them into JSON Schema; unknown fields are refused, plain fields
  are required). Handler groups live in `internal/api/handlers/<area>` and are registered in
  `router.RegisterHandlers`, in the order operations are listed.
- `internal/api/router` puts everything on one Chi router: operations, `internal/api/meta`
  (`/v1`, `/v1/openapi.json`, `/v1/ops`, `/llms.txt`), MCP, the dashboard API under `/api`
  (`handlers/account`, on Huma), OAuth 2.1 (`handlers/oauthserver`: discovery, dynamic
  registration, client ID metadata documents, PKCE, rotating refresh tokens, revocation), sign-in
  with Google (`handlers/signin`), files and uploads (`handlers/files`), GitHub webhooks and
  callbacks (`handlers/github`) and the web app (`internal/web`, the build copied into
  `internal/web/dist`; the committed `index.html` is a placeholder). `internal/api/middleware`
  has CORS, body limits, compression and the bearer check (OAuth access tokens and API keys,
  `rr_key_...`; grants and keys reach the workspaces chosen, or all of the user's when
  `workspace_ids` is NULL).
- `internal/services/<area>` hold the logic, checks and transactions
  (`repo.WithTx(ctx, func(tx) error)`); `internal/repositories/<area>` the Ent queries, taking the
  transaction (`nil` reads from the pool). Raw SQL only where Ent can't say it, through
  `repository.Raw`/`RawExpr` (`?` placeholders). `repo.Now(ctx, tx)` is Postgres' `now()` for
  the transaction; stored times use it.
- `internal/auth` is roles, scopes, signed keys and sessions; `auth.Actor` is the caller in one
  workspace (user, role, scopes, the teams they can see), picked by `access_service.Pick` (the
  `workspace` argument, links among the arguments, or the only one).
- `internal/services/resolve` turns what agents pass (names, keys, identifiers like ENG-123,
  "me", links) into ids and checks the caller may see the thing; errors list the valid options.
  `internal/services/events` writes issue history, notifications, webhook deliveries, the audit
  log and changes for GitHub inside the change's transaction. `internal/services/seed` is what new
  workspaces and teams start with. The issue filter language is `internal/services/issue/filter.go`.
- `internal/models` is what operations answer, built in Go from Ent entities (`XOf` functions,
  short forms in `refs.go`). Times are written as Postgres wrote them (`models.Time`), since the
  answers are a contract with agents and the web app.
- `internal/scheduler` runs the jobs: automations every minute (auto-archive, auto-close, trash,
  snoozes, due dates, cycles, recurring issues, emails, OAuth cleanup), webhook deliveries and the
  GitHub sync.
- `internal/infrastructure/mail` renders emails (`templates/`) and sends them over SMTP.
  `internal/integrations/github` is the GitHub App client (app JWT, installation tokens, payloads,
  finding ENG-123 in branches and messages); `internal/services/github` connects installations,
  links pull requests, branches and commits to issues and syncs issues of chosen repositories both
  ways. `docs/github-app.md` is how to create the app.
- Interfaces come from ifacemaker and mocks from Mockery (`.mockery.yml`, `make interfaces`);
  service unit tests use them through `internal/services/service_test_base.go`.

### apps/web (Vite, React, TanStack Router + Query + Form, Tailwind v4, shadcn on base-ui)

The app people work in: issues, projects, cycles, inbox, settings. TipTap edits Markdown,
dnd-kit drags, cmdk is the command panel.

- `src/routes` are the pages (file routes; `src/routeTree.gen.ts` is generated). Everything in a
  workspace lives under `/$workspace` (its URL key).
- `src/lib/server/client.ts` is the only way to the server: `call(workspace, "op_name", args)`
  runs an operation (`POST /v1/ops/{name}` with the session and `x-ricrac-workspace`), typed by
  `src/api/ops.gen.ts`; `account.*` is the dashboard API under `/api` (me, workspaces, keys, apps),
  typed by `src/api/account.gen.ts`. Both generated files come from `go test ./internal/api/router`
  in apps/server; never edit them, regenerate them. Use the generated shapes (`Issue`,
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
- Deleting issues, projects, initiatives and documents moves them to the trash (`trashed_at`);
  the trash is emptied after 30 days.
- Every mutation runs in one transaction (`WithTx`), records history (issues) and emits a webhook
  event (`events.Emit`) with the entity's answer.
- Answers, errors (`{"error": message}`), webhook payloads and emails are a contract: agents and
  the web app depend on them. Change them on purpose only.

## General Rules:

- Keep it simple. Do not overcomplicate things.
- Do not leave paragraphs of comments on top of the code. You should try to avoid them as much as possible with understandable function names and code. If they are necessary even then, make them concise. Remove such comments when you come by them in the codebase. Comments should always move with code, not be left behind.
- Use guard statement patterns in any code you write.
- Do not edit generated code: `go.sum`, `bun.lock`, `src/routeTree.gen.ts`, `src/api/*.gen.ts`,
  `apps/server/ent` (but `ent/schema`), `apps/server/mocks`, `*_iface.go`. Never edit an applied
  migration in `ent/migrate/migrations`, add a new one.
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
    cd apps/server && go run ./cmd/server      # with the variables from .env.example exported
    cd apps/web && bun install && bun dev      # http://localhost:5173, proxies to the server

Checks (`go test` needs the compose Postgres, or `DATABASE_URL` pointing at another one; each
end-to-end test gets a throwaway database):

    cd apps/server && make check
    cd apps/web && bun run lint && bun run build && bun run test

The end-to-end tests live in `apps/server/e2e/`: each starts from `world(t)` (a workspace
"acme" owned by Ann with the team ENG) and talks to the server only through its router, with
`call`, `fails` and `tryCall` for operations; `backend_test.go` is the one file that knows the
server's packages. Behaviour changes need a test there. Every test records what crosses the
server's edges (answers, webhooks, GitHub calls, emails) in `e2e/testdata/compat` and must match
it; `RICRAC_RECORD=1` rewrites the recordings, for intended changes only. `TestCutover`
(`RICRAC_CUTOVER_FROM=<commit>`) rehearses deploying over the server at that commit.

Signing in needs Google; in development `go run ./cmd/server session ann@acme.test` (in apps/server, with the
variables exported) prints a session cookie for that user instead, and
`node scripts/seed.mjs "<cookie>"` (in apps/web) fills the server with a workspace "acme" to
click through.

For browser checks use headless Chromium through `playwright-core`, never the editor's preview tab.

## Commit Messages

A short imperative sentence describing the change, no prefixes:

    Refuse redirect URIs with fragments

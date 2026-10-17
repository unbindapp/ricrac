# Porting the old server to the new layers (temporary, deleted when the port is done)

The server is being rebuilt on the architecture of Zigzag (https://github.com/zigzagapp-org/zigzag,
`apps/api`): Huma on Chi, Ent, Goose, Mockery, layered handlers → services → repositories. It is
already in production, so **every answer must stay byte-for-byte the same** (JSON shape, field
presence, null vs missing, error status and message, order of lists, webhook payloads, emails,
GitHub calls). The old code lives in `app/`, `ops/` and `server/` and keeps serving whatever isn't
ported yet through `internal/legacy` (the bridge). When an operation is registered by a new handler
group, the bridge stops serving it.

## How you know you're done

`go test ./e2e` must pass with **no differences from the recordings** in `e2e/testdata/compat`.
Those were recorded from the old server; every request, answer, webhook, GitHub call and email of
every test is compared after ids, secrets and times are numbered/masked. A difference prints the
exchange and the JSON path. Fix the new code until it answers the same; never re-record
(`RICRAC_RECORD`) and never edit the recordings or the tests' expectations. If a test exercises
too little of your area, add a new test (`e2e/*_test.go`) — but a new test has no recording, so
recordings for it must come from the OLD server: ask the lead (report it) rather than recording
from your code.

Also run: `gofmt -l`, `go vet ./...`, `go test ./internal/... ./e2e`, and
`go test ./internal/api/router` (regenerates `apps/web/src/api/ops.gen.ts`; `git diff apps/web`
must show no change: the operations' schemas are part of the contract).

Run everything from `apps/server`. Postgres is the compose one on localhost:5436 (already up;
tests make throwaway databases).

## Layout (follow it exactly)

- `internal/models/` — answer shapes (move the area's result structs from `ops/` here, same names,
  same field order, same tags; the struct names are the web client's type names) and `XOf(...)`
  functions turning ent entities into them. Shared shapes and refs are in `shapes.go`/`refs.go`
  (`UserRefOf`, `IssueRefOf`, ...). Times: `models.Time`, `TimePtr`, `Date`, `DatePtr` (they write
  what Postgres wrote in JSON). A string shown only when asked for, even empty: `models.Text` with
  `json:"x,omitzero"`. A list shown only when asked for, even empty: a named slice type with
  `IsZero() bool { return x == nil }` and `json:"x,omitzero"`.
- `internal/repositories/<area>/` (package `<area>_repo`) — Ent queries and writes. Struct
  `<Area>Repository{base *repository.BaseRepository}`, `New<Area>Repository(db *ent.Client)`,
  a `//go:generate ... ifacemaker` line like `label/label.go`, and methods
  `(ctx, tx repository.TxInterface, ...)` using `self.base.Client(tx)` (nil tx = the pool). Add
  the repository to `internal/repositories/repositories/repositories.go` (field, constructor, accessor).
- `internal/services/<area>/` (package `<area>_service`) — the logic of the old `ops/<file>.go`,
  ported line by line. Input structs are the old `XArgs` structs, renamed if you like but with the
  **same fields, tags, doc and order** (they are the operations' schemas). Services take
  `(ctx, actor *auth.Actor, input *XInput)` and return a value (not a pointer) of the result model.
  Transactions: `self.repo.WithTx(ctx, func(tx repository.TxInterface) error {...})`. Add an
  ifacemaker line. Wire it in `internal/api/server/server.go` and `build.go`.
- `internal/api/handlers/<area>/` (package `<area>_handler`) — `RegisterHandlers(srv, reg)` calling
  `oapi.Register(reg, oapi.Operation{Name, Access, Routes, Description}, srv.XService.Method)` for
  each operation, **in the old order, with the old names, routes, access and description text
  copied exactly** (`RegisterAccount` for operations that ran with the Caller). Call it from
  `router.RegisterHandlers` in the old group order (see `ops.All()`).
- Mocks: add the new interfaces to `.mockery.yml` and to `internal/services/service_test_base.go`;
  `make interfaces` regenerates interfaces and mocks. Write unit tests for the service with the
  mocks (see `internal/services/label/label_test.go`) for logic worth testing (validation,
  decisions); the e2e recordings cover the rest. No useless tests.

`labels` is the reference: compare `ops/labels.go` with `internal/{models/label.go,
repositories/label,services/label,api/handlers/labels}`.

## Shared pieces you use (don't fork them; add a new file to the package if you must extend)

- `auth.Actor` (old `app.Ctx`): `User`, `Workspace`, `Role`, `Scopes`, `Origin`, `ActorID()`,
  `TeamIDs(ctx)`, `CanSeeTeam`, `Require*`, `Fresh()`. `auth.Caller` is the old `app.Caller`.
- `resolve_service.ResolveService` (old `ops/resolve.go`): `User(s)`, `Team`, `OptionalTeam`,
  `State`, `Issue(s)`, `CanSeeIssue`, `Labels`, `Project(s)`, `Milestone`, `Cycle`, `Initiative(s)`,
  `Document`, `CanSeeDocument`, `ProjectStatus`, `ProjectLabels`, `Release(s)`, `Pipeline`,
  `Customer`, `CustomView`, `Template`, `ByID`, plus `FindTyped`, `TypedNames`, `LabelMatches`.
- `events_service.EventsService` (old `ops/events.go` + GitHub queueing): `Emit`, `History`,
  `Audit`, `Notify`, `IssueSubscribers`, `Subscribe`, `Mentions`, `QueueCommentForGitHub`, and
  `events_service.ChangedFields(before, after)` (old `changedFields`).
- `issue_service.IssueService` core: `Values/Value` (the `Issue` list shape), `SetState`,
  `DefaultState`, `FirstStateOfType`, `StateType`, `TriageResponsible`, `AllowedEstimates`,
  `CheckEstimate`. `seed_service`: `WorkspaceDefaults`, `CreateTeam`, `EnsureTriageState`,
  `TeamKey`, `CheckURLKey`, `CheckTeamKey`.
- `internal/common/args`: `Nullable[T]`, `Page` (`Size`, `Offset`, `Wrap`, `WrapOf`), `PageOf[T]`,
  `TextEdit`, `ApplyPatch`, `NewContent`, `Parse`, `SchemaOf`.
- `internal/common/utils`: dates/durations (`ParseDate`, `ParseDatetime`), `SlugID`, `CheckColor`,
  `PickColor`, `NonEmpty`, `CheckPriority`, `PriorityNames`, `OrEmpty`, `Trimmed`, `LikePattern`,
  `EscapeLike`, `AsUUID`, `LastSegment`, `SlugTail`, `NameSlug`, `MatchesName`, `Options`,
  `AppendUnique`, `ParseIdentifier`, `SplitGroup`, `MentionHandles`.
- `internal/common/errdefs`: `BadRequest`, `NotFound`, `Forbidden`, `Conflict`, `Errorf`, `As`.
- `repository.Raw(sql, args...)` / `repository.RawExpr`: SQL predicates/expressions for Ent with
  `?` placeholders (never write `$1` inside Ent builders). `self.repo.Now(ctx, tx)` is Postgres'
  `now()` for the transaction: use it wherever the old SQL wrote `now()` so rows written together
  share one time (never `time.Now()` for stored times unless the old code used Go's clock).

## Ent

Generated in `ent/` from `ent/schema` (one schema per table, mapping the existing columns; never
change the database schema — no migrations in this port). Edges: a foreign key column `x_id`
gives the edge `x` (`WithX()`, `HasXWith`), the referenced side gets `<table>` or
`<table>_by_<x>`; self references `parent_id` give `parent`/`children`; join tables with two keys
are edge schemas (`issue.WithLabels()`, `team.WithMembers()`, ...). Read `ent/schema/<table>.go`.
Fields with a database default are `Optional()`: leave them unset to get the default.

Write queries with Ent builders, predicates and eager loading. Where the old SQL used something
Ent can't express (CTEs, aggregates with FILTER, full text search, array operators, `lower()`
orders), use `Modify(func(s *entsql.Selector){...})`, `repository.Raw`, or as a last resort
`client.QueryContext`/`ExecContext` with the old SQL. Never build answers in SQL
(`jsonb_build_object`, `*_ref()`): load entities and build the model in Go.

**Match the old SQL's semantics exactly**: the same rows, in the same order (copy `ORDER BY`s,
including `lower(...)`, `NULLS LAST`, ties), the same `now()`, the same read connection (the old
code read with `c.DB()` — the pool — or with `tx`; pass nil or tx accordingly), the same error
messages. Answers are built after the change, from the database, like the old code did.

## Rules

- Self-documenting code, near-zero comments (one terse line for a real trap). Guard clauses, no
  nested if/else pyramids. Receivers are `self` like Zigzag. Match the surrounding code.
- Don't touch other areas' packages, the old packages (`app`, `ops`, `server`), `e2e` expectations
  or recordings. Don't change shared foundation files except to add your wiring lines.
- Don't commit generated code by hand-editing (`ent/`, `mocks/`, `*_iface.go`): regenerate.
- When done, commit your work on your branch with a short imperative sentence (no prefixes, no
  attribution lines), and report: what you ported, anything you could not make identical and why,
  shared files you touched.

## More shared pieces

- `comment_service.CommentService.Threads(ctx, tx, column, id)` and `models.CommentOf`,
  `CommentThreadOf` (old `commentThreads`/`commentJSON`); `comment_repo.WithAnswer`, `Targets`.
- `attachment_service.AttachmentService.Upsert` (old `upsertAttachment`/`saveAttachment`), `Value`
  (old `attachmentValue`), `AssetURL`, `URLHost`, `SourceType`; `models.AttachmentOf`, `Object`.
- `issue_repo.WithAnswer` loads what `models.IssueOf` needs.
- `srv.Ops` (`opcall.Late`): run an operation by name as an actor, like the old
  `Find("save_issue").Call(c, args)` in templates and projects. Inject it into your service.
- Waking background senders: `events_service.Waker` (already done by `Emit` and the GitHub queue).

## Background jobs

The old jobs (`server/jobs.go`, and `ops.RunCycleAutomations`, `RunRecurringTemplates`,
`SendEmails`, `DeliverDueWebhooks`, `cleanupOAuth`, GitHub jobs) move into the services of the
area that owns them, as exported methods taking `ctx` (e.g. `WebhookService.DeliverDue(ctx) (int,
error)`). Point the matching hook in `e2e/backend_test.go` at your method. Don't build the
scheduler; the lead wires it at the end.

## Areas (one agent each; stay inside yours)

- teams: `ops/workspace.go`, `teams.go`, `states.go`, `email.go` (users, workspaces, invitations
  and their emails, notification digests, teams, members, statuses).
- issues: `ops/issues.go`, `filter.go`, `import.go`, `views.go`.
- conversation: `ops/comments.go`, `attachments.go`, `notifications.go`, `search.go`, and the
  upload/download routes of `server/files.go`.
- planning: `ops/projects.go`, `milestones.go`, `status_updates.go`, `initiatives.go`.
- organizing: `ops/cycles.go`, `documents.go`, `favorites.go`, `templates.go`.
- integrations: `ops/customers.go`, `releases.go`, `webhooks.go`.
- account: `server/account.go`, `google.go`, `oauth.go`, `spa.go` (the dashboard API under
  `/api` on Huma, the OAuth server and well-known documents, sign-in with Google, the web app),
  and `apps/web/src/api/account.gen.ts` from the new structs.
- github (later): `ops/github*.go`, `server/github.go`.

If your area needs something another area owns that isn't shared yet, don't port it yourself:
call the old operation through `srv.Ops` when a whole operation fits, otherwise report it.

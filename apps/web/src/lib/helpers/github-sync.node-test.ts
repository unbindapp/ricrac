import assert from "node:assert/strict";
import { test } from "node:test";

import {
  githubErrorMessage,
  githubEventLabel,
  importSummary,
  isImporting,
  issueRepositories,
} from "./github-sync.ts";

const backfill = {
  status: "completed" as const,
  issuesCreated: 12,
  issuesUpdated: 3,
  commentsCreated: 1,
  skipped: 0,
};

test("a pending or running import is importing", () => {
  assert.equal(isImporting({ backfill: { ...backfill, status: "pending" } }), true);
  assert.equal(isImporting({ backfill: { ...backfill, status: "running" } }), true);
  assert.equal(isImporting({ backfill }), false);
  assert.equal(isImporting({ backfill: null }), false);
});

test("the import summary counts what was created and updated", () => {
  assert.equal(importSummary(backfill), "12 issues created, 3 updated, 1 comment");
  assert.equal(
    importSummary({ ...backfill, issuesCreated: 1, skipped: 2 }),
    "1 issue created, 3 updated, 1 comment, 2 skipped",
  );
});

test("a pending import or none has no summary", () => {
  assert.equal(importSummary(null), null);
  assert.equal(importSummary({ ...backfill, status: "pending" }), null);
});

test("known errors explain themselves and unknown ones fall back", () => {
  assert.match(githubErrorMessage("connected_elsewhere"), /another workspace/);
  assert.equal(githubErrorMessage("nonsense"), githubErrorMessage("failed"));
});

test("webhook events read as sentences", () => {
  assert.equal(githubEventLabel("pull_request.opened"), "Pull request opened");
  assert.equal(githubEventLabel("push"), "Push");
});

const repoSync = (
  name: string,
  linkedIssueCount: number,
  project: string | null = null,
  createdAt = "2026-01-01T00:00:00Z",
) => ({ name, linkedIssueCount, createdAt, project: project ? { id: project } : null });

test("new issues open in the project's repository, else the one with the most issues", () => {
  const syncs = [
    repoSync("api", 10),
    repoSync("web", 4, "p1"),
    repoSync("docs", 10, null, "2025-06-01T00:00:00Z"),
  ];
  const names = (projectId?: string) =>
    issueRepositories(syncs, projectId).map((item) => item.name);
  assert.deepEqual(names("p1"), ["web", "docs", "api"]);
  assert.deepEqual(names(), ["docs", "api", "web"]);
  assert.deepEqual(names("p2"), ["docs", "api", "web"]);
});

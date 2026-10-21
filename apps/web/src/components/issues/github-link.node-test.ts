import assert from "node:assert/strict";
import { test } from "node:test";

import { githubLinkOf } from "./github-link.ts";

test("pull request attachments read their repository, number and state", () => {
  assert.deepEqual(
    githubLinkOf({
      sourceType: "github",
      title: "#42 Limit reads",
      metadata: { repo: "unbindapp/unbind", number: 42, state: "merged", branch: "unb-1" },
    }),
    {
      kind: "pull_request",
      reference: "unbindapp/unbind#42",
      state: "merged",
      summary: "",
      base: "",
      stats: null,
    },
  );
});

test("an unknown pull request state shows no chip", () => {
  assert.deepEqual(
    githubLinkOf({
      sourceType: "github",
      title: "#1 Fix",
      metadata: { repo: "a/b", number: 1, state: "weird" },
    }),
    { kind: "pull_request", reference: "a/b#1", state: null, summary: "", base: "", stats: null },
  );
});

test("pull requests read their summary and size", () => {
  assert.deepEqual(
    githubLinkOf({
      sourceType: "github",
      title: "#42 Limit reads",
      metadata: {
        repo: "a/b",
        number: 42,
        state: "open",
        summary: " Resolves #7 \n",
        base: "main",
        additions: 12,
        deletions: 3,
        changed_files: 2,
      },
    }),
    {
      kind: "pull_request",
      reference: "a/b#42",
      state: "open",
      summary: "Resolves #7",
      base: "main",
      stats: { additions: 12, deletions: 3, changedFiles: 2 },
    },
  );
});

test("a plain GitHub link is no pull request", () => {
  assert.equal(githubLinkOf({ sourceType: "github", title: "README", metadata: {} }), null);
});

test("branch attachments read their repository", () => {
  assert.deepEqual(
    githubLinkOf({
      sourceType: "github_branch",
      title: "unb-1",
      metadata: { repo: "a/b", branch: "unb-1" },
    }),
    { kind: "branch", reference: "a/b" },
  );
});

test("commit attachments read their short sha and subject", () => {
  assert.deepEqual(
    githubLinkOf({
      sourceType: "github_commit",
      title: "3e1f0c9 Cap websocket frames",
      metadata: { repo: "unbindapp/unbind", sha: "3e1f0c9a7b5d2e8f", branch: "master" },
    }),
    {
      kind: "commit",
      reference: "unbindapp/unbind",
      sha: "3e1f0c9",
      subject: "Cap websocket frames",
    },
  );
});

test("a commit title without its sha is kept whole", () => {
  assert.deepEqual(
    githubLinkOf({
      sourceType: "github_commit",
      title: "Renamed",
      metadata: { repo: "a/b", sha: "abcdef0123" },
    }),
    { kind: "commit", reference: "a/b", sha: "abcdef0", subject: "Renamed" },
  );
});

test("links and uploads are not GitHub attachments", () => {
  assert.equal(githubLinkOf({ sourceType: "link", title: "Docs", metadata: {} }), null);
  assert.equal(githubLinkOf({ sourceType: "upload", title: "a.png", metadata: {} }), null);
});

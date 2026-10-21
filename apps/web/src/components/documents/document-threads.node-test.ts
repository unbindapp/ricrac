import assert from "node:assert/strict";
import { test } from "node:test";

import { groupThreads, linkedThreadId, threadAnnotations } from "./document-threads.ts";

const content = "# Plan\n\nShip the beta on Friday. Then write the launch post.";

function thread(
  id: string,
  quotedText: string | null,
  overrides: { resolvedAt?: string; createdAt?: string; suggestionStatus?: string } = {},
) {
  return {
    id,
    quotedText,
    resolvedAt: overrides.resolvedAt ?? null,
    createdAt: overrides.createdAt ?? "2026-10-20T10:00:00Z",
    suggestionStatus: overrides.suggestionStatus ?? null,
  };
}

test("open threads follow their quotes through the document, unquoted ones after", () => {
  const { open } = groupThreads(
    [
      thread("general", null, { createdAt: "2026-10-20T09:00:00Z" }),
      thread("post", "launch post"),
      thread("gone", "a removed sentence"),
      thread("beta", "the beta"),
    ],
    content,
  );
  assert.deepEqual(
    open.map((item) => item.id),
    ["beta", "post", "general", "gone"],
  );
});

test("resolved threads go apart, last resolved first", () => {
  const { open, resolved } = groupThreads(
    [
      thread("early", "Plan", { resolvedAt: "2026-10-19T10:00:00Z" }),
      thread("late", "Friday", { resolvedAt: "2026-10-21T10:00:00Z" }),
      thread("open", "Friday"),
    ],
    content,
  );
  assert.deepEqual(
    open.map((item) => item.id),
    ["open"],
  );
  assert.deepEqual(
    resolved.map((item) => item.id),
    ["late", "early"],
  );
});

test("only open quoted comments and pending suggestions are highlighted", () => {
  assert.deepEqual(
    threadAnnotations([
      thread("comment", "the beta"),
      thread("general", null),
      thread("pending", "Friday", { suggestionStatus: "pending" }),
      thread("rejected", "Plan", { suggestionStatus: "rejected" }),
      thread("resolved", "launch", { resolvedAt: "2026-10-21T10:00:00Z" }),
    ]),
    [
      { id: "comment", quote: "the beta" },
      { id: "pending", quote: "Friday" },
    ],
  );
});

test("a #comment- link finds its thread by the first comment or a reply", () => {
  const threads = [
    { id: "t1", replies: [] },
    { id: "t2", replies: [{ id: "r1" }] },
  ];
  assert.equal(linkedThreadId(threads, "comment-t1"), "t1");
  assert.equal(linkedThreadId(threads, "comment-r1"), "t2");
  assert.equal(linkedThreadId(threads, "comment-gone"), null);
  assert.equal(linkedThreadId(threads, "update-t1"), null);
});

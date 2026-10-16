import assert from "node:assert/strict";
import { test } from "node:test";

import type { Notification } from "../../api/ops.gen.ts";
import {
  commentExcerpt,
  groupNotifications,
  notificationLink,
  notificationSentence,
} from "./notifications.ts";

const now = Date.parse("2026-10-16T12:00:00Z");
const bob = { id: "u-bob", name: "Bob Builder", displayName: "bob", email: "bob@acme.test" };
const state = { id: "s1", name: "Done", type: "completed" as const, color: "#000" };
const eng1 = { id: "i1", identifier: "ENG-1", title: "Fix login", state };
const project = { id: "p1", name: "Billing", slugId: "a1b2c3" };

function notification(overrides: Partial<Notification>): Notification {
  return {
    id: "n",
    type: "issueNewComment",
    summary: "",
    actor: bob,
    issue: null,
    comment: null,
    project: null,
    initiative: null,
    document: null,
    statusUpdate: null,
    readAt: null,
    snoozedUntil: null,
    archivedAt: null,
    createdAt: "2026-10-16T11:00:00Z",
    ...overrides,
  };
}

test("notifications about the same issue make one row, newest first", () => {
  const groups = groupNotifications(
    [
      notification({ id: "a", issue: eng1, createdAt: "2026-10-16T11:30:00Z" }),
      notification({ id: "b", project, createdAt: "2026-10-16T11:20:00Z" }),
      notification({ id: "c", issue: eng1, readAt: "2026-10-16T11:40:00Z" }),
    ],
    "inbox",
    now,
  );
  assert.deepEqual(
    groups.map((group) => [group.key, group.notifications.map((n) => n.id), group.unreadIds]),
    [
      ["issue:i1", ["a", "c"], ["a"]],
      ["project:p1", ["b"], ["b"]],
    ],
  );
});

test("each filter shows its own notifications", () => {
  const nodes = [
    notification({ id: "unread", issue: eng1 }),
    notification({ id: "read", project, readAt: "2026-10-16T11:10:00Z" }),
    notification({
      id: "snoozed",
      document: { ...project, title: "FAQ" },
      snoozedUntil: "2026-10-17T09:00:00Z",
    }),
    notification({ id: "woke", initiative: project, snoozedUntil: "2026-10-16T10:00:00Z" }),
    notification({ id: "archived", archivedAt: "2026-10-16T11:10:00Z" }),
  ];
  const ids = (filter: "inbox" | "unread" | "snoozed" | "archived") =>
    groupNotifications(nodes, filter, now).map((group) => group.latest.id);
  assert.deepEqual(ids("inbox"), ["unread", "read", "woke"]);
  assert.deepEqual(ids("unread"), ["unread", "woke"]);
  assert.deepEqual(ids("snoozed"), ["snoozed"]);
  assert.deepEqual(ids("archived"), ["archived"]);
});

test("the open row stays in the unread filter after it is read", () => {
  const read = [notification({ id: "a", issue: eng1, readAt: "2026-10-16T11:50:00Z" })];
  assert.equal(groupNotifications(read, "unread", now).length, 0);
  assert.equal(groupNotifications(read, "unread", now, "issue:i1").length, 1);
});

test("sentences name who did what, or fall back to the summary", () => {
  const say = (overrides: Partial<Notification>) => notificationSentence(notification(overrides));
  assert.deepEqual(say({ type: "issueCommentReply" }), {
    actor: "Bob Builder",
    text: "replied in a thread you're in",
  });
  assert.deepEqual(
    say({ type: "projectUpdateCommentMention" }).text,
    "mentioned you in a comment on an update",
  );
  assert.deepEqual(
    say({ type: "issueStatusChanged", summary: "ENG-1 moved to In Review" }).text,
    "moved it to In Review",
  );
  assert.deepEqual(say({ type: "issueDue", actor: null, summary: "ENG-1 is overdue" }), {
    actor: null,
    text: "Overdue",
  });
  assert.deepEqual(say({ type: "triageIssue", actor: null }), {
    actor: null,
    text: "Needs triage",
  });
  assert.deepEqual(say({ type: "somethingNew", summary: "ENG-1 got a sticker" }), {
    actor: null,
    text: "ENG-1 got a sticker",
  });
});

test("links lead to the target, at the comment or update", () => {
  const issueLink = notificationLink(
    notification({ issue: eng1, comment: { id: "c1", body: "" } }),
    "acme",
  );
  assert.deepEqual(issueLink, {
    to: "/$workspace/issue/$identifier",
    params: { workspace: "acme", identifier: "ENG-1" },
    hash: "comment-c1",
  });
  const updateLink = notificationLink(
    notification({ project, statusUpdate: { id: "u1" } }),
    "acme",
  );
  assert.deepEqual(updateLink, {
    to: "/$workspace/project/$slug",
    params: { workspace: "acme", slug: "a1b2c3" },
    hash: "update-u1",
  });
  assert.equal(notificationLink(notification({}), "acme"), null);
});

test("comment excerpts are plain text on one line", () => {
  assert.equal(
    commentExcerpt("## Plan\n\n- **Ship** [the fix](https://x.test)\n> quoted ![img](a.png)"),
    "Plan Ship the fix quoted",
  );
});

import type { LinkProps } from "@tanstack/react-router";

import type { Notification, StateRef } from "@/api/ops.gen";
import type { TInboxFilter } from "@/lib/queries/notifications";

/** What a notification is about: the inbox shows one row per target. */
export type TNotificationTarget =
  | { kind: "issue"; id: string; identifier: string; title: string; state: StateRef }
  | { kind: "project" | "initiative" | "document"; id: string; title: string; slugId: string }
  | { kind: "none"; id: string; title: string };

export type TNotificationGroup = {
  key: string;
  target: TNotificationTarget;
  /** Newest first. */
  notifications: Notification[];
  latest: Notification;
  unreadIds: string[];
};

export function notificationTarget(notification: Notification): TNotificationTarget {
  const { issue, project, initiative, document } = notification;
  if (issue) return { kind: "issue", ...issue };
  if (project)
    return { kind: "project", id: project.id, title: project.name, slugId: project.slugId };
  if (initiative) {
    return {
      kind: "initiative",
      id: initiative.id,
      title: initiative.name,
      slugId: initiative.slugId,
    };
  }
  if (document) return { kind: "document", ...document };
  return { kind: "none", id: notification.id, title: notification.summary };
}

export function isSnoozed(notification: Notification, now: number) {
  return notification.snoozedUntil !== null && new Date(notification.snoozedUntil).getTime() > now;
}

export function matchesFilter(notification: Notification, filter: TInboxFilter, now: number) {
  if (filter === "archived") return notification.archivedAt !== null;
  if (notification.archivedAt !== null) return false;
  if (filter === "snoozed") return isSnoozed(notification, now);
  if (isSnoozed(notification, now)) return false;
  return filter === "inbox" || notification.readAt === null;
}

/**
 * The notifications of a filter, one group per issue, project, initiative or document, newest
 * first. `keep` stays listed in the unread filter once read, so opening it doesn't make it vanish.
 */
export function groupNotifications(
  notifications: Notification[],
  filter: TInboxFilter,
  now: number,
  keep?: string,
): TNotificationGroup[] {
  const groups = new Map<string, TNotificationGroup>();
  for (const notification of notifications) {
    const target = notificationTarget(notification);
    const key = `${target.kind}:${target.id}`;
    const kept = filter === "unread" && key === keep && matchesFilter(notification, "inbox", now);
    if (!kept && !matchesFilter(notification, filter, now)) continue;
    const group = groups.get(key);
    const unread = notification.readAt === null ? [notification.id] : [];
    if (!group) {
      groups.set(key, {
        key,
        target,
        notifications: [notification],
        latest: notification,
        unreadIds: unread,
      });
      continue;
    }
    group.notifications.push(notification);
    group.unreadIds.push(...unread);
  }
  return [...groups.values()].sort((a, b) => b.latest.createdAt.localeCompare(a.latest.createdAt));
}

const commentSentences: Record<string, string> = {
  NewComment: "commented",
  CommentReply: "replied in a thread you're in",
  CommentMention: "mentioned you in a comment",
  UpdateNewComment: "commented on an update",
  UpdateCommentReply: "replied in a thread on an update",
  UpdateCommentMention: "mentioned you in a comment on an update",
};

/** With someone who did it, and without (the inbox's own reminders). */
const sentences: Record<string, [string, string]> = {
  issueAssignedToYou: ["assigned it to you", "Assigned to you"],
  issueUnassignedFromYou: ["unassigned you", "Unassigned from you"],
  issueMention: ["mentioned you", "You were mentioned"],
  issueUnblocked: ["unblocked it", "No longer blocked"],
  issueShared: ["shared it with you", "Shared with you"],
  triageIssue: ["added it to triage", "Needs triage"],
  projectAssigned: ["made you the lead", "You are now the lead"],
  projectMention: ["mentioned you", "You were mentioned"],
  documentMention: ["mentioned you", "You were mentioned"],
  projectUpdateCreated: ["posted an update", "New update"],
  initiativeUpdateCreated: ["posted an update", "New update"],
  projectUpdateMention: ["mentioned you in an update", "You were mentioned in an update"],
};

/** What happened, said about the target the row already names: "moved it to Done". */
export function notificationSentence(notification: Notification): {
  actor: string | null;
  text: string;
} {
  const actor = notification.actor?.name || notification.actor?.displayName || null;
  const { type, summary } = notification;
  const say = (withActor: string, alone: string) =>
    actor ? { actor, text: withActor } : { actor: null, text: alone };

  const comment = type.match(/^(?:issue|project|initiative|document)(\w*Comment\w*)$/);
  if (comment && commentSentences[comment[1]]) {
    const text = commentSentences[comment[1]];
    return say(text, text.charAt(0).toUpperCase() + text.slice(1));
  }
  if (type === "issueStatusChanged") {
    const state = summary.split(" moved to ")[1];
    return state
      ? say(`moved it to ${state}`, `Moved to ${state}`)
      : say("changed the status", "Status changed");
  }
  if (type === "issueDue") {
    return { actor: null, text: summary.endsWith("overdue") ? "Overdue" : "Due today" };
  }
  const sentence = sentences[type];
  if (sentence) return say(...sentence);
  return { actor: null, text: summary };
}

/** A comment's opening words as one line of plain text. */
export function commentExcerpt(body: string) {
  return body
    .replace(/!\[[^\]]*\]\([^)]*\)/g, "")
    .replace(/\[([^\]]*)\]\([^)]*\)/g, "$1")
    .replace(/^\s*(?:#+|>|[-*+]|\d+\.)\s+/gm, "")
    .replace(/[*_`~]/g, "")
    .replace(/\s+/g, " ")
    .trim();
}

/** Where a notification leads: its issue (at the comment), project, initiative or document. */
export function notificationLink(notification: Notification, workspace: string): LinkProps | null {
  const target = notificationTarget(notification);
  const hash = notification.comment
    ? `comment-${notification.comment.id}`
    : notification.statusUpdate
      ? `update-${notification.statusUpdate.id}`
      : undefined;
  switch (target.kind) {
    case "issue":
      return {
        to: "/$workspace/issue/$identifier",
        params: { workspace, identifier: target.identifier },
        hash,
      };
    case "project":
      return { to: "/$workspace/project/$slug", params: { workspace, slug: target.slugId }, hash };
    case "initiative":
      return {
        to: "/$workspace/initiative/$slug",
        params: { workspace, slug: target.slugId },
        hash,
      };
    case "document":
      return { to: "/$workspace/document/$slug", params: { workspace, slug: target.slugId }, hash };
    case "none":
      return null;
  }
}

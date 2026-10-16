import type { HistoryEntry } from "@/api/ops.gen";

const PRIORITY_NAMES: Record<string, string> = {
  "0": "No priority",
  "1": "Urgent",
  "2": "High",
  "3": "Medium",
  "4": "Low",
};

const FIELD_NAMES: Record<string, string> = {
  project: "project",
  projectMilestone: "milestone",
  cycle: "cycle",
  parent: "parent issue",
  dueDate: "due date",
  estimate: "estimate",
};

type TRef = { name?: string | null; identifier?: string; number?: number; title?: string };

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/** A changed value as text: refs by their name (or identifier), dates and numbers as they are. */
function text(value: unknown, field: string): string {
  if (value === null || value === undefined || value === "") return "";
  if (field === "priority") return PRIORITY_NAMES[String(value)] ?? String(value);
  if (Array.isArray(value)) return value.map((item) => text(item, field)).join(", ");
  if (!isRecord(value)) return String(value);
  const ref = value as TRef;
  if (ref.identifier) return ref.identifier;
  if (ref.name) return ref.name;
  if (ref.number !== undefined) return `Cycle ${ref.number}`;
  return "";
}

function fromTo(change: unknown, field: string) {
  if (!isRecord(change)) return { from: "", to: text(change, field) };
  return { from: text(change.from, field), to: text(change.to, field) };
}

const names = (value: unknown) =>
  Array.isArray(value) ? value.filter((item): item is string => typeof item === "string") : [];

function describeChange(field: string, change: unknown): string | null {
  const { from, to } = fromTo(change, field);
  switch (field) {
    case "github":
    case "identifier":
      return null;
    case "title":
      return `changed the title to "${to}"`;
    case "description":
      return "updated the description";
    case "state":
      return from ? `changed status from ${from} to ${to}` : `set status to ${to}`;
    case "priority":
      return from ? `changed priority from ${from} to ${to}` : `set priority to ${to}`;
    case "assignee":
      if (!to) return from ? `unassigned ${from}` : "removed the assignee";
      return `assigned the issue to ${to}`;
    case "labels": {
      if (!isRecord(change)) return "changed the labels";
      const before = names(change.from);
      const after = names(change.to);
      const added = after.filter((name) => !before.includes(name));
      const removed = before.filter((name) => !after.includes(name));
      const parts = [
        added.length > 0
          ? `added ${added.length === 1 ? "label" : "labels"} ${added.join(", ")}`
          : "",
        removed.length > 0
          ? `removed ${removed.length === 1 ? "label" : "labels"} ${removed.join(", ")}`
          : "",
      ].filter(Boolean);
      return parts.join(" and ") || "changed the labels";
    }
    case "added_blocks":
      return `marked it as blocking ${to}`;
    case "added_blockedBy":
      return `marked it as blocked by ${to}`;
    case "added_relatedTo":
      return `related it to ${to}`;
    case "duplicateOf":
      return `marked it as a duplicate of ${to}`;
    case "team":
      return `moved the issue from ${from} to ${to}`;
    case "snoozedUntil":
      return to ? `snoozed the issue until ${to.slice(0, 10)}` : "woke the issue up";
    case "slaBreachesAt":
      return to ? `set the SLA to breach on ${to.slice(0, 10)}` : "removed the SLA";
    default: {
      const name = FIELD_NAMES[field] ?? field;
      if (!to) return `removed the ${name}`;
      return `set ${name} to ${to}`;
    }
  }
}

const actionLines: Record<string, string> = {
  archived: "archived the issue",
  auto_archived: "archived the issue",
  unarchived: "unarchived the issue",
  trashed: "moved the issue to the trash",
  restored: "restored the issue",
  shared: "shared the issue",
};

/** What a history entry says, one line per change. */
export function describeEntry(entry: Pick<HistoryEntry, "action" | "changes">): string[] {
  const { action, changes } = entry;
  if (action === "created") {
    return [changes.importedBy ? "imported the issue" : "created the issue"];
  }
  if (actionLines[action]) return [actionLines[action]];
  const lines = Object.entries(changes)
    .map(([field, change]) => describeChange(field, change))
    .filter((line): line is string => Boolean(line));
  const github = isRecord(changes.github) ? changes.github : null;
  if (lines.length === 0 && github && typeof github.label === "string") return [github.label];
  if (action === "auto_closed") return lines.map((line) => `${line} (closed for inactivity)`);
  if (action === "cycle_rollover") return lines.map((line) => `${line} (rolled over)`);
  return lines;
}

export type THistoryActor =
  { kind: "user"; name: string } | { kind: "github" } | { kind: "ricrac" };

/** Who made a change: a person, GitHub (its webhooks), or Ricrac's automations. */
export function historyActorOf(entry: Pick<HistoryEntry, "actor" | "changes">): THistoryActor {
  if (entry.actor) return { kind: "user", name: entry.actor.name || entry.actor.displayName };
  if (entry.changes.github) return { kind: "github" };
  return { kind: "ricrac" };
}

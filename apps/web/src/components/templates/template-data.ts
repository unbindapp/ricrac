// What the template editor edits, to and from a template's data (save_template in ops/templates.go).
// Keys the editor doesn't show (estimate, cycle, dueDate, ...) are kept as they are.

export type TTemplateKind = "issue" | "project" | "document";
type TData = Record<string, unknown>;

export type TSubIssueDraft = { title: string; priority: string };
export type TMilestoneDraft = { name: string; targetDate: string };
export type TProjectIssueDraft = { title: string; milestone: string };

export type TTemplateDraft = {
  title: string;
  description: string;
  priority: string;
  assignee: string;
  labels: string[];
  state: string;
  subIssues: TSubIssueDraft[];
  summary: string;
  lead: string;
  teams: string[];
  milestones: TMilestoneDraft[];
  issues: TProjectIssueDraft[];
};

const text = (value: unknown) => (typeof value === "string" ? value : "");
const list = (value: unknown) => (Array.isArray(value) ? value : []);
const objects = (value: unknown) =>
  list(value).filter((item): item is TData => typeof item === "object" && item !== null);
const priority = (value: unknown) => (typeof value === "number" ? String(value) : "0");

export function draftFromData(kind: TTemplateKind, data: TData): TTemplateDraft {
  return {
    title: text(kind === "project" ? data.name : data.title),
    description: text(kind === "document" ? data.content : data.description),
    priority: priority(data.priority),
    assignee: text(kind === "project" ? data.lead : data.assignee),
    labels: list(data.labels).map(String),
    state: text(data.state),
    subIssues: objects(data.subIssues).map((issue) => ({
      title: text(issue.title),
      priority: priority(issue.priority),
    })),
    summary: text(data.summary),
    lead: text(data.lead),
    teams: list(data.teams).map(String),
    milestones: objects(data.milestones).map((milestone) => ({
      name: text(milestone.name),
      targetDate: text(milestone.targetDate),
    })),
    issues: objects(data.issues).map((issue) => ({
      title: text(issue.title),
      milestone: text(issue.milestone),
    })),
  };
}

/** Sets key to value, or leaves it out when empty. */
function put(data: TData, key: string, value: unknown) {
  const isEmpty =
    value === "" || value === undefined || (Array.isArray(value) && value.length === 0);
  if (isEmpty) delete data[key];
  else data[key] = value;
}

/** Keeps the fields of the original list items the editor doesn't show, by position. */
function merge(original: unknown, items: TData[]) {
  const before = objects(original);
  return items.map((item, index) => {
    const merged: TData = { ...before[index] };
    for (const [key, value] of Object.entries(item)) put(merged, key, value);
    return merged;
  });
}

export function dataFromDraft(kind: TTemplateKind, draft: TTemplateDraft, original: TData): TData {
  const data: TData = { ...original };
  const priorityValue = Number(draft.priority) || undefined;
  const title = draft.title.trim() ? draft.title : "";
  if (kind === "document") {
    put(data, "title", title);
    put(data, "content", draft.description);
    return data;
  }
  if (kind === "project") {
    put(data, "name", title);
    put(data, "summary", draft.summary.trim());
    put(data, "description", draft.description);
    put(data, "priority", priorityValue);
    put(data, "lead", draft.lead);
    put(data, "teams", draft.teams);
    put(
      data,
      "milestones",
      merge(
        original.milestones,
        draft.milestones
          .filter((milestone) => milestone.name.trim())
          .map((milestone) => ({ name: milestone.name.trim(), targetDate: milestone.targetDate })),
      ),
    );
    put(
      data,
      "issues",
      merge(
        original.issues,
        draft.issues
          .filter((issue) => issue.title.trim())
          .map((issue) => ({ title: issue.title.trim(), milestone: issue.milestone })),
      ),
    );
    return data;
  }
  put(data, "title", title);
  put(data, "description", draft.description);
  put(data, "priority", priorityValue);
  put(data, "assignee", draft.assignee);
  put(data, "labels", draft.labels);
  put(data, "state", draft.state);
  put(
    data,
    "subIssues",
    merge(
      original.subIssues,
      draft.subIssues
        .filter((issue) => issue.title.trim())
        .map((issue) => ({
          title: issue.title.trim(),
          priority: Number(issue.priority) || undefined,
        })),
    ),
  );
  return data;
}

export type TRecurrenceDraft = { enabled: boolean; every: string; unit: string; startsAt: string };

/** "Every week", "Every 2 months". */
export function recurrenceText(recurrence: { every: number; unit: string }) {
  if (recurrence.every === 1) return `Every ${recurrence.unit}`;
  return `Every ${recurrence.every} ${recurrence.unit}s`;
}

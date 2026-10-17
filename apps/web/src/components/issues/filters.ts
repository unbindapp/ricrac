import { z } from "zod";

import type { ListIssuesArgs } from "@/api/ops.gen";
import { serverOrderBy, type TDisplaySettings } from "./display.ts";
import type { TStateType } from "@/components/shared/state-icon";

/** Stands for "no assignee" or "no project" among the ids. */
export const UNASSIGNED = "null";

/** What a list shows, as the filter bar edits it; `issueFilterOf` turns it into Ricrac's filter language. */
export type ListIssuesFilters = {
  team_ids?: string[];
  state_ids?: string[];
  state_types?: TStateType[];
  assignee_ids?: string[];
  creator_ids?: string[];
  label_ids?: string[];
  project_ids?: string[];
  milestone_ids?: string[];
  cycle_ids?: string[];
  parent_id?: string;
  priorities?: number[];
  due_before?: string;
  due_after?: string;
  subscriber_id?: string;
  query?: string;
  /** Leave out snoozed issues, as triage does, or show only them. */
  snoozed?: "hide" | "only";
};

export type TFilterField =
  | "status"
  | "assignee"
  | "creator"
  | "priority"
  | "labels"
  | "project"
  | "cycle"
  | "due_date"
  | "subscriber";

export const FILTER_FIELDS: { field: TFilterField; label: string }[] = [
  { field: "status", label: "Status" },
  { field: "assignee", label: "Assignee" },
  { field: "creator", label: "Creator" },
  { field: "priority", label: "Priority" },
  { field: "labels", label: "Labels" },
  { field: "project", label: "Project" },
  { field: "cycle", label: "Cycle" },
  { field: "due_date", label: "Due date" },
  { field: "subscriber", label: "Subscriber" },
];

type TListParam =
  "state_ids" | "assignee_ids" | "creator_ids" | "label_ids" | "project_ids" | "cycle_ids";

export const listParamOfField: Partial<Record<TFilterField, TListParam>> = {
  status: "state_ids",
  assignee: "assignee_ids",
  creator: "creator_ids",
  labels: "label_ids",
  project: "project_ids",
  cycle: "cycle_ids",
};

export const STATE_TYPE_VALUES = [
  "triage",
  "backlog",
  "unstarted",
  "started",
  "completed",
  "canceled",
] as const satisfies readonly TStateType[];

const CLOSED_TYPES: TStateType[] = ["completed", "canceled"];

const stringList = z.array(z.string()).optional().catch(undefined);
const optionalString = z.string().optional().catch(undefined);
const datePattern = /^\d{4}-\d{2}-\d{2}$/;
const optionalDate = z.string().regex(datePattern).optional().catch(undefined);

export const issueSearchSchema = z.object({
  state_ids: stringList,
  state_types: z.array(z.enum(STATE_TYPE_VALUES)).optional().catch(undefined),
  assignee_ids: stringList,
  creator_ids: stringList,
  label_ids: stringList,
  project_ids: stringList,
  cycle_ids: stringList,
  priorities: z.array(z.number().int().min(0).max(4)).optional().catch(undefined),
  due_before: optionalDate,
  due_after: optionalDate,
  subscriber_id: optionalString,
  peek: optionalString,
});

export type TIssueSearch = z.infer<typeof issueSearchSchema>;

export function compactFilters<T extends object>(filters: T): T {
  const entries = Object.entries(filters).filter(([, value]) => {
    if (value === undefined || value === null || value === "" || value === false) return false;
    if (Array.isArray(value)) return value.length > 0;
    return true;
  });
  return Object.fromEntries(entries) as T;
}

export function filtersFromSearch(search: TIssueSearch): ListIssuesFilters {
  const filters: TIssueSearch = { ...search, peek: undefined };
  return compactFilters(filters);
}

/** Search params for the filters: every filter key is set so removed filters leave the URL. */
export function filtersToSearch(filters: ListIssuesFilters): Partial<TIssueSearch> {
  const clean = compactFilters(filters);
  return {
    state_ids: clean.state_ids,
    state_types: clean.state_types,
    assignee_ids: clean.assignee_ids,
    creator_ids: clean.creator_ids,
    label_ids: clean.label_ids,
    project_ids: clean.project_ids,
    cycle_ids: clean.cycle_ids,
    priorities: clean.priorities,
    due_before: clean.due_before,
    due_after: clean.due_after,
    subscriber_id: clean.subscriber_id,
  };
}

export function activeFilterFields(filters: ListIssuesFilters): TFilterField[] {
  const clean = compactFilters(filters);
  return FILTER_FIELDS.map((item) => item.field).filter((field) => {
    switch (field) {
      case "status":
        return Boolean(clean.state_ids || clean.state_types);
      case "priority":
        return Boolean(clean.priorities);
      case "due_date":
        return Boolean(clean.due_before || clean.due_after);
      case "subscriber":
        return Boolean(clean.subscriber_id);
      default: {
        const param = listParamOfField[field];
        return Boolean(param && clean[param]);
      }
    }
  });
}

export function clearFilterField(filters: ListIssuesFilters, field: TFilterField) {
  const next = { ...filters };
  switch (field) {
    case "status":
      delete next.state_ids;
      delete next.state_types;
      break;
    case "priority":
      delete next.priorities;
      break;
    case "due_date":
      delete next.due_before;
      delete next.due_after;
      break;
    case "subscriber":
      delete next.subscriber_id;
      break;
    default: {
      const param = listParamOfField[field];
      if (param) delete next[param];
    }
  }
  return compactFilters(next);
}

function intersect<T>(a: T[] | undefined, b: T[] | undefined): T[] | undefined {
  if (!a) return b;
  if (!b) return a;
  return a.filter((value) => b.includes(value));
}

/** A page's own filters narrow what the person picked; they never widen it. */
export function mergeFilters(
  filters: ListIssuesFilters,
  fixed: ListIssuesFilters | undefined,
): ListIssuesFilters {
  if (!fixed) return compactFilters(filters);
  const merged: ListIssuesFilters = { ...filters, ...fixed };
  const listKeys = [
    "team_ids",
    "state_ids",
    "state_types",
    "assignee_ids",
    "creator_ids",
    "label_ids",
    "project_ids",
    "milestone_ids",
    "cycle_ids",
  ] as const;
  for (const key of listKeys) {
    (merged as Record<string, unknown>)[key] = intersect<string>(filters[key], fixed[key]);
  }
  merged.priorities = intersect(filters.priorities, fixed.priorities);
  return compactFilters(merged);
}

type TCondition = Record<string, unknown>;

const anyOf = (values: string[]) => ({ in: values });

/** The ids, with "null" standing for "not set" (no assignee, no project). */
function nullableIds(field: string, values: string[]): TCondition {
  const ids = values.filter((value) => value !== UNASSIGNED);
  const conditions: TCondition[] = [];
  if (values.includes(UNASSIGNED)) conditions.push({ [field]: { null: true } });
  if (ids.length > 0) conditions.push({ [field]: { id: anyOf(ids) } });
  return conditions.length === 1 ? conditions[0] : { or: conditions };
}

/** The filters in Ricrac's issue filter language (what list_issues and custom views take). */
export function issueFilterOf(filters: ListIssuesFilters): TCondition | undefined {
  const clean = compactFilters(filters);
  const parts: TCondition[] = [];
  if (clean.team_ids) parts.push({ team: { id: anyOf(clean.team_ids) } });
  if (clean.state_ids) parts.push({ state: { id: anyOf(clean.state_ids) } });
  if (clean.state_types) parts.push({ state: { type: anyOf(clean.state_types) } });
  if (clean.assignee_ids) parts.push(nullableIds("assignee", clean.assignee_ids));
  if (clean.creator_ids) parts.push({ creator: { id: anyOf(clean.creator_ids) } });
  if (clean.label_ids) parts.push({ labels: { some: { id: anyOf(clean.label_ids) } } });
  if (clean.project_ids) parts.push(nullableIds("project", clean.project_ids));
  if (clean.milestone_ids) parts.push({ projectMilestone: { id: anyOf(clean.milestone_ids) } });
  if (clean.cycle_ids) parts.push(nullableIds("cycle", clean.cycle_ids));
  if (clean.parent_id) parts.push({ parent: { id: { eq: clean.parent_id } } });
  if (clean.priorities) parts.push({ priority: { in: clean.priorities } });
  if (clean.due_before) parts.push({ dueDate: { lte: clean.due_before } });
  if (clean.due_after) parts.push({ dueDate: { gte: clean.due_after } });
  if (clean.subscriber_id) {
    parts.push({ subscribers: { some: { id: { eq: clean.subscriber_id } } } });
  }
  if (clean.query) parts.push({ searchableContent: { contains: clean.query } });
  if (clean.snoozed === "hide") {
    parts.push({ or: [{ snoozedUntil: { null: true } }, { snoozedUntil: { lt: "PT1S" } }] });
  }
  if (clean.snoozed === "only") parts.push({ snoozedUntil: { gt: "PT1S" } });
  if (parts.length === 0) return undefined;
  return parts.length === 1 ? parts[0] : { and: parts };
}

function stateTypesForDisplay(
  current: TStateType[] | undefined,
  showCompleted: TDisplaySettings["show_completed"],
): TStateType[] | undefined {
  if (showCompleted !== "none") return current;
  const base = current ?? [...STATE_TYPE_VALUES];
  return base.filter((type) => !CLOSED_TYPES.includes(type));
}

export function buildListIssuesArgs({
  filters,
  fixedFilters,
  display,
}: {
  filters: ListIssuesFilters;
  fixedFilters?: ListIssuesFilters;
  display: Pick<TDisplaySettings, "order_by" | "show_completed">;
}): ListIssuesArgs {
  const merged = mergeFilters(filters, fixedFilters);
  const { team_ids: teams, ...rest } = merged;
  const conditions = [
    issueFilterOf({
      ...rest,
      team_ids: teams && teams.length > 1 ? teams : undefined,
      state_types: stateTypesForDisplay(merged.state_types, display.show_completed),
    }),
    display.show_completed === "past_week" ? closedThisWeek : undefined,
  ].filter((condition) => condition !== undefined);
  return compactFilters({
    team: teams?.length === 1 ? teams[0] : undefined,
    orderBy: serverOrderBy[display.order_by],
    filter: conditions.length > 1 ? { and: conditions } : conditions[0],
  });
}

const closedThisWeek: TCondition = {
  or: [
    { state: { type: { nin: CLOSED_TYPES } } },
    { completedAt: { gte: "-P7D" } },
    { canceledAt: { gte: "-P7D" } },
  ],
};

const WEEK_MS = 7 * 24 * 60 * 60 * 1000;

/** Closed issues past the "completed issues" window are hidden even though they are loaded. */
export function isHiddenCompleted(
  issue: { state: { type: TStateType }; completedAt: string | null; canceledAt: string | null },
  showCompleted: TDisplaySettings["show_completed"],
  now: number,
) {
  if (!CLOSED_TYPES.includes(issue.state.type)) return false;
  switch (showCompleted) {
    case "all":
      return false;
    case "none":
      return true;
    case "past_week": {
      const closedAt = issue.completedAt ?? issue.canceledAt;
      if (!closedAt) return false;
      return now - new Date(closedAt).getTime() > WEEK_MS;
    }
  }
}

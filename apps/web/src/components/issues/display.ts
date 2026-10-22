export type TIssueLayout = "list" | "board";
export type TShowCompleted = "all" | "past_week" | "none";
export type TGroupBy =
  "state" | "assignee" | "priority" | "project" | "cycle" | "label" | "parent" | "none";
export type TOrderBy = "manual" | "priority" | "updated_at" | "created_at" | "due_date" | "title";
export type TIssueProperty =
  | "priority"
  | "identifier"
  | "state"
  | "labels"
  | "project"
  | "cycle"
  | "estimate"
  | "due_date"
  | "assignee"
  | "updated_at";

export type TDisplaySettings = {
  layout: TIssueLayout;
  group_by: TGroupBy;
  order_by: TOrderBy;
  order: "asc" | "desc";
  nest_sub_issues: boolean;
  show_completed: TShowCompleted;
  properties: TIssueProperty[];
};

export type DisplaySettings = TDisplaySettings;

export const ISSUE_PROPERTIES: { value: TIssueProperty; label: string }[] = [
  { value: "priority", label: "Priority" },
  { value: "identifier", label: "ID" },
  { value: "state", label: "Status" },
  { value: "labels", label: "Labels" },
  { value: "project", label: "Project" },
  { value: "cycle", label: "Cycle" },
  { value: "estimate", label: "Estimate" },
  { value: "due_date", label: "Due date" },
  { value: "assignee", label: "Assignee" },
  { value: "updated_at", label: "Updated" },
];

export const GROUP_BY_OPTIONS: { value: TGroupBy; label: string }[] = [
  { value: "state", label: "Status" },
  { value: "assignee", label: "Assignee" },
  { value: "priority", label: "Priority" },
  { value: "project", label: "Project" },
  { value: "cycle", label: "Cycle" },
  { value: "label", label: "Label" },
  { value: "parent", label: "Parent issue" },
  { value: "none", label: "No grouping" },
];

export const ORDER_BY_OPTIONS: { value: TOrderBy; label: string }[] = [
  { value: "manual", label: "Manual" },
  { value: "priority", label: "Priority" },
  { value: "updated_at", label: "Last updated" },
  { value: "created_at", label: "Last created" },
  { value: "due_date", label: "Due date" },
  { value: "title", label: "Title" },
];

export const SHOW_COMPLETED_OPTIONS: { value: TShowCompleted; label: string }[] = [
  { value: "all", label: "All" },
  { value: "past_week", label: "Past week" },
  { value: "none", label: "None" },
];

export const DEFAULT_DISPLAY: TDisplaySettings = {
  layout: "list",
  group_by: "state",
  order_by: "manual",
  order: "asc",
  nest_sub_issues: true,
  show_completed: "past_week",
  properties: ["priority", "identifier", "state", "labels", "project", "due_date", "assignee"],
};

/** The order the server loads issues in, so a list cut at its limit keeps the first ones. */
export const serverOrderBy: Record<TOrderBy, string> = {
  manual: "manual",
  priority: "priority",
  updated_at: "updatedAt",
  created_at: "createdAt",
  due_date: "dueDate",
  title: "updatedAt",
};

export function defaultOrderFor(orderBy: TOrderBy): "asc" | "desc" {
  switch (orderBy) {
    case "created_at":
    case "updated_at":
      return "desc";
    default:
      return "asc";
  }
}

const isGroupBy = (value: unknown): value is TGroupBy =>
  GROUP_BY_OPTIONS.some((option) => option.value === value);
const isOrderBy = (value: unknown): value is TOrderBy =>
  ORDER_BY_OPTIONS.some((option) => option.value === value);

/** Layers display settings over the defaults, dropping values this version doesn't know. */
export function resolveDisplay(
  ...layers: (Partial<TDisplaySettings> | undefined)[]
): TDisplaySettings {
  const merged = layers.reduce<TDisplaySettings>(
    (acc, layer) => ({ ...acc, ...layer }),
    DEFAULT_DISPLAY,
  );
  return {
    ...merged,
    group_by: isGroupBy(merged.group_by) ? merged.group_by : DEFAULT_DISPLAY.group_by,
    order_by: isOrderBy(merged.order_by) ? merged.order_by : DEFAULT_DISPLAY.order_by,
  };
}

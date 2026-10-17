import type {
  CycleRef,
  Issue,
  IssueRef,
  LabelRef,
  ProjectRef,
  StateRef,
  TeamRef,
  UserRef,
} from "@/api/ops.gen";
import type { TGroupBy, TIssueLayout, TOrderBy } from "./display.ts";
import type { TStateType } from "@/components/shared/state-icon";

export const STATE_TYPE_ORDER: TStateType[] = [
  "triage",
  "backlog",
  "unstarted",
  "started",
  "completed",
  "canceled",
];

/** Lists put the work closest to done on top; boards read left to right through the workflow. */
const LIST_STATE_TYPE_ORDER: TStateType[] = [
  "triage",
  "started",
  "unstarted",
  "backlog",
  "completed",
  "canceled",
];

export const PRIORITY_ORDER = [1, 2, 3, 4, 0];

const PRIORITY_NAMES: Record<number, string> = {
  0: "No priority",
  1: "Urgent",
  2: "High",
  3: "Medium",
  4: "Low",
};

export type TGroupMeta =
  | { kind: "none" }
  | { kind: "state"; state: StateRef; teamId: string | undefined }
  | { kind: "priority"; priority: number }
  | { kind: "assignee"; user: UserRef | null }
  | { kind: "project"; project: ProjectRef | null }
  | { kind: "cycle"; cycle: CycleRef | null }
  | { kind: "label"; label: LabelRef | null }
  | { kind: "parent"; parent: IssueRef | null }
  | { kind: "team"; team: TeamRef };

export type TIssueGroup = {
  key: string;
  label: string;
  meta: TGroupMeta;
  issues: Issue[];
};

export const NONE_KEY = "none";

function compareNullable(a: string | null, b: string | null) {
  if (a === b) return 0;
  if (!a) return 1;
  if (!b) return -1;
  return a.localeCompare(b);
}

/** Ties fall back to the newest first, so equal issues keep a stable order. */
export function compareIssues(orderBy: TOrderBy, order: "asc" | "desc") {
  const direction = order === "asc" ? 1 : -1;
  const compare = (a: Issue, b: Issue) => {
    switch (orderBy) {
      case "manual":
        return a.sortOrder - b.sortOrder;
      case "priority":
        return PRIORITY_ORDER.indexOf(a.priority) - PRIORITY_ORDER.indexOf(b.priority);
      case "created_at":
        return a.createdAt.localeCompare(b.createdAt);
      case "updated_at":
        return a.updatedAt.localeCompare(b.updatedAt);
      case "due_date":
        return compareNullable(a.dueDate, b.dueDate);
      case "title":
        return a.title.localeCompare(b.title);
    }
  };
  return (a: Issue, b: Issue) =>
    direction * compare(a, b) || b.createdAt.localeCompare(a.createdAt);
}

export const userName = (user: Pick<UserRef, "name" | "displayName">) =>
  user.name || user.displayName;

export const cycleName = (cycle: Pick<CycleRef, "name" | "number">) =>
  cycle.name || `Cycle ${cycle.number}`;

type TGroupSeed = { key: string; label: string; meta: TGroupMeta; rank: number | string };

function stateRank(state: StateRef, positions: Map<string, number>, layout: TIssueLayout) {
  const position = positions.get(state.id) ?? 9999;
  if (layout === "board") return STATE_TYPE_ORDER.indexOf(state.type) * 10000 + position;
  const isClosed = state.type === "completed" || state.type === "canceled";
  return LIST_STATE_TYPE_ORDER.indexOf(state.type) * 10000 + (isClosed ? position : -position);
}

/** States group by id within one team; across teams, same-named states of a type share a group. */
function stateSeed(
  state: StateRef,
  teamId: string | undefined,
  positions: Map<string, number>,
  layout: TIssueLayout,
) {
  const byId = positions.size > 0;
  const typeOrder = layout === "board" ? STATE_TYPE_ORDER : LIST_STATE_TYPE_ORDER;
  return {
    key: byId ? state.id : `${state.type}:${state.name.toLowerCase()}`,
    label: state.name,
    meta: { kind: "state", state, teamId } as const,
    rank: byId
      ? stateRank(state, positions, layout)
      : `${typeOrder.indexOf(state.type)}${state.name}`,
  };
}

function seedsFor(
  issue: Issue,
  groupBy: TGroupBy,
  positions: Map<string, number>,
  layout: TIssueLayout,
): TGroupSeed[] {
  switch (groupBy) {
    case "none":
      return [{ key: "all", label: "All issues", meta: { kind: "none" }, rank: 0 }];
    case "state":
      return [
        stateSeed(issue.state, positions.size > 0 ? issue.team.id : undefined, positions, layout),
      ];
    case "priority":
      return [
        {
          key: String(issue.priority),
          label: PRIORITY_NAMES[issue.priority] ?? PRIORITY_NAMES[0],
          meta: { kind: "priority", priority: issue.priority },
          rank: PRIORITY_ORDER.indexOf(issue.priority),
        },
      ];
    case "assignee": {
      const user = issue.assignee;
      return [
        {
          key: user?.id ?? NONE_KEY,
          label: user ? userName(user) : "Unassigned",
          meta: { kind: "assignee", user },
          rank: user ? `0${userName(user).toLowerCase()}` : "1",
        },
      ];
    }
    case "project": {
      const project = issue.project;
      return [
        {
          key: project?.id ?? NONE_KEY,
          label: project?.name ?? "No project",
          meta: { kind: "project", project },
          rank: project ? `0${project.name.toLowerCase()}` : "1",
        },
      ];
    }
    case "cycle": {
      const cycle = issue.cycle;
      return [
        {
          key: cycle?.id ?? NONE_KEY,
          label: cycle ? cycleName(cycle) : "No cycle",
          meta: { kind: "cycle", cycle },
          rank: cycle ? `0${cycle.startsAt}` : "1",
        },
      ];
    }
    case "label": {
      if (issue.labels.length === 0) {
        return [
          { key: NONE_KEY, label: "No label", meta: { kind: "label", label: null }, rank: "1" },
        ];
      }
      return issue.labels.map((label) => ({
        key: label.id,
        label: label.parent ? `${label.parent} / ${label.name}` : label.name,
        meta: { kind: "label", label },
        rank: `0${(label.parent ?? "").toLowerCase()}${label.name.toLowerCase()}`,
      }));
    }
    case "parent": {
      const parent = issue.parent;
      return [
        {
          key: parent?.id ?? NONE_KEY,
          label: parent ? `${parent.identifier} ${parent.title}` : "No parent issue",
          meta: { kind: "parent", parent },
          rank: parent ? `0${parent.identifier}` : "1",
        },
      ];
    }
    case "team":
      return [
        {
          key: issue.team.id,
          label: issue.team.name,
          meta: { kind: "team", team: issue.team },
          rank: issue.team.name.toLowerCase(),
        },
      ];
  }
}

function compareRanks(a: number | string, b: number | string) {
  if (typeof a === "number" && typeof b === "number") return a - b;
  return String(a).localeCompare(String(b));
}

/**
 * Groups and sorts loaded issues. With one team's states, status groups follow its workflow
 * (open work closest to done first in lists) and empty statuses still get a (droppable) group.
 */
export function groupIssues({
  issues,
  groupBy,
  orderBy,
  order,
  layout,
  team,
}: {
  issues: Issue[];
  groupBy: TGroupBy;
  orderBy: TOrderBy;
  order: "asc" | "desc";
  layout: TIssueLayout;
  team?: { id: string; states: (StateRef & { position: number })[] };
}): TIssueGroup[] {
  const positions = new Map((team?.states ?? []).map((state, index) => [state.id, index] as const));
  const groups = new Map<string, { group: TIssueGroup; rank: number | string }>();
  const ensure = (seed: TGroupSeed) => {
    const existing = groups.get(seed.key);
    if (existing) return existing.group;
    const group: TIssueGroup = { key: seed.key, label: seed.label, meta: seed.meta, issues: [] };
    groups.set(seed.key, { group, rank: seed.rank });
    return group;
  };
  if (groupBy === "state" && team) {
    for (const state of team.states) ensure(stateSeed(state, team.id, positions, layout));
  }
  if (groupBy === "priority") {
    for (const priority of PRIORITY_ORDER) {
      ensure({
        key: String(priority),
        label: PRIORITY_NAMES[priority],
        meta: { kind: "priority", priority },
        rank: PRIORITY_ORDER.indexOf(priority),
      });
    }
  }
  for (const issue of issues) {
    for (const seed of seedsFor(issue, groupBy, positions, layout)) {
      ensure(seed).issues.push(issue);
    }
  }
  const compare = compareIssues(orderBy, order);
  return [...groups.values()]
    .sort((a, b) => compareRanks(a.rank, b.rank))
    .map(({ group }) => ({ ...group, issues: group.issues.sort(compare) }));
}

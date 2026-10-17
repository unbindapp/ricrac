import assert from "node:assert/strict";
import { test } from "node:test";

import { groupIssues } from "./grouping.ts";
import type { Issue, StateRef } from "@/api/ops.gen";

const todo: StateRef = { id: "s-todo", name: "Todo", type: "unstarted", color: "#e2e2e2" };
const doing: StateRef = { id: "s-doing", name: "In Progress", type: "started", color: "#f2c94c" };
const backlog: StateRef = { id: "s-backlog", name: "Backlog", type: "backlog", color: "#bec2c8" };
const team = { id: "t", key: "ENG", name: "Engineering" };

function issue(id: string, overrides: Partial<Issue> = {}): Issue {
  return {
    id,
    identifier: `ENG-${id}`,
    number: 1,
    team,
    title: id,
    priority: 0,
    priorityLabel: "No priority",
    estimate: null,
    state: todo,
    labels: [],
    assignee: null,
    creator: null,
    project: null,
    projectMilestone: null,
    cycle: null,
    parent: null,
    subIssueCount: 0,
    sortOrder: 0,
    subIssueSortOrder: null,
    dueDate: null,
    startedAt: null,
    completedAt: null,
    canceledAt: null,
    startedTriageAt: null,
    triagedAt: null,
    snoozedUntil: null,
    slaBreachesAt: null,
    slaStartedAt: null,
    slaType: null,
    archivedAt: null,
    autoArchivedAt: null,
    autoClosedAt: null,
    trashed: false,
    trashedAt: null,
    branchName: "",
    previousIdentifiers: [],
    url: "",
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

test("state groups follow the workflow order and keep empty team states", () => {
  const groups = groupIssues({
    issues: [issue("a", { state: doing }), issue("b")],
    groupBy: "state",
    orderBy: "manual",
    order: "asc",
    team: {
      id: "t",
      states: [
        { ...doing, position: 0 },
        { ...todo, position: 0 },
        { ...backlog, position: 0 },
      ],
    },
  });
  assert.deepEqual(
    groups.map((group) => [group.label, group.issues.length]),
    [
      ["Backlog", 0],
      ["Todo", 1],
      ["In Progress", 1],
    ],
  );
});

test("across teams, statuses of the same name and type share a group", () => {
  const otherTodo = { ...todo, id: "s-other-todo" };
  const groups = groupIssues({
    issues: [issue("a"), issue("b", { state: otherTodo }), issue("c", { state: doing })],
    groupBy: "state",
    orderBy: "manual",
    order: "asc",
  });
  assert.deepEqual(
    groups.map((group) => [group.label, group.issues.length]),
    [
      ["Todo", 2],
      ["In Progress", 1],
    ],
  );
});

test("issues inside a group are ordered by the display ordering", () => {
  const [group] = groupIssues({
    issues: [issue("a", { sortOrder: 3 }), issue("b", { sortOrder: 1 })],
    groupBy: "none",
    orderBy: "manual",
    order: "asc",
  });
  assert.deepEqual(
    group.issues.map((item) => item.id),
    ["b", "a"],
  );
});

test("priority groups put urgent first and no priority last", () => {
  const groups = groupIssues({
    issues: [issue("a", { priority: 0 }), issue("b", { priority: 1 })],
    groupBy: "priority",
    orderBy: "manual",
    order: "asc",
  });
  assert.deepEqual(
    groups.map((group) => group.label),
    ["Urgent", "High", "Medium", "Low", "No priority"],
  );
});

test("an issue with two labels shows up under both", () => {
  const groups = groupIssues({
    issues: [
      issue("a", {
        labels: [
          { id: "l1", name: "Bug", color: "#f00", parent: null },
          { id: "l2", name: "UI", color: "#0f0", parent: "Area" },
        ],
      }),
      issue("b"),
    ],
    groupBy: "label",
    orderBy: "manual",
    order: "asc",
  });
  assert.deepEqual(
    groups.map((group) => [group.label, group.issues.map((item) => item.id)]),
    [
      ["Area / UI", ["a"]],
      ["Bug", ["a"]],
      ["No label", ["b"]],
    ],
  );
});

test("unassigned issues group last", () => {
  const groups = groupIssues({
    issues: [
      issue("a"),
      issue("b", { assignee: { id: "u1", name: "Zed", displayName: "zed", email: "z@x.test" } }),
    ],
    groupBy: "assignee",
    orderBy: "manual",
    order: "asc",
  });
  assert.deepEqual(
    groups.map((group) => group.label),
    ["Zed", "Unassigned"],
  );
});

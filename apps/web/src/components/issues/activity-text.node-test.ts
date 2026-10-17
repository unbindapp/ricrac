import assert from "node:assert/strict";
import { test } from "node:test";

import { describeEntry, historyActorOf } from "./activity-text.ts";

const updated = (changes: Record<string, unknown>) => ({ action: "updated", changes });
const todo = { id: "s1", name: "Todo", type: "unstarted", color: "#e2e2e2" };
const doing = { id: "s2", name: "In Progress", type: "started", color: "#f2c94c" };
const ada = { id: "u1", name: "Ada", displayName: "ada", email: "ada@acme.test" };

test("state changes read with the state names", () => {
  assert.deepEqual(describeEntry(updated({ state: { from: todo, to: doing } })), [
    "changed status from Todo to In Progress",
  ]);
});

test("priority values turn into their names", () => {
  assert.deepEqual(describeEntry(updated({ priority: { from: 0, to: 1 } })), [
    "changed priority from No priority to Urgent",
  ]);
});

test("clearing the assignee reads as unassigning", () => {
  assert.deepEqual(describeEntry(updated({ assignee: { from: ada, to: null } })), [
    "unassigned Ada",
  ]);
  assert.deepEqual(describeEntry(updated({ assignee: { from: null, to: ada } })), [
    "assigned the issue to Ada",
  ]);
});

test("label changes read as the labels added and removed", () => {
  assert.deepEqual(
    describeEntry(updated({ labels: { from: ["Bug", "UI"], to: ["Bug", "Feature"] } })),
    ["added label Feature and removed label UI"],
  );
});

test("one entry with several changes reads as several lines", () => {
  assert.deepEqual(
    describeEntry(
      updated({
        dueDate: { from: null, to: "2026-03-01" },
        project: { from: { id: "p1", name: "Launch", slugId: "x" }, to: null },
        added_blocks: ["ENG-3"],
      }),
    ),
    ["set due date to 2026-03-01", "removed the project", "marked it as blocking ENG-3"],
  );
});

test("lifecycle actions and GitHub events read as one line", () => {
  assert.deepEqual(describeEntry({ action: "archived", changes: {} }), ["archived the issue"]);
  assert.deepEqual(describeEntry({ action: "created", changes: { importedBy: "u1" } }), [
    "imported the issue",
  ]);
  assert.deepEqual(
    describeEntry({ action: "github", changes: { github: { label: "PR #42 merged", url: "" } } }),
    ["PR #42 merged"],
  );
});

test("changes without a person come from GitHub or from Ricrac", () => {
  assert.deepEqual(historyActorOf({ actor: null, changes: { github: { label: "x" } } }), {
    kind: "github",
  });
  assert.deepEqual(historyActorOf({ actor: ada, changes: {} }), { kind: "user", name: "Ada" });
  assert.deepEqual(historyActorOf({ actor: null, changes: {} }), { kind: "ricrac" });
});

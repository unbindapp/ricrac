import assert from "node:assert/strict";
import { test } from "node:test";

import { conditionsToFilter, filterToConditions, NONE, type TCondition } from "./view-filter.ts";

test("conditions become an and of filter language clauses", () => {
  const conditions: TCondition[] = [
    { field: "team", op: "is", values: ["ENG", "DES"] },
    { field: "priority", op: "isNot", values: ["0", "4"] },
    { field: "labels", op: "is", values: ["label-1"] },
    { field: "assignee", op: "is", values: ["me"] },
  ];
  assert.deepEqual(conditionsToFilter(conditions), {
    and: [
      { team: { key: { in: ["ENG", "DES"] } } },
      { priority: { nin: [0, 4] } },
      { labels: { some: { id: { in: ["label-1"] } } } },
      { assignee: { id: { in: ["me"] } } },
    ],
  });
});

test("empty conditions are left out and no conditions is no filter", () => {
  assert.deepEqual(conditionsToFilter([{ field: "team", op: "is", values: [] }]), {});
});

test("is not keeps the empty ones; none picks them", () => {
  assert.deepEqual(conditionsToFilter([{ field: "assignee", op: "isNot", values: ["u1"] }]), {
    and: [{ or: [{ assignee: { null: true } }, { assignee: { id: { nin: ["u1"] } } }] }],
  });
  assert.deepEqual(conditionsToFilter([{ field: "assignee", op: "is", values: [NONE] }]), {
    and: [{ assignee: { null: true } }],
  });
  assert.deepEqual(conditionsToFilter([{ field: "cycle", op: "is", values: ["current"] }]), {
    and: [{ cycle: { isActive: { eq: true } } }],
  });
});

test("every condition the builder makes reads back the same", () => {
  const cases: TCondition[] = [
    { field: "team", op: "isNot", values: ["ENG"] },
    { field: "statusType", op: "is", values: ["started", "unstarted"] },
    { field: "status", op: "is", values: ["In Review"] },
    { field: "assignee", op: "is", values: ["u1", NONE] },
    { field: "assignee", op: "isNot", values: ["u1"] },
    { field: "assignee", op: "isNot", values: ["u1", NONE] },
    { field: "assignee", op: "isNot", values: [NONE] },
    { field: "creator", op: "is", values: ["me"] },
    { field: "priority", op: "is", values: ["1", "2"] },
    { field: "labels", op: "isNot", values: ["l1", "l2"] },
    { field: "project", op: "is", values: [NONE] },
    { field: "cycle", op: "isNot", values: ["current"] },
    { field: "dueDate", op: "is", values: ["overdue", "none"] },
  ];
  for (const condition of cases) {
    assert.deepEqual(
      filterToConditions(conditionsToFilter([condition]), "issue"),
      [condition],
      JSON.stringify(condition),
    );
  }
  const project: TCondition = { field: "projectStatus", op: "is", values: ["started"] };
  assert.deepEqual(filterToConditions(conditionsToFilter([project]), "project"), [project]);
});

test("filters the builder can't show come back as null", () => {
  assert.equal(filterToConditions({ title: { contains: "crash" } }, "issue"), null);
  assert.equal(filterToConditions({ or: [{ priority: { eq: 1 } }] }, "issue"), null);
  assert.deepEqual(filterToConditions({ team: { key: { in: ["ENG"] } } }, "issue"), [
    { field: "team", op: "is", values: ["ENG"] },
  ]);
  assert.deepEqual(filterToConditions({}, "issue"), []);
  assert.deepEqual(
    filterToConditions({ priority: { in: [1, 2] }, team: { key: { in: ["ENG"] } } }, "issue"),
    [
      { field: "priority", op: "is", values: ["1", "2"] },
      { field: "team", op: "is", values: ["ENG"] },
    ],
  );
});

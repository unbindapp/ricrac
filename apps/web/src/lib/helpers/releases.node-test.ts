import assert from "node:assert/strict";
import { test } from "node:test";

import { formatDay, groupByStage, releaseDateText, shortSha } from "./releases.ts";

const stages = [
  { id: "done", position: 2 },
  { id: "planned", position: 0 },
  { id: "started", position: 1 },
];

test("releases group by stage in pipeline order, empty stages left out", () => {
  const releases = [
    { id: "a", stage: { id: "done" } },
    { id: "b", stage: { id: "planned" } },
    { id: "c", stage: { id: "done" } },
  ];
  const groups = groupByStage(stages, releases);
  assert.deepEqual(
    groups.map((group) => [group.stage?.id ?? null, group.releases.map((r) => r.id)]),
    [
      ["planned", ["b"]],
      ["done", ["a", "c"]],
    ],
  );
});

test("releases without a known stage come last", () => {
  const groups = groupByStage(stages, [
    { id: "a", stage: null },
    { id: "b", stage: { id: "gone" } },
  ]);
  assert.equal(groups.length, 1);
  assert.equal(groups[0].stage, null);
  assert.deepEqual(
    groups[0].releases.map((r) => r.id),
    ["a", "b"],
  );
});

test("days read without shifting across time zones", () => {
  assert.equal(formatDay("2026-01-01"), "Jan 1, 2026");
});

test("the most telling date wins", () => {
  const none = { completedAt: null, canceledAt: null, targetDate: null, startedAt: null };
  assert.equal(releaseDateText(none), null);
  assert.equal(releaseDateText({ ...none, targetDate: "2026-03-04" }), "Target Mar 4");
  assert.equal(
    releaseDateText({ ...none, targetDate: "2026-03-04", completedAt: "2026-03-02T10:00:00Z" }),
    "Released Mar 2",
  );
});

test("commits shorten to seven characters", () => {
  assert.equal(shortSha("0123456789abcdef"), "0123456");
});

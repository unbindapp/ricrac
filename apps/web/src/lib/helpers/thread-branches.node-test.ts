import assert from "node:assert/strict";
import { test } from "node:test";

import { branchGraph } from "./thread-branches.ts";

const thread = (id: string, createdAt: string, resolvedAt: string | null = null) => ({
  id,
  createdAt: `2026-10-0${createdAt}T12:00:00Z`,
  resolvedAt: resolvedAt && `2026-10-0${resolvedAt}T12:00:00Z`,
});

const shape = (graph: ReturnType<typeof branchGraph>) =>
  graph.rows.map(
    (row) =>
      `${row.kind} ${row.thread.id}@${row.lane} [${row.through.map((through) => `${through.thread.id}@${through.lane}`).join(" ")}]`,
  );

test("no threads leave only the main line", () => {
  assert.deepEqual(branchGraph([]), { rows: [], open: [], width: 1 });
});

test("threads fork and merge in the order things happened", () => {
  const graph = branchGraph([thread("a", "1", "4"), thread("b", "2"), thread("c", "3", "5")]);
  assert.deepEqual(shape(graph), [
    "fork a@1 []",
    "fork b@2 [a@1]",
    "fork c@3 [a@1 b@2]",
    "merge a@1 [b@2 c@3]",
    "merge c@3 [b@2]",
  ]);
  assert.deepEqual(
    graph.open.map((open) => `${open.thread.id}@${open.lane}`),
    ["b@2"],
  );
  assert.equal(graph.width, 4);
});

test("a thread takes the lowest lane a merged one freed", () => {
  const graph = branchGraph([thread("a", "1", "2"), thread("b", "1"), thread("c", "3")]);
  assert.deepEqual(shape(graph), [
    "fork a@1 []",
    "fork b@2 [a@1]",
    "merge a@1 [b@2]",
    "fork c@1 [b@2]",
  ]);
  assert.equal(graph.width, 3);
});

test("a thread resolved before it started still forks first", () => {
  const graph = branchGraph([thread("a", "3", "1")]);
  assert.deepEqual(shape(graph), ["fork a@1 []", "merge a@1 []"]);
  assert.deepEqual(graph.open, []);
});

import assert from "node:assert/strict";
import { test } from "node:test";

import { descendantIds, nestInitiatives } from "./initiative-tree.ts";

const item = (id: string, parent?: string) => ({ id, parent: parent ? { id: parent } : null });

test("sub-initiatives follow their parent, one level deeper", () => {
  const nested = nestInitiatives([item("b", "a"), item("a"), item("c"), item("d", "b")]);
  assert.deepEqual(
    nested.map(({ item, depth }) => `${item.id}${depth}`),
    ["a0", "b1", "d2", "c0"],
  );
});

test("a sub-initiative whose parent isn't listed stands at the top level", () => {
  const nested = nestInitiatives([item("b", "elsewhere")]);
  assert.deepEqual(nested, [{ item: item("b", "elsewhere"), depth: 0 }]);
});

test("an initiative can't move under itself or below", () => {
  const items = [item("a"), item("b", "a"), item("c", "b"), item("d")];
  assert.deepEqual([...descendantIds(items, "a")].sort(), ["a", "b", "c"]);
});

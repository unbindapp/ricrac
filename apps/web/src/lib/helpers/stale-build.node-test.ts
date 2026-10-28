import assert from "node:assert/strict";
import { test } from "node:test";

import { shouldReloadForBuild } from "./stale-build.ts";

test("reloads the first time", () => {
  assert.equal(shouldReloadForBuild(null, 1_000_000), true);
});

test("doesn't reload again within ten seconds", () => {
  assert.equal(shouldReloadForBuild(1_000_000, 1_009_000), false);
});

test("reloads again once ten seconds have passed", () => {
  assert.equal(shouldReloadForBuild(1_000_000, 1_010_001), true);
});

import assert from "node:assert/strict";
import { test } from "node:test";

import { cn } from "./utils.ts";

test("a later plain class overrides a theme class for the same property", () => {
  assert.equal(cn("h-8 dank:h-9", "h-11"), "h-11");
});

test("a theme class stays when nothing later sets its property", () => {
  assert.equal(cn("h-8 dank:h-9", "w-4"), "h-8 dank:h-9 w-4");
});

test("a later class for part of a property drops the theme class it overlaps", () => {
  assert.equal(cn("px-2.5 dank:pr-3 dank:pl-3", "pl-8"), "px-2.5 dank:pr-3 pl-8");
});

test("classes passed together keep their theme classes", () => {
  assert.equal(cn("dank:h-9 h-7"), "dank:h-9 h-7");
});

test("a later theme class replaces an earlier one", () => {
  assert.equal(cn("h-8 dank:h-9", "dank:h-10"), "h-8 dank:h-10");
});

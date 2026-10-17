import assert from "node:assert/strict";
import { test } from "node:test";

import { toggleTask } from "./task-list.ts";

test("toggling a task ticks or unticks the item at the offset", () => {
  const markdown = "Steps:\n\n- [ ] Reproduce\n  1. [X] Nested\n> * [ ] Quoted\n";
  const first = markdown.indexOf("- [ ]");
  assert.equal(toggleTask(markdown, first), markdown.replace("- [ ]", "- [x]"));
  const nested = markdown.indexOf("1.");
  assert.equal(toggleTask(markdown, nested), markdown.replace("[X]", "[ ]"));
  const quoted = markdown.indexOf("* [ ]");
  assert.equal(toggleTask(markdown, quoted), markdown.replace("* [ ]", "* [x]"));
});

test("an offset that isn't a task item leaves the Markdown as it is", () => {
  const markdown = "Steps:\n\n- Plain item\n";
  assert.equal(toggleTask(markdown, 0), markdown);
  assert.equal(toggleTask(markdown, markdown.indexOf("- Plain")), markdown);
});

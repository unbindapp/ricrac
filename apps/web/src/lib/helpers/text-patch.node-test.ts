import assert from "node:assert/strict";
import { test } from "node:test";

import { textPatch } from "./text-patch.ts";

function apply(before: string, after: string) {
  const patch = textPatch(before, after);
  if (!patch) return after;
  const [edit] = patch;
  assert.equal(before.split(edit.old_string).length, 2, "the old text occurs once");
  return before.replace(edit.old_string, edit.new_string);
}

const long =
  "## Context\n\nThe login page loops.\n\n- [ ] Reproduce\n- [ ] Fix\n\nMore notes at the end.";

test("a small edit goes as a patch of the changed span", () => {
  const after = long.replace("loops", "loops forever");
  assert.notEqual(textPatch(long, after), null);
  assert.equal(apply(long, after), after);
});

test("a patch still applies when someone else changed another part meanwhile", () => {
  const after = long.replace("Reproduce", "Reproduce on Safari");
  const [edit] = textPatch(long, after) ?? [];
  const theirs = long.replace("More notes", "Many more notes");
  assert.equal(
    theirs.replace(edit.old_string, edit.new_string),
    theirs.replace("Reproduce", "Reproduce on Safari"),
  );
});

test("an edit of repeated text widens until it is unique", () => {
  const before = "- [ ] a\n- [ ] a\n- [ ] b";
  const after = "- [ ] a\n- [x] a\n- [ ] b";
  assert.equal(apply(before, after), after);
});

test("short or emptied texts are sent whole", () => {
  assert.equal(textPatch("", "New"), null);
  assert.equal(textPatch("Old", ""), null);
});

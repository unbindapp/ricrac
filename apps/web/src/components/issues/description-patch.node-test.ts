import assert from "node:assert/strict";
import { test } from "node:test";

import { descriptionChange } from "./description-patch.ts";

function apply(before: string, change: ReturnType<typeof descriptionChange>) {
  if (change.description !== undefined) return change.description ?? "";
  const [edit] = change.patch ?? [];
  assert.ok(edit && edit.op === "replace");
  assert.equal(before.split(edit.old_string).length, 2, "the old text occurs once");
  return before.replace(edit.old_string, edit.new_string);
}

const long =
  "## Context\n\nThe login page loops.\n\n- [ ] Reproduce\n- [ ] Fix\n\nMore notes at the end.";

test("a small edit goes as a patch of the changed span", () => {
  const after = long.replace("loops", "loops forever");
  const change = descriptionChange(long, after);
  assert.equal(change.description, undefined);
  assert.equal(apply(long, change), after);
});

test("a patch still applies when someone else changed another part meanwhile", () => {
  const after = long.replace("Reproduce", "Reproduce on Safari");
  const change = descriptionChange(long, after);
  const theirs = long.replace("More notes", "Many more notes");
  assert.equal(apply(theirs, change), theirs.replace("Reproduce", "Reproduce on Safari"));
});

test("an edit of repeated text widens until it is unique", () => {
  const before = "- [ ] a\n- [ ] a\n- [ ] b";
  const after = "- [ ] a\n- [x] a\n- [ ] b";
  assert.equal(apply(before, descriptionChange(before, after)), after);
});

test("short or emptied descriptions are sent whole", () => {
  assert.deepEqual(descriptionChange("", "New"), { description: "New" });
  assert.deepEqual(descriptionChange("Old", ""), { description: "" });
});

import assert from "node:assert/strict";
import { test } from "node:test";

import { entitySlug, slugIdFromParam } from "./entity-slug.ts";

test("slugs match the links the server makes", () => {
  assert.equal(entitySlug({ name: "Launch v2", slugId: "12bc7006d1f5" }), "launch-v2-12bc7006d1f5");
  assert.equal(entitySlug({ title: "  Q3: Plan!  ", slugId: "ab" }), "q3-plan-ab");
  assert.equal(entitySlug({ name: "日本", slugId: "ab" }), "ab");
  assert.equal(entitySlug({ name: `${"a".repeat(59)} b`, slugId: "ab" }), `${"a".repeat(59)}-ab`);
});

test("link slugs reduce to their slug id", () => {
  assert.equal(slugIdFromParam("launch-v2-12bc7006d1f5"), "12bc7006d1f5");
  assert.equal(slugIdFromParam("12bc7006d1f5"), "12bc7006d1f5");
  assert.equal(slugIdFromParam("launch-v2"), "launch-v2");
  const id = "0181f3e3-366c-4162-9add-d6233f69bf6d";
  assert.equal(slugIdFromParam(id), id);
});

import assert from "node:assert/strict";
import { test } from "node:test";

import { ALL_TEAMS, resourceLabel, toSaveArgs, webhookFormSchema } from "./webhook-form.ts";

const valid = {
  url: " https://hooks.example.com/ricrac ",
  label: " Slack bridge ",
  resourceTypes: ["Issue", "Comment"],
  team: ALL_TEAMS,
  enabled: true,
};

test("a complete form maps to save_webhook's arguments", () => {
  const parsed = webhookFormSchema.parse(valid);
  assert.deepEqual(toSaveArgs(parsed), {
    url: "https://hooks.example.com/ricrac",
    label: "Slack bridge",
    resourceTypes: ["Issue", "Comment"],
    team: null,
    enabled: true,
  });
  assert.equal(toSaveArgs({ ...parsed, team: "team-1" }).team, "team-1");
  assert.equal(toSaveArgs({ ...parsed, label: "  " }).label, null);
});

test("urls are https, or http on localhost, and something must be received", () => {
  assert.equal(
    webhookFormSchema.safeParse({ ...valid, url: "http://localhost:8080/x" }).success,
    true,
  );
  assert.equal(webhookFormSchema.safeParse({ ...valid, url: "http://example.com" }).success, false);
  assert.equal(webhookFormSchema.safeParse({ ...valid, url: "hooks.example.com" }).success, false);
  assert.equal(webhookFormSchema.safeParse({ ...valid, resourceTypes: [] }).success, false);
  assert.equal(
    webhookFormSchema.safeParse({ ...valid, resourceTypes: ["Explosion"] }).success,
    false,
  );
});

test("resource types read as words", () => {
  assert.equal(resourceLabel("IssueLabel"), "Issue label");
  assert.equal(resourceLabel("Issue"), "Issue");
});

import assert from "node:assert/strict";
import { test } from "node:test";

import { dataFromDraft, draftFromData, recurrenceText } from "./template-data.ts";

test("an issue template reads into the editor and back, keeping what the editor doesn't show", () => {
  const data = {
    title: "Update dependencies",
    priority: 3,
    estimate: 2,
    labels: ["Type/Bug"],
    subIssues: [{ title: "Go modules", estimate: 1 }, { title: "Web packages" }],
  };
  const draft = draftFromData("issue", data);
  assert.equal(draft.title, "Update dependencies");
  assert.equal(draft.priority, "3");
  assert.deepEqual(draft.subIssues, [
    { title: "Go modules", priority: "0" },
    { title: "Web packages", priority: "0" },
  ]);
  assert.deepEqual(dataFromDraft("issue", draft, data), data);
});

test("emptied fields and blank sub-issues are left out", () => {
  const draft = draftFromData("issue", { title: "Bug", assignee: "me", priority: 2 });
  const data = dataFromDraft(
    "issue",
    { ...draft, assignee: "", priority: "0", subIssues: [{ title: " ", priority: "0" }] },
    { title: "Bug", assignee: "me", priority: 2 },
  );
  assert.deepEqual(data, { title: "Bug" });
});

test("project templates carry milestones and issues that point at them", () => {
  const draft = draftFromData("project", {});
  const data = dataFromDraft(
    "project",
    {
      ...draft,
      title: "Launch: ",
      milestones: [{ name: "Beta", targetDate: "" }],
      issues: [{ title: "Invite testers", milestone: "Beta" }],
    },
    {},
  );
  assert.deepEqual(data, {
    name: "Launch: ",
    milestones: [{ name: "Beta" }],
    issues: [{ title: "Invite testers", milestone: "Beta" }],
  });
});

test("document templates keep title and content", () => {
  const draft = draftFromData("document", { title: "Postmortem", content: "## Summary" });
  assert.equal(draft.description, "## Summary");
  assert.deepEqual(dataFromDraft("document", draft, {}), {
    title: "Postmortem",
    content: "## Summary",
  });
});

test("recurrences read as sentences", () => {
  assert.equal(recurrenceText({ every: 1, unit: "week" }), "Every week");
  assert.equal(recurrenceText({ every: 2, unit: "month" }), "Every 2 months");
});

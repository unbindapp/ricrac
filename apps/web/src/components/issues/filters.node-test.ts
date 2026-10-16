import assert from "node:assert/strict";
import { test } from "node:test";

import { DEFAULT_DISPLAY } from "./display.ts";
import {
  activeFilterFields,
  buildListIssuesArgs,
  clearFilterField,
  filtersFromSearch,
  isHiddenCompleted,
  issueFilterOf,
  issueSearchSchema,
  mergeFilters,
} from "./filters.ts";

const showAll = { ...DEFAULT_DISPLAY, show_completed: "all" as const };

test("search params parse into filters and drop bad values", () => {
  const search = issueSearchSchema.parse({
    assignee_ids: ["u1", "null"],
    priorities: [1, 9],
    due_before: "not a date",
    peek: "ENG-1",
  });
  assert.deepEqual(filtersFromSearch(search), { assignee_ids: ["u1", "null"] });
});

test("fixed filters narrow user filters instead of replacing them", () => {
  const merged = mergeFilters(
    { state_types: ["backlog", "started"], assignee_ids: ["u1"] },
    { state_types: ["backlog"], team_ids: ["t1"] },
  );
  assert.deepEqual(merged.state_types, ["backlog"]);
  assert.deepEqual(merged.team_ids, ["t1"]);
  assert.deepEqual(merged.assignee_ids, ["u1"]);
});

test("filters become Ricrac's filter language", () => {
  assert.deepEqual(issueFilterOf({ label_ids: ["l1"], priorities: [1, 2] }), {
    and: [{ labels: { some: { id: { in: ["l1"] } } } }, { priority: { in: [1, 2] } }],
  });
  assert.deepEqual(issueFilterOf({ assignee_ids: ["null", "u1"] }), {
    or: [{ assignee: { null: true } }, { assignee: { id: { in: ["u1"] } } }],
  });
  assert.equal(issueFilterOf({ assignee_ids: [] }), undefined);
});

test("one team goes as the team argument, several in the filter", () => {
  assert.deepEqual(
    buildListIssuesArgs({ filters: {}, fixedFilters: { team_ids: ["t1"] }, display: showAll }),
    { team: "t1", orderBy: "manual" },
  );
  assert.deepEqual(
    buildListIssuesArgs({ filters: { team_ids: ["t1", "t2"] }, display: showAll }).filter,
    { team: { id: { in: ["t1", "t2"] } } },
  );
});

test("hiding completed issues leaves out completed and canceled statuses", () => {
  const args = buildListIssuesArgs({
    filters: {},
    display: { ...DEFAULT_DISPLAY, show_completed: "none" },
  });
  assert.deepEqual(args.filter, {
    state: { type: { in: ["triage", "backlog", "unstarted", "started"] } },
  });
});

test("the past week of completed issues is asked of the server", () => {
  const args = buildListIssuesArgs({ filters: { priorities: [1] }, display: DEFAULT_DISPLAY });
  const filter = args.filter as { and: unknown[] };
  assert.equal(filter.and.length, 2);
  assert.deepEqual(filter.and[0], { priority: { in: [1] } });
});

test("active fields and clearing a field round trip", () => {
  const filters = { priorities: [1], due_after: "2026-01-01", label_ids: ["l"] };
  assert.deepEqual(activeFilterFields(filters), ["priority", "labels", "due_date"]);
  assert.deepEqual(clearFilterField(filters, "due_date"), { priorities: [1], label_ids: ["l"] });
});

test("past week keeps recently completed issues only", () => {
  const now = new Date("2026-03-10T00:00:00Z").getTime();
  const done = (completedAt: string) => ({
    state: { type: "completed" as const },
    completedAt,
    canceledAt: null,
  });
  assert.equal(isHiddenCompleted(done("2026-03-08T00:00:00Z"), "past_week", now), false);
  assert.equal(isHiddenCompleted(done("2026-02-01T00:00:00Z"), "past_week", now), true);
  assert.equal(isHiddenCompleted(done("2026-02-01T00:00:00Z"), "all", now), false);
});

import assert from "node:assert/strict";
import { test } from "node:test";

import { formatProjectDate, isPastTarget, snapToResolution } from "./project-dates.ts";

test("dates read at their resolution", () => {
  assert.equal(formatProjectDate("2026-10-16"), "Oct 16, 2026");
  assert.equal(formatProjectDate("2026-10-16", null), "Oct 16, 2026");
  assert.equal(formatProjectDate("2026-10-16", "month"), "Oct 2026");
  assert.equal(formatProjectDate("2026-10-16", "quarter"), "Q4 2026");
  assert.equal(formatProjectDate("2026-05-01", "halfYear"), "H1 2026");
  assert.equal(formatProjectDate("2026-07-01", "halfYear"), "H2 2026");
  assert.equal(formatProjectDate("2026-02-28", "year"), "2026");
});

test("target dates snap to the end of their period", () => {
  assert.equal(snapToResolution("2026-10-16", "day", "end"), "2026-10-16");
  assert.equal(snapToResolution("2026-02-03", "month", "end"), "2026-02-28");
  assert.equal(snapToResolution("2028-02-03", "month", "end"), "2028-02-29");
  assert.equal(snapToResolution("2026-08-10", "quarter", "end"), "2026-09-30");
  assert.equal(snapToResolution("2026-03-10", "halfYear", "end"), "2026-06-30");
  assert.equal(snapToResolution("2026-03-10", "year", "end"), "2026-12-31");
});

test("start dates snap to the start of their period", () => {
  assert.equal(snapToResolution("2026-10-16", "day", "start"), "2026-10-16");
  assert.equal(snapToResolution("2026-02-03", "month", "start"), "2026-02-01");
  assert.equal(snapToResolution("2026-08-10", "quarter", "start"), "2026-07-01");
  assert.equal(snapToResolution("2026-09-10", "halfYear", "start"), "2026-07-01");
  assert.equal(snapToResolution("2026-03-10", "year", "start"), "2026-01-01");
});

test("a target is past once its last day is", () => {
  assert.equal(isPastTarget("2026-10-15", "2026-10-16"), true);
  assert.equal(isPastTarget("2026-10-16", "2026-10-16"), false);
});

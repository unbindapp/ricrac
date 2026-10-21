import assert from "node:assert/strict";
import { test } from "node:test";

import { formatDate, formatDay, formatRelative } from "./dates.ts";

const now = new Date("2026-03-10T12:00:00Z");

test("times under a minute read as just now", () => {
  assert.equal(formatRelative(new Date("2026-03-10T11:59:30Z"), now), "just now");
  assert.equal(formatRelative(new Date("2026-03-10T12:00:20Z"), now), "just now");
});

test("past times are compact and end with ago", () => {
  assert.equal(formatRelative(new Date("2026-03-10T11:55:00Z"), now), "5m ago");
  assert.equal(formatRelative(new Date("2026-03-10T09:00:00Z"), now), "3h ago");
  assert.equal(formatRelative(new Date("2026-03-08T12:00:00Z"), now), "2d ago");
  assert.equal(formatRelative(new Date("2026-02-24T12:00:00Z"), now), "2w ago");
  assert.equal(formatRelative(new Date("2025-12-10T12:00:00Z"), now), "3mo ago");
  assert.equal(formatRelative(new Date("2024-03-10T12:00:00Z"), now), "2y ago");
});

test("future times start with in", () => {
  assert.equal(formatRelative(new Date("2026-03-10T14:00:00Z"), now), "in 2h");
  assert.equal(formatRelative(new Date("2026-03-13T12:00:00Z"), now), "in 3d");
});

test("accepts ISO strings and timestamps", () => {
  assert.equal(formatRelative("2026-03-10T11:00:00Z", now.getTime()), "1h ago");
});

test("absolute dates use MMM d, yyyy", () => {
  assert.equal(formatDate(new Date(2026, 0, 5, 12)), "Jan 5, 2026");
});

test("chat days read as today, yesterday, then dates", () => {
  const noon = new Date(2026, 2, 10, 12);
  assert.equal(formatDay(new Date(2026, 2, 10, 8), noon), "Today");
  assert.equal(formatDay(new Date(2026, 2, 9, 23), noon), "Yesterday");
  assert.equal(formatDay(new Date(2026, 2, 2, 9), noon), "Monday, March 2");
  assert.equal(formatDay(new Date(2025, 11, 31, 9), noon), "December 31, 2025");
});

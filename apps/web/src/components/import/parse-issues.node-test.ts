import assert from "node:assert/strict";
import { test } from "node:test";

import { EXAMPLE_IMPORT, parseIssuesJson } from "./parse-issues.ts";

test("the example reads as two issues with one comment", () => {
  const result = parseIssuesJson(EXAMPLE_IMPORT);
  assert.ok(result.ok);
  assert.equal(result.parsed.issues.length, 2);
  assert.equal(result.parsed.comments, 1);
  assert.equal(result.parsed.numbered, 2);
});

test("a bare list of issues works too", () => {
  const result = parseIssuesJson('[{"title": "One"}, {"title": "Two"}]');
  assert.ok(result.ok);
  assert.equal(result.parsed.numbered, 0);
});

test("broken input says what is wrong", () => {
  const cases: [string, RegExp][] = [
    ["{nope", /valid JSON/],
    ['{"tickets": []}', /list of issues/],
    ["[]", /no issues/],
    ['[{"title": "Ok"}, {"number": 2}]', /Issue 2 has no title/],
    ['["text"]', /Issue 1 isn't an object/],
  ];
  for (const [input, message] of cases) {
    const result = parseIssuesJson(input);
    assert.equal(result.ok, false, input);
    if (!result.ok) assert.match(result.error, message);
  }
});

test("more than 2000 issues are refused", () => {
  const issues = Array.from({ length: 2001 }, (_, n) => ({ title: `Issue ${n}` }));
  const result = parseIssuesJson(JSON.stringify(issues));
  assert.equal(result.ok, false);
});

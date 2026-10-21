import assert from "node:assert/strict";
import { test } from "node:test";

import { findQuote, plainMarkdown, type TTextRun } from "./quote-ranges.ts";

const BREAK: TTextRun = { text: "", from: null };

test("a quote inside one text node", () => {
  assert.deepEqual(findQuote([{ text: "Ship the beta on Friday", from: 1 }], "the beta"), {
    from: 6,
    to: 14,
  });
});

test("the first of several matches", () => {
  const runs = [{ text: "go go go", from: 1 }];
  assert.deepEqual(findQuote(runs, "go"), { from: 1, to: 3 });
});

test("a quote across paragraphs matches however the break is written", () => {
  const runs = [{ text: "First para", from: 1 }, BREAK, { text: "Second one", from: 13 }];
  assert.deepEqual(findQuote(runs, "para\n\nSecond"), { from: 7, to: 19 });
});

test("a quote across formatting matches through its markup", () => {
  const runs = [
    { text: "Hello ", from: 1 },
    { text: "bold", from: 7 },
    { text: " world", from: 11 },
  ];
  assert.deepEqual(findQuote(runs, "Hello **bold** world"), { from: 1, to: 17 });
});

test("a quote that isn't there", () => {
  assert.equal(findQuote([{ text: "Nothing here", from: 1 }], "missing"), null);
  assert.equal(findQuote([{ text: "Nothing here", from: 1 }], "  "), null);
});

test("markup comes off a Markdown quote", () => {
  assert.equal(plainMarkdown("See [the docs](https://x.test) for *more*"), "See the docs for more");
  assert.equal(plainMarkdown("## Plan\n- [ ] ship `v2`"), "Plan\nship v2");
  assert.equal(
    plainMarkdown("keep snake_case_names and 2 \\* 3"),
    "keep snake_case_names and 2 * 3",
  );
});

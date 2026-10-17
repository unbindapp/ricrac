import assert from "node:assert/strict";
import { test } from "node:test";

import {
  formatRevenue,
  formatSize,
  isDomain,
  parseDomains,
  parseWholeNumber,
} from "./customers.ts";

test("domains are read from a list of domains or links", () => {
  assert.deepEqual(parseDomains("Acme.com, https://www.acme.io/pricing\nacme.com"), [
    "acme.com",
    "acme.io",
  ]);
  assert.deepEqual(parseDomains("  "), []);
});

test("only real domains pass", () => {
  assert.equal(isDomain("acme.co.uk"), true);
  assert.equal(isDomain("acme"), false);
  assert.equal(isDomain("-acme.com"), false);
});

test("whole numbers allow separators, blanks clear and the rest is refused", () => {
  assert.equal(parseWholeNumber("1,200,000"), 1200000);
  assert.equal(parseWholeNumber(""), null);
  assert.equal(parseWholeNumber("12.5"), undefined);
  assert.equal(parseWholeNumber("lots"), undefined);
});

test("revenue and size read compactly", () => {
  assert.equal(formatRevenue(1250000), "$1.3M");
  assert.equal(formatRevenue(950), "$950");
  assert.equal(formatRevenue(null), null);
  assert.equal(formatSize(1200), "1.2K");
});

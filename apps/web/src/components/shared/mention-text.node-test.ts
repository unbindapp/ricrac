import assert from "node:assert/strict";
import { test } from "node:test";

import { splitMentions } from "./mention-text.ts";

test("mentions split out of the text around them", () => {
  assert.deepEqual(splitMentions("Thanks @ann.lee, see @bob."), [
    { kind: "text", value: "Thanks " },
    { kind: "mention", handle: "ann.lee" },
    { kind: "text", value: ", see " },
    { kind: "mention", handle: "bob" },
    { kind: "text", value: "." },
  ]);
});

test("emails and lone @ signs stay text", () => {
  assert.deepEqual(splitMentions("mail ann@acme.test @ noon"), [
    { kind: "text", value: "mail ann@acme.test @ noon" },
  ]);
});

test("a mention can start the text", () => {
  assert.deepEqual(splitMentions("@ann"), [{ kind: "mention", handle: "ann" }]);
});

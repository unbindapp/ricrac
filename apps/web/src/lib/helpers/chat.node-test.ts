import assert from "node:assert/strict";
import { test } from "node:test";

import { chatRows, joinAttachments, shorten, splitAttachments } from "./chat.ts";

const ann = { id: "ann" };
const bob = { id: "bob" };

const message = (id: string, createdAt: string, user: { id: string } | null) => ({
  id,
  createdAt,
  user,
});

const shape = (rows: ReturnType<typeof chatRows>) =>
  rows.map((row) =>
    row.kind === "day" ? "day" : `${row.message.id}${row.isContinuation ? "+" : ""}`,
  );

test("chatRows continues a person's messages a few minutes apart", () => {
  const rows = chatRows([
    message("1", "2026-10-20T10:00:00", ann),
    message("2", "2026-10-20T10:03:00", ann),
    message("3", "2026-10-20T10:20:00", ann),
    message("4", "2026-10-20T10:21:00", bob),
    message("5", "2026-10-20T10:22:00", null),
    message("6", "2026-10-20T10:22:30", null),
  ]);
  assert.deepEqual(shape(rows), ["day", "1", "2+", "3", "4", "5", "6"]);
});

test("chatRows starts every day with a divider", () => {
  const rows = chatRows([
    message("1", "2026-10-20T23:58:00", ann),
    message("2", "2026-10-21T00:01:00", ann),
  ]);
  assert.deepEqual(shape(rows), ["day", "1", "day", "2"]);
});

test("chatRows counts days from the message replies answer", () => {
  const replies = [
    message("2", "2026-10-20T10:05:00", bob),
    message("3", "2026-10-21T09:00:00", bob),
  ];
  assert.deepEqual(shape(chatRows(replies, "2026-10-20T10:00:00")), ["2", "day", "3"]);
  assert.deepEqual(shape(chatRows(replies, "2026-10-19T10:00:00")), ["day", "2", "day", "3"]);
});

test("shorten cuts long text with an ellipsis", () => {
  assert.equal(shorten("Short", 10), "Short");
  assert.equal(shorten("Invite email marked as spam", 12), "Invite emai…");
  assert.equal(shorten("Invite email marked", 14), "Invite email…");
});

test("splitAttachments takes embeds and uploaded files on their own lines out of the text", () => {
  const body = [
    "Here is the mockup",
    "",
    "![mockup.png](https://r.test/files/1#width=200)",
    "",
    "![demo.mp4](https://r.test/files/2)",
    "",
    "[notes.pdf](https://r.test/files/3)",
    "",
    "Thoughts? See [the spec](https://example.com/spec) and ![inline](https://r.test/files/4) here",
  ].join("\n");
  const { text, attachments } = splitAttachments(body);
  assert.equal(
    text,
    "Here is the mockup\n\nThoughts? See [the spec](https://example.com/spec) and ![inline](https://r.test/files/4) here",
  );
  assert.deepEqual(
    attachments.map(({ name, url, kind }) => [name, url, kind]),
    [
      ["mockup.png", "https://r.test/files/1", "image"],
      ["demo.mp4", "https://r.test/files/2", "video"],
      ["notes.pdf", "https://r.test/files/3", "file"],
    ],
  );
});

test("splitAttachments leaves code blocks alone", () => {
  const body = "```md\n![shot.png](https://r.test/files/1)\n```";
  assert.deepEqual(splitAttachments(body), { text: body, attachments: [] });
});

test("joinAttachments puts attachments after the text, each in its own paragraph", () => {
  const { text, attachments } = splitAttachments("Hi\n\n![a.png](https://r.test/files/1)");
  assert.equal(
    joinAttachments(`${text} there`, attachments),
    "Hi there\n\n![a.png](https://r.test/files/1)",
  );
  assert.equal(joinAttachments("", attachments), "![a.png](https://r.test/files/1)");
});

import assert from "node:assert/strict";
import { test } from "node:test";

import { chatRows, shorten } from "./chat.ts";

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

test("shorten cuts long text with an ellipsis", () => {
  assert.equal(shorten("Short", 10), "Short");
  assert.equal(shorten("Invite email marked as spam", 12), "Invite emai…");
  assert.equal(shorten("Invite email marked", 14), "Invite email…");
});

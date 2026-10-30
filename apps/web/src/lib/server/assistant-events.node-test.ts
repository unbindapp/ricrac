import assert from "node:assert/strict";
import { test } from "node:test";

import { parseAssistantEvents } from "./assistant-events.ts";

test("reads each named event with its data", () => {
  const stream = [
    'event: session\ndata: {"sessionId":"s1"}\n\n',
    'event: text\ndata: {"text":"Hi"}\n\n',
    'event: tool\ndata: {"name":"list_issues"}\n\n',
    'event: done\ndata: {"costUsd":0.04,"turns":5,"stopped":null}\n\n',
    'event: error\ndata: {"error":"it broke"}\n\n',
  ].join("");
  assert.deepEqual(parseAssistantEvents(stream), {
    events: [
      { event: "session", data: { sessionId: "s1" } },
      { event: "text", data: { text: "Hi" } },
      { event: "tool", data: { name: "list_issues" } },
      { event: "done", data: { costUsd: 0.04, turns: 5, stopped: null } },
      { event: "error", data: { error: "it broke" } },
    ],
    rest: "",
  });
});

test("holds back an event cut off mid-way until the rest arrives", () => {
  const first = parseAssistantEvents('event: text\ndata: {"text":"Hel');
  assert.deepEqual(first, { events: [], rest: 'event: text\ndata: {"text":"Hel' });

  const second = parseAssistantEvents(`${first.rest}lo"}\n\nevent: tool\n`);
  assert.deepEqual(second, {
    events: [{ event: "text", data: { text: "Hello" } }],
    rest: "event: tool\n",
  });
});

test("an event split between its two closing newlines comes out once both arrive", () => {
  const first = parseAssistantEvents('event: text\ndata: {"text":"a"}\n');
  assert.deepEqual(first.events, []);
  assert.deepEqual(parseAssistantEvents(`${first.rest}\n`).events, [
    { event: "text", data: { text: "a" } },
  ]);
});

test("keeps the newlines and spaces of text", () => {
  const { events } = parseAssistantEvents('event: text\ndata: {"text":"  one\\n\\ntwo"}\n\n');
  assert.deepEqual(events, [{ event: "text", data: { text: "  one\n\ntwo" } }]);
});

test("skips comments and events it doesn't know", () => {
  const stream = ': ping\n\nevent: progress\ndata: {}\n\nevent: text\ndata: {"text":"x"}\n\n';
  assert.deepEqual(parseAssistantEvents(stream).events, [{ event: "text", data: { text: "x" } }]);
});

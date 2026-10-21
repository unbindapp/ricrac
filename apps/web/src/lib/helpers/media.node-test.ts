import assert from "node:assert/strict";
import { test } from "node:test";

import {
  embedKind,
  embedName,
  fileKind,
  firstEmbed,
  mediaLayout,
  uploadKind,
  withMediaLayout,
} from "./media.ts";

test("an embed plays as a video when its name or path has a video extension", () => {
  assert.equal(embedKind("demo.mp4", "https://ricrac.app/files/1"), "video");
  assert.equal(embedKind("Demo.MOV", "/files/1"), "video");
  assert.equal(embedKind("clip", "https://cdn.test/clip.webm?sig=1"), "video");
  assert.equal(embedKind("shot.png", "/files/1"), "image");
  assert.equal(embedKind(null, "/files/1"), "image");
});

test("files are told apart by name and by type", () => {
  assert.equal(fileKind("shot.JPG"), "image");
  assert.equal(fileKind("demo.m4v"), "video");
  assert.equal(fileKind("log.txt"), null);
  assert.equal(uploadKind({ type: "image/png" }), "image");
  assert.equal(uploadKind({ type: "video/quicktime" }), "video");
  assert.equal(uploadKind({ type: "video/x-msvideo" }), null);
  assert.equal(uploadKind({ type: "application/pdf" }), null);
});

test("a video is embedded under a name that plays", () => {
  assert.equal(embedName({ name: "demo.mov", type: "video/quicktime" }), "demo.mov");
  assert.equal(embedName({ name: "recording", type: "video/webm" }), "recording.webm");
  assert.equal(embedName({ name: "[shot].png", type: "image/png" }), "shot.png");
});

test("width and alignment ride in the fragment and leave the URL as it was", () => {
  assert.deepEqual(mediaLayout("/files/1#width=480&align=center"), {
    url: "/files/1",
    layout: { width: 480, align: "center" },
  });
  assert.deepEqual(mediaLayout("/files/1#t=10"), {
    url: "/files/1#t=10",
    layout: { width: null, align: null },
  });
  assert.deepEqual(mediaLayout("/files/1#align=sideways"), {
    url: "/files/1",
    layout: { width: null, align: null },
  });
  assert.equal(withMediaLayout("/files/1", { width: 320.4 }), "/files/1#width=320");
  assert.equal(
    withMediaLayout("/files/1#width=320", { align: "right" }),
    "/files/1#width=320&align=right",
  );
  assert.equal(
    withMediaLayout("/files/1#width=320&align=right", { width: null, align: "left" }),
    "/files/1",
  );
});

test("the first embed is found with its layout left out", () => {
  assert.deepEqual(firstEmbed("Intro\n\n![shot.png](/files/1#width=480)\n\n![b.png](/files/2)"), {
    url: "/files/1",
    kind: "image",
  });
  assert.deepEqual(firstEmbed('![demo.mp4](https://ricrac.app/files/3 "Demo")'), {
    url: "https://ricrac.app/files/3",
    kind: "video",
  });
  assert.equal(firstEmbed("A [link](https://ricrac.app) only"), null);
});

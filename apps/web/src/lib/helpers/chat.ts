import { isSameDay } from "date-fns";

import { embedKind, mediaLayout, type TMediaKind } from "./media.ts";

const GROUP_WINDOW_MS = 7 * 60 * 1000;
const PLACEHOLDER_NAME_LENGTH = 32;

/** Text cut to `length` characters, ending in an ellipsis when cut. */
export function shorten(text: string, length: number) {
  if (text.length <= length) return text;
  return `${text.slice(0, length - 1).trimEnd()}…`;
}

/** What a composer says before you write: Message and where it goes. */
export function messagePlaceholder(name: string) {
  return `Message ${shorten(name, PLACEHOLDER_NAME_LENGTH)}`;
}

type TMessage = { id: string; createdAt: string; user: { id: string } | null };

export type TChatRow<T extends TMessage> =
  | { kind: "day"; key: string; date: string }
  | { kind: "message"; key: string; message: T; isContinuation: boolean };

/**
 * Messages with a divider before each new day, counting from `since` when they follow another
 * message; a message from the same person a few minutes after the one before continues it.
 */
export function chatRows<T extends TMessage>(messages: T[], since?: string): TChatRow<T>[] {
  const rows: TChatRow<T>[] = [];
  let previous: T | undefined;
  for (const message of messages) {
    const lastDay = previous?.createdAt ?? since;
    const isNewDay = !lastDay || !isSameDay(lastDay, message.createdAt);
    if (isNewDay) rows.push({ kind: "day", key: `day-${message.id}`, date: message.createdAt });
    rows.push({
      kind: "message",
      key: message.id,
      message,
      isContinuation: !isNewDay && continues(previous, message),
    });
    previous = message;
  }
  return rows;
}

function continues(previous: TMessage | undefined, message: TMessage) {
  if (!previous?.user || previous.user.id !== message.user?.id) return false;
  const gap = new Date(message.createdAt).getTime() - new Date(previous.createdAt).getTime();
  return gap >= 0 && gap < GROUP_WINDOW_MS;
}

export type TChatAttachment = {
  markdown: string;
  name: string;
  url: string;
  kind: TMediaKind | "file";
};

const MEDIA_LINE = /^!\[([^\]]*)\]\(<?([^)\s>]+)>?\)$/;
const FILE_LINE = /^\[([^\]]+)\]\(<?([^)\s>]*\/files\/[^)\s>]+)>?\)$/;
const FENCE = /^\s*(```|~~~)/;

function attachmentOf(line: string): TChatAttachment | null {
  const markdown = line.trim();
  const media = MEDIA_LINE.exec(markdown);
  if (media) {
    const [, name, src] = media;
    return { markdown, name, url: mediaLayout(src).url, kind: embedKind(name, src) };
  }
  const file = FILE_LINE.exec(markdown);
  if (file) return { markdown, name: file[1], url: file[2], kind: "file" };
  return null;
}

/** A message's text, and the embeds and uploaded files on lines of their own, which show under it. */
export function splitAttachments(body: string): { text: string; attachments: TChatAttachment[] } {
  const attachments: TChatAttachment[] = [];
  const lines: string[] = [];
  let isInFence = false;
  for (const line of body.split("\n")) {
    if (FENCE.test(line)) isInFence = !isInFence;
    const attachment = isInFence ? null : attachmentOf(line);
    if (!attachment) {
      lines.push(line);
      continue;
    }
    attachments.push(attachment);
    if (lines.at(-1)?.trim() === "") lines.pop();
  }
  return { text: lines.join("\n").trim(), attachments };
}

export function joinAttachments(text: string, attachments: { markdown: string }[]) {
  return [text.trim(), ...attachments.map((attachment) => attachment.markdown)]
    .filter(Boolean)
    .join("\n\n");
}

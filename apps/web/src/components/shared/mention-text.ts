/** A handle after @: letters, digits and . _ + -, not ending in . or - (as the server reads them). */
export const mentionPattern = "[\\p{L}\\p{N}_+](?:[\\p{L}\\p{N}._+-]*[\\p{L}\\p{N}_+])?";

const inTextPattern = new RegExp(`(^|[^\\p{L}\\p{N}@])@(${mentionPattern})`, "gu");

type TTextPart = { kind: "text"; value: string } | { kind: "mention"; handle: string };

/** Splits text into plain runs and @mentions; emails (ann@acme.test) stay text. */
export function splitMentions(text: string): TTextPart[] {
  const parts: TTextPart[] = [];
  let last = 0;
  for (const match of text.matchAll(inTextPattern)) {
    const at = (match.index ?? 0) + match[1].length;
    if (at > last) parts.push({ kind: "text", value: text.slice(last, at) });
    parts.push({ kind: "mention", handle: match[2] });
    last = at + 1 + match[2].length;
  }
  if (last < text.length) parts.push({ kind: "text", value: text.slice(last) });
  return parts;
}

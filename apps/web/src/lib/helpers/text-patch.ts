const CONTEXT_STEP = 8;

export type TTextPatch = { op: "replace"; old_string: string; new_string: string }[];

function occursOnce(text: string, needle: string) {
  const at = text.indexOf(needle);
  return at >= 0 && text.indexOf(needle, at + 1) < 0;
}

/**
 * The edit from `before` to `after` as one `replace` of the changed span (widened until it is
 * unique, a side at a time), so the server keeps changes others made elsewhere in the text
 * meanwhile. Null when the span would cover all of it: then the whole text goes.
 */
export function textPatch(before: string, after: string): TTextPatch | null {
  let prefix = 0;
  const shorter = Math.min(before.length, after.length);
  while (prefix < shorter && before[prefix] === after[prefix]) prefix++;
  let suffix = 0;
  while (
    suffix < shorter - prefix &&
    before[before.length - 1 - suffix] === after[after.length - 1 - suffix]
  ) {
    suffix++;
  }
  let start = prefix;
  let end = before.length - suffix;
  let widenLeft = true;
  while (end - start === 0 || !occursOnce(before, before.slice(start, end))) {
    if (start === 0 && end === before.length) return null;
    if ((widenLeft && start > 0) || end === before.length) {
      start = Math.max(0, start - CONTEXT_STEP);
    } else {
      end = Math.min(before.length, end + CONTEXT_STEP);
    }
    widenLeft = !widenLeft;
  }
  if (start === 0 && end === before.length) return null;
  return [
    {
      op: "replace",
      old_string: before.slice(start, end),
      new_string: after.slice(start, after.length - (before.length - end)),
    },
  ];
}

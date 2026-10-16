import type { SaveIssueArgs } from "@/api/ops.gen";

const CONTEXT_STEP = 8;

type TDescriptionChange = Pick<SaveIssueArgs, "description" | "patch">;

function occursOnce(text: string, needle: string) {
  const at = text.indexOf(needle);
  return at >= 0 && text.indexOf(needle, at + 1) < 0;
}

/**
 * The edit from `before` to `after` as one `replace` of the changed span (widened until it is
 * unique, a side at a time), so the server keeps changes others made elsewhere in the description meanwhile.
 * Falls back to the whole description when the span would cover all of it.
 */
export function descriptionChange(before: string, after: string): TDescriptionChange {
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
    if (start === 0 && end === before.length) return { description: after };
    if ((widenLeft && start > 0) || end === before.length) {
      start = Math.max(0, start - CONTEXT_STEP);
    } else {
      end = Math.min(before.length, end + CONTEXT_STEP);
    }
    widenLeft = !widenLeft;
  }
  if (start === 0 && end === before.length) return { description: after };
  return {
    patch: [
      {
        op: "replace",
        old_string: before.slice(start, end),
        new_string: after.slice(start, after.length - (before.length - end)),
      },
    ],
  };
}

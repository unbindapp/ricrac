const TASK_MARKER = /^[ \t>]*(?:[-*+]|\d+[.)])[ \t]+\[([ xX])\]/;

/** Ticks or unticks the task list item that starts at `offset` in the Markdown. */
export function toggleTask(markdown: string, offset: number) {
  const match = TASK_MARKER.exec(markdown.slice(offset));
  if (!match) return markdown;
  const at = offset + match[0].length - 2;
  const next = match[1] === " " ? "x" : " ";
  return markdown.slice(0, at) + next + markdown.slice(at + 1);
}

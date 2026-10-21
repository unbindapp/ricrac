/** A text node's text at its document position, or a break between blocks (`from: null`). */
export type TTextRun = { text: string; from: number | null };

export type TTextRange = { from: number; to: number };

const MARKDOWN_MARKUP: [RegExp, string][] = [
  [/!\[[^\]]*\]\([^)]*\)/g, " "],
  [/\[([^\]]*)\]\([^)]*\)/g, "$1"],
  [/^[ \t]*(?:#{1,6}[ \t]+|>[ \t]?|[-*+][ \t]+(?:\[[ xX]\][ \t]+)?|\d+[.)][ \t]+)/gm, ""],
  [/\*\*|__|~~|`/g, ""],
  [/(^|[^\w\\*])[*_](?=\S)(.*?\S)[*_](?![\w*])/g, "$1$2"],
  [/\\([\\`*_{}[\]()#+\-.!~>|])/g, "$1"],
];

/** The quote's Markdown as the text it renders. */
export function plainMarkdown(markdown: string) {
  return MARKDOWN_MARKUP.reduce(
    (text, [pattern, replacement]) => text.replace(pattern, replacement),
    markdown,
  );
}

const collapseWhitespace = (text: string) => text.replace(/\s+/g, " ").trim();

/**
 * Where the quote first appears in the runs, however whitespace and block breaks fall in it;
 * failing that, where its text without Markdown markup does.
 */
export function findQuote(runs: TTextRun[], quote: string): TTextRange | null {
  const { text, positions } = flatten(runs);
  const needles = new Set([collapseWhitespace(quote), collapseWhitespace(plainMarkdown(quote))]);
  for (const needle of needles) {
    if (!needle) continue;
    const at = text.indexOf(needle);
    if (at < 0) continue;
    const from = positions[at];
    const last = positions[at + needle.length - 1];
    if (from === null || last === null) continue;
    return { from, to: last + 1 };
  }
  return null;
}

function flatten(runs: TTextRun[]) {
  const chars: string[] = [];
  const positions: (number | null)[] = [];
  const push = (char: string, at: number | null) => {
    const isSpace = /\s/.test(char);
    if (isSpace && (chars.length === 0 || chars[chars.length - 1] === " ")) return;
    chars.push(isSpace ? " " : char);
    positions.push(at);
  };
  for (const run of runs) {
    if (run.from === null) {
      push(" ", null);
      continue;
    }
    for (let index = 0; index < run.text.length; index++) push(run.text[index], run.from + index);
  }
  return { text: chars.join(""), positions };
}

import { Mention } from "@tiptap/extension-mention";

import { mentionPattern } from "@/components/shared/mention-text";

const startPattern = new RegExp(`(?:^|(?<=[^\\p{L}\\p{N}@]))@${mentionPattern}`, "u");
const tokenPattern = new RegExp(`^@(${mentionPattern})`, "u");

/** People are mentioned as @displayName in Markdown, which is what Ricrac notifies. */
export const MarkdownMention = Mention.extend({
  parseMarkdown: (token, helpers) =>
    helpers.createNode("mention", { id: token.attributes?.id, label: token.attributes?.label }),
  markdownTokenizer: {
    name: "mention",
    level: "inline",
    start: (src) => src.search(startPattern),
    tokenize: (src) => {
      const match = src.match(tokenPattern);
      if (!match) return undefined;
      return { type: "mention", raw: match[0], attributes: { label: match[1], id: match[1] } };
    },
  },
  renderMarkdown: (node) => `@${node.attrs?.label ?? ""}`,
});

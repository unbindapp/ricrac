import type { Parent, PhrasingContent, Root, RootContent } from "mdast";
import ReactMarkdown, { defaultUrlTransform } from "react-markdown";
import remarkGfm from "remark-gfm";

import { splitMentions } from "@/components/shared/mention-text";

type TProps = {
  markdown: string;
};

const MENTION_URL = "mention:";

/** Turns @handles in text (not in code or links) into mention links, rendered as chips. */
function remarkMentions() {
  const visit = (node: Parent) => {
    node.children = node.children.flatMap((child: RootContent): RootContent[] => {
      if (child.type === "text") {
        return splitMentions(child.value).map((part): PhrasingContent =>
          part.kind === "text"
            ? { type: "text", value: part.value }
            : {
                type: "link",
                url: `${MENTION_URL}${part.handle}`,
                children: [{ type: "text", value: `@${part.handle}` }],
              },
        );
      }
      if ("children" in child && child.type !== "link") visit(child);
      return [child];
    }) as typeof node.children;
  };
  return (tree: Root) => visit(tree);
}

function urlTransform(url: string) {
  if (url.startsWith(MENTION_URL)) return url;
  return defaultUrlTransform(url);
}

export function MarkdownView({ markdown }: TProps) {
  return (
    <div data-slot="markdown-view" className="zz-prose w-full">
      <ReactMarkdown
        remarkPlugins={[remarkGfm, remarkMentions]}
        urlTransform={urlTransform}
        components={{
          a: ({ href, children }) => {
            if (href?.startsWith(MENTION_URL)) {
              return <span data-type="mention">{children}</span>;
            }
            return (
              <a href={href} target="_blank" rel="noreferrer noopener">
                {children}
              </a>
            );
          },
        }}
      >
        {markdown}
      </ReactMarkdown>
    </div>
  );
}

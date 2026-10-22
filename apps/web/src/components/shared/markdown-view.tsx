import type { Parent, PhrasingContent, Root, RootContent } from "mdast";
import { createContext, use, useState } from "react";
import ReactMarkdown, { type Components, defaultUrlTransform } from "react-markdown";
import remarkGfm from "remark-gfm";

import { MarkdownMedia } from "@/components/shared/markdown-media";
import { splitMentions } from "@/components/shared/mention-text";
import { TaskCheckbox } from "@/components/ui/task-checkbox";
import { toggleTask } from "@/lib/helpers/task-list";

type TProps = {
  markdown: string;
  /** Makes task list checkboxes clickable: called with the Markdown with the task toggled. */
  onChange?: (markdown: string) => Promise<unknown>;
};

const MENTION_URL = "mention:";

/** Where the list item a checkbox belongs to starts in the Markdown. */
const ItemOffset = createContext<number | null>(null);
const ToggleTask = createContext<((offset: number) => void) | null>(null);

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

const components: Components = {
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
  li: ({ node, children, ...props }) => (
    <li {...props}>
      <ItemOffset value={node?.position?.start.offset ?? null}>{children}</ItemOffset>
    </li>
  ),
  input: ({ checked }) => <TaskInput checked={Boolean(checked)} />,
  img: ({ src, alt }) =>
    typeof src === "string" && src ? <MarkdownMedia src={src} alt={alt} /> : null,
};

export function MarkdownView({ markdown, onChange }: TProps) {
  const [toggled, setToggled] = useState<{ from: string; to: string } | null>(null);
  const shown = toggled?.from === markdown ? toggled.to : markdown;
  const toggle = (offset: number) => {
    if (!onChange) return;
    const next = toggleTask(shown, offset);
    setToggled({ from: markdown, to: next });
    onChange(next).catch(() => setToggled(null));
  };
  return (
    <div data-slot="markdown-view" className="rr-prose w-full">
      <ToggleTask value={onChange ? toggle : null}>
        <ReactMarkdown
          remarkPlugins={[remarkGfm, remarkMentions]}
          urlTransform={urlTransform}
          components={components}
        >
          {shown}
        </ReactMarkdown>
      </ToggleTask>
    </div>
  );
}

function TaskInput({ checked }: { checked: boolean }) {
  const offset = use(ItemOffset);
  const toggle = use(ToggleTask);
  if (offset === null || !toggle) return <TaskCheckbox checked={checked} />;
  return <TaskCheckbox checked={checked} onToggle={() => toggle(offset)} />;
}

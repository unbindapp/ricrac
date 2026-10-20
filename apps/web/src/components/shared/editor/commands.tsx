import { Extension, type Editor, type Range } from "@tiptap/core";
import { PluginKey } from "@tiptap/pm/state";
import { Suggestion } from "@tiptap/suggestion";
import {
  AtSignIcon,
  CodeSquareIcon,
  FileCode2Icon,
  Heading1Icon,
  Heading2Icon,
  Heading3Icon,
  ImageIcon,
  ListChecksIcon,
  ListIcon,
  ListOrderedIcon,
  type LucideIcon,
  MinusIcon,
  PaperclipIcon,
  PilcrowIcon,
  SmileIcon,
  TableIcon,
  TextQuoteIcon,
} from "lucide-react";

import {
  suggestionRenderer,
  type TSuggestionSink,
  type TSuggestionStateRef,
} from "@/components/shared/editor/suggestions";
import type { TSuggestionItem } from "@/components/ui/suggestion-menu";

export type TPickFiles = (kind: "media" | "any") => void;

export type TBlockCommand = {
  id: string;
  label: string;
  icon: LucideIcon;
  hint?: string;
  keywords: string[];
  needsUpload?: boolean;
  run: (editor: Editor, pickFiles: TPickFiles) => void;
  isActive?: (editor: Editor) => boolean;
};

export const textStyles: TBlockCommand[] = [
  {
    id: "text",
    label: "Text",
    icon: PilcrowIcon,
    keywords: ["paragraph", "plain", "body"],
    run: (editor) => editor.chain().focus().setParagraph().run(),
    isActive: (editor) => editor.isActive("paragraph"),
  },
  ...([1, 2, 3] as const).map((level): TBlockCommand => ({
    id: `heading${level}`,
    label: `Heading ${level}`,
    icon: [Heading1Icon, Heading2Icon, Heading3Icon][level - 1],
    hint: `${"#".repeat(level)} `,
    keywords: ["title", "header", `h${level}`],
    run: (editor) => editor.chain().focus().setHeading({ level }).run(),
    isActive: (editor) => editor.isActive("heading", { level }),
  })),
];

export const listCommands: TBlockCommand[] = [
  {
    id: "bulletList",
    label: "Bulleted list",
    icon: ListIcon,
    hint: "- ",
    keywords: ["unordered", "ul", "bullet"],
    run: (editor) => editor.chain().focus().toggleBulletList().run(),
    isActive: (editor) => editor.isActive("bulletList"),
  },
  {
    id: "orderedList",
    label: "Numbered list",
    icon: ListOrderedIcon,
    hint: "1. ",
    keywords: ["ordered", "ol", "number"],
    run: (editor) => editor.chain().focus().toggleOrderedList().run(),
    isActive: (editor) => editor.isActive("orderedList"),
  },
  {
    id: "taskList",
    label: "Checklist",
    icon: ListChecksIcon,
    hint: "[] ",
    keywords: ["todo", "task", "checkbox"],
    run: (editor) => editor.chain().focus().toggleTaskList().run(),
    isActive: (editor) => editor.isActive("taskList"),
  },
];

export const blockCommands: TBlockCommand[] = [
  {
    id: "blockquote",
    label: "Quote",
    icon: TextQuoteIcon,
    hint: "> ",
    keywords: ["blockquote", "citation"],
    run: (editor) => editor.chain().focus().toggleBlockquote().run(),
    isActive: (editor) => editor.isActive("blockquote"),
  },
  {
    id: "codeBlock",
    label: "Code block",
    icon: FileCode2Icon,
    hint: "```",
    keywords: ["snippet", "pre", "source"],
    run: (editor) => editor.chain().focus().toggleCodeBlock().run(),
    isActive: (editor) => editor.isActive("codeBlock"),
  },
  {
    id: "table",
    label: "Table",
    icon: TableIcon,
    keywords: ["grid", "columns", "rows"],
    run: (editor) =>
      editor.chain().focus().insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run(),
    isActive: (editor) => editor.isActive("table"),
  },
  {
    id: "divider",
    label: "Divider",
    icon: MinusIcon,
    hint: "---",
    keywords: ["horizontal rule", "hr", "separator", "line"],
    run: (editor) => editor.chain().focus().setHorizontalRule().run(),
  },
];

export const insertCommands: TBlockCommand[] = [
  {
    id: "media",
    label: "Image or video",
    icon: ImageIcon,
    keywords: ["picture", "photo", "screenshot", "movie", "recording", "upload", "gif"],
    needsUpload: true,
    run: (_editor, pickFiles) => pickFiles("media"),
  },
  {
    id: "file",
    label: "File",
    icon: PaperclipIcon,
    keywords: ["attachment", "upload", "pdf", "document"],
    needsUpload: true,
    run: (_editor, pickFiles) => pickFiles("any"),
  },
  {
    id: "mention",
    label: "Mention someone",
    icon: AtSignIcon,
    hint: "@",
    keywords: ["person", "user", "people", "notify"],
    run: (editor) => editor.chain().focus().insertContent("@").run(),
  },
  {
    id: "emoji",
    label: "Emoji",
    icon: SmileIcon,
    hint: ":",
    keywords: ["emoticon", "smiley", "reaction"],
    run: (editor) => editor.chain().focus().insertContent(":").run(),
  },
  {
    id: "inlineCode",
    label: "Inline code",
    icon: CodeSquareIcon,
    hint: "`",
    keywords: ["monospace", "code"],
    run: (editor) => editor.chain().focus().toggleCode().run(),
  },
];

const slashCommands = [...textStyles, ...listCommands, ...blockCommands, ...insertCommands];

type TSlashItem = TSuggestionItem & { command: TBlockCommand };

export function matchCommands(query: string, canUpload: boolean): TBlockCommand[] {
  const needle = query.toLowerCase().trim();
  return slashCommands.filter((command) => {
    if (command.needsUpload && !canUpload) return false;
    if (!needle) return true;
    const words = [command.label, ...command.keywords].map((word) => word.toLowerCase());
    return words.some((word) => word.includes(needle));
  });
}

function slashItem(command: TBlockCommand): TSlashItem {
  const Icon = command.icon;
  return {
    id: command.id,
    label: command.label,
    description: command.hint?.trim(),
    icon: <Icon className="text-muted-foreground size-4" />,
    command,
  };
}

type TSlashOptions = {
  sink: TSuggestionSink;
  state: TSuggestionStateRef;
  pickFiles: { current: TPickFiles };
  canUpload: { current: boolean };
};

export const SlashCommands = Extension.create<TSlashOptions>({
  name: "slashCommands",
  addProseMirrorPlugins() {
    const { sink, state, pickFiles, canUpload } = this.options;
    return [
      Suggestion<TSlashItem, TSlashItem>({
        editor: this.editor,
        pluginKey: new PluginKey("slashCommands"),
        char: "/",
        allow: ({ editor }) => !editor.isActive("codeBlock"),
        items: ({ query }) => matchCommands(query, canUpload.current).map(slashItem),
        command: ({
          editor,
          range,
          props,
        }: {
          editor: Editor;
          range: Range;
          props: TSlashItem;
        }) => {
          editor.chain().focus().deleteRange(range).run();
          props.command.run(editor, pickFiles.current);
        },
        render: suggestionRenderer<TSlashItem>(sink, state, "No matching commands"),
      }),
    ];
  },
});

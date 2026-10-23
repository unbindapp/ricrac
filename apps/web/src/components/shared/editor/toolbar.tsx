import type { Editor } from "@tiptap/core";
import { useEditorState } from "@tiptap/react";
import { type FormEvent, type ReactNode, useState } from "react";

import { ShortcutKeys } from "@/components/command-panel/shortcut-keys";
import {
  ArrowDownToLineIcon,
  ArrowLeftToLineIcon,
  ArrowRightToLineIcon,
  ArrowUpToLineIcon,
  BoldIcon,
  ChevronDownIcon,
  CodeIcon,
  EllipsisIcon,
  HelpCircleIcon,
  ItalicIcon,
  LinkIcon,
  type LucideIcon,
  StrikethroughIcon,
  TableColumnsSplitIcon,
  TableRowsSplitIcon,
  Trash2Icon,
} from "@/components/icons";
import {
  blockCommands,
  insertCommands,
  listCommands,
  type TBlockCommand,
  type TPickFiles,
  textStyles,
} from "@/components/shared/editor/commands";
import { mediaAlignments, mediaSizes } from "@/components/shared/editor/media-node";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { Kbd } from "@/components/ui/kbd";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { cn } from "@/components/ui/utils";
import { mediaLayout } from "@/lib/helpers/media";

type TMark = { id: string; label: string; icon: LucideIcon; keys: string[] };

const marks: TMark[] = [
  { id: "bold", label: "Bold", icon: BoldIcon, keys: ["Mod+B"] },
  { id: "italic", label: "Italic", icon: ItalicIcon, keys: ["Mod+I"] },
  { id: "strike", label: "Strikethrough", icon: StrikethroughIcon, keys: ["Mod+Shift+S"] },
  { id: "code", label: "Inline code", icon: CodeIcon, keys: ["Mod+E"] },
];

const markToggles: Record<string, (editor: Editor) => void> = {
  bold: (editor) => editor.chain().focus().toggleBold().run(),
  italic: (editor) => editor.chain().focus().toggleItalic().run(),
  strike: (editor) => editor.chain().focus().toggleStrike().run(),
  code: (editor) => editor.chain().focus().toggleCode().run(),
};

const blockKeys: Record<string, string[]> = {
  bulletList: ["Mod+Shift+8"],
  orderedList: ["Mod+Shift+7"],
  taskList: ["Mod+Shift+9"],
  blockquote: ["Mod+Shift+B"],
  codeBlock: ["Mod+Alt+C"],
};

const toolbarBlocks = [...listCommands, ...blockCommands];
const toolbarInserts = insertCommands.filter((command) => command.needsUpload);
const overflowBlockIds = new Set(blockCommands.map((command) => command.id));
const hiddenWhenNarrow = "dank:@max-[49rem]:hidden";

type TTableAction = { label: string; icon: LucideIcon; run: (editor: Editor) => void };

const tableActions: TTableAction[] = [
  {
    label: "Add row above",
    icon: ArrowUpToLineIcon,
    run: (editor) => editor.chain().focus().addRowBefore().run(),
  },
  {
    label: "Add row below",
    icon: ArrowDownToLineIcon,
    run: (editor) => editor.chain().focus().addRowAfter().run(),
  },
  {
    label: "Add column left",
    icon: ArrowLeftToLineIcon,
    run: (editor) => editor.chain().focus().addColumnBefore().run(),
  },
  {
    label: "Add column right",
    icon: ArrowRightToLineIcon,
    run: (editor) => editor.chain().focus().addColumnAfter().run(),
  },
  {
    label: "Delete row",
    icon: TableRowsSplitIcon,
    run: (editor) => editor.chain().focus().deleteRow().run(),
  },
  {
    label: "Delete column",
    icon: TableColumnsSplitIcon,
    run: (editor) => editor.chain().focus().deleteColumn().run(),
  },
  {
    label: "Delete table",
    icon: Trash2Icon,
    run: (editor) => editor.chain().focus().deleteTable().run(),
  },
];

type TProps = {
  editor: Editor;
  canUpload: boolean;
  pickFiles: TPickFiles;
  isLinkOpen: boolean;
  onLinkOpenChange: (open: boolean) => void;
};

export function EditorToolbar({
  editor,
  canUpload,
  pickFiles,
  isLinkOpen,
  onLinkOpenChange,
}: TProps) {
  const state = useEditorState({
    editor,
    selector: ({ editor: current }) => ({
      textStyle: textStyles.find((style) => style.isActive?.(current))?.id ?? "text",
      marks: Object.fromEntries(marks.map((mark) => [mark.id, current.isActive(mark.id)])),
      blocks: Object.fromEntries(
        toolbarBlocks.map((command) => [command.id, command.isActive?.(current) ?? false]),
      ),
      isLink: current.isActive("link"),
      isInTable: current.isActive("table"),
      isInCode: current.isActive("codeBlock"),
      media: selectedMediaSrc(current),
    }),
  });
  const style = textStyles.find((item) => item.id === state.textStyle) ?? textStyles[0];
  const media = state.media ? mediaLayout(state.media).layout : null;

  return (
    <div
      role="toolbar"
      aria-label="Formatting"
      data-slot="editor-toolbar"
      className="bg-background dank:@container flex min-w-0 items-center gap-0.5 overflow-x-auto"
      onMouseDown={(event) => {
        if (event.target instanceof HTMLElement && event.target.closest("input")) return;
        event.preventDefault();
      }}
    >
      <TextStyleMenu editor={editor} current={style} disabled={state.isInCode} />
      <Divider />
      {marks.map((mark) => (
        <ToolButton
          key={mark.id}
          label={mark.label}
          keys={mark.keys}
          icon={mark.icon}
          isActive={state.marks[mark.id]}
          disabled={state.isInCode}
          onClick={() => markToggles[mark.id](editor)}
        />
      ))}
      <LinkButton
        editor={editor}
        isActive={state.isLink}
        disabled={state.isInCode}
        open={isLinkOpen}
        onOpenChange={onLinkOpenChange}
      />
      <Divider />
      {toolbarBlocks.map((command) => (
        <ToolButton
          key={command.id}
          label={command.label}
          keys={blockKeys[command.id]}
          hint={command.hint}
          icon={command.icon}
          isActive={state.blocks[command.id]}
          className={overflowBlockIds.has(command.id) ? hiddenWhenNarrow : undefined}
          onClick={() => command.run(editor, pickFiles)}
        />
      ))}
      {canUpload && <Divider />}
      {canUpload &&
        toolbarInserts.map((command) => (
          <ToolButton
            key={command.id}
            label={insertLabel(command)}
            icon={command.icon}
            className={hiddenWhenNarrow}
            onClick={() => command.run(editor, pickFiles)}
          />
        ))}
      <OverflowMenu
        editor={editor}
        pickFiles={pickFiles}
        commands={[...blockCommands, ...(canUpload ? toolbarInserts : [])]}
      />
      {state.isInTable && (
        <>
          <Divider />
          {tableActions.map((action) => (
            <ToolButton
              key={action.label}
              label={action.label}
              icon={action.icon}
              onClick={() => action.run(editor)}
            />
          ))}
        </>
      )}
      {media && (
        <>
          <Divider />
          {mediaAlignments.map(({ value, label, icon }) => (
            <ToolButton
              key={value}
              label={label}
              icon={icon}
              isActive={(media.align ?? "left") === value}
              onClick={() => editor.chain().focus().setMediaLayout({ align: value }).run()}
            />
          ))}
          {mediaSizes.map((size) => (
            <SimpleTooltip key={size.label} content={size.name}>
              <Button
                variant="ghost"
                size="icon"
                forceMinSize={false}
                aria-label={size.name}
                aria-pressed={media.width === size.width}
                className={cn(
                  "dank:type-label-large text-xs",
                  media.width === size.width && "bg-accent text-foreground",
                )}
                onClick={() => editor.chain().focus().setMediaLayout({ width: size.width }).run()}
              >
                {size.label}
              </Button>
            </SimpleTooltip>
          ))}
        </>
      )}
      <span className="flex-1" />
      <EditorHelp />
    </div>
  );
}

function selectedMediaSrc(editor: Editor) {
  const node = editor.state.doc.nodeAt(editor.state.selection.from);
  if (node?.type.name !== "image" || !node.attrs.src) return null;
  if (editor.state.selection.to - editor.state.selection.from !== node.nodeSize) return null;
  return node.attrs.src as string;
}

function Divider() {
  return (
    <span aria-hidden data-slot="toolbar-divider" className="bg-border mx-1 h-4 w-px shrink-0" />
  );
}

function insertLabel(command: TBlockCommand) {
  return command.id === "media" ? "Upload an image or video" : "Attach a file";
}

/** In narrow Dank toolbars, the blocks and uploads that don't fit. */
function OverflowMenu({
  editor,
  pickFiles,
  commands,
}: {
  editor: Editor;
  pickFiles: TPickFiles;
  commands: TBlockCommand[];
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            variant="ghost"
            size="icon"
            forceMinSize={false}
            aria-label="More formatting"
            className="dank:@max-[49rem]:flex hidden"
          >
            <EllipsisIcon className="size-4" />
          </Button>
        }
      />
      <DropdownMenuContent className="w-64" finalFocus={false}>
        <DropdownMenuGroup>
          {commands.map((command) => {
            const Icon = command.icon;
            return (
              <DropdownMenuItem key={command.id} onClick={() => command.run(editor, pickFiles)}>
                <Icon className="size-4" />
                <span className="flex-1">
                  {command.needsUpload ? insertLabel(command) : command.label}
                </span>
              </DropdownMenuItem>
            );
          })}
        </DropdownMenuGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function ToolButton({
  label,
  keys,
  hint,
  icon: Icon,
  isActive,
  disabled,
  className,
  onClick,
}: {
  label: string;
  keys?: string[];
  hint?: string;
  icon: LucideIcon;
  isActive?: boolean;
  disabled?: boolean;
  className?: string;
  onClick: () => void;
}) {
  return (
    <SimpleTooltip content={<TipContent label={label} keys={keys} hint={hint} />}>
      <Button
        variant="ghost"
        size="icon"
        forceMinSize={false}
        aria-label={label}
        aria-pressed={isActive === undefined ? undefined : isActive}
        disabled={disabled}
        className={cn("text-muted-foreground", isActive && "bg-accent text-foreground", className)}
        onClick={onClick}
      >
        <Icon className="size-4" />
      </Button>
    </SimpleTooltip>
  );
}

function TipContent({ label, keys, hint }: { label: string; keys?: string[]; hint?: string }) {
  return (
    <span className="flex items-center gap-2">
      {label}
      {keys && <ShortcutKeys keys={keys} />}
      {hint && <Kbd className="font-mono">{hint.trim()}</Kbd>}
    </span>
  );
}

function TextStyleMenu({
  editor,
  current,
  disabled,
}: {
  editor: Editor;
  current: TBlockCommand;
  disabled: boolean;
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        disabled={disabled}
        render={
          <Button
            variant="ghost"
            size="sm"
            forceMinSize={false}
            aria-label="Text style"
            className="text-muted-foreground shrink-0 font-normal"
          >
            {current.label}
            <ChevronDownIcon className="size-3.5" />
          </Button>
        }
      />
      <DropdownMenuContent className="w-48" finalFocus={false}>
        <DropdownMenuGroup>
          {textStyles.map((style) => {
            const Icon = style.icon;
            return (
              <DropdownMenuItem key={style.id} onClick={() => style.run(editor, () => {})}>
                <Icon className="size-4" />
                <span className="flex-1">{style.label}</span>
                {style.hint && (
                  <span className="text-muted-foreground font-mono text-xs">{style.hint}</span>
                )}
              </DropdownMenuItem>
            );
          })}
        </DropdownMenuGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function LinkButton({
  editor,
  isActive,
  disabled,
  open,
  onOpenChange,
}: {
  editor: Editor;
  isActive: boolean;
  disabled: boolean;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  return (
    <Popover open={open} onOpenChange={onOpenChange}>
      <SimpleTooltip content={<TipContent label="Link" keys={["Mod+K"]} />}>
        <PopoverTrigger
          disabled={disabled}
          render={
            <Button
              variant="ghost"
              size="icon"
              forceMinSize={false}
              aria-label="Link"
              aria-pressed={isActive}
              className={cn("text-muted-foreground", isActive && "bg-accent text-foreground")}
            >
              <LinkIcon className="size-4" />
            </Button>
          }
        />
      </SimpleTooltip>
      <PopoverContent align="start" className="w-80" finalFocus={false}>
        {open && <LinkForm editor={editor} onDone={() => onOpenChange(false)} />}
      </PopoverContent>
    </Popover>
  );
}

export function normalizeUrl(value: string) {
  const url = value.trim();
  if (!url) return "";
  if (/^[a-z][a-z0-9+.-]*:/i.test(url) || url.startsWith("/") || url.startsWith("#")) return url;
  if (url.includes("@") && !url.includes("/")) return `mailto:${url}`;
  return `https://${url}`;
}

function LinkForm({ editor, onDone }: { editor: Editor; onDone: () => void }) {
  const current = (editor.getAttributes("link").href as string | undefined) ?? "";
  const [value, setValue] = useState(current);
  const finish = () => {
    onDone();
    editor.commands.focus();
  };
  const submit = (event: FormEvent) => {
    event.preventDefault();
    const href = normalizeUrl(value);
    if (!href) {
      editor.chain().focus().extendMarkRange("link").unsetLink().run();
      finish();
      return;
    }
    if (editor.state.selection.empty && !editor.isActive("link")) {
      editor
        .chain()
        .focus()
        .insertContent({
          type: "text",
          text: value.trim(),
          marks: [{ type: "link", attrs: { href } }],
        })
        .run();
      finish();
      return;
    }
    editor.chain().focus().extendMarkRange("link").setLink({ href }).run();
    finish();
  };
  return (
    <form onSubmit={submit} className="flex flex-col gap-2">
      <Input
        autoFocus
        aria-label="Link URL"
        placeholder="Paste or type a link"
        value={value}
        onChange={(event) => setValue(event.target.value)}
      />
      <div className="flex items-center justify-end gap-2">
        {current && (
          <Button
            type="button"
            variant="ghost-destructive"
            size="sm"
            onClick={() => {
              editor.chain().focus().extendMarkRange("link").unsetLink().run();
              finish();
            }}
          >
            Remove link
          </Button>
        )}
        <Button type="submit" size="sm" disabled={!value.trim() && !current}>
          {current ? "Update" : "Add link"}
        </Button>
      </div>
    </form>
  );
}

const tips: { keys: string[]; text: ReactNode }[] = [
  { keys: ["/"], text: "Headings, lists, tables, images, videos and more" },
  { keys: ["@"], text: "Mention someone; they're notified" },
  { keys: [":"], text: "Emoji, like :tada" },
  { keys: ["Mod+K"], text: "Link the selected text" },
];

const markdownTips = [
  ["# ", "Heading"],
  ["- ", "Bulleted list"],
  ["1. ", "Numbered list"],
  ["[] ", "Checklist"],
  ["> ", "Quote"],
  ["```", "Code block"],
  ["---", "Divider"],
  ["**bold**", "Bold"],
  ["_italic_", "Italic"],
  ["~~strike~~", "Strike"],
  ["`code`", "Inline code"],
] as const;

function EditorHelp() {
  return (
    <Popover>
      <SimpleTooltip content="Editor help">
        <PopoverTrigger
          render={
            <Button
              variant="ghost"
              size="icon"
              forceMinSize={false}
              aria-label="Editor help"
              className="text-muted-foreground shrink-0"
            >
              <HelpCircleIcon className="size-4" />
            </Button>
          }
        />
      </SimpleTooltip>
      <PopoverContent align="end" className="flex w-80 flex-col gap-4 text-sm" finalFocus={false}>
        <section className="flex flex-col gap-2">
          <h3 className="text-muted-foreground dank:type-title-small text-xs font-medium">
            While you write
          </h3>
          {tips.map((tip) => (
            <div key={tip.keys.join()} className="flex items-center gap-2">
              <ShortcutKeys keys={tip.keys} />
              <span>{tip.text}</span>
            </div>
          ))}
        </section>
        <section className="flex flex-col gap-2">
          <h3 className="text-muted-foreground dank:type-title-small text-xs font-medium">
            Images, videos and files
          </h3>
          <p>
            Paste, drop or upload them from the toolbar. Hover one to move it by its handle, align
            it, resize it from its edges or open it full size. Click between two to type there.
          </p>
        </section>
        <section className="flex flex-col gap-2">
          <h3 className="text-muted-foreground dank:type-title-small text-xs font-medium">
            Markdown
          </h3>
          <div className="grid grid-cols-2 gap-x-3 gap-y-1.5">
            {markdownTips.map(([syntax, label]) => (
              <div key={label} className="flex items-center gap-2">
                <Kbd className="font-mono">{syntax.trim()}</Kbd>
                <span className="text-muted-foreground truncate text-xs">{label}</span>
              </div>
            ))}
          </div>
        </section>
      </PopoverContent>
    </Popover>
  );
}

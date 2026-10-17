import { CodeBlockLowlight } from "@tiptap/extension-code-block-lowlight";
import { Image } from "@tiptap/extension-image";
import { Link } from "@tiptap/extension-link";
import { Placeholder } from "@tiptap/extension-placeholder";
import { TaskItem } from "@tiptap/extension-task-item";
import { TaskList } from "@tiptap/extension-task-list";
import { Markdown } from "@tiptap/markdown";
import { type Editor, EditorContent, useEditor } from "@tiptap/react";
import { StarterKit } from "@tiptap/starter-kit";
import { common, createLowlight } from "lowlight";
import { XIcon } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";

import { Avatar } from "@/components/shared/avatar";
import { Button } from "@/components/ui/button";
import { MarkdownMention } from "@/components/shared/markdown-mention";
import { SuggestionMenu, type TSuggestionItem } from "@/components/ui/suggestion-menu";
import { toast } from "@/components/ui/toast";
import useDebouncedCallback from "@/lib/hooks/use-debounced-callback";

const lowlight = createLowlight(common);
const AUTOSAVE_DELAY_MS = 800;
const MAX_MENTION_RESULTS = 8;

type TSaveStatus = "idle" | "saving" | "saved" | "error";

/** Someone who can be @mentioned. */
export type TMentionUser = {
  id: string;
  name: string;
  displayName: string;
  avatarUrl?: string | null;
};

type TMentionState = {
  items: TSuggestionItem[];
  index: number;
  rect: DOMRect | null;
  command: (item: TSuggestionItem) => void;
};

type TProps = {
  value: string;
  onChange?: (markdown: string) => void;
  placeholder?: string;
  users?: TMentionUser[];
  onUploadImage?: (file: File) => Promise<string>;
  readOnly?: boolean;
  /**
   * Saves a moment after you stop writing. `value` is then the server's text: a new one replaces
   * the content when you have no unsaved edits, and is offered otherwise.
   */
  autosave?: {
    /** `base` is the server's text the edit started from; resolves to the server's text after. */
    onSave: (markdown: string, base: string) => Promise<string>;
    delayMs?: number;
  };
  onSubmit?: () => void;
  autoFocus?: boolean;
  label?: string;
  framed?: boolean;
};

export function MarkdownEditor({
  value,
  onChange,
  placeholder = "Write something…",
  users,
  onUploadImage,
  readOnly = false,
  autosave,
  onSubmit,
  autoFocus,
  label = "Editor",
  framed = false,
}: TProps) {
  const usersRef = useRef(users ?? []);
  const uploadRef = useRef(onUploadImage);
  const submitRef = useRef(onSubmit);
  const changeRef = useRef(onChange);
  const editorRef = useRef<Editor | null>(null);
  const lastMarkdownRef = useRef(value);
  const savedMarkdownRef = useRef<string | null>(null);
  const baseRef = useRef(value);
  const isSavingRef = useRef(false);
  const [incoming, setIncoming] = useState<string | null>(null);
  const [mention, setMention] = useState<TMentionState | null>(null);
  const mentionRef = useRef<TMentionState | null>(null);
  const [saveStatus, setSaveStatus] = useState<TSaveStatus>("idle");

  useEffect(() => {
    usersRef.current = users ?? [];
    uploadRef.current = onUploadImage;
    submitRef.current = onSubmit;
    changeRef.current = onChange;
  });

  const updateMention = (next: TMentionState | null) => {
    mentionRef.current = next;
    setMention(next);
  };

  const save = useDebouncedCallback(async (markdown: string) => {
    if (!autosave || markdown === savedMarkdownRef.current) return;
    setSaveStatus("saving");
    isSavingRef.current = true;
    try {
      const serverText = await autosave.onSave(markdown, baseRef.current);
      baseRef.current = markdown;
      savedMarkdownRef.current = markdown;
      isSavingRef.current = false;
      setSaveStatus("saved");
      setIncoming((current) => (current === markdown || current === serverText ? null : current));
      adoptServerText(serverText);
    } catch (error) {
      setSaveStatus("error");
      toast.add({
        type: "error",
        title: "Could not save",
        description: error instanceof Error ? error.message : undefined,
      });
    } finally {
      isSavingRef.current = false;
    }
  }, autosave?.delayMs ?? AUTOSAVE_DELAY_MS);

  const hasAutosave = Boolean(autosave);
  useEffect(() => {
    if (!hasAutosave) return;
    const flushBeforeUnload = (event: BeforeUnloadEvent) => {
      if (!save.isPending() && !isSavingRef.current) return;
      save.flush();
      event.preventDefault();
    };
    window.addEventListener("beforeunload", flushBeforeUnload);
    return () => window.removeEventListener("beforeunload", flushBeforeUnload);
  }, [hasAutosave, save]);

  const extensions = useMemo(
    () => [
      StarterKit.configure({ codeBlock: false, link: false }),
      Link.configure({ openOnClick: false, autolink: true }),
      CodeBlockLowlight.configure({ lowlight }),
      TaskList,
      TaskItem.configure({ nested: true }),
      Image,
      Placeholder.configure({ placeholder }),
      Markdown,
      MarkdownMention.configure({
        HTMLAttributes: { class: "mention" },
        suggestion: {
          char: "@",
          items: ({ query }) => mentionItems(usersRef.current, query),
          render: () => ({
            onStart: (props) =>
              updateMention({
                items: props.items,
                index: 0,
                rect: props.clientRect?.() ?? null,
                command: (item) => props.command({ id: item.id, label: item.label }),
              }),
            onUpdate: (props) =>
              updateMention({
                items: props.items,
                index: 0,
                rect: props.clientRect?.() ?? null,
                command: (item) => props.command({ id: item.id, label: item.label }),
              }),
            onKeyDown: ({ event }) => handleMentionKey(event, mentionRef.current, updateMention),
            onExit: () => updateMention(null),
          }),
        },
      }),
    ],
    [placeholder],
  );

  const insertImages = async (files: File[]) => {
    const upload = uploadRef.current;
    const editor = editorRef.current;
    if (!upload || !editor) return;
    for (const file of files) {
      try {
        const src = await upload(file);
        editor.chain().focus().setImage({ src, alt: file.name }).run();
      } catch (error) {
        toast.add({
          type: "error",
          title: "Image upload failed",
          description: error instanceof Error ? error.message : undefined,
        });
      }
    }
  };

  const editor = useEditor({
    extensions,
    content: value,
    contentType: "markdown",
    editable: !readOnly,
    autofocus: autoFocus ? "end" : false,
    shouldRerenderOnTransaction: false,
    editorProps: {
      attributes: { "aria-label": label, class: "min-h-16 outline-none" },
      handleKeyDown: (_view, event) => {
        if (event.key !== "Enter" || !(event.metaKey || event.ctrlKey)) return false;
        if (!submitRef.current) return false;
        submitRef.current();
        return true;
      },
      handlePaste: (_view, event) => {
        const files = imageFiles(event.clipboardData?.files);
        if (files.length === 0 || !uploadRef.current) return false;
        void insertImages(files);
        return true;
      },
      handleDrop: (_view, event) => {
        const files = imageFiles(event.dataTransfer?.files);
        if (files.length === 0 || !uploadRef.current) return false;
        event.preventDefault();
        void insertImages(files);
        return true;
      },
    },
    onCreate: ({ editor: created }) => {
      savedMarkdownRef.current = created.getMarkdown();
    },
    onUpdate: ({ editor: updated, transaction, appendedTransactions }) => {
      const docChanged = transaction.docChanged || appendedTransactions.some((tr) => tr.docChanged);
      if (!docChanged) return;
      const markdown = updated.getMarkdown();
      lastMarkdownRef.current = markdown;
      changeRef.current?.(markdown);
      if (!autosave) return;
      if (markdown === savedMarkdownRef.current) {
        save.cancel();
        return;
      }
      save.call(markdown);
    },
  });

  useEffect(() => {
    editorRef.current = editor;
  }, [editor]);

  useEffect(() => {
    if (!editor || editor.isDestroyed) return;
    editor.setEditable(!readOnly, false);
  }, [editor, readOnly]);

  useEffect(() => {
    if (!editor || editor.isDestroyed || hasAutosave) return;
    if (value === lastMarkdownRef.current) return;
    lastMarkdownRef.current = value;
    editor.commands.setContent(value, { contentType: "markdown", emitUpdate: false });
    savedMarkdownRef.current = editor.getMarkdown();
  }, [editor, value, hasAutosave]);

  /** Takes the server's text when you have no unsaved edits, and offers it otherwise. */
  function adoptServerText(text: string) {
    const current = editorRef.current;
    if (!current || current.isDestroyed || text === baseRef.current) return;
    const markdown = current.getMarkdown();
    const hasUnsavedEdits =
      save.isPending() || isSavingRef.current || markdown !== savedMarkdownRef.current;
    if (hasUnsavedEdits && text !== markdown) {
      setIncoming(text);
      return;
    }
    takeServerText(current, text);
  }
  function takeServerText(current: Editor, text: string) {
    if (text !== current.getMarkdown()) replaceContent(current, text);
    baseRef.current = text;
    savedMarkdownRef.current = current.getMarkdown();
    lastMarkdownRef.current = savedMarkdownRef.current;
    setIncoming(null);
  }
  const adoptRef = useRef(adoptServerText);
  useEffect(() => {
    adoptRef.current = adoptServerText;
  });
  useEffect(() => {
    if (!editor || !hasAutosave) return;
    adoptRef.current(value);
  }, [editor, value, hasAutosave]);

  const showIncoming = () => {
    if (!editor || incoming === null) return;
    setIncoming(null);
    if (save.isPending()) {
      save.cancel();
      takeServerText(editor, incoming);
      return;
    }
    editor.commands.setContent(incoming, { contentType: "markdown" });
  };

  return (
    <div data-slot="markdown-editor" className="flex w-full flex-col gap-1.5">
      <EditorContent
        editor={editor}
        data-framed={framed || undefined}
        className="rr-prose data-framed:bg-input data-framed:has-hover:hover:ring-border data-framed:focus-within:ring-primary/8-10 w-full data-framed:rounded-md data-framed:border data-framed:px-3 data-framed:py-2 data-framed:focus-within:ring-1 data-framed:has-hover:hover:ring-1"
      />
      {mention && (
        <SuggestionMenu
          items={mention.items}
          selectedIndex={mention.index}
          rect={mention.rect}
          onSelect={mention.command}
          emptyText="No matching people"
        />
      )}
      {autosave && (
        <div className="flex min-h-4 items-center gap-2">
          {incoming !== null && (
            <IncomingNotice onShow={showIncoming} onDismiss={() => setIncoming(null)} />
          )}
          <SaveIndicator status={saveStatus} />
        </div>
      )}
    </div>
  );
}

/** Keeps the cursor about where it was when the server's text replaces the content. */
function replaceContent(editor: Editor, markdown: string) {
  const { from, to } = editor.state.selection;
  editor.commands.setContent(markdown, { contentType: "markdown", emitUpdate: false });
  if (!editor.isFocused) return;
  const end = editor.state.doc.content.size;
  editor.commands.setTextSelection({ from: Math.min(from, end), to: Math.min(to, end) });
}

function IncomingNotice({ onShow, onDismiss }: { onShow: () => void; onDismiss: () => void }) {
  return (
    <div role="status" className="text-muted-foreground flex items-center gap-1.5 text-xs">
      <span>Updated by someone else.</span>
      <Button
        variant="link"
        size="sm"
        forceMinSize={false}
        className="h-auto px-0 text-xs"
        onClick={onShow}
      >
        Show their version
      </Button>
      <Button
        variant="ghost"
        size="icon"
        forceMinSize={false}
        aria-label="Keep your version"
        className="size-5"
        onClick={onDismiss}
      >
        <XIcon className="size-3.5" />
      </Button>
    </div>
  );
}

export function MarkdownField({
  label,
  ...props
}: Omit<TProps, "framed" | "autosave"> & { label: string }) {
  return (
    <div className="flex min-w-0 flex-col gap-1.5">
      <span className="text-sm font-medium">{label}</span>
      <MarkdownEditor {...props} label={label} framed />
    </div>
  );
}

function SaveIndicator({ status }: { status: TSaveStatus }) {
  const text = saveStatusText[status];
  return (
    <p aria-live="polite" className="text-muted-foreground ml-auto h-4 text-xs">
      {text}
    </p>
  );
}

const saveStatusText: Record<TSaveStatus, string> = {
  idle: "",
  saving: "Saving…",
  saved: "Saved",
  error: "Not saved",
};

function imageFiles(list: FileList | null | undefined) {
  return Array.from(list ?? []).filter((file) => file.type.startsWith("image/"));
}

function mentionItems(users: TMentionUser[], query: string): TSuggestionItem[] {
  const needle = query.toLowerCase();
  return users
    .filter(
      (user) =>
        user.name.toLowerCase().includes(needle) || user.displayName.toLowerCase().includes(needle),
    )
    .slice(0, MAX_MENTION_RESULTS)
    .map((user) => ({
      id: user.id,
      label: user.displayName,
      description: user.name !== user.displayName ? user.name : undefined,
      icon: <Avatar id={user.id} name={user.name} src={user.avatarUrl} size="xs" />,
    }));
}

function handleMentionKey(
  event: KeyboardEvent,
  state: TMentionState | null,
  update: (next: TMentionState | null) => void,
) {
  if (!state) return false;
  const count = state.items.length;
  switch (event.key) {
    case "ArrowDown":
      if (count === 0) return true;
      update({ ...state, index: (state.index + 1) % count });
      return true;
    case "ArrowUp":
      if (count === 0) return true;
      update({ ...state, index: (state.index - 1 + count) % count });
      return true;
    case "Enter":
    case "Tab": {
      const item = state.items[state.index];
      if (!item) return false;
      state.command(item);
      return true;
    }
    case "Escape":
      update(null);
      return true;
    default:
      return false;
  }
}

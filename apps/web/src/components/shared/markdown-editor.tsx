import { Link } from "@tiptap/extension-link";
import { Placeholder } from "@tiptap/extension-placeholder";
import { TableKit } from "@tiptap/extension-table";
import { TaskItem } from "@tiptap/extension-task-item";
import { TaskList } from "@tiptap/extension-task-list";
import { Markdown } from "@tiptap/markdown";
import { type AnyExtension, type Editor, EditorContent, useEditor } from "@tiptap/react";
import { StarterKit } from "@tiptap/starter-kit";
import { type DragEvent, type ReactNode, useEffect, useMemo, useRef, useState } from "react";

import { CirclePlusIcon, UploadIcon, XIcon } from "@/components/icons";
import { Avatar } from "@/components/shared/avatar";
import { BlockGaps } from "@/components/shared/editor/block-gaps";
import { EditorCodeBlock } from "@/components/shared/editor/code-block";
import { SlashCommands, type TPickFiles } from "@/components/shared/editor/commands";
import { EmojiSuggestions } from "@/components/shared/editor/emoji";
import { LinkBubble } from "@/components/shared/editor/link-bubble";
import { EditorMedia } from "@/components/shared/editor/media-node";
import { suggestionRenderer, type TSuggestionState } from "@/components/shared/editor/suggestions";
import { EditorToolbar } from "@/components/shared/editor/toolbar";
import { useIsActive } from "@/components/shared/editor/use-is-active";
import {
  editorHasPendingUploads,
  hasPendingUploads,
  insertUploads,
  type TEditorUpload,
} from "@/components/shared/editor/uploads";
import { MarkdownMention } from "@/components/shared/markdown-mention";
import { Button } from "@/components/ui/button";
import { useFilePicker } from "@/components/ui/file-trigger";
import { SuggestionMenu, type TSuggestionItem } from "@/components/ui/suggestion-menu";
import { toast } from "@/components/ui/toast";
import useDebouncedCallback from "@/lib/hooks/use-debounced-callback";
import { mediaAccept } from "@/lib/helpers/media";

const AUTOSAVE_DELAY_MS = 800;
const MAX_MENTION_RESULTS = 8;
const NO_EXTENSIONS: AnyExtension[] = [];

type TSaveStatus = "idle" | "saving" | "saved" | "error";

/** Someone who can be @mentioned. */
/** More for one editor: extensions (keep the array stable) and what renders with it, like a bubble menu. */
export type TEditorAddons = {
  extensions: AnyExtension[];
  render: (editor: Editor) => ReactNode;
};

export type TMentionUser = {
  id: string;
  name: string;
  displayName: string;
  avatarUrl?: string | null;
};

type TProps = {
  value: string;
  onChange?: (markdown: string) => void;
  placeholder?: string;
  users?: TMentionUser[];
  onUpload?: TEditorUpload;
  /** Called when uploads start and when the last one ends; their embeds aren't in `value` until then. */
  onUploadingChange?: (isUploading: boolean) => void;
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
  toolbar?: boolean;
  addons?: TEditorAddons;
  /** Like a chat box: one line tall, no toolbar, Enter sends and Shift+Enter starts a new line. */
  chat?: boolean;
};

export function MarkdownEditor({
  value,
  onChange,
  placeholder = "Write something…",
  users,
  onUpload,
  onUploadingChange,
  readOnly = false,
  autosave,
  onSubmit,
  autoFocus,
  label = "Editor",
  framed = false,
  toolbar = true,
  addons,
  chat = false,
}: TProps) {
  const usersRef = useRef(users ?? []);
  const uploadRef = useRef(onUpload);
  const canUploadRef = useRef(Boolean(onUpload));
  const uploadingChangeRef = useRef(onUploadingChange);
  const isUploadingRef = useRef(false);
  const submitRef = useRef(onSubmit);
  const changeRef = useRef(onChange);
  const editorRef = useRef<Editor | null>(null);
  const lastMarkdownRef = useRef(value);
  const savedMarkdownRef = useRef<string | null>(null);
  const baseRef = useRef(value);
  const isSavingRef = useRef(false);
  const insertAtRef = useRef<number | undefined>(undefined);
  const dragDepthRef = useRef(0);
  const [incoming, setIncoming] = useState<string | null>(null);
  const rootRef = useRef<HTMLDivElement>(null);
  const isActive = useIsActive(rootRef, Boolean(autoFocus));
  const [suggestion, setSuggestion] = useState<TSuggestionState | null>(null);
  const suggestionRef = useRef<TSuggestionState | null>(null);
  const suggestionSinkRef = useRef((next: TSuggestionState | null) => {
    suggestionRef.current = next;
    setSuggestion(next);
  });
  const [saveStatus, setSaveStatus] = useState<TSaveStatus>("idle");
  const [isLinkOpen, setIsLinkOpen] = useState(false);
  const [isDraggingFiles, setIsDraggingFiles] = useState(false);

  useEffect(() => {
    usersRef.current = users ?? [];
    uploadRef.current = onUpload;
    canUploadRef.current = Boolean(onUpload);
    uploadingChangeRef.current = onUploadingChange;
    submitRef.current = onSubmit;
    changeRef.current = onChange;
  });

  const uploadFiles = (files: File[], at?: number) => {
    const upload = uploadRef.current;
    const editor = editorRef.current;
    if (!upload || !editor || files.length === 0) return;
    insertUploads(editor, files, upload, at);
  };
  const onPicked = (files: File[]) => {
    uploadFiles(files, insertAtRef.current);
    insertAtRef.current = undefined;
  };
  const mediaPicker = useFilePicker({ accept: mediaAccept, multiple: true, onSelect: onPicked });
  const filePicker = useFilePicker({ multiple: true, onSelect: onPicked });
  const pickFiles: TPickFiles = (kind) => {
    insertAtRef.current = editorRef.current?.state.selection.from;
    if (kind === "media") mediaPicker.open();
    else filePicker.open();
  };
  const pickFilesRef = useRef(pickFiles);
  useEffect(() => {
    pickFilesRef.current = pickFiles;
  });

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
    const flushBeforeUnload = (event: BeforeUnloadEvent) => {
      const isSaving = hasAutosave && (save.isPending() || isSavingRef.current);
      if (!isSaving && !hasPendingUploads()) return;
      if (hasAutosave) save.flush();
      event.preventDefault();
    };
    window.addEventListener("beforeunload", flushBeforeUnload);
    return () => window.removeEventListener("beforeunload", flushBeforeUnload);
  }, [hasAutosave, save]);

  const extraExtensions = addons?.extensions ?? NO_EXTENSIONS;
  const extensions = useMemo(
    () => [
      StarterKit.configure({
        codeBlock: false,
        link: false,
        underline: false,
        dropcursor: { class: "rr-dropcursor", width: 2 },
      }),
      Link.configure({ openOnClick: false, autolink: true, defaultProtocol: "https" }),
      EditorCodeBlock,
      TaskList,
      TaskItem.configure({ nested: true }),
      TableKit.configure({ table: { resizable: false } }),
      EditorMedia,
      BlockGaps,
      Placeholder.configure({
        placeholder: ({ editor, node }) => {
          if (editor.isEmpty) return placeholder;
          if (node.type.name === "heading") return `Heading ${node.attrs.level}`;
          return node.type.name === "paragraph" ? "Type / for commands" : "";
        },
      }),
      Markdown,
      MarkdownMention.configure({
        HTMLAttributes: { class: "mention" },
        suggestion: {
          char: "@",
          items: ({ query }) => mentionItems(usersRef.current, query),
          render: suggestionRenderer<TSuggestionItem>(
            suggestionSinkRef,
            suggestionRef,
            "No matching people",
            { toProps: (item) => ({ id: item.id, label: item.label }) },
          ),
        },
      }),
      SlashCommands.configure({
        sink: suggestionSinkRef,
        state: suggestionRef,
        pickFiles: pickFilesRef,
        canUpload: canUploadRef,
      }),
      EmojiSuggestions.configure({ sink: suggestionSinkRef, state: suggestionRef }),
      ...extraExtensions,
    ],
    [placeholder, extraExtensions],
  );

  const editor = useEditor({
    extensions,
    content: value,
    contentType: "markdown",
    editable: !readOnly,
    autofocus: autoFocus ? "end" : false,
    shouldRerenderOnTransaction: false,
    editorProps: {
      attributes: {
        "aria-label": label,
        class: chat ? "min-h-6 outline-none" : "min-h-16 outline-none",
      },
      handleKeyDown: (_view, event) => {
        const isMod = event.metaKey || event.ctrlKey;
        if (
          chat &&
          isPlainEnter(event) &&
          !suggestionRef.current &&
          !isInBlock(editorRef.current)
        ) {
          return submitFromKeyboard();
        }
        if (!isMod) return false;
        if (event.key === "Enter") return submitFromKeyboard();
        if (event.key.toLowerCase() !== "k" || event.shiftKey || event.altKey) return false;
        const current = editorRef.current;
        if (!current || (current.state.selection.empty && !current.isActive("link"))) return false;
        event.stopPropagation();
        setIsLinkOpen(true);
        return true;
      },
      handleClick: (view, _pos, event) => {
        if (!(event.metaKey || event.ctrlKey) && view.editable) return false;
        const anchor = (event.target as HTMLElement | null)?.closest("a[href]");
        if (!(anchor instanceof HTMLAnchorElement)) return false;
        window.open(anchor.href, "_blank", "noopener");
        return true;
      },
      handlePaste: (_view, event) => {
        const files = Array.from(event.clipboardData?.files ?? []);
        if (files.length === 0 || !uploadRef.current) return false;
        uploadFiles(files);
        return true;
      },
      handleDrop: (view, event, _slice, moved) => {
        if (moved) return false;
        const files = Array.from(event.dataTransfer?.files ?? []);
        if (files.length === 0 || !uploadRef.current) return false;
        event.preventDefault();
        const at = view.posAtCoords({ left: event.clientX, top: event.clientY })?.pos;
        uploadFiles(files, at);
        return true;
      },
    },
    onCreate: ({ editor: created }) => {
      savedMarkdownRef.current = created.getMarkdown();
    },
    onUpdate: ({ editor: updated, transaction, appendedTransactions }) => {
      const docChanged = transaction.docChanged || appendedTransactions.some((tr) => tr.docChanged);
      if (!docChanged) return;
      reportUploading(updated);
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

  function reportUploading(current: Editor) {
    const isUploading = editorHasPendingUploads(current);
    if (isUploading === isUploadingRef.current) return;
    isUploadingRef.current = isUploading;
    uploadingChangeRef.current?.(isUploading);
  }

  function submitFromKeyboard() {
    if (!submitRef.current) return false;
    if (isUploadingRef.current) {
      toast.add({ type: "info", title: "Wait for the upload to finish" });
      return true;
    }
    submitRef.current();
    return true;
  }

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
      save.isPending() ||
      isSavingRef.current ||
      markdown !== savedMarkdownRef.current ||
      editorHasPendingUploads(current);
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

  const canUpload = Boolean(onUpload) && !readOnly;
  const isFileDrag = (event: DragEvent) => canUpload && event.dataTransfer.types.includes("Files");
  const dragHandlers = {
    onDragEnter: (event: DragEvent) => {
      if (!isFileDrag(event)) return;
      dragDepthRef.current += 1;
      setIsDraggingFiles(true);
    },
    onDragLeave: (event: DragEvent) => {
      if (!isFileDrag(event)) return;
      dragDepthRef.current = Math.max(0, dragDepthRef.current - 1);
      if (dragDepthRef.current === 0) setIsDraggingFiles(false);
    },
    onDrop: () => {
      dragDepthRef.current = 0;
      setIsDraggingFiles(false);
    },
  };

  const content = (
    <EditorContent
      editor={editor}
      data-framed={framed || undefined}
      className="rr-prose w-full data-chat:py-0.5 data-framed:px-3 data-framed:py-2"
      data-chat={chat || undefined}
    />
  );
  const showToolbar = toolbar && !chat && !readOnly && editor && (isActive || isLinkOpen);

  return (
    <div
      ref={rootRef}
      data-slot="markdown-editor"
      className="relative flex w-full flex-col gap-1.5"
      {...dragHandlers}
    >
      <div
        data-framed={framed || undefined}
        data-inline={(hasAutosave && !framed && !readOnly) || undefined}
        className="data-framed:bg-input data-framed:has-hover:hover:ring-border data-framed:focus-within:ring-primary/8-10 data-inline:has-hover:hover:ring-border data-inline:focus-within:ring-border flex w-full flex-col transition-shadow data-framed:rounded-md data-framed:border data-framed:focus-within:ring-1 data-framed:has-hover:hover:ring-1 data-inline:-mx-3 data-inline:-my-2 data-inline:w-auto data-inline:rounded-md data-inline:px-3 data-inline:py-2 data-inline:ring-1 data-inline:ring-transparent"
      >
        {showToolbar && (
          <div
            data-framed={framed || undefined}
            data-sticky={hasAutosave || undefined}
            className="bg-background data-framed:bg-input mb-2 border-b pb-1 data-framed:mb-0 data-framed:rounded-t-md data-framed:px-1 data-framed:pt-1 data-sticky:sticky data-sticky:top-11 data-sticky:z-20 group-data-[bar=apart]/page:data-sticky:top-0 group-data-[width=narrow]/page:data-sticky:top-0"
          >
            <EditorToolbar
              editor={editor}
              canUpload={canUpload}
              pickFiles={pickFiles}
              isLinkOpen={isLinkOpen}
              onLinkOpenChange={setIsLinkOpen}
            />
          </div>
        )}
        {chat && canUpload ? (
          <div className="flex w-full items-start gap-2">
            <Button
              type="button"
              variant="ghost"
              size="icon"
              forceMinSize={false}
              aria-label="Attach images or videos"
              className="text-muted-foreground size-7 shrink-0"
              onClick={() => pickFiles("media")}
            >
              <CirclePlusIcon className="size-5" />
            </Button>
            {content}
          </div>
        ) : (
          content
        )}
      </div>
      {isDraggingFiles && (
        <div
          aria-hidden
          className="bg-primary/1-10 border-primary text-primary pointer-events-none absolute inset-0 z-20 flex items-center justify-center gap-1.5 rounded-md border-2 border-dashed text-sm font-medium"
        >
          <UploadIcon className="size-4" />
          Drop to upload
        </div>
      )}
      {suggestion && (
        <SuggestionMenu
          items={suggestion.items}
          selectedIndex={suggestion.index}
          rect={suggestion.rect}
          onSelect={suggestion.command}
          emptyText={suggestion.emptyText}
        />
      )}
      {editor && !readOnly && <LinkBubble editor={editor} onEdit={() => setIsLinkOpen(true)} />}
      {editor && addons?.render(editor)}
      {autosave && (
        <div className="flex min-h-4 items-center gap-2">
          {incoming !== null && (
            <IncomingNotice onShow={showIncoming} onDismiss={() => setIncoming(null)} />
          )}
          <SaveIndicator status={saveStatus} />
        </div>
      )}
      {canUpload && mediaPicker.input}
      {canUpload && filePicker.input}
    </div>
  );
}

function isPlainEnter(event: KeyboardEvent) {
  const hasModifier = event.shiftKey || event.altKey || event.metaKey || event.ctrlKey;
  return event.key === "Enter" && !hasModifier && !event.isComposing;
}

/** Enter keeps making new items and lines in lists, code and quotes. */
function isInBlock(editor: Editor | null) {
  if (!editor) return false;
  return ["codeBlock", "listItem", "taskItem", "blockquote"].some((name) => editor.isActive(name));
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

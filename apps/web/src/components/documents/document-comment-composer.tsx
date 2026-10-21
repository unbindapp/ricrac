import { useState } from "react";

import { useWorkspaceUsers } from "@/components/issues/picker-sources";
import { MarkdownEditor } from "@/components/shared/markdown-editor";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useSaveCommentMutation } from "@/lib/queries/comments";
import { useEditorUploads } from "@/lib/queries/uploads";

const SUGGESTION_BODY = "Suggested an edit.";

type TCommentComposerProps = {
  documentId: string;
  /** Set for a reply. */
  parentId?: string;
  quotedText?: string;
  label: string;
  placeholder: string;
  autoFocus?: boolean;
  onDone?: () => void;
  onCancel?: () => void;
};

export function DocumentCommentComposer({
  documentId,
  parentId,
  quotedText,
  label,
  placeholder,
  autoFocus,
  onDone,
  onCancel,
}: TCommentComposerProps) {
  const { workspace } = useWorkspace();
  const { users } = useWorkspaceUsers();
  const uploads = useEditorUploads(workspace.id);
  const { mutateAsync: saveComment, isPending } = useSaveCommentMutation(workspace.id);
  const [body, setBody] = useState("");
  const [formKey, setFormKey] = useState(0);
  const isReady = Boolean(body.trim()) && !uploads.isUploading;

  const submit = async () => {
    if (!isReady || isPending) return;
    const args = parentId ? { parentId, body } : { documentId, body, quotedText };
    const saved = await saveComment(args).catch(() => null);
    if (!saved) return;
    setBody("");
    setFormKey((key) => key + 1);
    onDone?.();
  };

  return (
    <div className="flex w-full flex-col gap-2">
      <MarkdownEditor
        key={formKey}
        label={label}
        value={body}
        onChange={setBody}
        onSubmit={() => void submit()}
        placeholder={placeholder}
        users={users}
        {...uploads.editorProps}
        autoFocus={autoFocus}
        framed
        toolbar={false}
      />
      <div className="flex items-center justify-end gap-2">
        {onCancel && (
          <Button variant="ghost" size="sm" onClick={onCancel}>
            Cancel
          </Button>
        )}
        <Button size="sm" isPending={isPending} disabled={!isReady} onClick={() => void submit()}>
          {label}
        </Button>
      </div>
    </div>
  );
}

type TSuggestionComposerProps = {
  documentId: string;
  quotedText: string;
  onDone: () => void;
};

/** Suggests text in place of the quoted passage, with an optional note. */
export function SuggestionComposer({ documentId, quotedText, onDone }: TSuggestionComposerProps) {
  const { workspace } = useWorkspace();
  const { users } = useWorkspaceUsers();
  const { mutateAsync: saveComment, isPending } = useSaveCommentMutation(workspace.id);
  const [replacement, setReplacement] = useState(quotedText);
  const [note, setNote] = useState("");
  const isChanged = replacement !== quotedText;

  const submit = async () => {
    if (!isChanged || isPending) return;
    const saved = await saveComment({
      documentId,
      quotedText,
      suggestedText: replacement,
      body: note.trim() || SUGGESTION_BODY,
    }).catch(() => null);
    if (saved) onDone();
  };

  return (
    <div className="flex w-full flex-col gap-2">
      <label className="flex flex-col gap-1.5">
        <span className="text-muted-foreground text-xs">Replace with</span>
        <Textarea
          value={replacement}
          autoFocus
          onChange={(event) => setReplacement(event.target.value)}
          onKeyDown={(event) => {
            if (event.key !== "Enter" || !(event.metaKey || event.ctrlKey)) return;
            event.preventDefault();
            void submit();
          }}
        />
      </label>
      <MarkdownEditor
        label="Note"
        value={note}
        onChange={setNote}
        onSubmit={() => void submit()}
        placeholder="Add a note (optional)…"
        users={users}
        framed
        toolbar={false}
      />
      <div className="flex items-center justify-end gap-2">
        <Button variant="ghost" size="sm" onClick={onDone}>
          Cancel
        </Button>
        <Button size="sm" isPending={isPending} disabled={!isChanged} onClick={() => void submit()}>
          Suggest
        </Button>
      </div>
    </div>
  );
}

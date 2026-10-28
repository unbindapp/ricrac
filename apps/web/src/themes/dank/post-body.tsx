import { useState } from "react";

import type { DiscussionDetail } from "@/api/ops.gen";
import { useCanRewrite } from "@/components/forum/use-can-rewrite";
import { PencilIcon } from "@/components/icons";
import { useWorkspaceUsers } from "@/components/issues/picker-sources";
import { MarkdownEditor } from "@/components/shared/markdown-editor";
import { MarkdownView } from "@/components/shared/markdown-view";
import { Button } from "@/components/ui/button";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useSaveDiscussionMutation } from "@/lib/queries/discussions";
import { useEditorUploads } from "@/lib/queries/uploads";

/** The post's text, edited like a message: from the pencil that shows on hover, then saved. */
export function PostBody({ discussion }: { discussion: DiscussionDetail }) {
  const { workspace } = useWorkspace();
  const canRewrite = useCanRewrite(discussion.creator);
  const [isEditing, setIsEditing] = useState(false);
  const { mutateAsync: save } = useSaveDiscussionMutation(workspace.id);
  const body = discussion.body ?? "";

  if (isEditing) return <PostEditor discussion={discussion} onDone={() => setIsEditing(false)} />;
  return (
    <div data-slot="post-body" className="group/message relative">
      {body ? (
        <MarkdownView
          markdown={body}
          onChange={
            canRewrite
              ? (after) => save({ id: discussion.id, edit: { before: body, after } })
              : undefined
          }
        />
      ) : (
        canRewrite && <p className="text-muted-foreground text-sm">No description yet.</p>
      )}
      {canRewrite && (
        <div
          role="toolbar"
          aria-label="Post actions"
          data-slot="message-tools"
          className="touch:hidden absolute -top-4.5 right-2 z-10 flex gap-0.5 rounded-full p-0.75 opacity-0 group-hover/message:opacity-100 focus-within:opacity-100 has-data-popup-open:opacity-100"
        >
          <SimpleTooltip content="Edit post">
            <Button
              variant="ghost"
              size="icon"
              forceMinSize={false}
              aria-label="Edit post"
              onClick={() => setIsEditing(true)}
            >
              <PencilIcon />
            </Button>
          </SimpleTooltip>
        </div>
      )}
    </div>
  );
}

function PostEditor({ discussion, onDone }: { discussion: DiscussionDetail; onDone: () => void }) {
  const { workspace } = useWorkspace();
  const { users } = useWorkspaceUsers();
  const uploads = useEditorUploads(workspace.id);
  const { mutateAsync: save, isPending } = useSaveDiscussionMutation(workspace.id);
  const original = discussion.body ?? "";
  const [draft, setDraft] = useState(original);
  const submit = async () => {
    if (uploads.isUploading) return;
    if (draft === original) return onDone();
    const saved = await save({ id: discussion.id, edit: { before: original, after: draft } }).catch(
      () => null,
    );
    if (saved) onDone();
  };
  return (
    <div data-slot="post-editor" className="flex flex-col gap-2">
      <MarkdownEditor
        label="Post"
        value={draft}
        onChange={setDraft}
        onSubmit={() => void submit()}
        users={users}
        {...uploads.editorProps}
        autoFocus
        framed
      />
      <div className="flex items-center justify-end gap-2">
        <Button variant="ghost" size="sm" onClick={onDone}>
          Cancel
        </Button>
        <Button
          size="sm"
          isPending={isPending}
          disabled={uploads.isUploading}
          onClick={() => void submit()}
        >
          Save
        </Button>
      </div>
    </div>
  );
}

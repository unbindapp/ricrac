import { formatForDisplay } from "@tanstack/react-hotkeys";
import { useQuery } from "@tanstack/react-query";
import {
  CheckCircle2Icon,
  ChevronRightIcon,
  EllipsisIcon,
  PencilIcon,
  ReplyIcon,
  Trash2Icon,
  UndoIcon,
} from "lucide-react";
import { useState } from "react";

import type { Comment, CommentThread as TCommentThread } from "@/api/ops.gen";
import { userName } from "@/components/issues/grouping";
import { useWorkspaceUsers } from "@/components/issues/picker-sources";
import { UserAvatar } from "@/components/issues/pickers";
import { AddReactionButton, groupReactions, ReactionBar } from "@/components/issues/reaction-bar";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { MarkdownEditor } from "@/components/shared/markdown-editor";
import { MarkdownView } from "@/components/shared/markdown-view";
import { RelativeTime } from "@/components/shared/relative-time";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Kbd } from "@/components/ui/kbd";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import {
  useCommentReactionMutation,
  useDeleteCommentMutation,
  useResolveCommentMutation,
  useSaveCommentMutation,
} from "@/lib/queries/comments";
import { meQuery } from "@/lib/queries/me";
import { useUploadImage } from "@/lib/queries/uploads";

export function CommentComposer({
  issueId,
  parentId,
  placeholder = "Leave a comment…",
  autoFocus,
  onDone,
}: {
  issueId: string;
  parentId?: string;
  placeholder?: string;
  autoFocus?: boolean;
  onDone?: () => void;
}) {
  const { workspace } = useWorkspace();
  const [body, setBody] = useState("");
  const [formKey, setFormKey] = useState(0);
  const { users } = useWorkspaceUsers();
  const uploadImage = useUploadImage(workspace.id);
  const { mutateAsync: saveComment, isPending } = useSaveCommentMutation(workspace.id);
  const submit = async () => {
    if (!body.trim() || isPending) return;
    const saved = await saveComment({ issueId, body, parentId }).catch(() => null);
    if (!saved) return;
    setBody("");
    setFormKey((key) => key + 1);
    onDone?.();
  };
  return (
    <div className="bg-card flex w-full flex-col gap-2 rounded-lg border p-3">
      <MarkdownEditor
        key={formKey}
        label={parentId ? "Reply" : "Comment"}
        value={body}
        onChange={setBody}
        onSubmit={() => void submit()}
        placeholder={placeholder}
        users={users}
        onUploadImage={uploadImage}
        autoFocus={autoFocus}
      />
      <div className="flex items-center justify-end gap-2">
        {onDone && (
          <Button variant="ghost" size="sm" onClick={onDone}>
            Cancel
          </Button>
        )}
        <Button
          size="sm"
          isPending={isPending}
          disabled={!body.trim()}
          onClick={() => void submit()}
        >
          {parentId ? "Reply" : "Comment"}
          <Kbd className="bg-primary-foreground/2-10 text-primary-foreground border-transparent">
            {formatForDisplay("Mod+Enter")}
          </Kbd>
        </Button>
      </div>
    </div>
  );
}

export function CommentThread({ thread, issueId }: { thread: TCommentThread; issueId: string }) {
  const [isReplying, setIsReplying] = useState(false);
  const [isExpanded, setIsExpanded] = useState(false);
  const { canEdit } = useWorkspace();
  const isResolved = Boolean(thread.resolvedAt);
  const author = thread.user ? userName(thread.user) : "a deleted user";

  if (isResolved && !isExpanded) {
    return (
      <Button
        variant="ghost"
        size="sm"
        className="text-muted-foreground self-start font-normal"
        aria-expanded={false}
        onClick={() => setIsExpanded(true)}
      >
        <ChevronRightIcon className="size-3.5" />
        <CheckCircle2Icon className="size-4" />
        Resolved thread from {author}
        {thread.replies.length > 0 &&
          ` and ${thread.replies.length} repl${thread.replies.length === 1 ? "y" : "ies"}`}
      </Button>
    );
  }

  return (
    <div className="bg-card flex w-full flex-col rounded-lg border">
      <CommentItem
        comment={thread}
        thread={thread}
        onReply={canEdit ? () => setIsReplying(true) : undefined}
      />
      {thread.replies.map((reply) => (
        <div key={reply.id} className="border-t">
          <CommentItem comment={reply} />
        </div>
      ))}
      {isReplying ? (
        <div className="border-t p-2">
          <CommentComposer
            issueId={issueId}
            parentId={thread.id}
            placeholder="Leave a reply…"
            autoFocus
            onDone={() => setIsReplying(false)}
          />
        </div>
      ) : (
        canEdit &&
        thread.replies.length > 0 && (
          <div className="border-t px-2 py-1">
            <Button
              variant="ghost"
              size="sm"
              className="text-muted-foreground font-normal"
              onClick={() => setIsReplying(true)}
            >
              <ReplyIcon className="size-4" />
              Reply
            </Button>
          </div>
        )
      )}
      {isResolved && (
        <div className="border-t px-2 py-1">
          <Button
            variant="ghost"
            size="sm"
            className="text-muted-foreground font-normal"
            onClick={() => setIsExpanded(false)}
          >
            Collapse resolved thread
          </Button>
        </div>
      )}
    </div>
  );
}

function CommentItem({
  comment,
  thread,
  onReply,
}: {
  comment: Comment;
  /** Set on the thread's first comment, which resolves the thread. */
  thread?: TCommentThread;
  onReply?: () => void;
}) {
  const { workspace, canEdit, isAdmin } = useWorkspace();
  const [isEditing, setIsEditing] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [draft, setDraft] = useState(comment.body);
  const { data: me } = useQuery(meQuery);
  const { users } = useWorkspaceUsers();
  const uploadImage = useUploadImage(workspace.id);
  const { mutateAsync: saveComment, isPending: isSaving } = useSaveCommentMutation(workspace.id);
  const { mutateAsync: deleteComment } = useDeleteCommentMutation(workspace.id);
  const { mutate: setResolved } = useResolveCommentMutation(workspace.id);
  const { mutate: toggleReaction } = useCommentReactionMutation(workspace.id);
  const isAuthor = Boolean(me && comment.user?.id === me.id);
  const authorName = comment.user ? userName(comment.user) : "Deleted user";
  const reactions = groupReactions(comment.reactions, me?.id);
  const react = (emoji: string, reacted: boolean) =>
    toggleReaction({ commentId: comment.id, emoji, reacted });

  const save = async () => {
    if (!draft.trim()) return;
    await saveComment({ id: comment.id, body: draft });
    setIsEditing(false);
  };

  return (
    <article
      aria-label={`Comment from ${authorName}`}
      className="group/comment group/reactable flex flex-col gap-2 p-3"
    >
      <header className="flex min-h-7 items-center gap-2">
        {comment.user && <UserAvatar user={comment.user} size="sm" />}
        <span className="truncate text-sm font-medium">{authorName}</span>
        <span className="text-muted-foreground text-xs">
          <RelativeTime value={comment.createdAt} />
        </span>
        {comment.editedAt && <span className="text-muted-foreground text-xs">(edited)</span>}
        {comment.resolvedAt && (
          <span className="text-muted-foreground inline-flex items-center gap-1.5 text-xs">
            <CheckCircle2Icon className="size-3.5" />
            Resolved
          </span>
        )}
        <span className="flex-1" />
        {canEdit && reactions.length === 0 && (
          <AddReactionButton reactions={reactions} onToggle={react} revealOnHover />
        )}
        {onReply && (
          <Button
            variant="ghost"
            size="icon"
            aria-label="Reply"
            className="size-7 opacity-0 group-hover/comment:opacity-100 focus-visible:opacity-100"
            onClick={onReply}
          >
            <ReplyIcon className="size-4" />
          </Button>
        )}
        {(isAuthor || isAdmin || (canEdit && thread)) && (
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label="Comment actions"
                  className="size-7 opacity-0 group-hover/comment:opacity-100 focus-visible:opacity-100 data-popup-open:opacity-100"
                >
                  <EllipsisIcon className="size-4" />
                </Button>
              }
            />
            <DropdownMenuContent align="end" className="w-48">
              {thread && canEdit && (
                <DropdownMenuItem
                  onClick={() => setResolved({ id: thread.id, resolved: !thread.resolvedAt })}
                >
                  {thread.resolvedAt ? (
                    <UndoIcon className="size-4" />
                  ) : (
                    <CheckCircle2Icon className="size-4" />
                  )}
                  {thread.resolvedAt ? "Reopen thread" : "Resolve thread"}
                </DropdownMenuItem>
              )}
              {isAuthor && (
                <DropdownMenuItem
                  onClick={() => {
                    setDraft(comment.body);
                    setIsEditing(true);
                  }}
                >
                  <PencilIcon className="size-4" />
                  Edit
                </DropdownMenuItem>
              )}
              {(isAuthor || isAdmin) && (
                <DropdownMenuItem className="text-destructive" onClick={() => setIsDeleting(true)}>
                  <Trash2Icon className="size-4" />
                  Delete
                </DropdownMenuItem>
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </header>
      {comment.quotedText && (
        <blockquote className="text-muted-foreground border-l-2 pl-3 text-sm">
          {comment.quotedText}
        </blockquote>
      )}
      {isEditing ? (
        <div className="flex flex-col gap-2">
          <MarkdownEditor
            label="Edit comment"
            value={draft}
            onChange={setDraft}
            onSubmit={() => void save().catch(() => undefined)}
            users={users}
            onUploadImage={uploadImage}
            autoFocus
            framed
          />
          <div className="flex items-center justify-end gap-2">
            <Button variant="ghost" size="sm" onClick={() => setIsEditing(false)}>
              Cancel
            </Button>
            <Button
              size="sm"
              isPending={isSaving}
              disabled={!draft.trim() || draft === comment.body}
              onClick={() => void save().catch(() => undefined)}
            >
              Save
            </Button>
          </div>
        </div>
      ) : (
        <MarkdownView markdown={comment.body} />
      )}
      {reactions.length > 0 && (
        <ReactionBar reactions={reactions} disabled={!canEdit} onToggle={react} />
      )}
      <ConfirmDialog
        open={isDeleting}
        onOpenChange={setIsDeleting}
        title="Delete comment"
        description={
          thread && thread.replies.length > 0
            ? "The comment and its replies are deleted for everyone."
            : "The comment is deleted for everyone."
        }
        confirmLabel="Delete comment"
        onConfirm={() => deleteComment(comment.id)}
      />
    </article>
  );
}

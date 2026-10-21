import { useQuery } from "@tanstack/react-query";
import {
  CornerDownRightIcon,
  EllipsisIcon,
  MessageSquareReplyIcon,
  PencilIcon,
  SendHorizontalIcon,
  SplitIcon,
  Trash2Icon,
} from "lucide-react";
import { useState } from "react";

import type { Comment, UserRef } from "@/api/ops.gen";
import { userName } from "@/components/issues/grouping";
import { useWorkspaceUsers } from "@/components/issues/picker-sources";
import { UserAvatar } from "@/components/issues/pickers";
import { AddReactionButton, groupReactions, ReactionBar } from "@/components/issues/reaction-bar";
import { useChatScroll } from "@/components/layout/chat-layout";
import { AvatarStack } from "@/components/projects/people";
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
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { formatDateTime, formatDay, formatTime } from "@/lib/helpers/dates";
import {
  type TCommentTarget,
  useCommentReactionMutation,
  useDeleteCommentMutation,
  useSaveCommentMutation,
} from "@/lib/queries/comments";
import { meQuery } from "@/lib/queries/me";
import { useEditorUploads } from "@/lib/queries/uploads";

type TMessageProps = {
  message: Comment;
  /** Follows a message from the same person moments before: no avatar or name. */
  isContinuation?: boolean;
  /** Whom it answers, when replies follow the messages they answer. */
  replyTo?: UserRef | null;
  /** The message's thread, when replies open beside the chat. */
  thread?: { replies: Comment[]; isOpen: boolean; onOpen: () => void };
  onNewThread?: () => void;
};

export function ChatMessage({
  message,
  isContinuation = false,
  replyTo,
  thread,
  onNewThread,
}: TMessageProps) {
  const { workspace, canEdit, isAdmin } = useWorkspace();
  const [isEditing, setIsEditing] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const { data: me } = useQuery(meQuery);
  const { mutateAsync: saveComment } = useSaveCommentMutation(workspace.id);
  const { mutateAsync: deleteComment } = useDeleteCommentMutation(workspace.id);
  const { mutate: toggleReaction } = useCommentReactionMutation(workspace.id);
  const isAuthor = Boolean(me && message.user?.id === me.id);
  const authorName = message.user ? userName(message.user) : "Deleted user";
  const reactions = groupReactions(message.reactions, me?.id);
  const react = (emoji: string, reacted: boolean) =>
    toggleReaction({ commentId: message.id, emoji, reacted });
  const replyCount = thread?.replies.length ?? 0;

  return (
    <article
      id={`comment-${message.id}`}
      aria-label={`Message from ${authorName}`}
      data-continuation={isContinuation || undefined}
      data-current={thread?.isOpen || undefined}
      className="group/message group/reactable has-hover:hover:bg-foreground/2-10 data-current:bg-primary/1-10 relative -mx-2 flex gap-3 rounded-md px-2 py-1 not-data-continuation:mt-2"
    >
      <div className="flex w-8 shrink-0 justify-center">
        {isContinuation ? (
          <SimpleTooltip content={formatDateTime(message.createdAt)}>
            <time
              dateTime={message.createdAt}
              className="text-muted-foreground pt-0.5 text-xs whitespace-nowrap opacity-0 group-hover/message:opacity-100"
            >
              {formatTime(message.createdAt)}
            </time>
          </SimpleTooltip>
        ) : (
          message.user && <UserAvatar user={message.user} size="lg" />
        )}
      </div>
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        {!isContinuation && (
          <header className="flex min-w-0 items-baseline gap-2">
            <span className="truncate text-sm font-semibold">{authorName}</span>
            <span className="text-muted-foreground text-xs">
              <RelativeTime value={message.createdAt} />
            </span>
          </header>
        )}
        {replyTo !== undefined && (
          <span className="text-muted-foreground flex items-center gap-1.5 text-xs">
            <CornerDownRightIcon className="size-3.5" />
            Replying to {replyTo ? userName(replyTo) : "a deleted user"}
          </span>
        )}
        {isEditing ? (
          <MessageEditor
            message={message}
            onDone={() => setIsEditing(false)}
            onSave={(body) => saveComment({ id: message.id, body })}
          />
        ) : (
          <div className="flex min-w-0 flex-col">
            <MarkdownView
              markdown={message.body}
              onChange={
                canEdit && isAuthor ? (body) => saveComment({ id: message.id, body }) : undefined
              }
            />
            {message.editedAt && <span className="text-muted-foreground text-xs">(edited)</span>}
          </div>
        )}
        {reactions.length > 0 && (
          <ReactionBar reactions={reactions} disabled={!canEdit} onToggle={react} />
        )}
        {thread && replyCount > 0 && <ThreadChip replies={thread.replies} onOpen={thread.onOpen} />}
      </div>
      {!isEditing && (canEdit || isAdmin) && (
        <div className="bg-popover absolute -top-4 right-2 flex items-center rounded-md border p-0.5 opacity-0 shadow-sm group-hover/message:opacity-100 focus-within:opacity-100 has-data-popup-open:opacity-100">
          {canEdit && <AddReactionButton reactions={reactions} onToggle={react} />}
          {canEdit && thread && (
            <SimpleTooltip content={replyCount > 0 ? "Open thread" : "Reply in thread"}>
              <Button
                variant="ghost"
                size="icon"
                forceMinSize={false}
                aria-label={replyCount > 0 ? "Open thread" : "Reply in thread"}
                className="size-7"
                onClick={thread.onOpen}
              >
                <MessageSquareReplyIcon className="text-muted-foreground size-4" />
              </Button>
            </SimpleTooltip>
          )}
          {(isAuthor || isAdmin || (canEdit && onNewThread)) && (
            <DropdownMenu>
              <DropdownMenuTrigger
                render={
                  <Button
                    variant="ghost"
                    size="icon"
                    forceMinSize={false}
                    aria-label="Message actions"
                    className="size-7"
                  >
                    <EllipsisIcon className="text-muted-foreground size-4" />
                  </Button>
                }
              />
              <DropdownMenuContent align="end" className="w-56">
                {canEdit && onNewThread && (
                  <DropdownMenuItem onClick={onNewThread}>
                    <SplitIcon className="size-4" />
                    New thread from message
                  </DropdownMenuItem>
                )}
                {isAuthor && canEdit && (
                  <DropdownMenuItem onClick={() => setIsEditing(true)}>
                    <PencilIcon className="size-4" />
                    Edit
                  </DropdownMenuItem>
                )}
                {(isAuthor || isAdmin) && (
                  <DropdownMenuItem
                    className="text-destructive"
                    onClick={() => setIsDeleting(true)}
                  >
                    <Trash2Icon className="size-4" />
                    Delete
                  </DropdownMenuItem>
                )}
              </DropdownMenuContent>
            </DropdownMenu>
          )}
        </div>
      )}
      <ConfirmDialog
        open={isDeleting}
        onOpenChange={setIsDeleting}
        title="Delete message"
        description={
          replyCount > 0
            ? "The message and its thread are deleted for everyone."
            : "The message is deleted for everyone."
        }
        confirmLabel="Delete message"
        onConfirm={() => deleteComment(message.id)}
      />
    </article>
  );
}

function ThreadChip({ replies, onOpen }: { replies: Comment[]; onOpen: () => void }) {
  const last = replies[replies.length - 1];
  const people = [
    ...new Map(
      replies.flatMap((reply) => (reply.user ? [[reply.user.id, reply.user] as const] : [])),
    ).values(),
  ];
  return (
    <Button
      variant="outline"
      size="sm"
      className="max-w-full self-start font-normal"
      onClick={onOpen}
    >
      <AvatarStack users={people} max={3} />
      <span className="text-primary font-medium">
        {replies.length} {replies.length === 1 ? "reply" : "replies"}
      </span>
      <span className="text-muted-foreground truncate text-xs max-sm:hidden">
        Last reply <RelativeTime value={last.createdAt} />
      </span>
    </Button>
  );
}

function MessageEditor({
  message,
  onSave,
  onDone,
}: {
  message: Comment;
  onSave: (body: string) => Promise<unknown>;
  onDone: () => void;
}) {
  const { workspace } = useWorkspace();
  const { users } = useWorkspaceUsers();
  const uploads = useEditorUploads(workspace.id);
  const [draft, setDraft] = useState(message.body);
  const [isSaving, setIsSaving] = useState(false);
  const save = async () => {
    if (!draft.trim() || uploads.isUploading) return;
    if (draft === message.body) return onDone();
    setIsSaving(true);
    const saved = await onSave(draft).catch(() => null);
    setIsSaving(false);
    if (saved) onDone();
  };
  return (
    <div className="flex flex-col gap-2">
      <MarkdownEditor
        label="Edit message"
        value={draft}
        onChange={setDraft}
        onSubmit={() => void save()}
        users={users}
        {...uploads.editorProps}
        autoFocus
        framed
        chat
      />
      <div className="text-muted-foreground flex items-center gap-2 text-xs">
        <span className="max-sm:hidden">Enter to save</span>
        <span className="flex-1" />
        <Button variant="ghost" size="sm" onClick={onDone}>
          Cancel
        </Button>
        <Button
          size="sm"
          isPending={isSaving}
          disabled={!draft.trim() || uploads.isUploading}
          onClick={() => void save()}
        >
          Save
        </Button>
      </div>
    </div>
  );
}

export function ChatDayDivider({ date }: { date: string }) {
  return (
    <div role="separator" className="text-muted-foreground flex items-center gap-2 py-2 text-xs">
      <span className="bg-border h-px flex-1" />
      <span className="font-medium">{formatDay(date)}</span>
      <span className="bg-border h-px flex-1" />
    </div>
  );
}

export function ChatComposer({
  target,
  parentId,
  placeholder,
}: {
  target: TCommentTarget;
  parentId?: string;
  placeholder: string;
}) {
  const { workspace } = useWorkspace();
  const { scrollToEnd } = useChatScroll();
  const [body, setBody] = useState("");
  const [sent, setSent] = useState(0);
  const { users } = useWorkspaceUsers();
  const uploads = useEditorUploads(workspace.id);
  const { mutateAsync: saveComment, isPending } = useSaveCommentMutation(workspace.id);
  const canSend = Boolean(body.trim()) && !uploads.isUploading;
  const send = async () => {
    if (!canSend || isPending) return;
    const saved = await saveComment({ ...target, body, parentId }).catch(() => null);
    if (!saved) return;
    setBody("");
    setSent((count) => count + 1);
    scrollToEnd();
  };
  return (
    <div className="flex flex-col gap-1.5">
      <div className="bg-input focus-within:ring-primary/8-10 flex items-end gap-2 rounded-lg border py-1 pr-1 pl-3 focus-within:ring-1">
        <div className="min-w-0 flex-1 py-1">
          <MarkdownEditor
            key={sent}
            label={parentId ? "Reply" : "Message"}
            value={body}
            onChange={setBody}
            onSubmit={() => void send()}
            placeholder={placeholder}
            users={users}
            {...uploads.editorProps}
            autoFocus={sent > 0}
            chat
          />
        </div>
        <Button
          variant={canSend ? "default" : "ghost"}
          size="icon"
          aria-label="Send"
          isPending={isPending}
          disabled={!canSend}
          onClick={() => void send()}
        >
          <SendHorizontalIcon className="size-4" />
        </Button>
      </div>
      <p className="text-muted-foreground px-1 text-xs max-sm:hidden">
        Enter to send, Shift+Enter for a new line, / for commands
      </p>
    </div>
  );
}

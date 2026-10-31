import { useQuery } from "@tanstack/react-query";
import { type KeyboardEvent, type ReactNode, useState } from "react";

import type { Comment } from "@/api/ops.gen";
import {
  AttachmentTray,
  MessageAttachments,
  useDraftAttachments,
} from "@/components/forum/chat-attachments";
import {
  CirclePlusIcon,
  CopyIcon,
  CornerDownRightIcon,
  EllipsisIcon,
  HistoryIcon,
  ImageIcon,
  MessageSquareReplyIcon,
  PencilIcon,
  PlusIcon,
  ReplyIcon,
  SendHorizontalIcon,
  SplitIcon,
  Trash2Icon,
  XIcon,
} from "@/components/icons";
import { MessageHistoryDialog } from "@/components/forum/message-history";
import { userName } from "@/components/issues/grouping";
import { useWorkspaceUsers } from "@/components/issues/picker-sources";
import { UserAvatar } from "@/components/issues/pickers";
import { AddReactionButton, groupReactions, ReactionBar } from "@/components/issues/reaction-bar";
import { commentExcerpt } from "@/components/inbox/notifications";
import { useChatReply, useChatScroll } from "@/components/layout/chat-layout";
import { AvatarStack } from "@/components/projects/people";
import { useNow } from "@/components/providers/now-provider";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { MarkdownEditor } from "@/components/shared/markdown-editor";
import { MarkdownView } from "@/components/shared/markdown-view";
import { RelativeTime } from "@/components/shared/relative-time";
import { Button } from "@/components/ui/button";
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuTrigger,
  keepNativeMenu,
} from "@/components/ui/context-menu";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { toast } from "@/components/ui/toast";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useFilePicker } from "@/components/ui/file-trigger";
import { cn } from "@/components/ui/utils";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useCopyToClipboard } from "@/lib/hooks/use-copy";
import { joinAttachments, splitAttachments } from "@/lib/helpers/chat";
import { formatDateTime, formatDay, formatMessageTime, formatTime } from "@/lib/helpers/dates";
import {
  type TCommentTarget,
  useCommentReactionMutation,
  useDeleteCommentMutation,
  useSaveCommentMutation,
} from "@/lib/queries/comments";
import { meQuery } from "@/lib/queries/me";
import { useUiTheme } from "@/themes/ui-theme-provider";

type TMessageProps = {
  message: Comment;
  /** Follows a message from the same person moments before: no avatar or name. */
  isContinuation?: boolean;
  /** The message it answers, when replies follow the messages they answer. */
  replyTo?: Comment;
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
  const { inlineReplies, chatBubbles } = useUiTheme();
  const chatReply = useChatReply();
  const [isEditing, setIsEditing] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [isShowingHistory, setIsShowingHistory] = useState(false);
  const { data: me } = useQuery(meQuery);
  const { mutateAsync: saveComment } = useSaveCommentMutation(workspace.id);
  const { mutateAsync: deleteComment } = useDeleteCommentMutation(workspace.id);
  const { mutate: toggleReaction } = useCommentReactionMutation(workspace.id);
  const isAuthor = Boolean(me && message.user?.id === me.id);
  const authorName = message.user ? userName(message.user) : "Deleted user";
  const reactions = groupReactions(message.reactions, me?.id);
  const isDeleted = Boolean(message.deletedAt);
  const isEdited = Boolean(message.editedAt) && !isDeleted;
  const { text, attachments } = splitAttachments(message.body);
  const react = (emoji: string, reacted: boolean) =>
    toggleReaction({ commentId: message.id, emoji, reacted });
  const { copyToClipboard } = useCopyToClipboard();
  const replyCount = thread?.replies.length ?? 0;
  const canStartThread = canEdit && Boolean(onNewThread) && !isDeleted;
  const canChange = isAuthor && canEdit && !isDeleted;
  const canDelete = (isAuthor || isAdmin) && !isDeleted;
  const edit = canChange ? () => setIsEditing(true) : undefined;
  const reply =
    canEdit && chatReply && !isDeleted
      ? () => chatReply.setReplyingTo(replyTo ?? message)
      : undefined;
  const historyItem = isEdited && (
    <DropdownMenuItem onClick={() => setIsShowingHistory(true)}>
      <HistoryIcon className="size-4" />
      History
    </DropdownMenuItem>
  );

  const actionItems = (
    <>
      {canStartThread && (
        <DropdownMenuItem onClick={onNewThread}>
          <SplitIcon className="size-4" />
          New thread from message
        </DropdownMenuItem>
      )}
      {canChange && (
        <DropdownMenuItem onClick={edit}>
          <PencilIcon className="size-4" />
          Edit
        </DropdownMenuItem>
      )}
      {historyItem}
      {canDelete && (
        <DropdownMenuItem className="text-destructive" onClick={() => setIsDeleting(true)}>
          <Trash2Icon className="size-4" />
          Delete
        </DropdownMenuItem>
      )}
    </>
  );

  const hasActions = !isEditing && !isDeleted && (canEdit || isAdmin);
  const isMine = chatBubbles && isAuthor;

  const actionButtons = (
    <>
      {canEdit && <AddReactionButton reactions={reactions} onToggle={react} />}
      {canEdit && thread && (
        <SimpleTooltip content={replyCount > 0 ? "Open thread" : "Reply in thread"}>
          <Button
            variant="ghost"
            size="icon"
            forceMinSize={false}
            aria-label={replyCount > 0 ? "Open thread" : "Reply in thread"}
            className="dank:size-8 dank:[&>svg]:size-5 size-7"
            onClick={thread.onOpen}
          >
            <MessageSquareReplyIcon className="text-muted-foreground size-4" />
          </Button>
        </SimpleTooltip>
      )}
      {(canStartThread || canChange || canDelete || isEdited) && (
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button
                variant="ghost"
                size="icon"
                forceMinSize={false}
                aria-label="Message actions"
                className="dank:size-8 dank:[&>svg]:size-5 size-7"
              >
                <EllipsisIcon className="text-muted-foreground size-4" />
              </Button>
            }
          />
          <DropdownMenuContent align="end" className="w-56">
            <DropdownMenuGroup>{actionItems}</DropdownMenuGroup>
          </DropdownMenuContent>
        </DropdownMenu>
      )}
    </>
  );

  const replyToNote = replyTo && (
    <span className="text-muted-foreground flex items-center gap-1.5 text-xs">
      <CornerDownRightIcon className="size-3.5" />
      Replying to {replyTo.user ? userName(replyTo.user) : "a deleted user"}
    </span>
  );

  const editor = (
    <MessageEditor
      message={message}
      onDone={() => setIsEditing(false)}
      onSave={(body) => saveComment({ id: message.id, body })}
    />
  );

  const textView = (
    <MarkdownView
      markdown={text}
      onChange={
        canEdit && isAuthor
          ? (next) => saveComment({ id: message.id, body: joinAttachments(next, attachments) })
          : undefined
      }
    />
  );

  const deletedNote = isDeleted && (
    <div data-slot="message-bubble" data-deleted className="text-muted-foreground italic">
      Deleted message
    </div>
  );

  const edited = isEdited && (
    <span data-slot="message-edited" className="text-muted-foreground text-xs">
      (edited)
    </span>
  );

  const after = (
    <>
      {reactions.length > 0 && (
        <ReactionBar reactions={reactions} disabled={!canEdit} onToggle={react} revealOnHover />
      )}
      {thread && replyCount > 0 && !inlineReplies && (
        <ThreadChip replies={thread.replies} onOpen={thread.onOpen} />
      )}
    </>
  );

  const bubbleTools = (
    <div
      role="toolbar"
      aria-label="Message actions"
      data-slot="message-tools"
      className={cn(
        "touch:hidden absolute top-1/2 z-10 flex -translate-y-1/2 gap-0.5 rounded-full p-1 opacity-0 group-hover/message:opacity-100 group-data-popup-open/message:opacity-100 focus-within:opacity-100 has-data-popup-open:opacity-100",
        isMine ? "right-full mr-2" : "left-full ml-2",
      )}
    >
      {canEdit && <AddReactionButton reactions={reactions} onToggle={react} />}
      {reply && (
        <SimpleTooltip content="Reply">
          <Button
            variant="ghost"
            size="icon"
            forceMinSize={false}
            aria-label="Reply"
            onClick={reply}
          >
            <ReplyIcon />
          </Button>
        </SimpleTooltip>
      )}
      {canChange && (
        <SimpleTooltip content="Edit">
          <Button variant="ghost" size="icon" forceMinSize={false} aria-label="Edit" onClick={edit}>
            <PencilIcon />
          </Button>
        </SimpleTooltip>
      )}
      {(canStartThread || canDelete || isEdited) && (
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button variant="ghost" size="icon" forceMinSize={false} aria-label="More actions">
                <EllipsisIcon />
              </Button>
            }
          />
          <DropdownMenuContent align={isMine ? "start" : "end"} className="w-56">
            <DropdownMenuGroup>
              {canStartThread && (
                <DropdownMenuItem onClick={onNewThread}>
                  <SplitIcon className="size-4" />
                  New thread from message
                </DropdownMenuItem>
              )}
              {historyItem}
              {canDelete && (
                <DropdownMenuItem className="text-destructive" onClick={() => setIsDeleting(true)}>
                  <Trash2Icon className="size-4" />
                  Delete
                </DropdownMenuItem>
              )}
            </DropdownMenuGroup>
          </DropdownMenuContent>
        </DropdownMenu>
      )}
    </div>
  );

  const bubbleContent = (
    <>
      {replyTo && <ReplyGlimpse message={replyTo} />}
      <div className={cn("flex gap-2.5", isMine && "justify-end")}>
        {!isMine && (
          <div data-slot="message-avatar" className="flex w-9 shrink-0 justify-center">
            {!isContinuation && message.user && <UserAvatar user={message.user} size="lg" />}
          </div>
        )}
        <div
          data-slot="message-body"
          data-editing={isEditing || undefined}
          className={cn(
            "flex min-w-0 flex-col gap-0.5 data-editing:flex-1",
            isMine ? "items-end" : "items-start",
          )}
        >
          {!isContinuation && (
            <header className="flex max-w-full min-w-0 items-baseline gap-2 px-1 pb-0.5">
              <span data-slot="message-name" className="truncate">
                {isMine ? "You" : authorName}
              </span>
              <MessageTime value={message.createdAt} />
            </header>
          )}
          {isEditing ? (
            <div className="w-full">{editor}</div>
          ) : (
            <div
              className={cn(
                "relative flex max-w-full min-w-0 flex-col gap-1",
                isMine ? "items-end" : "items-start",
              )}
              onContextMenu={keepNativeMenu}
              onDoubleClick={edit}
            >
              {deletedNote}
              {text && (
                <div
                  data-slot="message-bubble"
                  className="rr-chat flex max-w-full min-w-0 flex-wrap items-baseline gap-x-1.5"
                >
                  {textView}
                  {edited}
                </div>
              )}
              <MessageAttachments attachments={attachments} />
              {!text && edited}
              {!isDeleted && (canEdit || canDelete) && bubbleTools}
            </div>
          )}
          {reactions.length > 0 && (
            <div data-slot="message-reactions">
              <ReactionBar
                reactions={reactions}
                disabled={!canEdit}
                onToggle={react}
                withAddButton={false}
              />
            </div>
          )}
        </div>
      </div>
    </>
  );

  const rowContent = (
    <>
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
            <span className="dank:type-title-small truncate text-sm font-semibold">
              {authorName}
            </span>
            <MessageTime value={message.createdAt} />
          </header>
        )}
        {replyToNote}
        {isEditing ? (
          editor
        ) : (
          <div
            className="flex min-w-0 flex-col gap-1.5"
            onContextMenu={keepNativeMenu}
            onDoubleClick={edit}
          >
            {isDeleted && (
              <span className="text-muted-foreground text-sm italic">Deleted message</span>
            )}
            {text && <div className="rr-chat flex min-w-0 flex-col">{textView}</div>}
            <MessageAttachments attachments={attachments} />
            {edited}
          </div>
        )}
        {after}
      </div>
      {hasActions && (
        <div className="bg-popover dank:rounded-full dank:border-0 dank:p-1 dank:gap-1 dank:shadow-elevation-2 dank:-top-5 absolute -top-4 right-2 flex items-center rounded-md border p-0.5 opacity-0 shadow-sm group-hover/message:opacity-100 focus-within:opacity-100 has-data-popup-open:opacity-100">
          {actionButtons}
        </div>
      )}
    </>
  );

  return (
    <ContextMenu disabled={isEditing || isDeleted}>
      <ContextMenuTrigger
        render={
          <article
            id={`comment-${message.id}`}
            aria-label={`Message from ${authorName}`}
            data-continuation={isContinuation || undefined}
            data-current={thread?.isOpen || undefined}
            data-side={chatBubbles ? (isMine ? "me" : "them") : undefined}
            className={
              chatBubbles
                ? "group/message group/reactable data-current:bg-m3-secondary-container relative -mx-2 flex flex-col rounded-md px-2 py-0.5 not-data-continuation:mt-2.5"
                : "group/message group/reactable has-hover:hover:bg-foreground/2-10 data-current:bg-primary/1-10 data-popup-open:bg-foreground/2-10 dank:state-layer dank:has-hover:hover:bg-transparent dank:data-popup-open:bg-transparent dank:data-current:bg-m3-secondary-container relative -mx-2 flex gap-3 rounded-md px-2 py-1 not-data-continuation:mt-2"
            }
          />
        }
      >
        {chatBubbles ? bubbleContent : rowContent}
        {isEdited && (
          <MessageHistoryDialog
            target={{ commentId: message.id }}
            open={isShowingHistory}
            onOpenChange={setIsShowingHistory}
          />
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
      </ContextMenuTrigger>
      <ContextMenuContent className="w-56">
        <DropdownMenuGroup>
          {canEdit && thread && (
            <DropdownMenuItem onClick={thread.onOpen}>
              <MessageSquareReplyIcon className="size-4" />
              {replyCount > 0 ? "Open thread" : "Reply in thread"}
            </DropdownMenuItem>
          )}
          <DropdownMenuItem
            onClick={() => {
              copyToClipboard(message.body);
              toast.add({ type: "success", title: "Text copied" });
            }}
          >
            <CopyIcon className="size-4" />
            Copy text
          </DropdownMenuItem>
          {actionItems}
        </DropdownMenuGroup>
      </ContextMenuContent>
    </ContextMenu>
  );
}

/** The message a reply answers, in a line above it that jumps to it. */
function ReplyGlimpse({ message }: { message: Comment }) {
  const { text, attachments } = splitAttachments(message.body);
  const name = message.user ? userName(message.user) : "deleted user";
  const excerpt = commentExcerpt(text);
  return (
    <Button
      variant="ghost"
      forceMinSize={false}
      data-slot="reply-glimpse"
      aria-label={`Replying to ${name}: ${excerpt || "attachment"}`}
      onClick={() =>
        document
          .getElementById(`comment-${message.id}`)
          ?.scrollIntoView({ behavior: "smooth", block: "center" })
      }
    >
      {message.user && <UserAvatar user={message.user} size="xs" />}
      <span data-slot="reply-glimpse-name">@{name}</span>
      {excerpt ? (
        <span data-slot="reply-glimpse-text">{excerpt}</span>
      ) : (
        <>
          <span data-slot="reply-glimpse-text" className="italic">
            Click to see attachment
          </span>
          {attachments.length > 0 && <ImageIcon className="size-4.5" />}
        </>
      )}
    </Button>
  );
}

export function MessageTime({ value }: { value: string }) {
  const now = useNow();
  return (
    <SimpleTooltip content={formatDateTime(value)}>
      <time dateTime={value} className="text-muted-foreground text-xs whitespace-nowrap">
        {formatMessageTime(value, now)}
      </time>
    </SimpleTooltip>
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
      data-m3-chip="assist"
      className="max-w-full self-start font-normal"
      onClick={onOpen}
    >
      <AvatarStack users={people} max={3} />
      <span className="text-primary dank:text-m3-on-surface font-medium">
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
  const [original] = useState(() => splitAttachments(message.body));
  const attachments = useDraftAttachments(workspace.id, original.attachments);
  const [draft, setDraft] = useState(original.text);
  const [isSaving, setIsSaving] = useState(false);
  const body = joinAttachments(draft, attachments.uploaded);
  const canSave = Boolean(body) && !attachments.isUploading;
  const save = async () => {
    if (!canSave) return;
    if (body === message.body) return onDone();
    setIsSaving(true);
    const saved = await onSave(body).catch(() => null);
    setIsSaving(false);
    if (saved) onDone();
  };
  const cancel = (event: KeyboardEvent) => {
    if (event.key !== "Escape" || event.defaultPrevented) return;
    event.stopPropagation();
    onDone();
  };
  return (
    <div className="flex flex-col gap-2" onKeyDown={cancel}>
      <ComposerBox attachments={attachments}>
        <MarkdownEditor
          label="Edit message"
          value={draft}
          onChange={setDraft}
          onSubmit={() => void save()}
          users={users}
          onAttach={attachments.add}
          autoFocus
          chat
        />
      </ComposerBox>
      <div className="text-muted-foreground flex items-center gap-2 text-xs">
        <span className="max-sm:hidden">Enter to save, Escape to cancel</span>
        <span className="flex-1" />
        <Button variant="ghost" size="sm" onClick={onDone}>
          Cancel
        </Button>
        <Button size="sm" isPending={isSaving} disabled={!canSave} onClick={() => void save()}>
          Save
        </Button>
      </div>
    </div>
  );
}

/** A message being written: attachments above the text, the buttons adding things bottom left. */
function ComposerBox({
  attachments,
  tools,
  action,
  children,
}: {
  attachments: ReturnType<typeof useDraftAttachments>;
  tools?: ReactNode;
  action?: ReactNode;
  children: ReactNode;
}) {
  const picker = useFilePicker({ multiple: true, onSelect: attachments.add });
  const { chatBubbles } = useUiTheme();
  return (
    <div
      data-slot="composer"
      className="bg-input focus-within:ring-primary/8-10 flex flex-col rounded-lg border focus-within:ring-1"
    >
      {attachments.items.length > 0 && (
        <div data-slot="composer-tray" className="border-b p-2">
          <AttachmentTray items={attachments.items} onRemove={attachments.remove} />
        </div>
      )}
      <div data-slot="composer-row" className="flex items-end gap-2 p-1">
        <SimpleTooltip content="Attach files">
          <Button
            variant="ghost"
            size="icon"
            aria-label="Attach files"
            data-slot="composer-tool"
            className="text-muted-foreground shrink-0"
            onClick={picker.open}
          >
            {chatBubbles ? <PlusIcon /> : <CirclePlusIcon className="size-5" />}
          </Button>
        </SimpleTooltip>
        {tools}
        {picker.input}
        <div
          data-slot="composer-field"
          className="rr-chat rr-chat-draft max-h-80 min-w-0 flex-1 self-center overflow-y-auto py-0.5"
        >
          {children}
        </div>
        {action}
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
  tools,
}: {
  target: TCommentTarget;
  parentId?: string;
  placeholder: string;
  /** More buttons beside the one attaching files. */
  tools?: ReactNode;
}) {
  const { workspace } = useWorkspace();
  const { scrollToEnd } = useChatScroll();
  const chatReply = useChatReply();
  const { data: me } = useQuery(meQuery);
  const replyingTo = parentId ? null : (chatReply?.replyingTo ?? null);
  const [text, setText] = useState("");
  const [sent, setSent] = useState(0);
  const { users } = useWorkspaceUsers();
  const attachments = useDraftAttachments(workspace.id);
  const { mutateAsync: saveComment, isPending } = useSaveCommentMutation(workspace.id);
  const body = joinAttachments(text, attachments.uploaded);
  const canSend = Boolean(body) && !attachments.isUploading;
  const send = async () => {
    if (!canSend || isPending) return;
    const saved = await saveComment({
      ...target,
      body,
      parentId: parentId ?? replyingTo?.id,
    }).catch(() => null);
    if (!saved) return;
    setText("");
    attachments.clear();
    chatReply?.setReplyingTo(null);
    setSent((count) => count + 1);
    scrollToEnd();
  };
  const cancelReply = (event: KeyboardEvent) => {
    if (event.key !== "Escape" || event.defaultPrevented || !replyingTo) return;
    event.stopPropagation();
    chatReply?.setReplyingTo(null);
  };
  return (
    <div className="flex flex-col gap-1.5" onKeyDown={cancelReply}>
      {replyingTo && (
        <div
          data-slot="reply-bar"
          className="bg-muted text-muted-foreground flex items-center gap-2 rounded-lg py-1 pr-1 pl-3 text-sm"
        >
          <ReplyIcon className="size-4" />
          <span className="min-w-0 flex-1 truncate">
            Replying to{" "}
            <span className="text-foreground font-semibold">
              {replyingTo.user?.id === me?.id
                ? "yourself"
                : replyingTo.user
                  ? userName(replyingTo.user)
                  : "a deleted user"}
            </span>
          </span>
          <Button
            variant="ghost"
            size="icon"
            forceMinSize={false}
            aria-label="Cancel reply"
            className="text-foreground size-8"
            onClick={() => chatReply?.setReplyingTo(null)}
          >
            <XIcon className="size-4" />
          </Button>
        </div>
      )}
      <ComposerBox
        attachments={attachments}
        tools={tools}
        action={
          <Button
            variant={canSend ? "default" : "ghost"}
            size="icon"
            aria-label="Send"
            data-slot="composer-send"
            isPending={isPending}
            disabled={!canSend}
            onClick={() => void send()}
          >
            <SendHorizontalIcon className="size-4" />
          </Button>
        }
      >
        <MarkdownEditor
          key={`${sent}:${replyingTo?.id ?? ""}`}
          label={parentId || replyingTo ? "Reply" : "Message"}
          value={text}
          onChange={setText}
          onSubmit={() => void send()}
          placeholder={placeholder}
          users={users}
          onAttach={attachments.add}
          autoFocus={sent > 0 || Boolean(replyingTo)}
          chat
        />
      </ComposerBox>
      <p className="text-muted-foreground dank:hidden px-1 text-xs max-sm:hidden">
        Enter to send, Shift+Enter for a new line, / for commands
      </p>
    </div>
  );
}

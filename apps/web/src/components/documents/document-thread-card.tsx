import { useQuery } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";

import type { Comment, CommentThread } from "@/api/ops.gen";
import { DocumentCommentComposer } from "@/components/documents/document-comment-composer";
import {
  CheckCircle2Icon,
  CheckIcon,
  EllipsisIcon,
  ReplyIcon,
  Trash2Icon,
  UndoIcon,
  XIcon,
} from "@/components/icons";
import { userName } from "@/components/issues/grouping";
import { UserAvatar } from "@/components/issues/pickers";
import { AddReactionButton, groupReactions, ReactionBar } from "@/components/issues/reaction-bar";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { annotationElement } from "@/components/shared/editor/annotations";
import { MarkdownView } from "@/components/shared/markdown-view";
import { RelativeTime } from "@/components/shared/relative-time";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import {
  useCommentReactionMutation,
  useDeleteCommentMutation,
  useResolveCommentMutation,
  useSaveCommentMutation,
} from "@/lib/queries/comments";
import {
  type TSuggestionDecision,
  useDecideSuggestionMutation,
} from "@/lib/queries/document-comments";
import { meQuery } from "@/lib/queries/me";

type TProps = {
  thread: CommentThread;
  documentId: string;
  isActive: boolean;
  onActivate: (id: string) => void;
  onHover: (id: string | null) => void;
  /** After a suggestion was accepted, which changed the document. */
  onAccepted: () => void;
};

export function DocumentThreadCard({
  thread,
  documentId,
  isActive,
  onActivate,
  onHover,
  onAccepted,
}: TProps) {
  const cardRef = useRef<HTMLElement>(null);
  const [isReplying, setIsReplying] = useState(false);

  useEffect(() => {
    if (!isActive) return;
    cardRef.current?.scrollIntoView({ block: "nearest" });
    annotationElement(thread.id)?.scrollIntoView({ block: "nearest", behavior: "smooth" });
  }, [isActive, thread.id]);

  return (
    <article
      ref={cardRef}
      aria-label={`Thread from ${thread.user ? userName(thread.user) : "a deleted user"}`}
      data-active={isActive || undefined}
      className="bg-card data-active:border-primary/6-10 dank:rounded-md dank:data-active:border-m3-primary dank:data-active:shadow-elevation-1 flex w-full flex-col rounded-lg border data-active:shadow-sm"
      onMouseEnter={() => onHover(thread.id)}
      onMouseLeave={() => onHover(null)}
      onFocus={() => onActivate(thread.id)}
      onClick={() => onActivate(thread.id)}
    >
      <ThreadComment comment={thread} thread={thread} onAccepted={onAccepted} />
      {thread.replies.map((reply) => (
        <div key={reply.id} className="border-t">
          <ThreadComment comment={reply} />
        </div>
      ))}
      <div className="border-t p-2">
        {isReplying ? (
          <DocumentCommentComposer
            documentId={documentId}
            parentId={thread.id}
            label="Reply"
            placeholder="Leave a reply…"
            autoFocus
            onDone={() => setIsReplying(false)}
            onCancel={() => setIsReplying(false)}
          />
        ) : (
          <Button
            variant="ghost"
            size="sm"
            className="text-muted-foreground font-normal"
            onClick={() => setIsReplying(true)}
          >
            <ReplyIcon className="size-4" />
            Reply
          </Button>
        )}
      </div>
    </article>
  );
}

function ThreadComment({
  comment,
  thread,
  onAccepted,
}: {
  comment: Comment;
  /** Set on the thread's first comment, which resolves the thread. */
  thread?: CommentThread;
  onAccepted?: () => void;
}) {
  const { workspace, canEdit, isAdmin } = useWorkspace();
  const { data: me } = useQuery(meQuery);
  const [isDeleting, setIsDeleting] = useState(false);
  const { mutateAsync: saveComment } = useSaveCommentMutation(workspace.id);
  const { mutateAsync: deleteComment } = useDeleteCommentMutation(workspace.id);
  const { mutate: toggleReaction } = useCommentReactionMutation(workspace.id);
  const isAuthor = Boolean(me && comment.user?.id === me.id);
  const authorName = comment.user ? userName(comment.user) : "Deleted user";
  const reactions = groupReactions(comment.reactions, me?.id);
  const react = (emoji: string, reacted: boolean) =>
    toggleReaction({ commentId: comment.id, emoji, reacted });

  return (
    <div className="group/reactable dank:p-4 flex flex-col gap-2 p-3">
      <header className="flex min-h-7 items-center gap-2">
        {comment.user && <UserAvatar user={comment.user} size="sm" />}
        <span className="dank:type-title-small min-w-0 truncate text-sm font-medium">
          {authorName}
        </span>
        <span className="text-muted-foreground shrink-0 text-xs">
          <RelativeTime value={comment.createdAt} />
        </span>
        <span className="flex-1" />
        {reactions.length === 0 && (
          <AddReactionButton reactions={reactions} onToggle={react} revealOnHover />
        )}
        {thread && (canEdit || isAuthor) && <ResolveButton thread={thread} />}
        {(isAuthor || isAdmin) && (
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <Button variant="ghost" size="icon" aria-label="Comment actions">
                  <EllipsisIcon className="size-4" />
                </Button>
              }
            />
            <DropdownMenuContent align="end" className="w-48">
              <DropdownMenuItem className="text-destructive" onClick={() => setIsDeleting(true)}>
                <Trash2Icon className="size-4" />
                Delete
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </header>
      {thread && <ThreadPassage thread={thread} onAccepted={onAccepted} />}
      <MarkdownView
        markdown={comment.body}
        onChange={isAuthor ? (body) => saveComment({ id: comment.id, body }) : undefined}
      />
      {reactions.length > 0 && <ReactionBar reactions={reactions} onToggle={react} />}
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
    </div>
  );
}

function ResolveButton({ thread }: { thread: CommentThread }) {
  const { workspace } = useWorkspace();
  const { mutate: setResolved, isPending } = useResolveCommentMutation(workspace.id);
  const isResolved = Boolean(thread.resolvedAt);
  const label = isResolved ? "Reopen thread" : "Resolve thread";
  return (
    <SimpleTooltip content={label}>
      <Button
        variant="ghost"
        size="icon"
        aria-label={label}
        isPending={isPending}
        onClick={() => setResolved({ id: thread.id, resolved: !isResolved })}
      >
        {isResolved ? <UndoIcon className="size-4" /> : <CheckIcon className="size-4" />}
      </Button>
    </SimpleTooltip>
  );
}

function ThreadPassage({ thread, onAccepted }: { thread: CommentThread; onAccepted?: () => void }) {
  if (thread.suggestedText !== null) return <Suggestion thread={thread} onAccepted={onAccepted} />;
  if (!thread.quotedText) return <ResolvedNote thread={thread} />;
  return (
    <>
      <blockquote className="text-muted-foreground line-clamp-3 border-l-2 pl-3 text-sm break-words">
        {thread.quotedText}
      </blockquote>
      <ResolvedNote thread={thread} />
    </>
  );
}

function ResolvedNote({ thread }: { thread: CommentThread }) {
  if (!thread.resolvedAt || thread.suggestionStatus) return null;
  return (
    <p className="text-muted-foreground flex items-center gap-1.5 text-xs">
      <CheckCircle2Icon className="size-3.5" />
      Resolved{thread.resolvingUser && ` by ${userName(thread.resolvingUser)}`}
    </p>
  );
}

const suggestionBadges = {
  pending: { label: "Suggestion", variant: "warning" },
  accepted: { label: "Accepted", variant: "success" },
  rejected: { label: "Rejected", variant: "muted" },
} as const;

function Suggestion({ thread, onAccepted }: { thread: CommentThread; onAccepted?: () => void }) {
  const { workspace, canEdit } = useWorkspace();
  const { mutateAsync: decide, isPending, variables } = useDecideSuggestionMutation(workspace.id);
  const status = (thread.suggestionStatus ?? "pending") as keyof typeof suggestionBadges;
  const badge = suggestionBadges[status] ?? suggestionBadges.pending;
  const isPendingDecision = status === "pending";
  const decider = !isPendingDecision && thread.resolvingUser;

  const settle = (decision: TSuggestionDecision) =>
    void decide({ id: thread.id, decision })
      .then(() => {
        if (decision === "accept") onAccepted?.();
      })
      .catch(() => undefined);

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-1.5">
        <Badge variant={badge.variant}>{badge.label}</Badge>
        {decider && (
          <span className="text-muted-foreground truncate text-xs">by {userName(decider)}</span>
        )}
      </div>
      <p className="line-clamp-6 text-sm break-words whitespace-pre-wrap">
        <del className="text-muted-foreground">{thread.quotedText}</del>{" "}
        <ins className="bg-success/2-10 no-underline">{thread.suggestedText}</ins>
      </p>
      {canEdit && isPendingDecision && (
        <div className="flex items-center gap-2">
          <Button
            size="sm"
            variant="outline"
            isPending={isPending && variables?.decision === "accept"}
            disabled={isPending}
            onClick={() => settle("accept")}
          >
            <CheckIcon className="size-4" />
            Accept
          </Button>
          <Button
            size="sm"
            variant="ghost"
            isPending={isPending && variables?.decision === "reject"}
            disabled={isPending}
            onClick={() => settle("reject")}
          >
            <XIcon className="size-4" />
            Reject
          </Button>
        </div>
      )}
    </div>
  );
}

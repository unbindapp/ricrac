import { useInfiniteQuery } from "@tanstack/react-query";
import { useEffect } from "react";

import type { Comment, CommentThread, DiscussionThread } from "@/api/ops.gen";
import { ChatDayDivider, ChatMessage, MessageTime } from "@/components/forum/chat-message";
import { threadKindOf, threadKinds } from "@/components/forum/thread-kinds";
import { MessageSquareIcon, MessagesSquareIcon, SplitIcon } from "@/components/icons";
import { userName } from "@/components/issues/grouping";
import { EmptyState } from "@/components/layout/empty-state";
import { ErrorCard } from "@/components/layout/error-card";
import { LinkButton } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { chatRows } from "@/lib/helpers/chat";
import { entitySlug } from "@/lib/helpers/entity-slug";
import { commentsQuery, type TCommentTarget } from "@/lib/queries/comments";
import { useUiTheme } from "@/themes/ui-theme-provider";

const SKELETON_MESSAGES = 3;

/** Every message of a target, live, reading page after page until it has them all. */
export function useChatMessages(target: TCommentTarget) {
  const { workspace } = useWorkspace();
  const query = useInfiniteQuery(commentsQuery(workspace.id, target, { live: true }));
  const { hasNextPage, isFetchingNextPage, fetchNextPage } = query;
  useEffect(() => {
    if (hasNextPage && !isFetchingNextPage) void fetchNextPage();
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);
  const threads = query.data?.pages.flatMap((page) => page.nodes) ?? [];
  return { ...query, threads, isComplete: query.isSuccess && !hasNextPage };
}

type TProps = {
  target: TCommentTarget;
  /** Replies open in a thread beside the chat, unless the theme puts them in the timeline; without it they follow the message they answer. */
  threads?: { openId?: string; onOpen: (messageId: string) => void };
  onNewThread?: (message: Comment) => void;
  /** Discussion threads to note in the timeline when they were started. */
  startedThreads?: DiscussionThread[];
  empty: { title: string; description: string };
};

export function ChatMessages({ target, threads, onNewThread, startedThreads = [], empty }: TProps) {
  const { inlineReplies, chatBubbles } = useUiTheme();
  const threadView = inlineReplies ? undefined : threads;
  const query = useChatMessages(target);
  const { isPending, error, refetch } = query;
  if (isPending) return <ChatSkeleton />;
  if (error) return <ErrorCard message={error.message} onRetry={() => void refetch()} />;
  if (query.threads.length === 0 && startedThreads.length === 0) {
    return (
      <EmptyState icon={MessagesSquareIcon} title={empty.title} description={empty.description} />
    );
  }
  const replyTo = new Map<string, Comment>();
  const messages = threadView ? query.threads : withRepliesInline(query.threads, replyTo);
  const byId = new Map(query.threads.map((thread) => [thread.id, thread]));
  return (
    <ol aria-label="Messages" className="flex flex-col">
      {chatRows(withThreadStarts(messages, startedThreads)).map((row) => {
        if (row.kind === "day") {
          if (chatBubbles) return null;
          return (
            <li key={row.key} className="flex flex-col">
              <ChatDayDivider date={row.date} />
            </li>
          );
        }
        const { message } = row;
        if ("thread" in message) {
          return (
            <li key={row.key} className="flex flex-col">
              <ThreadStarted thread={message.thread} />
            </li>
          );
        }
        return (
          <li key={row.key} className="flex flex-col">
            <ChatMessage
              message={message}
              isContinuation={row.isContinuation && !replyTo.has(message.id)}
              replyTo={replyTo.get(message.id)}
              thread={
                threadView && {
                  replies: byId.get(message.id)?.replies ?? [],
                  isOpen: threadView.openId === message.id,
                  onOpen: () => threadView.onOpen(message.id),
                }
              }
              onNewThread={onNewThread && (() => onNewThread(message))}
            />
          </li>
        );
      })}
    </ol>
  );
}

/** Replies to a message, with day dividers counted from the message they answer. */
export function ReplyList({ replies, since }: { replies: Comment[]; since: string }) {
  return (
    <ol aria-label="Replies" className="flex w-full flex-col">
      {chatRows(replies, since).map((row) => (
        <li key={row.key} className="flex flex-col">
          {row.kind === "day" ? (
            <ChatDayDivider date={row.date} />
          ) : (
            <ChatMessage message={row.message} isContinuation={row.isContinuation} />
          )}
        </li>
      ))}
    </ol>
  );
}

type TThreadStart = { id: string; createdAt: string; user: null; thread: DiscussionThread };

/** Messages with a note where each thread started; notes have no user, so they break up groups. */
function withThreadStarts<T extends Comment>(
  messages: T[],
  threads: DiscussionThread[],
): (T | TThreadStart)[] {
  if (threads.length === 0) return messages;
  const starts: TThreadStart[] = threads.map((thread) => ({
    id: `thread-${thread.id}`,
    createdAt: thread.createdAt,
    user: null,
    thread,
  }));
  return [...messages, ...starts].sort(
    (a, b) => new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime(),
  );
}

function ThreadStarted({ thread }: { thread: DiscussionThread }) {
  const { workspace } = useWorkspace();
  const { chatBubbles } = useUiTheme();
  const kindId = threadKindOf(thread.kind);
  const { label: kind, icon: KindIcon } = threadKinds[kindId];
  const count = thread.commentCount;
  const isResolved = Boolean(thread.resolvedAt);
  const creator = thread.creator ? userName(thread.creator) : "A deleted user";
  if (chatBubbles) {
    return (
      <LinkButton
        to="/$workspace/discussion/$slug"
        params={{ workspace: workspace.urlKey, slug: entitySlug(thread.discussion) }}
        search={{ thread: thread.id }}
        replace
        variant="ghost"
        forceMinSize={false}
        data-slot="thread-card"
        data-kind={kindId}
        data-resolved={isResolved || undefined}
        aria-label={`${kind} thread: ${thread.title}, started by ${creator}, ${count} ${count === 1 ? "reply" : "replies"}. Open thread`}
      >
        <span data-slot="thread-card-icon">
          <KindIcon className="size-5" />
        </span>
        <span className="flex min-w-0 flex-1 flex-col items-start">
          <span data-slot="thread-card-title">{thread.title}</span>
          <span data-slot="thread-card-by">
            Started by {creator}
            {isResolved && ", resolved"}
          </span>
        </span>
        <span aria-hidden data-slot="thread-card-count">
          <MessageSquareIcon />
          {count}
        </span>
      </LinkButton>
    );
  }
  return (
    <div className="text-muted-foreground flex flex-wrap items-center justify-center gap-x-2 gap-y-1 py-3 text-xs">
      <SplitIcon className="size-3.5" />
      <span>
        <span className="text-foreground font-medium">{creator}</span> started a thread
      </span>
      <LinkButton
        to="/$workspace/discussion/$slug"
        params={{ workspace: workspace.urlKey, slug: entitySlug(thread.discussion) }}
        search={{ thread: thread.id }}
        replace
        variant="outline"
        size="sm"
        data-m3-chip="assist"
        className="max-w-full font-normal"
      >
        <KindIcon aria-label={kind} role="img" className="size-4" />
        <span className="text-foreground truncate font-medium">{thread.title}</span>
        <span className="text-muted-foreground font-normal tabular-nums">
          {count} {count === 1 ? "message" : "messages"}
        </span>
      </LinkButton>
      <MessageTime value={thread.createdAt} />
    </div>
  );
}

/** Each thread's replies right after it, oldest first, noting the message they answer in `replyTo`. */
function withRepliesInline(threads: CommentThread[], replyTo: Map<string, Comment>) {
  return threads
    .flatMap((thread) => [
      thread,
      ...thread.replies.map((reply) => {
        replyTo.set(reply.id, thread);
        return reply;
      }),
    ])
    .sort((a, b) => new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime());
}

export function ChatSkeleton() {
  return (
    <div aria-busy aria-label="Loading" className="flex flex-col gap-4">
      {Array.from({ length: SKELETON_MESSAGES }, (_, index) => (
        <div key={index} className="flex gap-3">
          <Skeleton className="size-8 shrink-0 rounded-full" />
          <div className="flex flex-1 flex-col gap-2 pt-1">
            <Skeleton className="h-3 w-32" />
            <Skeleton className="h-3 w-3/4" />
          </div>
        </div>
      ))}
    </div>
  );
}

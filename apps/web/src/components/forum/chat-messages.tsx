import { useInfiniteQuery } from "@tanstack/react-query";
import { useEffect } from "react";

import type { Comment, CommentThread, UserRef } from "@/api/ops.gen";
import { ChatDayDivider, ChatMessage } from "@/components/forum/chat-message";
import { MessagesSquareIcon } from "@/components/icons";
import { EmptyState } from "@/components/layout/empty-state";
import { ErrorCard } from "@/components/layout/error-card";
import { Skeleton } from "@/components/ui/skeleton";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { chatRows } from "@/lib/helpers/chat";
import { commentsQuery, type TCommentTarget } from "@/lib/queries/comments";

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
  /** Replies open in a thread beside the chat; without it they follow the message they answer. */
  threads?: { openId?: string; onOpen: (messageId: string) => void };
  onNewThread?: (message: Comment) => void;
  empty: { title: string; description: string };
};

export function ChatMessages({ target, threads: threadView, onNewThread, empty }: TProps) {
  const { threads, isPending, error, refetch } = useChatMessages(target);
  if (isPending) return <ChatSkeleton />;
  if (error) return <ErrorCard message={error.message} onRetry={() => void refetch()} />;
  if (threads.length === 0) {
    return (
      <EmptyState icon={MessagesSquareIcon} title={empty.title} description={empty.description} />
    );
  }
  const replyTo = new Map<string, UserRef | null>();
  const messages = threadView ? threads : withRepliesInline(threads, replyTo);
  const byId = new Map(threads.map((thread) => [thread.id, thread]));
  return (
    <ol aria-label="Messages" className="flex flex-col">
      {chatRows(messages).map((row) => (
        <li key={row.key} className="flex flex-col">
          {row.kind === "day" ? (
            <ChatDayDivider date={row.date} />
          ) : (
            <ChatMessage
              message={row.message}
              isContinuation={row.isContinuation}
              replyTo={replyTo.has(row.message.id) ? replyTo.get(row.message.id) : undefined}
              thread={
                threadView && {
                  replies: byId.get(row.message.id)?.replies ?? [],
                  isOpen: threadView.openId === row.message.id,
                  onOpen: () => threadView.onOpen(row.message.id),
                }
              }
              onNewThread={onNewThread && (() => onNewThread(row.message))}
            />
          )}
        </li>
      ))}
    </ol>
  );
}

/** Each thread's replies right after it, oldest first, noting whom they answer in `replyTo`. */
function withRepliesInline(threads: CommentThread[], replyTo: Map<string, UserRef | null>) {
  return threads
    .flatMap((thread) => [
      thread,
      ...thread.replies.map((reply) => {
        replyTo.set(reply.id, thread.user);
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

import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { BotIcon } from "lucide-react";

import type { CommentThread as TCommentThread, HistoryEntry } from "@/api/ops.gen";
import { GithubIcon } from "@/components/icons/github";
import { mergeFeed } from "@/components/issues/activity-feed";
import { describeEntry, historyActorOf } from "@/components/issues/activity-text";
import { CommentComposer, CommentThread } from "@/components/issues/comment-thread";
import { UserAvatar } from "@/components/issues/pickers";
import { ErrorCard } from "@/components/layout/error-card";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { SectionTitle } from "@/components/layout/section-title";
import { RelativeTime } from "@/components/shared/relative-time";
import { Button } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { issueCommentsQuery } from "@/lib/queries/comments";
import { issueHistoryQuery } from "@/lib/queries/issues";

type TFeedItem =
  | { kind: "history"; id: string; at: string; entry: HistoryEntry }
  | { kind: "comment"; id: string; at: string; thread: TCommentThread };

/** The issue's history and comment threads, oldest first, with the composer at the end. */
export function IssueActivity({ issueId }: { issueId: string }) {
  const { workspace, canEdit } = useWorkspace();
  const history = useQuery(issueHistoryQuery(workspace.id, issueId));
  const comments = useInfiniteQuery(issueCommentsQuery(workspace.id, issueId));

  const items = mergeFeed<TFeedItem>([
    {
      items: (history.data?.history ?? []).map((entry, index): TFeedItem => ({
        kind: "history",
        id: `${entry.createdAt}-${index}`,
        at: entry.createdAt,
        entry,
      })),
      hasMore: false,
    },
    {
      items: (comments.data?.pages ?? []).flatMap((page) =>
        page.nodes.map((thread): TFeedItem => ({
          kind: "comment",
          id: thread.id,
          at: thread.createdAt,
          thread,
        })),
      ),
      hasMore: comments.hasNextPage,
    },
  ]);

  const error = history.error ?? comments.error;
  return (
    <section aria-label="Activity" className="flex w-full flex-col gap-4">
      <SectionTitle title="Activity" />
      {history.isPending || comments.isPending ? (
        <ListSkeleton rows={3} />
      ) : error ? (
        <ErrorCard
          message={error.message}
          onRetry={() => {
            void history.refetch();
            void comments.refetch();
          }}
        />
      ) : (
        <ol className="flex flex-col gap-3">
          {items.map((item) => (
            <li key={`${item.kind}-${item.id}`} className="flex flex-col">
              {item.kind === "history" ? (
                <HistoryLines entry={item.entry} />
              ) : (
                <CommentThread thread={item.thread} issueId={issueId} />
              )}
            </li>
          ))}
        </ol>
      )}
      {comments.hasNextPage && (
        <div className="flex">
          <Button
            variant="outline"
            size="sm"
            isPending={comments.isFetchingNextPage}
            onClick={() => void comments.fetchNextPage()}
          >
            Load more
          </Button>
        </div>
      )}
      {canEdit && <CommentComposer issueId={issueId} />}
    </section>
  );
}

function HistoryLines({ entry }: { entry: HistoryEntry }) {
  return (
    <div className="flex flex-col gap-1">
      {describeEntry(entry).map((line, index) => (
        <p
          key={`${line}-${index}`}
          className="text-muted-foreground flex min-w-0 items-center gap-1.5 px-1 text-xs"
        >
          <ActorMark entry={entry} />
          <span className="min-w-0 truncate">{line}</span>
          <span aria-hidden>·</span>
          <span className="shrink-0">
            <RelativeTime value={entry.createdAt} />
          </span>
        </p>
      ))}
    </div>
  );
}

function ActorMark({ entry }: { entry: HistoryEntry }) {
  const actor = historyActorOf(entry);
  switch (actor.kind) {
    case "user":
      return (
        <>
          {entry.actor && <UserAvatar user={entry.actor} size="xs" />}
          <span className="text-foreground shrink-0 font-medium">{actor.name}</span>
        </>
      );
    case "github":
      return (
        <>
          <GithubIcon className="size-3.5" />
          <span className="text-foreground shrink-0 font-medium">GitHub</span>
        </>
      );
    case "ricrac":
      return (
        <>
          <BotIcon className="size-3.5 shrink-0" />
          <span className="text-foreground shrink-0 font-medium">Ricrac</span>
        </>
      );
  }
}

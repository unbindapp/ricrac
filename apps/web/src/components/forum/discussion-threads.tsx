import { Link } from "@tanstack/react-router";
import {
  CircleCheckIcon,
  CircleDotIcon,
  MessageSquareIcon,
  PlusIcon,
  SearchIcon,
  SplitIcon,
} from "lucide-react";
import { useState } from "react";

import type { DiscussionDetail, DiscussionThread } from "@/api/ops.gen";
import { NewThreadDialog } from "@/components/forum/new-thread-dialog";
import {
  THREAD_KINDS,
  threadKindOf,
  threadKinds,
  ThreadKindBadge,
} from "@/components/forum/thread-kinds";
import { EmptyState } from "@/components/layout/empty-state";
import { List } from "@/components/layout/list";
import { SectionTitle } from "@/components/layout/section-title";
import { Button } from "@/components/ui/button";
import { Tabs, TabsList, TabsTab } from "@/components/ui/tabs";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { entitySlug } from "@/lib/helpers/entity-slug";
import type { TThreadKind } from "@/lib/queries/discussions";

type TStatusFilter = "all" | "open" | "resolved";
const ALL_KINDS = "all";

type TProps = { discussion: DiscussionDetail; openThreadId?: string };

export function DiscussionThreads({ discussion, openThreadId }: TProps) {
  const { canEdit } = useWorkspace();
  const [status, setStatus] = useState<TStatusFilter>("all");
  const [kind, setKind] = useState<TThreadKind | typeof ALL_KINDS>(ALL_KINDS);
  const { threads } = discussion;
  const kinds = THREAD_KINDS.filter((known) =>
    threads.some((thread) => threadKindOf(thread.kind) === known),
  );
  const shown = threads.filter(
    (thread) =>
      matchesStatus(thread, status) && (kind === ALL_KINDS || threadKindOf(thread.kind) === kind),
  );

  return (
    <section aria-label="Threads" className="flex w-full flex-col gap-4">
      <SectionTitle
        title="Threads"
        actions={
          canEdit && (
            <NewThreadDialog
              discussion={discussion}
              trigger={
                <Button variant="outline" size="sm">
                  <PlusIcon className="size-4" />
                  New thread
                </Button>
              }
            />
          )
        }
      />
      {threads.length > 0 && (
        <div className="flex flex-wrap items-center gap-2">
          <Tabs value={status} onValueChange={(value) => setStatus(value as TStatusFilter)}>
            <TabsList>
              <TabsTab value="all">All</TabsTab>
              <TabsTab value="open">Open</TabsTab>
              <TabsTab value="resolved">Resolved</TabsTab>
            </TabsList>
          </Tabs>
          {kinds.length > 1 && (
            <Tabs value={kind} onValueChange={(value) => setKind(value as typeof kind)}>
              <TabsList>
                <TabsTab value={ALL_KINDS}>All kinds</TabsTab>
                {kinds.map((known) => {
                  const { label, icon: Icon } = threadKinds[known];
                  return (
                    <TabsTab key={known} value={known}>
                      <Icon />
                      <span className="max-sm:sr-only">{label}</span>
                    </TabsTab>
                  );
                })}
              </TabsList>
            </Tabs>
          )}
        </div>
      )}
      <ThreadList discussion={discussion} threads={shown} openThreadId={openThreadId} />
    </section>
  );
}

function ThreadList({
  discussion,
  threads,
  openThreadId,
}: {
  discussion: DiscussionDetail;
  threads: DiscussionThread[];
  openThreadId?: string;
}) {
  if (discussion.threads.length === 0) {
    return (
      <EmptyState
        icon={SplitIcon}
        title="No threads yet"
        description="Split out a bug, a design question or an idea to settle it on its own."
      />
    );
  }
  if (threads.length === 0) {
    return (
      <EmptyState
        icon={SearchIcon}
        title="No matching threads"
        description="No thread matches these filters."
      />
    );
  }
  return (
    <List>
      {threads.map((thread) => (
        <ThreadRow
          key={thread.id}
          discussion={discussion}
          thread={thread}
          isOpen={thread.id === openThreadId}
        />
      ))}
    </List>
  );
}

function ThreadRow({
  discussion,
  thread,
  isOpen,
}: {
  discussion: DiscussionDetail;
  thread: DiscussionThread;
  isOpen: boolean;
}) {
  const { workspace } = useWorkspace();
  const isResolved = Boolean(thread.resolvedAt);
  return (
    <Link
      role="listitem"
      to="/$workspace/discussion/$slug"
      params={{ workspace: workspace.urlKey, slug: entitySlug(discussion) }}
      search={{ thread: thread.id }}
      replace
      aria-current={isOpen || undefined}
      className="has-hover:hover:bg-muted focus-visible:bg-muted aria-current:bg-muted flex h-9 w-full items-center gap-2 border-b px-3 text-sm outline-hidden last:border-b-0"
    >
      <SimpleTooltip content={isResolved ? "Resolved" : "Open"}>
        <span className="flex shrink-0">
          {isResolved ? (
            <CircleCheckIcon aria-label="Resolved" className="text-success size-4" />
          ) : (
            <CircleDotIcon aria-label="Open" className="text-process size-4" />
          )}
        </span>
      </SimpleTooltip>
      <ThreadKindBadge kind={thread.kind} />
      <span
        data-resolved={isResolved || undefined}
        className="data-resolved:text-muted-foreground min-w-0 flex-1 truncate font-medium"
      >
        {thread.title}
      </span>
      {thread.issue && (
        <span className="text-muted-foreground hidden shrink-0 font-mono text-xs sm:block">
          {thread.issue.identifier}
        </span>
      )}
      <span className="text-muted-foreground flex w-10 shrink-0 items-center justify-end gap-1.5 text-xs tabular-nums">
        <MessageSquareIcon className="size-3.5" />
        {thread.commentCount}
      </span>
    </Link>
  );
}

function matchesStatus(thread: DiscussionThread, status: TStatusFilter) {
  switch (status) {
    case "all":
      return true;
    case "open":
      return !thread.resolvedAt;
    case "resolved":
      return Boolean(thread.resolvedAt);
  }
}

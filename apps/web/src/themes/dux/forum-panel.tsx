import { useInfiniteQuery } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useLayoutEffect, useRef, useState } from "react";

import type { Discussion } from "@/api/ops.gen";
import { DiscussionContextMenu, discussionCover } from "@/components/forum/discussion-card";
import {
  forumDiscussionsQuery,
  forumEmptyState,
  type TForumFilters,
  TrashedDiscussionRow,
} from "@/components/forum/forum-views";
import { NewDiscussionDialog } from "@/components/forum/new-discussion-dialog";
import { ChevronRightIcon, MessagesSquareIcon, PinIcon, PlusIcon } from "@/components/icons";
import { userName } from "@/components/issues/grouping";
import { CreateButton } from "@/components/layout/create-button";
import { VirtualList } from "@/components/layout/list";
import { ListStates } from "@/components/layout/list-states";
import { RelativeTime } from "@/components/shared/relative-time";
import { Button } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useLoadMore } from "@/lib/hooks/use-load-more";
import { entitySlug } from "@/lib/helpers/entity-slug";
import { useHotkeyScope } from "@/lib/hotkeys/use-hotkey-scope";
import { DiscussionPanel } from "@/themes/dux/discussion-panel";
import { activeFilterCount, ForumChips, ForumFilterButton } from "@/themes/dux/forum-controls";
import type { TForumPanelProps } from "@/themes/types";

const ROW_HEIGHT = 90;

/** The forum in one card: its discussions as a list, or the open one in their place. */
export function DuxForumPanel({
  filters,
  discussion,
  chat,
  threadId,
  isThreadOpen = false,
  onBackToChat,
  fallback,
}: TForumPanelProps) {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const isOpen = Boolean(discussion || fallback);
  const close = () =>
    void navigate({
      to: "/$workspace/forum",
      params: { workspace: workspace.urlKey },
      search: filters,
    });
  useHotkeyScope([{ hotkey: "Escape", callback: close }], { enabled: isOpen && !isThreadOpen });

  if (!isOpen) return <ForumList filters={filters} />;
  return (
    <div data-slot="forum-detail">
      {discussion ? (
        <DiscussionPanel
          discussion={discussion}
          chat={chat}
          threadId={threadId}
          onBackToChat={onBackToChat ?? close}
          onClose={close}
        />
      ) : (
        <section data-slot="discussion-panel">
          <div data-slot="dux-pane-main">{fallback}</div>
        </section>
      )}
    </div>
  );
}

function ForumList({ filters }: { filters: TForumFilters }) {
  const { workspace, canEdit } = useWorkspace();
  const navigate = useNavigate();
  const [areChipsShown, setAreChipsShown] = useState(activeFilterCount(filters) > 0);
  const status = filters.status ?? "open";
  const isTrash = status === "trash";
  const list = useInfiniteQuery(
    forumDiscussionsQuery(workspace.id, {
      status,
      tag: filters.tag,
      sort: filters.sort,
      order: filters.order,
    }),
  );
  const discussions = list.data?.pages.flatMap((page) => page.nodes) ?? [];
  const loadMore = useLoadMore(list);
  const filter = (next: TForumFilters) =>
    void navigate({
      to: "/$workspace/forum",
      params: { workspace: workspace.urlKey },
      search: { ...filters, ...next },
      replace: true,
    });

  return (
    <section aria-label="Forum" data-slot="forum-list">
      <div data-slot="dux-pane-head">
        <h1 data-slot="dux-pane-title">Forum</h1>
        <ForumFilterButton
          filters={filters}
          isExpanded={areChipsShown}
          onToggle={() => setAreChipsShown((shown) => !shown)}
        />
        {canEdit && !isTrash && (
          <NewDiscussionDialog trigger={<CreateButton label="New discussion" size="sm" />} />
        )}
      </div>
      <div data-slot="dux-pane-main">
        {areChipsShown && <ForumChips filters={filters} onFilter={filter} />}
        <ListStates
          isPending={list.isPending}
          error={list.error}
          onRetry={() => void list.refetch()}
          isEmpty={discussions.length === 0}
          empty={forumEmptyState({
            status,
            isFiltered: Boolean(filters.tag),
            action: canEdit && status !== "closed" && !isTrash && (
              <NewDiscussionDialog
                trigger={
                  <Button size="sm" variant="outline">
                    <PlusIcon className="size-4" />
                    New discussion
                  </Button>
                }
              />
            ),
          })}
        >
          {isTrash ? (
            <VirtualList
              flush
              label="Trashed discussions"
              items={discussions}
              getKey={(discussion) => discussion.id}
              onEndReached={loadMore}
              renderItem={(discussion) => <TrashedDiscussionRow discussion={discussion} />}
            />
          ) : (
            <VirtualList
              flush
              label="Discussions"
              items={discussions}
              getKey={(discussion) => discussion.id}
              estimateSize={() => ROW_HEIGHT}
              onEndReached={loadMore}
              renderItem={(discussion) => <ForumRow discussion={discussion} filters={filters} />}
            />
          )}
          {list.hasNextPage && (
            <Button
              variant="ghost"
              size="sm"
              className="self-center"
              isPending={list.isFetchingNextPage}
              onClick={() => void list.fetchNextPage()}
            >
              Show more
            </Button>
          )}
        </ListStates>
      </div>
    </section>
  );
}

function ForumRow({ discussion, filters }: { discussion: Discussion; filters: TForumFilters }) {
  const { workspace } = useWorkspace();
  const replies = discussion.commentCount;
  const upvotes = discussion.upvoteCount;
  return (
    <DiscussionContextMenu
      discussion={discussion}
      trigger={
        <Link
          role="listitem"
          data-slot="forum-card"
          data-closed={discussion.closedAt ? true : undefined}
          to="/$workspace/discussion/$slug"
          params={{ workspace: workspace.urlKey, slug: entitySlug(discussion) }}
          search={filters}
        />
      }
    >
      <ForumThumb discussion={discussion} />
      <span data-slot="forum-card-text">
        <span data-slot="forum-card-title">{discussion.title}</span>
        <span data-slot="forum-card-meta">
          {discussion.pinnedAt && <PinIcon aria-label="Pinned" className="fill-current" />}
          {discussion.creator && `by ${userName(discussion.creator)} · `}
          <RelativeTime value={discussion.lastActivityAt} />
          {` · ${replies} ${replies === 1 ? "reply" : "replies"}`}
          {upvotes > 0 && ` · ${upvotes} ${upvotes === 1 ? "upvote" : "upvotes"}`}
          {discussion.closedAt && " · Closed"}
        </span>
        {discussion.tags.length > 0 && <FittingTags tags={discussion.tags} />}
      </span>
      <ChevronRightIcon data-slot="forum-card-trail" />
    </DiscussionContextMenu>
  );
}

const MORE_CHIP_WIDTH = 36;
const TAG_GAP = 6;

/** The tags on one line, with a +n chip for those that don't fit. */
function FittingTags({ tags }: { tags: string[] }) {
  const rowRef = useRef<HTMLSpanElement>(null);
  const [shown, setShown] = useState(tags.length);
  useLayoutEffect(() => {
    const row = rowRef.current;
    if (!row) return;
    const fit = () => {
      const widths = [...row.querySelectorAll<HTMLElement>("[data-tag]")].map(
        (chip) => chip.offsetWidth,
      );
      let used = 0;
      let count = 0;
      for (const [index, width] of widths.entries()) {
        const isLast = index === widths.length - 1;
        const needed = used + (index > 0 ? TAG_GAP : 0) + width;
        if (needed + (isLast ? 0 : TAG_GAP + MORE_CHIP_WIDTH) > row.clientWidth) break;
        used = needed;
        count += 1;
      }
      setShown(count);
    };
    fit();
    const observer = new ResizeObserver(fit);
    observer.observe(row);
    return () => observer.disconnect();
  }, [tags]);
  const hidden = tags.length - shown;
  return (
    <span ref={rowRef} data-slot="forum-card-tags">
      {tags.map((tag, index) => (
        <span key={tag} data-tag data-overflow={index >= shown || undefined}>
          {tag}
        </span>
      ))}
      {hidden > 0 && (
        <span data-slot="tags-more" aria-label={`${hidden} more tags`}>
          +{hidden}
        </span>
      )}
    </span>
  );
}

function ForumThumb({ discussion }: { discussion: Discussion }) {
  const cover = discussionCover(discussion);
  return (
    <span data-slot="forum-thumb" aria-hidden>
      {cover?.kind === "image" && <img src={cover.url} alt="" loading="lazy" />}
      {cover?.kind === "video" && <video src={cover.url} muted preload="metadata" />}
      {!cover && <MessagesSquareIcon />}
    </span>
  );
}

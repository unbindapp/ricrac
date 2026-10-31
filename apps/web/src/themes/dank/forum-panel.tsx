import { useInfiniteQuery } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useLayoutEffect, useRef, useState } from "react";
import { useLocalStorage } from "usehooks-ts";

import type { Discussion } from "@/api/ops.gen";
import { DiscussionContextMenu, discussionCover } from "@/components/forum/discussion-card";
import {
  forumDiscussionsQuery,
  forumEmptyState,
  type TForumFilters,
  TrashedDiscussionRow,
} from "@/components/forum/forum-views";
import { NewDiscussionDialog } from "@/components/forum/new-discussion-dialog";
import { ImageIcon, PencilIcon, PlusIcon } from "@/components/icons";
import { commentExcerpt } from "@/components/inbox/notifications";
import { CreateButton } from "@/components/layout/create-button";
import { List } from "@/components/layout/list";
import { ListStates } from "@/components/layout/list-states";
import { PageSlotsContext } from "@/components/layout/page-container";
import { RelativeTime } from "@/components/shared/relative-time";
import { Button } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { entitySlug } from "@/lib/helpers/entity-slug";
import { useHotkeyScope } from "@/lib/hotkeys/use-hotkey-scope";
import { DiscussionPanel } from "@/themes/dank/discussion-panel";
import { ForumToolbar, type TForumLayout } from "@/themes/dank/forum-controls";
import { UpvoteChip } from "@/themes/dank/upvote-chip";
import type { TForumPanelProps } from "@/themes/types";

/** The forum in the page's panel: its discussions as cards, or the open one in their place. */
export function DankForumPanel({
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
        <section data-slot="discussion-panel">{fallback}</section>
      )}
    </div>
  );
}

function ForumList({ filters }: { filters: TForumFilters }) {
  const { workspace, canEdit } = useWorkspace();
  const navigate = useNavigate();
  const [layout, setLayout] = useLocalStorage<TForumLayout>("forum-layout", "grid");
  const [fab, setFab] = useState<HTMLElement | null>(null);
  const status = filters.status ?? "open";
  const isTrash = status === "trash";
  const isGrid = layout === "grid" && !isTrash;
  const list = useInfiniteQuery(
    forumDiscussionsQuery(workspace.id, {
      status,
      tag: filters.tag,
      sort: filters.sort,
      order: filters.order,
    }),
  );
  const discussions = list.data?.pages.flatMap((page) => page.nodes) ?? [];
  const filter = (next: TForumFilters) =>
    void navigate({
      to: "/$workspace/forum",
      params: { workspace: workspace.urlKey },
      search: { ...filters, ...next },
      replace: true,
    });

  return (
    <PageSlotsContext value={{ bar: null, fab }}>
      <section aria-label="Forum" data-slot="forum-list">
        <ForumToolbar filters={filters} layout={layout} onFilter={filter} onLayout={setLayout} />
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
            <List flush>
              {discussions.map((discussion) => (
                <TrashedDiscussionRow key={discussion.id} discussion={discussion} />
              ))}
            </List>
          ) : (
            <div role="list" data-slot="forum-cards" data-layout={isGrid ? "grid" : "list"}>
              {discussions.map((discussion) => (
                <ForumCard key={discussion.id} discussion={discussion} filters={filters} />
              ))}
            </div>
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
        {canEdit && !isTrash && (
          <NewDiscussionDialog
            trigger={<CreateButton label="New discussion" icon={PencilIcon} isLarge />}
          />
        )}
        <div ref={setFab} data-slot="forum-fab" />
      </section>
    </PageSlotsContext>
  );
}

function ForumCard({ discussion, filters }: { discussion: Discussion; filters: TForumFilters }) {
  const { workspace } = useWorkspace();
  const excerpt = commentExcerpt(discussion.body ?? "");
  const replies = discussion.commentCount;
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
        {excerpt && <span data-slot="forum-card-excerpt">{excerpt}</span>}
        <span data-slot="forum-card-meta">
          <span>
            {replies} {replies === 1 ? "reply" : "replies"}
            {discussion.closedAt && " · Closed"}
            {" · "}
            <RelativeTime value={discussion.lastActivityAt} />
          </span>
          <UpvoteChip discussion={discussion} />
        </span>
        {discussion.tags.length > 0 && <FittingTags tags={discussion.tags} />}
      </span>
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
          #{tag}
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
      {!cover && <ImageIcon />}
    </span>
  );
}

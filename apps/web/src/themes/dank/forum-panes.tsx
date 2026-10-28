import { useInfiniteQuery } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { type CSSProperties, type RefObject, useLayoutEffect, useRef, useState } from "react";
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
import { ImageIcon, MessageSquareIcon, PlusIcon } from "@/components/icons";
import { CreateButton } from "@/components/layout/create-button";
import { List } from "@/components/layout/list";
import { ListStates } from "@/components/layout/list-states";
import { PageSlotsContext } from "@/components/layout/page-container";
import { useDeviceSize } from "@/components/providers/device-size-provider";
import { RelativeTime } from "@/components/shared/relative-time";
import { Button } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { entitySlug } from "@/lib/helpers/entity-slug";
import { useHotkeyScope } from "@/lib/hotkeys/use-hotkey-scope";
import { DiscussionPanel } from "@/themes/dank/discussion-panel";
import { ForumControls, type TForumLayout } from "@/themes/dank/forum-controls";
import type { TForumPanesProps } from "@/themes/types";

const MIN_LIST_WIDTH = 280;
const MIN_DETAIL_WIDTH = 360;
const RESIZER_WIDTH = 12;
const RESIZE_STEP = 16;

/** The forum's list in a pane, and the open discussion in a panel beside it on wide screens. */
export function DankForumPanes({
  filters,
  discussion,
  chat,
  threadId,
  isThreadOpen = false,
  onBackToChat,
  fallback,
}: TForumPanesProps) {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const { size } = useDeviceSize();
  const panesRef = useRef<HTMLDivElement>(null);
  const [listWidth, setListWidth] = useLocalStorage<number | null>("forum-list-width", null);
  const isOpen = Boolean(discussion || fallback);
  const isWide = size === "lg" || size === "xl";
  const close = () =>
    void navigate({
      to: "/$workspace/forum",
      params: { workspace: workspace.urlKey },
      search: filters,
    });
  useHotkeyScope([{ hotkey: "Escape", callback: close }], { enabled: isOpen && !isThreadOpen });

  const detail = isOpen && (
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
  if (detail && !isWide) return detail;
  return (
    <div
      ref={panesRef}
      data-slot="forum-panes"
      data-open={isOpen || undefined}
      style={listWidth ? ({ "--list-w": `${listWidth}px` } as CSSProperties) : undefined}
    >
      <ForumListPane filters={filters} openId={discussion?.id} />
      {detail && (
        <PaneResizer
          panesRef={panesRef}
          onResize={setListWidth}
          onReset={() => setListWidth(null)}
        />
      )}
      {detail}
    </div>
  );
}

function ForumListPane({ filters, openId }: { filters: TForumFilters; openId?: string }) {
  const { workspace, canEdit } = useWorkspace();
  const navigate = useNavigate();
  const [layout, setLayout] = useLocalStorage<TForumLayout>("forum-layout", "grid");
  const [fab, setFab] = useState<HTMLElement | null>(null);
  const status = filters.status ?? "open";
  const isTrash = status === "trash";
  const isGrid = layout === "grid" && !isTrash;
  const list = useInfiniteQuery(forumDiscussionsQuery(workspace.id, { status, tag: filters.tag }));
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
        <div data-slot="forum-list-tools">
          <ForumControls
            filters={filters}
            layout={layout}
            onFilter={filter}
            onLayout={setLayout}
          />
        </div>
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
            <div role="list" data-slot="forum-items" data-layout={isGrid ? "cards" : "list"}>
              {discussions.map((discussion) => (
                <ForumItem
                  key={discussion.id}
                  discussion={discussion}
                  filters={filters}
                  isOpen={discussion.id === openId}
                />
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
          <NewDiscussionDialog trigger={<CreateButton label="New discussion" />} />
        )}
        <div
          ref={setFab}
          className="pointer-events-none sticky bottom-0 z-20 mt-auto flex justify-end pt-3 *:pointer-events-auto empty:hidden"
        />
      </section>
    </PageSlotsContext>
  );
}

function ForumItem({
  discussion,
  filters,
  isOpen,
}: {
  discussion: Discussion;
  filters: TForumFilters;
  isOpen: boolean;
}) {
  const { workspace } = useWorkspace();
  const content = (
    <>
      <ForumThumb discussion={discussion} />
      <span data-slot="forum-item-text">
        <span className="truncate">{discussion.title}</span>
        <span data-slot="forum-item-supporting">
          <RelativeTime value={discussion.lastActivityAt} />
          <span
            data-slot="count-chip"
            aria-label={`${discussion.commentCount} ${discussion.commentCount === 1 ? "message" : "messages"}`}
          >
            <MessageSquareIcon />
            {discussion.commentCount}
          </span>
        </span>
        {discussion.tags.length > 0 && <FittingTags tags={discussion.tags} />}
      </span>
    </>
  );
  return (
    <DiscussionContextMenu
      discussion={discussion}
      trigger={
        isOpen ? (
          <Link
            role="listitem"
            data-slot="forum-item"
            data-selected
            aria-current="page"
            to="/$workspace/forum"
            params={{ workspace: workspace.urlKey }}
            search={filters}
          />
        ) : (
          <Link
            role="listitem"
            data-slot="forum-item"
            to="/$workspace/discussion/$slug"
            params={{ workspace: workspace.urlKey, slug: entitySlug(discussion) }}
            search={filters}
          />
        )
      }
    >
      {content}
    </DiscussionContextMenu>
  );
}

const MORE_CHIP_WIDTH = 36;
const TAG_GAP = 4;

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
    <span ref={rowRef} data-slot="forum-item-tags">
      {tags.map((tag, index) => (
        <span key={tag} data-tag data-overflow={index >= shown || undefined}>
          <span aria-hidden data-slot="tag-hash">
            #
          </span>
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
      {!cover && <ImageIcon />}
    </span>
  );
}

/** The divider between the list and the open discussion: drag or use the arrow keys to resize the list. */
function PaneResizer({
  panesRef,
  onResize,
  onReset,
}: {
  panesRef: RefObject<HTMLDivElement | null>;
  onResize: (width: number) => void;
  onReset: () => void;
}) {
  const resize = (width: number) => {
    const panes = panesRef.current;
    if (!panes) return;
    const max = panes.getBoundingClientRect().width - RESIZER_WIDTH - MIN_DETAIL_WIDTH;
    onResize(Math.round(Math.max(MIN_LIST_WIDTH, Math.min(width, max))));
  };
  const listWidth = () =>
    panesRef.current?.querySelector("[data-slot=forum-list]")?.getBoundingClientRect().width ?? 0;

  return (
    <div
      role="separator"
      aria-orientation="vertical"
      aria-label="Resize panels"
      tabIndex={0}
      data-slot="pane-resizer"
      data-focus-indicator="child"
      onPointerDown={(event) => {
        const handle = event.currentTarget;
        handle.setPointerCapture(event.pointerId);
        document.body.dataset.resizing = "";
        const left = panesRef.current?.getBoundingClientRect().left ?? 0;
        const move = (moved: PointerEvent) => resize(moved.clientX - left - RESIZER_WIDTH / 2);
        const end = () => {
          delete document.body.dataset.resizing;
          handle.removeEventListener("pointermove", move);
          handle.removeEventListener("pointerup", end);
          handle.removeEventListener("pointercancel", end);
        };
        handle.addEventListener("pointermove", move);
        handle.addEventListener("pointerup", end);
        handle.addEventListener("pointercancel", end);
      }}
      onKeyDown={(event) => {
        if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;
        event.preventDefault();
        resize(listWidth() + (event.key === "ArrowRight" ? RESIZE_STEP : -RESIZE_STEP));
      }}
      onDoubleClick={onReset}
    />
  );
}

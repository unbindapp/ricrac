import { useInfiniteQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { zodValidator } from "@tanstack/zod-adapter";
import { type ReactNode, useState } from "react";
import { useDebounceValue, useLocalStorage } from "usehooks-ts";
import { z } from "zod";

import type { Discussion } from "@/api/ops.gen";
import {
  DiscussionCard,
  DiscussionGrid,
  DiscussionGridSkeleton,
  DiscussionRow,
} from "@/components/forum/discussion-card";
import { DiscussionTagFilter } from "@/components/forum/discussion-tags";
import { NewDiscussionDialog } from "@/components/forum/new-discussion-dialog";
import { useCanRewrite } from "@/components/forum/use-can-rewrite";
import {
  LayersIcon,
  LayoutGridIcon,
  ListIcon,
  LockIcon,
  LockOpenIcon,
  type LucideIcon,
  MessagesSquareIcon,
  PlusIcon,
  SearchIcon,
  Trash2Icon,
} from "@/components/icons";
import { List } from "@/components/layout/list";
import { ListStates } from "@/components/layout/list-states";
import { PageContainer } from "@/components/layout/page-container";
import { PageHeader } from "@/components/layout/page-header";
import { PageToolbar } from "@/components/layout/page-toolbar";
import { CreateButton } from "@/components/layout/create-button";
import { TrashedRow } from "@/components/projects/entity-menu";
import { Button } from "@/components/ui/button";
import { SearchField } from "@/components/ui/search-field";
import { toast } from "@/components/ui/toast";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { FilterMenu } from "@/components/shared/filter-menu";
import { LayoutToggle } from "@/components/shared/layout-toggle";
import { discussionsQuery, useDiscussionLifecycleMutation } from "@/lib/queries/discussions";

const VIEWS = ["open", "closed", "all", "trash"] as const;
type TForumView = (typeof VIEWS)[number];
type TLayout = "grid" | "list";

const searchSchema = z.object({
  status: z.enum(["closed", "all", "trash"]).optional(),
  team: z.string().optional(),
  tag: z.string().optional(),
});

export const Route = createFileRoute("/$workspace/_workspace/forum")({
  validateSearch: zodValidator(searchSchema),
  component: ForumPage,
});

const views: Record<TForumView, { label: string; icon: LucideIcon }> = {
  open: { label: "Open", icon: LockOpenIcon },
  closed: { label: "Closed", icon: LockIcon },
  all: { label: "All", icon: LayersIcon },
  trash: { label: "Trash", icon: Trash2Icon },
};

function ForumPage() {
  const { workspace, canEdit } = useWorkspace();
  const { status = "open", tag } = Route.useSearch();
  const navigate = Route.useNavigate();
  const [layout, setLayout] = useLocalStorage<TLayout>("forum-layout", "grid");
  const [query, setQuery] = useState("");
  const [debouncedQuery] = useDebounceValue(query.trim(), 300);
  const isTrash = status === "trash";
  const isGrid = layout === "grid" && !isTrash;
  const list = useInfiniteQuery(
    discussionsQuery(workspace.id, {
      status: isTrash ? "all" : status,
      onlyTrashed: isTrash || undefined,
      tag,
      query: debouncedQuery || undefined,
      includeBody: !isTrash || undefined,
    }),
  );
  const discussions = list.data?.pages.flatMap((page) => page.nodes) ?? [];
  const newButton = (placement: "header" | "empty") => (
    <NewDiscussionDialog
      trigger={
        placement === "header" ? (
          <CreateButton label="New discussion" />
        ) : (
          <Button size="sm" variant="outline">
            <PlusIcon className="size-4" />
            New discussion
          </Button>
        )
      }
    />
  );

  return (
    <PageContainer>
      <PageHeader
        title="Forum"
        icon={<MessagesSquareIcon />}
        actions={canEdit && newButton("header")}
        toolbar={
          <PageToolbar
            actions={
              <>
                <FilterMenu
                  label="Show"
                  value={status}
                  options={VIEWS.map((value) => ({ value, ...views[value] }))}
                  onChange={(value) =>
                    void navigate({
                      search: (prev) => ({ ...prev, status: value !== "open" ? value : undefined }),
                      replace: true,
                    })
                  }
                />
                {!isTrash && (
                  <LayoutToggle
                    value={layout}
                    options={[
                      { value: "grid", label: "Grid", icon: LayoutGridIcon },
                      { value: "list", label: "List", icon: ListIcon },
                    ]}
                    onChange={setLayout}
                  />
                )}
              </>
            }
          >
            <SearchField
              aria-label="Search discussions"
              placeholder="Search discussions…"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Escape") setQuery("");
              }}
            />
            <DiscussionTagFilter
              value={tag ?? null}
              onChange={(next) =>
                void navigate({
                  search: (prev) => ({ ...prev, tag: next ?? undefined }),
                  replace: true,
                })
              }
            />
          </PageToolbar>
        }
      />
      <ListStates
        isPending={list.isPending}
        skeleton={isGrid ? <DiscussionGridSkeleton /> : undefined}
        error={list.error}
        onRetry={() => void list.refetch()}
        isEmpty={discussions.length === 0}
        empty={emptyState({
          status,
          isFiltered: Boolean(debouncedQuery || tag),
          action: canEdit && status !== "closed" && !isTrash && newButton("empty"),
        })}
      >
        <div className="flex w-full flex-col gap-3">
          {isTrash && (
            <List flush>
              {discussions.map((discussion) => (
                <TrashedDiscussionRow key={discussion.id} discussion={discussion} />
              ))}
            </List>
          )}
          {isGrid && (
            <DiscussionGrid>
              {discussions.map((discussion) => (
                <DiscussionCard key={discussion.id} discussion={discussion} />
              ))}
            </DiscussionGrid>
          )}
          {!isTrash && !isGrid && (
            <List flush>
              {discussions.map((discussion) => (
                <DiscussionRow key={discussion.id} discussion={discussion} />
              ))}
            </List>
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
        </div>
      </ListStates>
    </PageContainer>
  );
}

function TrashedDiscussionRow({ discussion }: { discussion: Discussion }) {
  const { workspace } = useWorkspace();
  const canRestore = useCanRewrite(discussion.creator);
  const { mutate: change, isPending } = useDiscussionLifecycleMutation(workspace.id);
  return (
    <TrashedRow
      icon={<MessagesSquareIcon className="size-4" />}
      name={discussion.title}
      trashedAt={discussion.trashedAt}
      isPending={isPending}
      onRestore={
        canRestore
          ? () =>
              change(
                { id: discussion.id, action: "restore" },
                { onSuccess: () => toast.add({ type: "success", title: "Discussion restored" }) },
              )
          : undefined
      }
    />
  );
}

function emptyState({
  status,
  isFiltered,
  action,
}: {
  status: TForumView;
  isFiltered: boolean;
  action: ReactNode;
}) {
  if (isFiltered) {
    return {
      icon: SearchIcon,
      title: "No matching discussions",
      description: "Nothing matches these filters.",
    };
  }
  if (status === "trash") {
    return {
      icon: Trash2Icon,
      title: "No trashed discussions yet",
      description: "Discussions moved to the trash stay here for 30 days.",
    };
  }
  if (status === "closed") {
    return {
      icon: MessagesSquareIcon,
      title: "No closed discussions yet",
      description: "Discussions closed once they are settled show up here.",
    };
  }
  return {
    icon: MessagesSquareIcon,
    title: "No discussions yet",
    description: "Start one to talk through an idea, a design or a problem together.",
    action,
  };
}

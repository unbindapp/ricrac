import { useInfiniteQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { zodValidator } from "@tanstack/zod-adapter";
import { useState } from "react";
import { useDebounceValue, useLocalStorage } from "usehooks-ts";
import { z } from "zod";

import {
  DiscussionCard,
  DiscussionGrid,
  DiscussionGridSkeleton,
  DiscussionRow,
} from "@/components/forum/discussion-card";
import { DiscussionTagFilter } from "@/components/forum/discussion-tags";
import { NewDiscussionDialog } from "@/components/forum/new-discussion-dialog";
import {
  forumDiscussionsQuery,
  forumEmptyState,
  forumStatusSearch,
  forumViewOptions,
  TrashedDiscussionRow,
} from "@/components/forum/forum-views";
import { LayoutGridIcon, ListIcon, MessagesSquareIcon, PlusIcon } from "@/components/icons";
import { List } from "@/components/layout/list";
import { ListStates } from "@/components/layout/list-states";
import { PageContainer } from "@/components/layout/page-container";
import { PageHeader } from "@/components/layout/page-header";
import { PageToolbar } from "@/components/layout/page-toolbar";
import { CreateButton } from "@/components/layout/create-button";
import { Button } from "@/components/ui/button";
import { SearchField } from "@/components/ui/search-field";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { FilterMenu } from "@/components/shared/filter-menu";
import { LayoutToggle } from "@/components/shared/layout-toggle";
import { useUiTheme } from "@/themes/ui-theme-provider";

type TLayout = "grid" | "list";

const searchSchema = z.object({
  status: z.enum(["closed", "all", "trash"]).optional(),
  sort: z.enum(["newest", "top"]).optional(),
  team: z.string().optional(),
  tag: z.string().optional(),
});

export const Route = createFileRoute("/$workspace/_workspace/forum")({
  validateSearch: zodValidator(searchSchema),
  component: ForumPage,
});

function ForumPage() {
  const { ForumPanes } = useUiTheme();
  const { status, tag, sort } = Route.useSearch();
  if (ForumPanes) return <ForumPanes filters={{ status, tag, sort }} />;
  return <ForumList />;
}

function ForumList() {
  const { workspace, canEdit } = useWorkspace();
  const { status = "open", tag } = Route.useSearch();
  const navigate = Route.useNavigate();
  const [layout, setLayout] = useLocalStorage<TLayout>("forum-layout", "grid");
  const [query, setQuery] = useState("");
  const [debouncedQuery] = useDebounceValue(query.trim(), 300);
  const isTrash = status === "trash";
  const isGrid = layout === "grid" && !isTrash;
  const list = useInfiniteQuery(
    forumDiscussionsQuery(workspace.id, { status, tag, query: debouncedQuery }),
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
                  options={forumViewOptions}
                  onChange={(value) =>
                    void navigate({
                      search: (prev) => ({ ...prev, status: forumStatusSearch(value) }),
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
        empty={forumEmptyState({
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

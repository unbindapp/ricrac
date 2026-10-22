import { useInfiniteQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { zodValidator } from "@tanstack/zod-adapter";
import { type ReactNode, useState } from "react";
import { useDebounceValue } from "usehooks-ts";
import { z } from "zod";

import type { Discussion } from "@/api/ops.gen";
import {
  DiscussionCard,
  DiscussionGrid,
  DiscussionGridSkeleton,
} from "@/components/forum/discussion-card";
import { DiscussionTagFilter } from "@/components/forum/discussion-tags";
import { NewDiscussionDialog } from "@/components/forum/new-discussion-dialog";
import { useCanRewrite } from "@/components/forum/use-can-rewrite";
import { MessagesSquareIcon, PlusIcon, SearchIcon, Trash2Icon } from "@/components/icons";
import { List } from "@/components/layout/list";
import { ListStates } from "@/components/layout/list-states";
import { PageContainer } from "@/components/layout/page-container";
import { PageHeader } from "@/components/layout/page-header";
import { PageToolbar } from "@/components/layout/page-toolbar";
import { CreateButton } from "@/components/layout/create-button";
import { TrashedRow } from "@/components/projects/entity-menu";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { toast } from "@/components/ui/toast";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { FilterMenu } from "@/components/shared/filter-menu";
import { discussionsQuery, useDiscussionLifecycleMutation } from "@/lib/queries/discussions";

const VIEWS = ["open", "closed", "all", "trash"] as const;
type TForumView = (typeof VIEWS)[number];

const searchSchema = z.object({
  status: z.enum(["closed", "all", "trash"]).optional(),
  team: z.string().optional(),
  tag: z.string().optional(),
});

export const Route = createFileRoute("/$workspace/_workspace/forum")({
  validateSearch: zodValidator(searchSchema),
  component: ForumPage,
});

const viewLabels: Record<TForumView, string> = {
  open: "Open",
  closed: "Closed",
  all: "All",
  trash: "Trash",
};

function ForumPage() {
  const { workspace, canEdit } = useWorkspace();
  const { status = "open", tag } = Route.useSearch();
  const navigate = Route.useNavigate();
  const [query, setQuery] = useState("");
  const [debouncedQuery] = useDebounceValue(query.trim(), 300);
  const isTrash = status === "trash";
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
              <FilterMenu
                label="Show"
                value={status}
                options={VIEWS.map((value) => ({
                  value,
                  label: viewLabels[value],
                  icon: value === "trash" ? <Trash2Icon className="size-4" /> : undefined,
                }))}
                onChange={(value) =>
                  void navigate({
                    search: (prev) => ({ ...prev, status: value !== "open" ? value : undefined }),
                    replace: true,
                  })
                }
              />
            }
          >
            <div className="relative w-full max-w-xs">
              <SearchIcon className="text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
              <Input
                aria-label="Search discussions"
                placeholder="Search discussions…"
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Escape") setQuery("");
                }}
                className="h-8 pl-8"
              />
            </div>
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
        skeleton={!isTrash && <DiscussionGridSkeleton />}
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
          {isTrash ? (
            <List flush>
              {discussions.map((discussion) => (
                <TrashedDiscussionRow key={discussion.id} discussion={discussion} />
              ))}
            </List>
          ) : (
            <DiscussionGrid>
              {discussions.map((discussion) => (
                <DiscussionCard key={discussion.id} discussion={discussion} />
              ))}
            </DiscussionGrid>
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

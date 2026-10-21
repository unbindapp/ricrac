import { useInfiniteQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { zodValidator } from "@tanstack/zod-adapter";
import { MessagesSquareIcon, PlusIcon, SearchIcon } from "lucide-react";
import { type ReactNode, useState } from "react";
import { useDebounceValue } from "usehooks-ts";
import { z } from "zod";

import { DiscussionRow } from "@/components/forum/discussion-row";
import { DiscussionTeamPicker } from "@/components/forum/discussion-team-picker";
import { NewDiscussionDialog } from "@/components/forum/new-discussion-dialog";
import { List } from "@/components/layout/list";
import { ListStates } from "@/components/layout/list-states";
import { PageContainer } from "@/components/layout/page-container";
import { PageHeader } from "@/components/layout/page-header";
import { PageToolbar } from "@/components/layout/page-toolbar";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Tabs, TabsList, TabsTab } from "@/components/ui/tabs";
import { useTeams } from "@/components/workspace/teams-provider";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { discussionsQuery, type TDiscussionStatus } from "@/lib/queries/discussions";

const searchSchema = z.object({
  status: z.enum(["closed", "all"]).optional(),
  /** A team's key. */
  team: z.string().optional(),
});

export const Route = createFileRoute("/$workspace/_workspace/forum")({
  validateSearch: zodValidator(searchSchema),
  component: ForumPage,
});

const statusLabels: Record<TDiscussionStatus, string> = {
  open: "Open",
  closed: "Closed",
  all: "All",
};

function ForumPage() {
  const { workspace, canEdit } = useWorkspace();
  const { teams } = useTeams();
  const { status = "open", team: teamKey } = Route.useSearch();
  const navigate = Route.useNavigate();
  const [query, setQuery] = useState("");
  const [debouncedQuery] = useDebounceValue(query.trim(), 300);
  const team = teams.find((item) => item.key === teamKey);
  const list = useInfiniteQuery(
    discussionsQuery(workspace.id, {
      status,
      team: teamKey,
      query: debouncedQuery || undefined,
    }),
  );
  const discussions = list.data?.pages.flatMap((page) => page.nodes) ?? [];
  const newButton = (placement: "header" | "empty") => (
    <NewDiscussionDialog
      defaultTeamId={team?.id ?? null}
      trigger={
        <Button
          size={placement === "header" ? "default" : "sm"}
          variant={placement === "header" ? "default" : "outline"}
        >
          <PlusIcon className="size-4" />
          New discussion
        </Button>
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
              <Tabs
                value={status}
                onValueChange={(value) =>
                  void navigate({
                    search: (prev) => ({
                      ...prev,
                      status: value === "closed" || value === "all" ? value : undefined,
                    }),
                    replace: true,
                  })
                }
              >
                <TabsList>
                  {(["open", "closed", "all"] as const).map((value) => (
                    <TabsTab key={value} value={value}>
                      {statusLabels[value]}
                    </TabsTab>
                  ))}
                </TabsList>
              </Tabs>
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
            <DiscussionTeamPicker
              label="Filter by team"
              noneLabel="All teams"
              value={team?.id ?? null}
              onChange={(teamId) =>
                void navigate({
                  search: (prev) => ({
                    ...prev,
                    team: teams.find((item) => item.id === teamId)?.key,
                  }),
                  replace: true,
                })
              }
            />
          </PageToolbar>
        }
      />
      <ListStates
        isPending={list.isPending}
        error={list.error}
        onRetry={() => void list.refetch()}
        isEmpty={discussions.length === 0}
        empty={emptyState({
          status,
          isFiltered: Boolean(debouncedQuery || teamKey),
          action: canEdit && status !== "closed" && newButton("empty"),
        })}
      >
        <div className="flex w-full flex-col gap-2">
          <List flush>
            {discussions.map((discussion) => (
              <DiscussionRow key={discussion.id} discussion={discussion} />
            ))}
          </List>
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

function emptyState({
  status,
  isFiltered,
  action,
}: {
  status: TDiscussionStatus;
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
    description: "Start one to talk through an idea, a design or a problem with your team.",
    action,
  };
}

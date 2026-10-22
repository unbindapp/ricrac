import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { zodValidator } from "@tanstack/zod-adapter";
import { z } from "zod";

import type { Team } from "@/api/ops.gen";
import {
  CircleDotIcon,
  ListTodoIcon,
  PlusIcon,
  TriangleIcon,
} from "@/components/icons/symbols.gen";
import {
  filtersFromSearch,
  filtersToSearch,
  issueSearchSchema,
  type ListIssuesFilters,
} from "@/components/issues/filters";
import { IssuesView } from "@/components/issues/issues-view";
import { useNewIssueStore } from "@/components/issues/new-issue-store";
import { EmptyState } from "@/components/layout/empty-state";
import { FilterMenu } from "@/components/shared/filter-menu";
import type { TStateType } from "@/components/shared/state-icon";
import { Button, LinkButton } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { WorkspaceTeamGate } from "@/components/workspace/workspace-team-gate";
import { teamStatusesQuery } from "@/lib/queries/teams";

const TABS = ["active", "backlog", "all"] as const;
type TIssuesTab = (typeof TABS)[number];

const searchSchema = issueSearchSchema.extend({
  tab: z.enum(["backlog", "all"]).optional().catch(undefined),
});

export const Route = createFileRoute("/$workspace/_workspace/issues")({
  validateSearch: zodValidator(searchSchema),
  component: IssuesPage,
});

const STATE_TYPES: Record<TIssuesTab, TStateType[]> = {
  active: ["unstarted", "started"],
  backlog: ["backlog"],
  all: ["backlog", "unstarted", "started", "completed", "canceled"],
};

const TAB_LABELS: Record<TIssuesTab, string> = {
  active: "Active",
  backlog: "Backlog",
  all: "All issues",
};

function IssuesPage() {
  const { tab, ...search } = Route.useSearch();
  const navigate = Route.useNavigate();
  return (
    <WorkspaceTeamGate>
      {(team) => (
        <Issues
          team={team}
          tab={tab ?? "active"}
          filters={filtersFromSearch(search)}
          onTabChange={(next) =>
            void navigate({
              search: (prev) => ({ ...prev, tab: next === "active" ? undefined : next }),
            })
          }
          onFiltersChange={(filters) =>
            void navigate({ search: (prev) => ({ ...prev, ...filtersToSearch(filters) }) })
          }
        />
      )}
    </WorkspaceTeamGate>
  );
}

/** Open issues per tab, from the statuses' counts (lists don't count). */
function useTabCounts(workspaceId: string, teamId: string) {
  const { data } = useQuery(teamStatusesQuery(workspaceId, teamId));
  const count = (types: TStateType[]) =>
    data
      ?.filter((status) => types.includes(status.type))
      .reduce((sum, status) => sum + status.issueCount, 0);
  return {
    active: count(["unstarted", "started"]),
    backlog: count(["backlog"]),
    triage: count(["triage"]),
  };
}

/** Active (todo and in progress), Backlog and All, with the backlog's count always in sight. */
function Issues({
  team,
  tab,
  filters,
  onTabChange,
  onFiltersChange,
}: {
  team: Team;
  tab: TIssuesTab;
  filters: ListIssuesFilters;
  onTabChange: (tab: TIssuesTab) => void;
  onFiltersChange: (filters: ListIssuesFilters) => void;
}) {
  const { workspace, canEdit } = useWorkspace();
  const openNewIssue = useNewIssueStore((state) => state.open);
  const counts = useTabCounts(workspace.id, team.id);

  const tabs = (
    <FilterMenu
      label="Issues"
      value={tab}
      options={TABS.map((item) => ({
        value: item,
        label: TAB_LABELS[item],
        suffix: item !== "all" && counts[item] !== undefined && (
          <span className="text-xs font-normal opacity-75">{counts[item]}</span>
        ),
      }))}
      onChange={onTabChange}
    />
  );

  const backlogCount = counts.backlog ?? 0;
  const triageCount = counts.triage ?? 0;
  const waiting = [
    backlogCount > 0 &&
      `${backlogCount} ${backlogCount === 1 ? "issue waits" : "issues wait"} in the backlog`,
    triageCount > 0 && `${triageCount} in triage`,
  ].filter(Boolean);
  const emptyState =
    tab === "active" && Object.keys(filters).length === 0 ? (
      <EmptyState
        icon={CircleDotIcon}
        title="No active issues yet"
        description={
          waiting.length > 0
            ? `Nothing is in Todo or In Progress; ${waiting.join(" and ")}.`
            : "Issues being worked on, or up next, show up here."
        }
        action={
          <>
            {triageCount > 0 && team.triageEnabled && (
              <LinkButton
                to="/$workspace/triage"
                params={{ workspace: workspace.urlKey }}
                variant="outline"
                size="sm"
              >
                <TriangleIcon className="size-4" />
                Open triage
              </LinkButton>
            )}
            {backlogCount > 0 && (
              <Button variant="outline" size="sm" onClick={() => onTabChange("backlog")}>
                <ListTodoIcon className="size-4" />
                Open the backlog
              </Button>
            )}
            {canEdit && waiting.length === 0 && (
              <Button variant="outline" size="sm" onClick={() => openNewIssue()}>
                <PlusIcon className="size-4" />
                New issue
              </Button>
            )}
          </>
        }
      />
    ) : undefined;

  return (
    <IssuesView
      key={`${team.id}:${tab}`}
      title="Issues"
      icon={<CircleDotIcon />}
      storageKey={`team:${team.id}:${tab}`}
      teamId={team.id}
      newIssueStateType={tab === "active" ? "unstarted" : undefined}
      fixedFilters={{ state_types: STATE_TYPES[tab] }}
      defaultDisplay={tab === "backlog" ? { order_by: "priority" } : undefined}
      filters={filters}
      onFiltersChange={onFiltersChange}
      tabs={tabs}
      emptyState={emptyState}
    />
  );
}

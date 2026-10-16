import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { CircleDotIcon, ListTodoIcon, PlusIcon, TriangleIcon } from "lucide-react";

import type { Team } from "@/api/ops.gen";
import type { ListIssuesFilters } from "@/components/issues/filters";
import { IssuesView } from "@/components/issues/issues-view";
import { EmptyState } from "@/components/layout/empty-state";
import type { TStateType } from "@/components/shared/state-icon";
import { useNewIssueStore } from "@/components/issues/new-issue-store";
import { Button, LinkButton } from "@/components/ui/button";
import { Tabs, TabsList, TabsTab } from "@/components/ui/tabs";
import { teamCrumb } from "@/components/workspace/team-crumb";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { teamStatusesQuery } from "@/lib/queries/teams";

export type TTeamIssuesTab = "active" | "backlog" | "all";

const STATE_TYPES: Record<TTeamIssuesTab, TStateType[]> = {
  active: ["unstarted", "started", "completed", "canceled"],
  backlog: ["backlog"],
  all: ["backlog", "unstarted", "started", "completed", "canceled"],
};

const TITLES: Record<TTeamIssuesTab, string> = {
  active: "Active",
  backlog: "Backlog",
  all: "All issues",
};

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

/**
 * A team's issues in three tabs: Active (todo and in progress, plus what was recently closed),
 * Backlog, and All. The backlog's count is always in sight so it isn't forgotten.
 */
export function TeamIssues({
  team,
  tab,
  filters,
  onFiltersChange,
}: {
  team: Team;
  tab: TTeamIssuesTab;
  filters: ListIssuesFilters;
  onFiltersChange: (filters: ListIssuesFilters) => void;
}) {
  const { workspace, canEdit } = useWorkspace();
  const navigate = useNavigate();
  const openNewIssue = useNewIssueStore((state) => state.open);
  const counts = useTabCounts(workspace.id, team.id);
  const params = { workspace: workspace.urlKey, key: team.key };
  const goTo = (next: TTeamIssuesTab) => {
    if (next === "backlog") {
      void navigate({ to: "/$workspace/team/$key/backlog", params });
      return;
    }
    void navigate({
      to: "/$workspace/team/$key/issues",
      params,
      search: next === "all" ? { tab: "all" } : {},
    });
  };

  const tabs = (
    <Tabs value={tab} onValueChange={(value) => goTo(value as TTeamIssuesTab)}>
      <TabsList>
        {(["active", "backlog", "all"] as const).map((item) => (
          <TabsTab key={item} value={item}>
            {TITLES[item]}
            {item !== "all" && counts[item] !== undefined && (
              <span className="text-muted-foreground text-xs font-normal">{counts[item]}</span>
            )}
          </TabsTab>
        ))}
      </TabsList>
    </Tabs>
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
            : "Issues the team is working on, or will work on next, show up here."
        }
        action={
          <>
            {triageCount > 0 && team.triageEnabled && (
              <LinkButton
                to="/$workspace/team/$key/triage"
                params={params}
                variant="outline"
                size="sm"
              >
                <TriangleIcon className="size-4" />
                Open triage
              </LinkButton>
            )}
            {backlogCount > 0 && (
              <Button variant="outline" size="sm" onClick={() => goTo("backlog")}>
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
      breadcrumb={[teamCrumb(team, workspace.urlKey)]}
      storageKey={`team:${team.id}:${tab}`}
      teamId={team.id}
      newIssueStateType={tab === "active" ? "unstarted" : undefined}
      fixedFilters={{ team_ids: [team.id], state_types: STATE_TYPES[tab] }}
      defaultDisplay={tab === "backlog" ? { order_by: "priority" } : undefined}
      filters={filters}
      onFiltersChange={onFiltersChange}
      tabs={tabs}
      emptyState={emptyState}
    />
  );
}

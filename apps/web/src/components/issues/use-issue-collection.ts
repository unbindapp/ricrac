import { useInfiniteQuery } from "@tanstack/react-query";
import { useEffect, useMemo } from "react";

import { resolveDisplay, type TDisplaySettings } from "@/components/issues/display";
import {
  buildListIssuesArgs,
  isHiddenCompleted,
  type ListIssuesFilters,
  mergeFilters,
} from "@/components/issues/filters";
import { groupIssues, type TIssueGroup } from "@/components/issues/grouping";
import type { TNewIssueDefaults } from "@/components/issues/new-issue-store";
import { useTeamDetail } from "@/components/issues/picker-sources";
import { useNow } from "@/components/providers/now-provider";
import { issuesListQuery } from "@/lib/queries/issues";

const HOUR_MS = 60 * 60 * 1000;
const CLOSED = ["completed", "canceled"];

export type TIssueCollectionProps = {
  workspaceId: string;
  filters: ListIssuesFilters;
  display?: Partial<TDisplaySettings>;
  fixedFilters?: Partial<ListIssuesFilters>;
};

/**
 * Loads every page of a list (up to LIST_MAX_ISSUES) and groups and sorts it here, since the
 * server lists don't group or count.
 */
export function useIssueCollection({
  workspaceId,
  filters,
  display: displayOverrides,
  fixedFilters,
}: TIssueCollectionProps) {
  const display = resolveDisplay(displayOverrides);
  const args = buildListIssuesArgs({ filters, fixedFilters, display });
  const query = useInfiniteQuery(issuesListQuery(workspaceId, args));
  const { hasNextPage, isFetchingNextPage, isError, fetchNextPage } = query;
  useEffect(() => {
    if (hasNextPage && !isFetchingNextPage && !isError) void fetchNextPage();
  }, [hasNextPage, isFetchingNextPage, isError, fetchNextPage]);

  const merged = mergeFilters(filters, fixedFilters);
  const teamId = merged.team_ids?.length === 1 ? merged.team_ids[0] : undefined;
  const fixedTeamId = fixedFilters?.team_ids?.length === 1 ? fixedFilters.team_ids[0] : undefined;
  const team = useTeamDetail(teamId);
  const now = useNow();
  const nowMs = Math.floor(now / HOUR_MS) * HOUR_MS;

  const pages = query.data?.pages;
  const issues = useMemo(
    () =>
      (pages ?? [])
        .flatMap((page) => page.nodes)
        .filter((issue) => !isHiddenCompleted(issue, display.show_completed, nowMs)),
    [pages, display.show_completed, nowMs],
  );

  const stateTypes = merged.state_types;
  const showsClosed = display.show_completed !== "none";
  const groups = useMemo(() => {
    const states = team?.states.filter(
      (state) =>
        (!stateTypes || stateTypes.includes(state.type)) &&
        (showsClosed || !CLOSED.includes(state.type)),
    );
    return groupIssues({
      issues,
      groupBy: display.group_by,
      orderBy: display.order_by,
      order: display.order,
      team: team && states ? { id: team.id, states } : undefined,
    });
  }, [issues, display.group_by, display.order_by, display.order, team, stateTypes, showsClosed]);

  const lastPage = pages?.[pages.length - 1];
  const isCapped = Boolean(lastPage?.pageInfo.hasNextPage) && !hasNextPage;

  return {
    query,
    issues,
    groups,
    display,
    fixedTeamId: fixedTeamId ?? teamId,
    isLoadingMore: hasNextPage || isFetchingNextPage,
    isCapped,
  };
}

export function newIssueDefaultsForGroup(
  group: TIssueGroup,
  base: TNewIssueDefaults,
): TNewIssueDefaults {
  const meta = group.meta;
  switch (meta.kind) {
    case "none":
      return base;
    case "state":
      if (!meta.teamId) return base;
      return { ...base, teamId: meta.teamId, draft: { ...base.draft, state: meta.state } };
    case "priority":
      return { ...base, draft: { ...base.draft, priority: meta.priority } };
    case "assignee":
      return { ...base, draft: { ...base.draft, assignee: meta.user } };
    case "project":
      return { ...base, draft: { ...base.draft, project: meta.project } };
    case "cycle":
      return { ...base, draft: { ...base.draft, cycle: meta.cycle } };
    case "label":
      return { ...base, draft: { ...base.draft, labels: meta.label ? [meta.label] : [] } };
    case "parent":
      return { ...base, draft: { ...base.draft, parent: meta.parent } };
    case "team":
      return { ...base, teamId: meta.team.id };
  }
}

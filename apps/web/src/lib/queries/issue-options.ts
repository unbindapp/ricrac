import { queryOptions } from "@tanstack/react-query";

import { callAll } from "@/lib/server/client";

/** What issue pickers offer. Keys sit under each area's prefix so its changes refresh them. */
export const queryKeyIssueOptions = {
  labels: (workspaceId: string, team: string | undefined) =>
    [workspaceId, "labels", "issue-options", team ?? "all"] as const,
  projects: (workspaceId: string) => [workspaceId, "projects", "issue-options"] as const,
  milestones: (workspaceId: string, project: string) =>
    [workspaceId, "projects", "issue-options", "milestones", project] as const,
  cycles: (workspaceId: string, team: string) =>
    [workspaceId, "cycles", "issue-options", team] as const,
};

/** A team's labels and the workspace's, or every label without a team. */
export const issueLabelsQuery = (workspaceId: string, team: string | undefined) =>
  queryOptions({
    queryKey: queryKeyIssueOptions.labels(workspaceId, team),
    queryFn: () => callAll(workspaceId, "list_issue_labels", team ? { team } : {}),
  });

export const issueProjectsQuery = (workspaceId: string) =>
  queryOptions({
    queryKey: queryKeyIssueOptions.projects(workspaceId),
    queryFn: () => callAll(workspaceId, "list_projects", { orderBy: "manual" }),
  });

export const projectMilestonesQuery = (workspaceId: string, project: string) =>
  queryOptions({
    queryKey: queryKeyIssueOptions.milestones(workspaceId, project),
    queryFn: () => callAll(workspaceId, "list_milestones", { project }),
  });

export const teamCyclesQuery = (workspaceId: string, team: string) =>
  queryOptions({
    queryKey: queryKeyIssueOptions.cycles(workspaceId, team),
    queryFn: () => callAll(workspaceId, "list_cycles", { team }),
  });

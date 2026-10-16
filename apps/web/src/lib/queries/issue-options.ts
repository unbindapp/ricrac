import { queryOptions } from "@tanstack/react-query";

import { callAll } from "@/lib/server/client";

/** What issue pickers offer. Keys sit under each area's prefix so its changes refresh them. */
export const queryKeyIssueOptions = {
  projects: (workspaceId: string) => [workspaceId, "projects", "issue-options"] as const,
  milestones: (workspaceId: string, project: string) =>
    [workspaceId, "projects", "issue-options", "milestones", project] as const,
};

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

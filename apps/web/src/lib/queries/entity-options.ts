import { keepPreviousData, queryOptions } from "@tanstack/react-query";

import { call, callAll } from "@/lib/server/client";

const ISSUE_SEARCH_LIMIT = 20;

/**
 * Small lists for pickers outside the issues and projects areas. Their keys sit under "projects"
 * and "issues", so the invalidations of those areas refresh them too.
 */
export const queryKeyEntityOptions = {
  projects: (workspaceId: string) => [workspaceId, "projects", "options"] as const,
  issues: (workspaceId: string, query: string) =>
    [workspaceId, "issues", "options", query] as const,
};

export const projectOptionsQuery = (workspaceId: string) =>
  queryOptions({
    queryKey: queryKeyEntityOptions.projects(workspaceId),
    queryFn: () => callAll(workspaceId, "list_projects", {}),
    staleTime: 30 * 1000,
  });

const identifierPattern = /^[a-z][a-z0-9]*-\d+$/i;

/** Issues whose title or description match, recently updated first; an identifier finds its issue. */
export const issueOptionsQuery = (workspaceId: string, query: string) =>
  queryOptions({
    queryKey: queryKeyEntityOptions.issues(workspaceId, query),
    queryFn: async () => {
      const trimmed = query.trim();
      const [page, exact] = await Promise.all([
        call(workspaceId, "list_issues", { query: trimmed || null, limit: ISSUE_SEARCH_LIMIT }),
        identifierPattern.test(trimmed)
          ? call(workspaceId, "get_issue", { id: trimmed }).catch(() => null)
          : null,
      ]);
      if (!exact) return page.nodes;
      return [exact, ...page.nodes.filter((issue) => issue.id !== exact.id)];
    },
    placeholderData: keepPreviousData,
    staleTime: 10 * 1000,
  });

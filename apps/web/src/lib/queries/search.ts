import { keepPreviousData, queryOptions } from "@tanstack/react-query";

import { call } from "@/lib/server/client";

export type TSearchHit = Awaited<ReturnType<typeof search>>[number];

const SEARCH_TYPES = ["issue", "project", "initiative", "document", "discussion", "comment"];
const SEARCH_LIMIT = 25;

export const queryKeySearch = {
  all: (workspaceId: string) => [workspaceId, "search"] as const,
  query: (workspaceId: string, query: string) => [workspaceId, "search", query] as const,
};

async function search(workspaceId: string, query: string) {
  const results = await call(workspaceId, "search", {
    query,
    types: SEARCH_TYPES,
    limit: SEARCH_LIMIT,
  });
  return results.nodes;
}

/** Issues, projects, initiatives, documents, discussions and comments you can see, best matches first. */
export const searchQuery = (workspaceId: string, query: string) =>
  queryOptions({
    queryKey: queryKeySearch.query(workspaceId, query),
    queryFn: () => search(workspaceId, query),
    enabled: query.length > 0,
    staleTime: 30 * 1000,
    placeholderData: keepPreviousData,
  });

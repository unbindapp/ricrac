import {
  infiniteQueryOptions,
  queryOptions,
  useMutation,
  useQueryClient,
} from "@tanstack/react-query";

import type { CustomView } from "@/api/ops.gen";
import { queryKeyFavorites } from "@/lib/queries/favorites";
import { call, callAll, type OpArgs } from "@/lib/server/client";

export type TView = CustomView;
export type TViewModel = CustomView["model"];
export type TFilter = Record<string, unknown>;

export const queryKeyViews = {
  all: (workspaceId: string) => [workspaceId, "views"] as const,
  list: (workspaceId: string) => [workspaceId, "views", "list"] as const,
  detail: (workspaceId: string, view: string) => [workspaceId, "views", "detail", view] as const,
  results: (workspaceId: string, model: TViewModel, filter: TFilter) =>
    [workspaceId, "views", "results", model, filter] as const,
};

export const viewsQuery = (workspaceId: string) =>
  queryOptions({
    queryKey: queryKeyViews.list(workspaceId),
    queryFn: () => callAll(workspaceId, "list_custom_views", {}),
  });

/** A view by id, slug or link tail (its route param). */
export const viewQuery = (workspaceId: string, view: string) =>
  queryOptions({
    queryKey: queryKeyViews.detail(workspaceId, view),
    queryFn: () => call(workspaceId, "get_custom_view", { id: view }),
    retry: false,
  });

/** The part of a view's link after /view/: its name and slug id, which get_custom_view takes. */
export function viewParam(view: Pick<TView, "url" | "slugId">) {
  return view.url.split("/view/")[1] ?? view.slugId;
}

const RESULTS_PAGE_SIZE = 100;

/** The issues a filter matches, as an issue view shows them (also unsaved edits of its filter). */
export const viewIssuesQuery = (workspaceId: string, filter: TFilter) =>
  infiniteQueryOptions({
    queryKey: queryKeyViews.results(workspaceId, "issue", filter),
    queryFn: ({ pageParam }) =>
      call(workspaceId, "list_issues", { filter, cursor: pageParam, limit: RESULTS_PAGE_SIZE }),
    initialPageParam: null as string | null,
    getNextPageParam: (page) => (page.pageInfo.hasNextPage ? page.pageInfo.endCursor : null),
  });

export const viewProjectsQuery = (workspaceId: string, filter: TFilter) =>
  infiniteQueryOptions({
    queryKey: queryKeyViews.results(workspaceId, "project", filter),
    queryFn: ({ pageParam }) =>
      call(workspaceId, "list_projects", { filter, cursor: pageParam, limit: RESULTS_PAGE_SIZE }),
    initialPageParam: null as string | null,
    getNextPageParam: (page) => (page.pageInfo.hasNextPage ? page.pageInfo.endCursor : null),
  });

/** The projects a view filter can pick from. */
export const projectOptionsQuery = (workspaceId: string) =>
  queryOptions({
    queryKey: [workspaceId, "views", "projectOptions"] as const,
    queryFn: () => callAll(workspaceId, "list_projects", {}),
  });

function useInvalidateViews(workspaceId: string) {
  const queryClient = useQueryClient();
  return () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: queryKeyViews.all(workspaceId) }),
      queryClient.invalidateQueries({ queryKey: queryKeyFavorites.all(workspaceId) }),
    ]);
}

export function useSaveViewMutation(workspaceId: string) {
  const invalidate = useInvalidateViews(workspaceId);
  return useMutation({
    mutationFn: (args: OpArgs<"save_custom_view">) => call(workspaceId, "save_custom_view", args),
    meta: { errorTitle: "Could not save view" },
    onSuccess: invalidate,
  });
}

export function useDeleteViewMutation(workspaceId: string) {
  const invalidate = useInvalidateViews(workspaceId);
  return useMutation({
    mutationFn: (id: string) => call(workspaceId, "delete_custom_view", { id }),
    meta: { errorTitle: "Could not delete view" },
    onSuccess: invalidate,
  });
}

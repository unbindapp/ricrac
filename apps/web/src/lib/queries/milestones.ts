import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";

import { DETAIL_REFETCH_INTERVAL_MS } from "@/lib/queries/issues";
import { call, callAll, type OpArgs } from "@/lib/server/client";

export const queryKeyMilestones = {
  all: (workspaceId: string) => [workspaceId, "milestones"] as const,
  list: (workspaceId: string) => [workspaceId, "milestones", "list"] as const,
  detail: (workspaceId: string, id: string) => [workspaceId, "milestones", "detail", id] as const,
};

/** The workspace's own milestones, those in no project; open ones first. */
export const milestonesQuery = (workspaceId: string) =>
  queryOptions({
    queryKey: queryKeyMilestones.list(workspaceId),
    queryFn: () => callAll(workspaceId, "list_milestones", {}),
  });

export const milestoneQuery = (workspaceId: string, id: string) =>
  queryOptions({
    queryKey: queryKeyMilestones.detail(workspaceId, id),
    queryFn: () => call(workspaceId, "get_milestone", { id }),
    refetchInterval: DETAIL_REFETCH_INTERVAL_MS,
  });

/** Milestones show in projects and on issues, so those refresh with them. */
function useInvalidateMilestones(workspaceId: string) {
  const queryClient = useQueryClient();
  return () =>
    Promise.all(
      ["milestones", "projects", "issues"].map((area) =>
        queryClient.invalidateQueries({ queryKey: [workspaceId, area] }),
      ),
    );
}

export function useSaveMilestoneMutation(workspaceId: string) {
  const invalidate = useInvalidateMilestones(workspaceId);
  return useMutation({
    mutationFn: (args: OpArgs<"save_milestone">) => call(workspaceId, "save_milestone", args),
    meta: { errorTitle: "Could not save milestone" },
    onSuccess: invalidate,
  });
}

export function useDeleteMilestoneMutation(workspaceId: string) {
  const invalidate = useInvalidateMilestones(workspaceId);
  return useMutation({
    mutationFn: (id: string) => call(workspaceId, "delete_milestone", { id }),
    meta: { errorTitle: "Could not delete milestone" },
    onSuccess: invalidate,
  });
}

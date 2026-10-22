import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";

import type { Cycle, CycleDetail, CycleRef } from "@/api/ops.gen";
import { queryKeyFavorites } from "@/lib/queries/favorites";
import { queryKeyTeams } from "@/lib/queries/teams";
import { call, callAll, type OpArgs } from "@/lib/server/client";

export type TCycle = Cycle;
export type TCycleDetail = CycleDetail;

export const queryKeyCycles = {
  all: (workspaceId: string) => [workspaceId, "cycles"] as const,
  list: (workspaceId: string, team: string, includeArchived: boolean) =>
    [workspaceId, "cycles", "list", team, includeArchived] as const,
  detail: (workspaceId: string, team: string, number: number) =>
    [workspaceId, "cycles", "detail", team, number] as const,
};

/** A team's cycles, newest first. */
export const cyclesQuery = (workspaceId: string, team: string, includeArchived = false) =>
  queryOptions({
    queryKey: queryKeyCycles.list(workspaceId, team, includeArchived),
    queryFn: async () => {
      const cycles = await callAll(workspaceId, "list_cycles", { team, includeArchived });
      return cycles.sort((a, b) => b.startsAt.localeCompare(a.startsAt));
    },
  });

/** A cycle by its number in the team, with its issues and daily history. */
export const cycleQuery = (workspaceId: string, team: string, number: number) =>
  queryOptions({
    queryKey: queryKeyCycles.detail(workspaceId, team, number),
    queryFn: () => call(workspaceId, "get_cycle", { id: String(number), team }),
  });

function useInvalidateCycles(workspaceId: string) {
  const queryClient = useQueryClient();
  return () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: queryKeyCycles.all(workspaceId) }),
      queryClient.invalidateQueries({ queryKey: queryKeyTeams.all(workspaceId) }),
      queryClient.invalidateQueries({ queryKey: queryKeyFavorites.all(workspaceId) }),
    ]);
}

export function useSaveCycleMutation(workspaceId: string) {
  const invalidate = useInvalidateCycles(workspaceId);
  return useMutation({
    mutationFn: (args: OpArgs<"save_cycle">) => call(workspaceId, "save_cycle", args),
    meta: { errorTitle: "Could not save cycle" },
    onSuccess: invalidate,
  });
}

export function useArchiveCycleMutation(workspaceId: string) {
  const invalidate = useInvalidateCycles(workspaceId);
  return useMutation({
    mutationFn: (id: string) => call(workspaceId, "archive_cycle", { id }),
    meta: { errorTitle: "Could not archive cycle" },
    onSuccess: invalidate,
  });
}

export function cycleName(cycle: Pick<CycleRef, "name" | "number">) {
  return cycle.name || `Cycle ${cycle.number}`;
}

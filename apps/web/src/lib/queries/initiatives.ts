import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";
import { z } from "zod";

import type { Initiative } from "@/api/ops.gen";
import { slugIdFromParam } from "@/lib/helpers/entity-slug";
import { DETAIL_REFETCH_INTERVAL_MS } from "@/lib/queries/issues";
import { savePatchOrText, type TTextEdit } from "@/lib/queries/text-edits";
import { call, callAll, type OpArgs } from "@/lib/server/client";

export type TInitiativeStatus = Initiative["status"];
export type TInitiativeListArgs = Omit<OpArgs<"list_initiatives">, "cursor" | "limit">;

export const queryKeyInitiatives = {
  all: (workspaceId: string) => [workspaceId, "initiatives"] as const,
  list: (workspaceId: string, args: TInitiativeListArgs) =>
    [workspaceId, "initiatives", "list", args] as const,
  detail: (workspaceId: string, slugId: string) =>
    [workspaceId, "initiatives", "detail", slugId] as const,
};

export const initiativesQuery = (workspaceId: string, args: TInitiativeListArgs = {}) =>
  queryOptions({
    queryKey: queryKeyInitiatives.list(workspaceId, args),
    queryFn: async () =>
      (await callAll(workspaceId, "list_initiatives", args)).sort(
        (a, b) => a.sortOrder - b.sortOrder,
      ),
  });

/** An initiative by its link slug, slug id or id; cached by slug id. */
export const initiativeQuery = (workspaceId: string, initiative: string) => {
  const slugId = slugIdFromParam(initiative);
  return queryOptions({
    queryKey: queryKeyInitiatives.detail(workspaceId, slugId),
    queryFn: () => call(workspaceId, "get_initiative", { id: slugId }),
    refetchInterval: DETAIL_REFETCH_INTERVAL_MS,
  });
};

/** Initiatives list their projects, and projects their initiatives, so both refresh. */
function useInvalidateInitiatives(workspaceId: string) {
  const queryClient = useQueryClient();
  return () =>
    Promise.all(
      ["initiatives", "projects", "status-updates"].map((area) =>
        queryClient.invalidateQueries({ queryKey: [workspaceId, area] }),
      ),
    );
}

export function useSaveInitiativeMutation(workspaceId: string) {
  const invalidate = useInvalidateInitiatives(workspaceId);
  return useMutation({
    mutationFn: ({ edit, ...args }: OpArgs<"save_initiative"> & { edit?: TTextEdit }) => {
      if (!edit) return call(workspaceId, "save_initiative", args);
      return savePatchOrText(edit, (patch) =>
        call(
          workspaceId,
          "save_initiative",
          patch ? { ...args, patch } : { ...args, description: edit.after },
        ),
      );
    },
    meta: { errorTitle: "Could not save initiative" },
    onSuccess: invalidate,
  });
}

export type TInitiativeLifecycleAction = "archive" | "unarchive" | "trash" | "restore";

const initiativeLifecycleOps = {
  archive: "archive_initiative",
  unarchive: "unarchive_initiative",
  trash: "delete_initiative",
  restore: "restore_initiative",
} as const;

export function useInitiativeLifecycleMutation(workspaceId: string) {
  const invalidate = useInvalidateInitiatives(workspaceId);
  return useMutation({
    mutationFn: ({ id, action }: { id: string; action: TInitiativeLifecycleAction }) =>
      call(workspaceId, initiativeLifecycleOps[action], { id }),
    meta: { errorTitle: "Could not change the initiative" },
    onSuccess: invalidate,
  });
}

export const initiativeNameMaxLength = 80;

export const InitiativeFormSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, "Name is required.")
    .max(initiativeNameMaxLength, `Name should be at most ${initiativeNameMaxLength} characters.`),
  summary: z.string().trim().max(255, "Keep the summary under 255 characters."),
});

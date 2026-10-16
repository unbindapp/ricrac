import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";

import type { IssueLabel } from "@/api/ops.gen";
import { call, callAll, type OpArgs } from "@/lib/server/client";

export type TLabel = IssueLabel;
export type TLabelGroupType = NonNullable<IssueLabel["groupType"]>;
/** A label with the labels of its group, when it is one. */
export type TLabelNode = TLabel & { labels: TLabel[] };

export const queryKeyLabels = {
  all: (workspaceId: string) => [workspaceId, "labels"] as const,
  list: (workspaceId: string, team: string | null, includeRetired: boolean) =>
    [workspaceId, "labels", "list", team, includeRetired] as const,
};

/** With team: its labels and the workspace's; without: the workspace's and every visible team's. */
export const labelsQuery = (
  workspaceId: string,
  { team = null, includeRetired = false }: { team?: string | null; includeRetired?: boolean } = {},
) =>
  queryOptions({
    queryKey: queryKeyLabels.list(workspaceId, team, includeRetired),
    queryFn: () => callAll(workspaceId, "list_issue_labels", { team, includeRetired }),
  });

/** Top-level labels and groups, each group holding its labels, both by name. */
export function labelTree(labels: TLabel[]): TLabelNode[] {
  const byName = (a: TLabel, b: TLabel) => a.name.localeCompare(b.name);
  return labels
    .filter((label) => !label.parent)
    .sort(byName)
    .map((label) => ({
      ...label,
      labels: labels.filter((child) => child.parent?.id === label.id).sort(byName),
    }));
}

function useInvalidateLabels(workspaceId: string) {
  const queryClient = useQueryClient();
  return () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: queryKeyLabels.all(workspaceId) }),
      queryClient.invalidateQueries({ queryKey: [workspaceId, "issues"] }),
    ]);
}

export function useSaveLabelMutation(workspaceId: string) {
  const invalidate = useInvalidateLabels(workspaceId);
  return useMutation({
    mutationFn: (args: OpArgs<"save_issue_label">) => call(workspaceId, "save_issue_label", args),
    meta: { errorTitle: "Could not save label" },
    onSuccess: invalidate,
  });
}

export function useRetireLabelMutation(workspaceId: string) {
  const invalidate = useInvalidateLabels(workspaceId);
  return useMutation({
    mutationFn: (id: string) => call(workspaceId, "retire_issue_label", { id }),
    meta: { errorTitle: "Could not retire label" },
    onSuccess: invalidate,
  });
}

export function useRestoreLabelMutation(workspaceId: string) {
  const invalidate = useInvalidateLabels(workspaceId);
  return useMutation({
    mutationFn: (id: string) => call(workspaceId, "restore_issue_label", { id }),
    meta: { errorTitle: "Could not restore label" },
    onSuccess: invalidate,
  });
}

export function useDeleteLabelMutation(workspaceId: string) {
  const invalidate = useInvalidateLabels(workspaceId);
  return useMutation({
    mutationFn: (id: string) => call(workspaceId, "delete_issue_label", { id }),
    meta: { errorTitle: "Could not delete label" },
    onSuccess: invalidate,
  });
}

import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";

import { call, callAll, type OpArgs } from "@/lib/server/client";

export const queryKeyReleases = {
  all: (workspaceId: string) => [workspaceId, "releases"] as const,
  pipelines: (workspaceId: string) => [workspaceId, "releases", "pipelines"] as const,
  list: (workspaceId: string, pipeline: string) =>
    [workspaceId, "releases", "list", pipeline] as const,
  detail: (workspaceId: string, id: string) => [workspaceId, "releases", "detail", id] as const,
  notes: (workspaceId: string, pipeline: string) =>
    [workspaceId, "releases", "notes", pipeline] as const,
  note: (workspaceId: string, id: string) => [workspaceId, "releases", "note", id] as const,
};

/** Every pipeline you can see, with its stages in order and its teams. */
export const pipelinesQuery = (workspaceId: string) =>
  queryOptions({
    queryKey: queryKeyReleases.pipelines(workspaceId),
    queryFn: () =>
      callAll(workspaceId, "list_release_pipelines", { includeStages: true, includeTeams: true }),
  });

export const releasesQuery = (workspaceId: string, pipeline: string) =>
  queryOptions({
    queryKey: queryKeyReleases.list(workspaceId, pipeline),
    queryFn: () => callAll(workspaceId, "list_releases", { pipeline }),
  });

/** A release by id, slug or name, with its issues and the notes that cover it. */
export const releaseQuery = (workspaceId: string, id: string) =>
  queryOptions({
    queryKey: queryKeyReleases.detail(workspaceId, id),
    queryFn: () => call(workspaceId, "get_release", { id }),
  });

export const releaseNotesQuery = (workspaceId: string, pipeline: string) =>
  queryOptions({
    queryKey: queryKeyReleases.notes(workspaceId, pipeline),
    queryFn: () => callAll(workspaceId, "list_release_notes", { pipeline }),
  });

export const releaseNoteQuery = (workspaceId: string, id: string) =>
  queryOptions({
    queryKey: queryKeyReleases.note(workspaceId, id),
    queryFn: () => call(workspaceId, "get_release_note", { id }),
  });

function useInvalidateReleases(workspaceId: string) {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: queryKeyReleases.all(workspaceId) });
}

export function useSavePipelineMutation(workspaceId: string) {
  const invalidate = useInvalidateReleases(workspaceId);
  return useMutation({
    mutationFn: (args: OpArgs<"save_release_pipeline">) =>
      call(workspaceId, "save_release_pipeline", args),
    meta: { errorTitle: "Could not save pipeline" },
    onSuccess: invalidate,
  });
}

export function useDeletePipelineMutation(workspaceId: string) {
  const invalidate = useInvalidateReleases(workspaceId);
  return useMutation({
    mutationFn: (id: string) => call(workspaceId, "delete_release_pipeline", { id }),
    meta: { errorTitle: "Could not archive pipeline" },
    onSuccess: invalidate,
  });
}

export function useSaveStageMutation(workspaceId: string) {
  const invalidate = useInvalidateReleases(workspaceId);
  return useMutation({
    mutationFn: (args: OpArgs<"save_release_stage">) =>
      call(workspaceId, "save_release_stage", args),
    meta: { errorTitle: "Could not save stage" },
    onSuccess: invalidate,
  });
}

export function useDeleteStageMutation(workspaceId: string) {
  const invalidate = useInvalidateReleases(workspaceId);
  return useMutation({
    mutationFn: (args: OpArgs<"delete_release_stage">) =>
      call(workspaceId, "delete_release_stage", args),
    meta: { errorTitle: "Could not delete stage" },
    onSuccess: invalidate,
  });
}

export function useSaveReleaseMutation(workspaceId: string) {
  const invalidate = useInvalidateReleases(workspaceId);
  return useMutation({
    mutationFn: (args: OpArgs<"save_release">) => call(workspaceId, "save_release", args),
    meta: { errorTitle: "Could not save release" },
    onSuccess: invalidate,
  });
}

export function useDeleteReleaseMutation(workspaceId: string) {
  const invalidate = useInvalidateReleases(workspaceId);
  return useMutation({
    mutationFn: (id: string) => call(workspaceId, "delete_release", { id }),
    meta: { errorTitle: "Could not archive release" },
    onSuccess: invalidate,
  });
}

/** Adds issues to a release or takes them out, through save_issue; issue queries refresh too. */
export function useReleaseIssuesMutation(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      release,
      issue,
      action,
    }: {
      release: string;
      issue: string;
      action: "add" | "remove";
    }) =>
      call(workspaceId, "save_issue", {
        id: issue,
        ...(action === "add" ? { addReleases: [release] } : { removeReleases: [release] }),
      }),
    meta: { errorTitle: "Could not change the release's issues" },
    onSuccess: () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeyReleases.all(workspaceId) }),
        queryClient.invalidateQueries({ queryKey: [workspaceId, "issues"] }),
      ]),
  });
}

export function useSaveReleaseNoteMutation(workspaceId: string) {
  const invalidate = useInvalidateReleases(workspaceId);
  return useMutation({
    mutationFn: (args: OpArgs<"save_release_note">) => call(workspaceId, "save_release_note", args),
    meta: { errorTitle: "Could not save release notes" },
    onSuccess: invalidate,
  });
}

export function useDeleteReleaseNoteMutation(workspaceId: string) {
  const invalidate = useInvalidateReleases(workspaceId);
  return useMutation({
    mutationFn: (id: string) => call(workspaceId, "delete_release_note", { id }),
    meta: { errorTitle: "Could not delete release notes" },
    onSuccess: invalidate,
  });
}

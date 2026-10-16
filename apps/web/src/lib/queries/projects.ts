import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";
import { z } from "zod";

import type { ProjectDetail, ProjectStatus, StatusUpdate } from "@/api/ops.gen";
import { slugIdFromParam } from "@/lib/helpers/entity-slug";
import { call, callAll, type OpArgs } from "@/lib/server/client";

export type TProjectStatusType = ProjectStatus["type"];
export type THealth = StatusUpdate["health"];
export type TDateResolution = ProjectDetail["targetDateResolution"];
export type TProjectListArgs = Omit<OpArgs<"list_projects">, "cursor" | "limit">;

export const queryKeyProjects = {
  all: (workspaceId: string) => [workspaceId, "projects"] as const,
  list: (workspaceId: string, args: TProjectListArgs) =>
    [workspaceId, "projects", "list", args] as const,
  detail: (workspaceId: string, project: string) =>
    [workspaceId, "projects", "detail", project] as const,
  statuses: (workspaceId: string) => [workspaceId, "projects", "statuses"] as const,
  labels: (workspaceId: string) => [workspaceId, "projects", "labels"] as const,
  milestone: (workspaceId: string, id: string) =>
    [workspaceId, "projects", "milestone", id] as const,
};

export const queryKeyStatusUpdates = {
  all: (workspaceId: string) => [workspaceId, "status-updates"] as const,
  list: (workspaceId: string, parent: TStatusUpdateParent) =>
    [workspaceId, "status-updates", "list", parent] as const,
  comments: (workspaceId: string, updateId: string) =>
    [workspaceId, "status-updates", "comments", updateId] as const,
};

/** Projects of the workspace, every page, in the order the server gives (manual by default here). */
export const projectsQuery = (workspaceId: string, args: TProjectListArgs = {}) =>
  queryOptions({
    queryKey: queryKeyProjects.list(workspaceId, args),
    queryFn: () => callAll(workspaceId, "list_projects", { orderBy: "manual", ...args }),
  });

/** A project by its link slug (`launch-v2-0a1b2c3d4e5f`), slug id or id; cached by slug id. */
export const projectQuery = (workspaceId: string, project: string) => {
  const slugId = slugIdFromParam(project);
  return queryOptions({
    queryKey: queryKeyProjects.detail(workspaceId, slugId),
    queryFn: () => call(workspaceId, "get_project", { id: slugId }),
  });
};

export const projectStatusesQuery = (workspaceId: string) =>
  queryOptions({
    queryKey: queryKeyProjects.statuses(workspaceId),
    queryFn: async () =>
      (await callAll(workspaceId, "list_project_statuses", {})).sort(
        (a, b) => a.position - b.position,
      ),
  });

export const projectLabelsQuery = (workspaceId: string) =>
  queryOptions({
    queryKey: queryKeyProjects.labels(workspaceId),
    queryFn: () => callAll(workspaceId, "list_project_labels", { includeRetired: true }),
  });

export const milestoneQuery = (workspaceId: string, id: string) =>
  queryOptions({
    queryKey: queryKeyProjects.milestone(workspaceId, id),
    queryFn: () => call(workspaceId, "get_milestone", { id }),
  });

/** A project's issues; keyed under the workspace's issues so issue changes refresh them. */
export const projectIssuesQuery = (workspaceId: string, projectId: string) =>
  queryOptions({
    queryKey: [workspaceId, "issues", "project", projectId] as const,
    queryFn: () => callAll(workspaceId, "list_issues", { project: projectId, orderBy: "manual" }),
  });

export type TStatusUpdateParent = { project: string } | { initiative: string };

export const statusUpdatesQuery = (workspaceId: string, parent: TStatusUpdateParent) =>
  queryOptions({
    queryKey: queryKeyStatusUpdates.list(workspaceId, parent),
    queryFn: () => callAll(workspaceId, "get_status_updates", parent),
  });

export const statusUpdateCommentsQuery = (workspaceId: string, updateId: string) =>
  queryOptions({
    queryKey: queryKeyStatusUpdates.comments(workspaceId, updateId),
    queryFn: () => callAll(workspaceId, "list_comments", { statusUpdateId: updateId }),
  });

/** Projects show up in initiatives, documents and issues too, so those refresh with them. */
function useInvalidateProjects(workspaceId: string) {
  const queryClient = useQueryClient();
  return () =>
    Promise.all(
      ["projects", "initiatives", "status-updates"].map((area) =>
        queryClient.invalidateQueries({ queryKey: [workspaceId, area] }),
      ),
    );
}

export function useSaveProjectMutation(workspaceId: string) {
  const queryClient = useQueryClient();
  const invalidate = useInvalidateProjects(workspaceId);
  return useMutation({
    mutationFn: (args: OpArgs<"save_project">) => call(workspaceId, "save_project", args),
    meta: { errorTitle: "Could not save project" },
    onSuccess: (project) => {
      queryClient.setQueryData(queryKeyProjects.detail(workspaceId, project.slugId), project);
      return invalidate();
    },
  });
}

export type TProjectLifecycleAction = "archive" | "unarchive" | "trash" | "restore";

const projectLifecycleOps = {
  archive: "archive_project",
  unarchive: "unarchive_project",
  trash: "delete_project",
  restore: "restore_project",
} as const;

export function useProjectLifecycleMutation(workspaceId: string) {
  const invalidate = useInvalidateProjects(workspaceId);
  return useMutation({
    mutationFn: ({ id, action }: { id: string; action: TProjectLifecycleAction }) =>
      call(workspaceId, projectLifecycleOps[action], { id }),
    meta: { errorTitle: "Could not change the project" },
    onSuccess: invalidate,
  });
}

export function useSaveProjectStatusMutation(workspaceId: string) {
  const invalidate = useInvalidateProjects(workspaceId);
  return useMutation({
    mutationFn: (args: OpArgs<"save_project_status">) =>
      call(workspaceId, "save_project_status", args),
    meta: { errorTitle: "Could not save status" },
    onSuccess: invalidate,
  });
}

export function useDeleteProjectStatusMutation(workspaceId: string) {
  const invalidate = useInvalidateProjects(workspaceId);
  return useMutation({
    mutationFn: (args: OpArgs<"delete_project_status">) =>
      call(workspaceId, "delete_project_status", args),
    meta: { errorTitle: "Could not delete status" },
    onSuccess: invalidate,
  });
}

export function useSaveProjectLabelMutation(workspaceId: string) {
  const invalidate = useInvalidateProjects(workspaceId);
  return useMutation({
    mutationFn: (args: OpArgs<"save_project_label">) =>
      call(workspaceId, "save_project_label", args),
    meta: { errorTitle: "Could not save label" },
    onSuccess: invalidate,
  });
}

export type TProjectLabelAction = "retire" | "restore" | "delete";

const projectLabelOps = {
  retire: "retire_project_label",
  restore: "restore_project_label",
  delete: "delete_project_label",
} as const;

export function useProjectLabelActionMutation(workspaceId: string) {
  const invalidate = useInvalidateProjects(workspaceId);
  return useMutation({
    mutationFn: ({ id, action }: { id: string; action: TProjectLabelAction }) =>
      call(workspaceId, projectLabelOps[action], { id }),
    meta: { errorTitle: "Could not change the label" },
    onSuccess: invalidate,
  });
}

/** Milestones also show on issues, so issues refresh with them. */
function useInvalidateMilestones(workspaceId: string) {
  const queryClient = useQueryClient();
  const invalidate = useInvalidateProjects(workspaceId);
  return () =>
    Promise.all([
      invalidate(),
      queryClient.invalidateQueries({ queryKey: [workspaceId, "issues"] }),
    ]);
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

/** Status updates set the health of their project or initiative, so both refresh. */
export function useSaveStatusUpdateMutation(workspaceId: string) {
  const invalidate = useInvalidateProjects(workspaceId);
  return useMutation({
    mutationFn: (args: OpArgs<"save_status_update">) =>
      call(workspaceId, "save_status_update", args),
    meta: { errorTitle: "Could not save update" },
    onSuccess: invalidate,
  });
}

export function useDeleteStatusUpdateMutation(workspaceId: string) {
  const invalidate = useInvalidateProjects(workspaceId);
  return useMutation({
    mutationFn: (id: string) => call(workspaceId, "delete_status_update", { id }),
    meta: { errorTitle: "Could not delete update" },
    onSuccess: invalidate,
  });
}

export function useStatusUpdateReactionMutation(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (input: { updateId: string; emoji: string; reacted: boolean }) => {
      const target = { statusUpdateId: input.updateId, emoji: input.emoji };
      if (input.reacted) await call(workspaceId, "remove_reaction", target);
      else await call(workspaceId, "add_reaction", target);
    },
    meta: { errorTitle: "Could not react" },
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: queryKeyStatusUpdates.all(workspaceId) }),
  });
}

export function useSaveStatusUpdateCommentMutation(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (args: OpArgs<"save_comment">) => call(workspaceId, "save_comment", args),
    meta: { errorTitle: "Could not save comment" },
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: queryKeyStatusUpdates.all(workspaceId) }),
  });
}

export function useDeleteStatusUpdateCommentMutation(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => call(workspaceId, "delete_comment", { id }),
    meta: { errorTitle: "Could not delete comment" },
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: queryKeyStatusUpdates.all(workspaceId) }),
  });
}

export const projectNameMaxLength = 80;

export const ProjectFormSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, "Name is required.")
    .max(projectNameMaxLength, `Name should be at most ${projectNameMaxLength} characters.`),
  summary: z.string().trim().max(255, "Keep the summary under 255 characters."),
});

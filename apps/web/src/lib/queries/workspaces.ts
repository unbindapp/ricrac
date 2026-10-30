import { type QueryClient, queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";
import { z } from "zod";

import type { CreateWorkspace, WorkspaceSummary } from "@/api/account.gen";
import type { Invitation, Workspace } from "@/api/ops.gen";
import { workspaceUrlKeyPattern } from "@/lib/helpers/identifiers";
import { queryKeyMe } from "@/lib/queries/me";
import { account, call, callAll, type OpArgs } from "@/lib/server/client";
import { isApiError } from "@/lib/server/errors";

export type TWorkspaceRole = Workspace["role"];

export const WORKSPACE_ROLES: TWorkspaceRole[] = ["owner", "admin", "member", "guest"];
export const INVITABLE_ROLES: Invitation["role"][] = ["admin", "member", "guest"];

export const workspaceRoleLabels: Record<TWorkspaceRole, string> = {
  owner: "Owner",
  admin: "Admin",
  member: "Member",
  guest: "Guest",
};

export const workspaceRoleDescriptions: Record<TWorkspaceRole, string> = {
  owner: "Everything admins do, and can delete the workspace.",
  admin: "Manages members and settings.",
  member: "Creates and edits issues, projects and documents.",
  guest: "Read-only, and only sees issues once invited to them.",
};

/** The role a summary from /api/me carries, as the operations type it. */
export function workspaceRole(workspace: WorkspaceSummary): TWorkspaceRole {
  return WORKSPACE_ROLES.find((role) => role === workspace.role) ?? "guest";
}

export const isAdminRole = (role: TWorkspaceRole) => role === "owner" || role === "admin";
export const canEditRole = (role: TWorkspaceRole) => role !== "guest";

/** Every workspace query key starts with the workspace id, so one invalidation refreshes it all. */
export const queryKeyWorkspace = {
  all: (workspaceId: string) => [workspaceId] as const,
  detail: (workspaceId: string) => [workspaceId, "workspace"] as const,
  invitations: (workspaceId: string) => [workspaceId, "invitations"] as const,
};

export const workspaceQuery = (workspaceId: string) =>
  queryOptions({
    queryKey: queryKeyWorkspace.detail(workspaceId),
    queryFn: () => call(workspaceId, "get_workspace", {}),
  });

/** A public workspace by URL key, read as a visitor; null when it isn't one. */
export async function fetchPublicWorkspace(
  queryClient: QueryClient,
  urlKey: string,
): Promise<WorkspaceSummary | null> {
  try {
    const workspace = await call(urlKey, "get_workspace", {});
    queryClient.setQueryData(queryKeyWorkspace.detail(workspace.id), workspace);
    const { id, name, logoUrl, theme } = workspace;
    return {
      id,
      name,
      urlKey: workspace.urlKey,
      logoUrl,
      theme,
      ownTheme: null,
      role: "guest",
      assistant: false,
      managesAssistant: false,
    };
  } catch (error) {
    if (isApiError(error) && error.status < 500) return null;
    throw error;
  }
}

export const invitationsQuery = (workspaceId: string) =>
  queryOptions({
    queryKey: queryKeyWorkspace.invitations(workspaceId),
    queryFn: () => callAll(workspaceId, "list_invitations", {}),
  });

export function useCreateWorkspaceMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateWorkspace) => account.createWorkspace(input),
    meta: { errorTitle: "Could not create workspace" },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: queryKeyMe }),
  });
}

export function useUpdateWorkspaceMutation(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (args: OpArgs<"update_workspace">) => call(workspaceId, "update_workspace", args),
    meta: { errorTitle: "Could not update workspace" },
    onSuccess: async (workspace) => {
      queryClient.setQueryData(queryKeyWorkspace.detail(workspaceId), workspace);
      await queryClient.invalidateQueries({ queryKey: queryKeyMe });
    },
  });
}

/** Deleting and leaving go through the dashboard API; me is refetched before moving on. */
export function useDeleteWorkspaceMutation(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (confirmUrlKey: string) => account.deleteWorkspace(workspaceId, { confirmUrlKey }),
    meta: { errorTitle: "Could not delete workspace" },
    onSuccess: async () => {
      await queryClient.refetchQueries({ queryKey: queryKeyMe });
      queryClient.removeQueries({ queryKey: queryKeyWorkspace.all(workspaceId) });
    },
  });
}

export function useLeaveWorkspaceMutation(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => account.leaveWorkspace(workspaceId),
    meta: { errorTitle: "Could not leave workspace" },
    onSuccess: async () => {
      await queryClient.refetchQueries({ queryKey: queryKeyMe });
      queryClient.removeQueries({ queryKey: queryKeyWorkspace.all(workspaceId) });
    },
  });
}

export function useAcceptInvitationMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (invitationId: string) => account.acceptInvitation(invitationId),
    meta: { errorTitle: "Could not accept invitation" },
    onSuccess: () => queryClient.refetchQueries({ queryKey: queryKeyMe }),
  });
}

export function useDeclineInvitationMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (invitationId: string) => account.declineInvitation(invitationId),
    meta: { errorTitle: "Could not decline invitation" },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: queryKeyMe }),
  });
}

export function useInviteUserMutation(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (args: OpArgs<"invite_user">) => call(workspaceId, "invite_user", args),
    meta: { errorTitle: "Could not invite" },
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: queryKeyWorkspace.invitations(workspaceId) }),
  });
}

export function useDeleteInvitationMutation(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => call(workspaceId, "delete_invitation", { id }),
    meta: { errorTitle: "Could not revoke invitation" },
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: queryKeyWorkspace.invitations(workspaceId) }),
  });
}

export const workspaceNameMaxLength = 80;

export const WorkspaceFormSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, "Name is required.")
    .max(workspaceNameMaxLength, `Name should be at most ${workspaceNameMaxLength} characters.`),
  urlKey: z
    .string()
    .trim()
    .regex(workspaceUrlKeyPattern, "Use 2 to 32 lowercase letters, numbers or dashes."),
});

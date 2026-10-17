import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";

import { queryKeyMe } from "@/lib/queries/me";
import { call, callAll, type OpArgs } from "@/lib/server/client";

export const queryKeyUsers = {
  all: (workspaceId: string) => [workspaceId, "users"] as const,
  list: (workspaceId: string) => [workspaceId, "users", "list"] as const,
  me: (workspaceId: string) => [workspaceId, "users", "me"] as const,
};

/** Everyone in the workspace, removed members left out. */
export const usersQuery = (workspaceId: string) =>
  queryOptions({
    queryKey: queryKeyUsers.list(workspaceId),
    queryFn: () => callAll(workspaceId, "list_users", {}),
  });

/** You as a member of the workspace: role, timezone and email notifications. */
export const workspaceMeQuery = (workspaceId: string) =>
  queryOptions({
    queryKey: queryKeyUsers.me(workspaceId),
    queryFn: () => call(workspaceId, "get_user", { id: "me" }),
  });

export function useUpdateUserMutation(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (args: OpArgs<"update_user">) => call(workspaceId, "update_user", args),
    meta: { errorTitle: "Could not update" },
    onSuccess: async (user) => {
      await queryClient.invalidateQueries({ queryKey: queryKeyUsers.all(workspaceId) });
      if (user.isMe) await queryClient.invalidateQueries({ queryKey: queryKeyMe });
    },
  });
}

export function useRemoveUserMutation(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => call(workspaceId, "remove_user", { id }),
    meta: { errorTitle: "Could not remove member" },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: [workspaceId] }),
  });
}

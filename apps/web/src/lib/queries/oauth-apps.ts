import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";

import type { SaveOAuthApp } from "@/api/account.gen";
import { account } from "@/lib/server/client";

export const queryKeyOAuthApps = (workspaceId: string) => [workspaceId, "oauth-apps"] as const;

export const oauthAppsQuery = (workspaceId: string) =>
  queryOptions({
    queryKey: queryKeyOAuthApps(workspaceId),
    queryFn: () => account.oauthApps(workspaceId),
  });

function useInvalidateOAuthApps(workspaceId: string) {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: queryKeyOAuthApps(workspaceId) });
}

export function useCreateOAuthAppMutation(workspaceId: string) {
  const invalidate = useInvalidateOAuthApps(workspaceId);
  return useMutation({
    mutationFn: (input: SaveOAuthApp) => account.createOAuthApp(workspaceId, input),
    meta: { errorTitle: "Could not create app" },
    onSuccess: invalidate,
  });
}

export function useUpdateOAuthAppMutation(workspaceId: string) {
  const invalidate = useInvalidateOAuthApps(workspaceId);
  return useMutation({
    mutationFn: ({ id, ...input }: SaveOAuthApp & { id: string }) =>
      account.updateOAuthApp(workspaceId, id, input),
    meta: { errorTitle: "Could not save app" },
    onSuccess: invalidate,
  });
}

export function useDeleteOAuthAppMutation(workspaceId: string) {
  const invalidate = useInvalidateOAuthApps(workspaceId);
  return useMutation({
    mutationFn: (id: string) => account.deleteOAuthApp(workspaceId, id),
    meta: { errorTitle: "Could not delete app" },
    onSuccess: invalidate,
  });
}

export function useRotateOAuthSecretMutation(workspaceId: string) {
  return useMutation({
    mutationFn: (id: string) => account.rotateOAuthSecret(workspaceId, id),
    meta: { errorTitle: "Could not make a new secret" },
  });
}

import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";

import type { CreateAPIKey } from "@/api/account.gen";
import { account } from "@/lib/server/client";

export const API_KEY_SCOPES = ["read", "write", "admin"] as const;
export type TScope = (typeof API_KEY_SCOPES)[number];

export const scopeLabels: Record<TScope, string> = {
  read: "Read",
  write: "Write",
  admin: "Admin",
};

export const scopeDescriptions: Record<TScope, string> = {
  read: "Read everything you can see",
  write: "Create and change issues, projects, comments and more",
  admin: "Administer the workspaces where you're an admin: members, teams and settings",
};

export const queryKeyApiKeys = ["api-keys"] as const;

export const apiKeysQuery = queryOptions({
  queryKey: queryKeyApiKeys,
  queryFn: () => account.apiKeys(),
});

export function useCreateApiKeyMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateAPIKey) => account.createAPIKey(input),
    meta: { errorTitle: "Could not create API key" },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: queryKeyApiKeys }),
  });
}

export function useDeleteApiKeyMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => account.deleteAPIKey(id),
    meta: { errorTitle: "Could not revoke API key" },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: queryKeyApiKeys }),
  });
}

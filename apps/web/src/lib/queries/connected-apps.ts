import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";

import type { OAuthDecision } from "@/api/account.gen";
import { account } from "@/lib/server/client";

export const queryKeyConnectedApps = ["connected-apps"] as const;

export const connectedAppsQuery = queryOptions({
  queryKey: queryKeyConnectedApps,
  queryFn: () => account.connectedApps(),
});

/** The app asking for access on the consent page, from the authorization request's query. */
export const oauthClientQuery = (query: string) =>
  queryOptions({
    queryKey: ["oauth-client", query],
    queryFn: () => account.oauthClient(query),
    staleTime: Infinity,
    retry: false,
  });

export function useRevokeConnectedAppMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => account.revokeConnectedApp(id),
    meta: { errorTitle: "Could not revoke access" },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: queryKeyConnectedApps }),
  });
}

export function useOAuthDecisionMutation() {
  return useMutation({
    mutationFn: (decision: OAuthDecision) => account.oauthDecide(decision),
    meta: { errorTitle: "Could not finish connecting" },
  });
}

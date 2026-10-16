import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";

import type { Me, UpdateMe, WorkspaceSummary } from "@/api/account.gen";
import { account } from "@/lib/server/client";

export type TMe = Me;
export type TMeWorkspace = WorkspaceSummary;

export const queryKeyMe = ["me"] as const;

export const meQuery = queryOptions({
  queryKey: queryKeyMe,
  queryFn: () => account.me(),
  staleTime: 5 * 60 * 1000,
});

export const appConfigQuery = queryOptions({
  queryKey: ["config"],
  queryFn: () => account.config(),
  staleTime: Infinity,
});

export function meWorkspaces(me: TMe | null | undefined): TMeWorkspace[] {
  return me?.workspaces ?? [];
}

export function useUpdateMeMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: UpdateMe) => account.updateMe(input),
    meta: { errorTitle: "Could not update profile" },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeyMe });
      await queryClient.invalidateQueries({ queryKey: ["users"] });
    },
  });
}

/** Where Google sign-in starts; it comes back to returnTo. */
export function signInHref(returnTo = "/") {
  return `/api/auth/sign-in?return_to=${encodeURIComponent(returnTo)}`;
}

export async function signOut() {
  await account.signOut();
  window.location.href = "/";
}

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

/** A path on this site to go on to, or null for anything that could leave it. */
export function safeRedirect(path: string | undefined): string | null {
  if (!path?.startsWith("/") || path.startsWith("//") || path.startsWith("/\\")) return null;
  return path;
}

/**
 * Where Google sign-in starts. Google comes back to the sign-in page, which goes on to `redirect`
 * or shows why signing in failed.
 */
export function signInHref(redirect?: string) {
  const back = redirect ? `/sign-in?redirect=${encodeURIComponent(redirect)}` : "/sign-in";
  return `/api/auth/sign-in?return_to=${encodeURIComponent(back)}`;
}

export async function signOut() {
  await account.signOut();
  window.location.href = "/";
}

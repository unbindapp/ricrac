import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";

import type { GitHubIntegration } from "@/api/ops.gen";
import { isImporting } from "@/lib/helpers/github-sync";
import { queryKeyMe } from "@/lib/queries/me";
import { call, callAll, type OpArgs } from "@/lib/server/client";

const IMPORT_POLL_MS = 3000;

export const queryKeyGithub = {
  all: (workspaceId: string) => [workspaceId, "github"] as const,
  integration: (workspaceId: string) => [workspaceId, "github", "integration"] as const,
  repositories: (workspaceId: string) => [workspaceId, "github", "repositories"] as const,
  repoSyncs: (workspaceId: string) => [workspaceId, "github", "repo-syncs"] as const,
};

/** The connection, its installations and synced repositories; polls while an import runs. */
export const githubIntegrationQuery = (workspaceId: string) =>
  queryOptions({
    queryKey: queryKeyGithub.integration(workspaceId),
    queryFn: () => call(workspaceId, "get_github_integration", {}),
    refetchInterval: (query) => (hasRunningImport(query.state.data) ? IMPORT_POLL_MS : false),
  });

function hasRunningImport(integration: GitHubIntegration | undefined) {
  return integration?.repoSyncs.some(isImporting) ?? false;
}

/** The repositories the GitHub App was given (admins only); asks GitHub, so it isn't refetched often. */
export const githubRepositoriesQuery = (workspaceId: string) =>
  queryOptions({
    queryKey: queryKeyGithub.repositories(workspaceId),
    queryFn: () => callAll(workspaceId, "list_github_repositories", {}),
    staleTime: 60 * 1000,
  });

/** The repositories synced with the workspace's team; members can read them. */
export const githubRepoSyncsQuery = (workspaceId: string) =>
  queryOptions({
    queryKey: queryKeyGithub.repoSyncs(workspaceId),
    queryFn: () => callAll(workspaceId, "list_github_repo_syncs", {}),
  });

function useInvalidateGithub(workspaceId: string) {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: queryKeyGithub.all(workspaceId) });
}

/** Answers the link that installs the GitHub App and comes back to this workspace. */
export function useConnectGithubMutation(workspaceId: string) {
  return useMutation({
    mutationFn: () => call(workspaceId, "connect_github", {}),
    meta: { errorTitle: "Could not connect GitHub" },
  });
}

export function useDisconnectGithubMutation(workspaceId: string) {
  const invalidate = useInvalidateGithub(workspaceId);
  return useMutation({
    mutationFn: (installation: string) => call(workspaceId, "disconnect_github", { installation }),
    meta: { errorTitle: "Could not disconnect GitHub" },
    onSuccess: invalidate,
  });
}

export function useSaveGithubRepoSyncMutation(workspaceId: string) {
  const invalidate = useInvalidateGithub(workspaceId);
  return useMutation({
    mutationFn: (args: OpArgs<"save_github_repo_sync">) =>
      call(workspaceId, "save_github_repo_sync", args),
    meta: { errorTitle: "Could not save the repository sync" },
    onSuccess: invalidate,
  });
}

export function useDeleteGithubRepoSyncMutation(workspaceId: string) {
  const invalidate = useInvalidateGithub(workspaceId);
  return useMutation({
    mutationFn: (id: string) => call(workspaceId, "delete_github_repo_sync", { id }),
    meta: { errorTitle: "Could not remove the repository" },
    onSuccess: invalidate,
  });
}

export function useDisconnectGithubAccountMutation(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => call(workspaceId, "disconnect_github_account", {}),
    meta: { errorTitle: "Could not unlink your GitHub account" },
    onSuccess: () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeyMe }),
        queryClient.invalidateQueries({ queryKey: queryKeyGithub.all(workspaceId) }),
      ]),
  });
}

/** Where linking your GitHub account starts; GitHub comes back to returnTo. */
export function githubAccountConnectHref(returnTo: string) {
  return `/api/github/connect?return_to=${encodeURIComponent(returnTo)}`;
}

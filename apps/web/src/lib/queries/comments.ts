import { infiniteQueryOptions, useMutation, useQueryClient } from "@tanstack/react-query";

import { queryKeyDiscussions } from "@/lib/queries/discussions";
import { DETAIL_REFETCH_INTERVAL_MS, queryKeyIssues } from "@/lib/queries/issues";
import { call, type OpArgs } from "@/lib/server/client";

const COMMENTS_PAGE_SIZE = 100;

/** What comment threads hang off. */
export type TCommentTarget =
  { issueId: string } | { discussionId: string } | { discussionThreadId: string };

export const queryKeyComments = {
  all: (workspaceId: string) => [workspaceId, "comments"] as const,
  target: (workspaceId: string, target: TCommentTarget) =>
    [workspaceId, "comments", target] as const,
};

/** A target's comment threads, oldest first, each with its replies. */
export const commentsQuery = (workspaceId: string, target: TCommentTarget) =>
  infiniteQueryOptions({
    queryKey: queryKeyComments.target(workspaceId, target),
    queryFn: ({ pageParam }) =>
      call(workspaceId, "list_comments", {
        ...target,
        limit: COMMENTS_PAGE_SIZE,
        cursor: pageParam,
      }),
    initialPageParam: null as string | null,
    getNextPageParam: (last) =>
      last.pageInfo.hasNextPage ? (last.pageInfo.endCursor ?? undefined) : undefined,
    refetchInterval: DETAIL_REFETCH_INTERVAL_MS,
  });

/** Comments change the comment counts and subscribers of issues and discussions too. */
function useInvalidateComments(workspaceId: string) {
  const queryClient = useQueryClient();
  return () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: queryKeyComments.all(workspaceId) }),
      queryClient.invalidateQueries({ queryKey: queryKeyIssues.details(workspaceId) }),
      queryClient.invalidateQueries({ queryKey: queryKeyDiscussions.all(workspaceId) }),
    ]);
}

/** Posts a comment (without id) or edits one of yours (with it). */
export function useSaveCommentMutation(workspaceId: string) {
  const invalidate = useInvalidateComments(workspaceId);
  return useMutation({
    mutationFn: (args: OpArgs<"save_comment">) => call(workspaceId, "save_comment", args),
    meta: { errorTitle: "Could not save comment" },
    onSuccess: invalidate,
  });
}

export function useDeleteCommentMutation(workspaceId: string) {
  const invalidate = useInvalidateComments(workspaceId);
  return useMutation({
    mutationFn: (id: string) => call(workspaceId, "delete_comment", { id }),
    meta: { errorTitle: "Could not delete comment" },
    onSuccess: invalidate,
  });
}

export function useResolveCommentMutation(workspaceId: string) {
  const invalidate = useInvalidateComments(workspaceId);
  return useMutation({
    mutationFn: async ({ id, resolved }: { id: string; resolved: boolean }) =>
      call(workspaceId, resolved ? "resolve_comment" : "unresolve_comment", { id }),
    meta: { errorTitle: "Could not update the thread" },
    onSuccess: invalidate,
  });
}

export function useCommentReactionMutation(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({
      commentId,
      emoji,
      reacted,
    }: {
      commentId: string;
      emoji: string;
      reacted: boolean;
    }) => {
      if (reacted) await call(workspaceId, "remove_reaction", { commentId, emoji });
      else await call(workspaceId, "add_reaction", { commentId, emoji });
    },
    meta: { errorTitle: "Could not update reaction" },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: queryKeyComments.all(workspaceId) }),
  });
}

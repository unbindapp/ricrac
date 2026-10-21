import {
  infiniteQueryOptions,
  queryOptions,
  useMutation,
  useQueryClient,
} from "@tanstack/react-query";

import { slugIdFromParam } from "@/lib/helpers/entity-slug";
import { DETAIL_REFETCH_INTERVAL_MS } from "@/lib/queries/issues";
import { savePatchOrText, type TTextEdit } from "@/lib/queries/text-edits";
import { call, type OpArgs } from "@/lib/server/client";

export type TDiscussionListArgs = Omit<OpArgs<"list_discussions">, "cursor" | "limit">;
export type TDiscussionStatus = NonNullable<TDiscussionListArgs["status"]>;
export type TThreadKind = NonNullable<OpArgs<"save_discussion_thread">["kind"]>;

export const DISCUSSION_TITLE_MAX_LENGTH = 255;
const DISCUSSIONS_PAGE_SIZE = 50;

export const queryKeyDiscussions = {
  all: (workspaceId: string) => [workspaceId, "discussions"] as const,
  list: (workspaceId: string, args: TDiscussionListArgs) =>
    [workspaceId, "discussions", "list", args] as const,
  detail: (workspaceId: string, slugId: string) =>
    [workspaceId, "discussions", "detail", slugId] as const,
  thread: (workspaceId: string, threadId: string) =>
    [workspaceId, "discussions", "thread", threadId] as const,
};

/** Pinned first, then the most recently active. */
export const discussionsQuery = (workspaceId: string, args: TDiscussionListArgs) =>
  infiniteQueryOptions({
    queryKey: queryKeyDiscussions.list(workspaceId, args),
    queryFn: ({ pageParam }) =>
      call(workspaceId, "list_discussions", {
        ...args,
        limit: DISCUSSIONS_PAGE_SIZE,
        cursor: pageParam,
      }),
    initialPageParam: null as string | null,
    getNextPageParam: (page) => (page.pageInfo.hasNextPage ? page.pageInfo.endCursor : null),
  });

/** A discussion by its link slug, slug id or id; cached by slug id. */
export const discussionQuery = (workspaceId: string, discussion: string) => {
  const slugId = slugIdFromParam(discussion);
  return queryOptions({
    queryKey: queryKeyDiscussions.detail(workspaceId, slugId),
    queryFn: () => call(workspaceId, "get_discussion", { id: slugId }),
    refetchInterval: DETAIL_REFETCH_INTERVAL_MS,
  });
};

export const discussionThreadQuery = (workspaceId: string, threadId: string) =>
  queryOptions({
    queryKey: queryKeyDiscussions.thread(workspaceId, threadId),
    queryFn: () => call(workspaceId, "get_discussion_thread", { id: threadId }),
    refetchInterval: DETAIL_REFETCH_INTERVAL_MS,
  });

function useInvalidateDiscussions(workspaceId: string) {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: queryKeyDiscussions.all(workspaceId) });
}

export function useSaveDiscussionMutation(workspaceId: string) {
  const queryClient = useQueryClient();
  const invalidate = useInvalidateDiscussions(workspaceId);
  return useMutation({
    mutationFn: ({ edit, ...args }: OpArgs<"save_discussion"> & { edit?: TTextEdit }) => {
      if (!edit) return call(workspaceId, "save_discussion", args);
      return savePatchOrText(edit, (patch) =>
        call(
          workspaceId,
          "save_discussion",
          patch ? { ...args, patch } : { ...args, body: edit.after },
        ),
      );
    },
    meta: { errorTitle: "Could not save discussion" },
    onSuccess: (discussion) => {
      queryClient.setQueryData(
        queryKeyDiscussions.detail(workspaceId, discussion.slugId),
        discussion,
      );
      return invalidate();
    },
  });
}

export type TDiscussionLifecycleAction = "trash" | "restore";

export function useDiscussionLifecycleMutation(workspaceId: string) {
  const invalidate = useInvalidateDiscussions(workspaceId);
  return useMutation({
    mutationFn: ({ id, action }: { id: string; action: TDiscussionLifecycleAction }) =>
      call(workspaceId, action === "trash" ? "delete_discussion" : "restore_discussion", { id }),
    meta: { errorTitle: "Could not change the discussion" },
    onSuccess: invalidate,
  });
}

export function useSaveDiscussionThreadMutation(workspaceId: string) {
  const queryClient = useQueryClient();
  const invalidate = useInvalidateDiscussions(workspaceId);
  return useMutation({
    mutationFn: ({ edit, ...args }: OpArgs<"save_discussion_thread"> & { edit?: TTextEdit }) => {
      if (!edit) return call(workspaceId, "save_discussion_thread", args);
      return savePatchOrText(edit, (patch) =>
        call(
          workspaceId,
          "save_discussion_thread",
          patch ? { ...args, patch } : { ...args, body: edit.after },
        ),
      );
    },
    meta: { errorTitle: "Could not save thread" },
    onSuccess: (thread) => {
      queryClient.setQueryData(queryKeyDiscussions.thread(workspaceId, thread.id), thread);
      return invalidate();
    },
  });
}

export function useDeleteDiscussionThreadMutation(workspaceId: string) {
  const invalidate = useInvalidateDiscussions(workspaceId);
  return useMutation({
    mutationFn: (id: string) => call(workspaceId, "delete_discussion_thread", { id }),
    meta: { errorTitle: "Could not delete thread" },
    onSuccess: invalidate,
  });
}

import {
  type InfiniteData,
  infiniteQueryOptions,
  type QueryClient,
  queryOptions,
  useMutation,
  useQueryClient,
} from "@tanstack/react-query";

import type { Issue, IssueDetail, ListIssuesArgs, PageInfo, SaveIssueArgs } from "@/api/ops.gen";
import { savePatchOrText, type TTextEdit } from "@/lib/queries/text-edits";
import { call, type OpArgs } from "@/lib/server/client";

export const ISSUE_TITLE_MAX_LENGTH = 255;
export const LIST_REFETCH_INTERVAL_MS = 15 * 1000;
/** Agents change things constantly, so open pages refresh too (and on focus). */
export const DETAIL_REFETCH_INTERVAL_MS = 15 * 1000;

const LIST_PAGE_SIZE = 250;
/** Lists stop loading pages here; grouping and counts are of what is loaded. */
export const LIST_MAX_ISSUES = 2000;
const SEARCH_LIMIT = 8;

export type TIssueChanges = Omit<SaveIssueArgs, "id" | "template">;
type TIssuePage = { nodes: Issue[]; pageInfo: PageInfo };

export const queryKeyIssues = {
  all: (workspaceId: string) => [workspaceId, "issues"] as const,
  lists: (workspaceId: string) => [workspaceId, "issues", "list"] as const,
  list: (workspaceId: string, args: ListIssuesArgs) =>
    [workspaceId, "issues", "list", args] as const,
  details: (workspaceId: string) => [workspaceId, "issues", "detail"] as const,
  detail: (workspaceId: string, identifier: string) =>
    [workspaceId, "issues", "detail", identifier.toUpperCase()] as const,
  history: (workspaceId: string, issueId: string) =>
    [workspaceId, "issues", "history", issueId] as const,
  search: (workspaceId: string, query: string) => [workspaceId, "issues", "search", query] as const,
};

/** Every page of a list, loaded one after the other up to LIST_MAX_ISSUES. */
export const issuesListQuery = (workspaceId: string, args: ListIssuesArgs) =>
  infiniteQueryOptions({
    queryKey: queryKeyIssues.list(workspaceId, args),
    queryFn: ({ pageParam }) =>
      call(workspaceId, "list_issues", { ...args, limit: LIST_PAGE_SIZE, cursor: pageParam }),
    initialPageParam: null as string | null,
    getNextPageParam: (last, pages) => {
      if (!last.pageInfo.hasNextPage) return undefined;
      if (pages.length * LIST_PAGE_SIZE >= LIST_MAX_ISSUES) return undefined;
      return last.pageInfo.endCursor ?? undefined;
    },
    refetchInterval: LIST_REFETCH_INTERVAL_MS,
  });

/** An issue by identifier (ENG-123) or id; open ones follow others' changes. */
export const issueQuery = (workspaceId: string, identifier: string) =>
  queryOptions({
    queryKey: queryKeyIssues.detail(workspaceId, identifier),
    queryFn: () => call(workspaceId, "get_issue", { id: identifier }),
    refetchInterval: DETAIL_REFETCH_INTERVAL_MS,
  });

export const issueHistoryQuery = (workspaceId: string, issueId: string) =>
  queryOptions({
    queryKey: queryKeyIssues.history(workspaceId, issueId),
    queryFn: () => call(workspaceId, "get_issue_history", { id: issueId }),
    refetchInterval: DETAIL_REFETCH_INTERVAL_MS,
  });

/** Issues matching words or an identifier, for pickers. */
export const searchIssuesQuery = (workspaceId: string, query: string) =>
  queryOptions({
    queryKey: queryKeyIssues.search(workspaceId, query.trim()),
    queryFn: async () => {
      const page = await call(workspaceId, "list_issues", {
        query: query.trim(),
        limit: SEARCH_LIMIT,
      });
      return page.nodes;
    },
    enabled: query.trim().length > 0,
  });

/** Issues change projects' and cycles' progress and statuses' counts too. */
export function invalidateIssues(queryClient: QueryClient, workspaceId: string) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: queryKeyIssues.all(workspaceId) }),
    queryClient.invalidateQueries({ queryKey: [workspaceId, "teams", "statuses"] }),
    queryClient.invalidateQueries({ queryKey: [workspaceId, "projects"] }),
    queryClient.invalidateQueries({ queryKey: [workspaceId, "cycles"] }),
    queryClient.invalidateQueries({ queryKey: [workspaceId, "milestones"] }),
  ]);
}

function useInvalidateIssues(workspaceId: string) {
  const queryClient = useQueryClient();
  return () => invalidateIssues(queryClient, workspaceId);
}

type TIssueCachePatch = { id: string; patch: Partial<Issue> };
type TCacheSnapshot = [readonly unknown[], unknown][];

function patchIssueCaches(
  queryClient: QueryClient,
  workspaceId: string,
  patches: TIssueCachePatch[],
) {
  const byId = new Map(patches.map((item) => [item.id, item.patch]));
  const apply = <T extends { id: string }>(issue: T): T => {
    const patch = byId.get(issue.id);
    return patch ? { ...issue, ...patch } : issue;
  };
  queryClient.setQueriesData<InfiniteData<TIssuePage>>(
    { queryKey: queryKeyIssues.lists(workspaceId) },
    (data) =>
      data && {
        ...data,
        pages: data.pages.map((page) => ({ ...page, nodes: page.nodes.map(apply) })),
      },
  );
  queryClient.setQueriesData<IssueDetail>(
    { queryKey: queryKeyIssues.details(workspaceId) },
    (data) => data && { ...apply(data), children: data.children.map(apply) },
  );
}

async function optimisticallyPatch(
  queryClient: QueryClient,
  workspaceId: string,
  patches: TIssueCachePatch[],
): Promise<TCacheSnapshot> {
  if (patches.length === 0) return [];
  await queryClient.cancelQueries({ queryKey: queryKeyIssues.all(workspaceId) });
  const snapshot: TCacheSnapshot = [
    ...queryClient.getQueriesData({ queryKey: queryKeyIssues.lists(workspaceId) }),
    ...queryClient.getQueriesData({ queryKey: queryKeyIssues.details(workspaceId) }),
  ];
  patchIssueCaches(queryClient, workspaceId, patches);
  return snapshot;
}

function restoreIssueCaches(queryClient: QueryClient, snapshot: TCacheSnapshot | undefined) {
  snapshot?.forEach(([queryKey, data]) => queryClient.setQueryData(queryKey, data));
}

export function useCreateIssueMutation(workspaceId: string) {
  const invalidate = useInvalidateIssues(workspaceId);
  return useMutation({
    mutationFn: (args: TIssueChanges) => call(workspaceId, "save_issue", args),
    meta: { errorTitle: "Could not create issue" },
    onSuccess: invalidate,
  });
}

export type TUpdateIssueVariables = {
  id: string;
  changes: TIssueChanges;
  optimistic?: Partial<Issue>;
};

export function useUpdateIssueMutation(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, changes }: TUpdateIssueVariables) =>
      call(workspaceId, "save_issue", { ...changes, id }),
    meta: { errorTitle: "Could not update issue" },
    onMutate: ({ id, optimistic }) =>
      optimisticallyPatch(queryClient, workspaceId, optimistic ? [{ id, patch: optimistic }] : []),
    onError: (_error, _variables, snapshot) => restoreIssueCaches(queryClient, snapshot),
    onSettled: () => invalidateIssues(queryClient, workspaceId),
  });
}

export function useSaveDescriptionMutation(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, edit }: { id: string; edit: TTextEdit }) =>
      savePatchOrText(edit, (patch) =>
        call(workspaceId, "save_issue", patch ? { id, patch } : { id, description: edit.after }),
      ),
    meta: { errorTitle: "Could not save the description" },
    onSettled: () => invalidateIssues(queryClient, workspaceId),
  });
}

export type TUpdateIssuesVariables = {
  ids: string[];
  changes: TIssueChanges;
  optimistic?: Partial<Issue>;
};

export function useUpdateIssuesMutation(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ ids, changes }: TUpdateIssuesVariables) =>
      call(workspaceId, "update_issues", { ids, changes }),
    meta: { errorTitle: "Could not update issues" },
    onMutate: ({ ids, optimistic }) =>
      optimisticallyPatch(
        queryClient,
        workspaceId,
        optimistic ? ids.map((id) => ({ id, patch: optimistic })) : [],
      ),
    onError: (_error, _variables, snapshot) => restoreIssueCaches(queryClient, snapshot),
    onSettled: () => invalidateIssues(queryClient, workspaceId),
  });
}

export type TIssueLifecycleAction = "archive" | "unarchive" | "delete" | "restore";

const lifecycleOps = {
  archive: "archive_issue",
  unarchive: "unarchive_issue",
  delete: "delete_issue",
  restore: "restore_issue",
} as const;

export function useIssueLifecycleMutation(workspaceId: string) {
  const invalidate = useInvalidateIssues(workspaceId);
  return useMutation({
    mutationFn: ({ ids, action }: { ids: string[]; action: TIssueLifecycleAction }) =>
      Promise.all(ids.map((id) => call(workspaceId, lifecycleOps[action], { id }))),
    meta: { errorTitle: "Could not update issue" },
    onSuccess: invalidate,
  });
}

export function useIssueSubscriptionMutation(workspaceId: string) {
  const invalidate = useInvalidateIssues(workspaceId);
  return useMutation({
    mutationFn: ({ id, users, subscribed }: { id: string; users: string[]; subscribed: boolean }) =>
      call(
        workspaceId,
        "save_issue",
        subscribed ? { id, addSubscribers: users } : { id, removeSubscribers: users },
      ),
    meta: { errorTitle: "Could not change subscribers" },
    onSuccess: invalidate,
  });
}

export function useShareIssueMutation(workspaceId: string) {
  const invalidate = useInvalidateIssues(workspaceId);
  return useMutation({
    mutationFn: ({ id, user, shared }: { id: string; user: string; shared: boolean }) =>
      call(workspaceId, shared ? "share_issue" : "unshare_issue", { id, user }),
    meta: { errorTitle: "Could not change sharing" },
    onSuccess: invalidate,
  });
}

export function useIssueReactionMutation(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({
      issueId,
      emoji,
      reacted,
    }: {
      issueId: string;
      emoji: string;
      reacted: boolean;
    }) => {
      if (reacted) await call(workspaceId, "remove_reaction", { issueId, emoji });
      else await call(workspaceId, "add_reaction", { issueId, emoji });
    },
    meta: { errorTitle: "Could not update reaction" },
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: queryKeyIssues.details(workspaceId) }),
  });
}

export function useCreateAttachmentMutation(workspaceId: string) {
  const invalidate = useInvalidateIssues(workspaceId);
  return useMutation({
    mutationFn: (args: OpArgs<"create_attachment">) => call(workspaceId, "create_attachment", args),
    meta: { errorTitle: "Could not add the link" },
    onSuccess: invalidate,
  });
}

export function useAttachUploadMutation(workspaceId: string) {
  const invalidate = useInvalidateIssues(workspaceId);
  return useMutation({
    mutationFn: (args: OpArgs<"create_attachment_from_upload">) =>
      call(workspaceId, "create_attachment_from_upload", args),
    meta: { errorTitle: "Could not attach the file" },
    onSuccess: invalidate,
  });
}

export function useDeleteAttachmentMutation(workspaceId: string) {
  const invalidate = useInvalidateIssues(workspaceId);
  return useMutation({
    mutationFn: (id: string) => call(workspaceId, "delete_attachment", { id }),
    meta: { errorTitle: "Could not remove the attachment" },
    onSuccess: invalidate,
  });
}

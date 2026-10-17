import {
  type InfiniteData,
  infiniteQueryOptions,
  queryOptions,
  useMutation,
  useQueryClient,
} from "@tanstack/react-query";

import type { GetNotificationsArgs, Inbox, Notification } from "@/api/ops.gen";
import { call } from "@/lib/server/client";

export type TInboxFilter = "inbox" | "unread" | "snoozed" | "archived";
export type TNotificationAction =
  "read" | "unread" | "archive" | "unarchive" | "snooze" | "unsnooze";

const PAGE_SIZE = 100;
const POLL_MS = 30 * 1000;
/** mark_notification takes at most this many ids at once. */
const MARK_BATCH = 250;

export const queryKeyNotifications = {
  all: (workspaceId: string) => [workspaceId, "notifications"] as const,
  lists: (workspaceId: string) => [workspaceId, "notifications", "list"] as const,
  list: (workspaceId: string, filter: TInboxFilter) =>
    [workspaceId, "notifications", "list", filter] as const,
  unreadCount: (workspaceId: string) => [workspaceId, "notifications", "unread-count"] as const,
};

/** What each filter asks the server for; the inbox narrows snoozed and archived ones further. */
const filterArgs: Record<TInboxFilter, GetNotificationsArgs> = {
  inbox: {},
  unread: { unreadOnly: true },
  snoozed: { includeSnoozed: true },
  archived: { includeArchived: true },
};

export const notificationsQuery = (workspaceId: string, filter: TInboxFilter) =>
  infiniteQueryOptions({
    queryKey: queryKeyNotifications.list(workspaceId, filter),
    queryFn: ({ pageParam }) =>
      call(workspaceId, "get_notifications", {
        ...filterArgs[filter],
        limit: PAGE_SIZE,
        cursor: pageParam,
      }),
    initialPageParam: null as string | null,
    getNextPageParam: (page) => (page.pageInfo.hasNextPage ? page.pageInfo.endCursor : undefined),
    refetchInterval: POLL_MS,
  });

/** How many unread notifications wait in the inbox, for the sidebar badge. */
export const unreadCountQuery = (workspaceId: string) =>
  queryOptions({
    queryKey: queryKeyNotifications.unreadCount(workspaceId),
    queryFn: async () =>
      (await call(workspaceId, "get_notifications", { unreadOnly: true, limit: 1 })).unreadCount,
    refetchInterval: POLL_MS,
  });

type TMarkInput = { ids: string[]; action: TNotificationAction; until?: string };

function applyAction(notification: Notification, { action, until }: TMarkInput): Notification {
  const now = new Date().toISOString();
  switch (action) {
    case "read":
      return { ...notification, readAt: notification.readAt ?? now };
    case "unread":
      return { ...notification, readAt: null };
    case "archive":
      return { ...notification, archivedAt: notification.archivedAt ?? now };
    case "unarchive":
      return { ...notification, archivedAt: null };
    case "snooze":
      return { ...notification, snoozedUntil: until ?? null };
    case "unsnooze":
      return { ...notification, snoozedUntil: null };
  }
}

/** Whether the server counts it in unreadCount. */
function countsAsUnread(notification: Notification) {
  const isSnoozed =
    notification.snoozedUntil !== null &&
    new Date(notification.snoozedUntil).getTime() > Date.now();
  return notification.readAt === null && notification.archivedAt === null && !isSnoozed;
}

/**
 * Marks notifications at once in every loaded list, so moving through the inbox doesn't wait on
 * the server, and puts them back if the server refuses.
 */
export function useMarkNotificationsMutation(workspaceId: string) {
  const queryClient = useQueryClient();
  const lists = queryKeyNotifications.lists(workspaceId);
  return useMutation({
    mutationFn: async ({ ids, action, until }: TMarkInput) => {
      for (let start = 0; start < ids.length; start += MARK_BATCH) {
        const batch = ids.slice(start, start + MARK_BATCH);
        await call(workspaceId, "mark_notification", { ids: batch, action, until: until ?? null });
      }
    },
    meta: { errorTitle: "Could not update the notification" },
    onMutate: async (input) => {
      const countKey = queryKeyNotifications.unreadCount(workspaceId);
      await queryClient.cancelQueries({ queryKey: queryKeyNotifications.all(workspaceId) });
      const previous = queryClient.getQueriesData<InfiniteData<Inbox>>({ queryKey: lists });
      const previousCount = queryClient.getQueryData<number>(countKey);
      const ids = new Set(input.ids);
      const marked = new Map<string, Notification>();
      for (const [, data] of previous) {
        for (const node of data?.pages.flatMap((page) => page.nodes) ?? []) {
          if (ids.has(node.id)) marked.set(node.id, node);
        }
      }
      const delta = [...marked.values()].reduce(
        (sum, node) =>
          sum + Number(countsAsUnread(applyAction(node, input))) - Number(countsAsUnread(node)),
        0,
      );
      if (previousCount !== undefined)
        queryClient.setQueryData(countKey, Math.max(0, previousCount + delta));
      queryClient.setQueriesData<InfiniteData<Inbox>>({ queryKey: lists }, (data) =>
        data
          ? {
              ...data,
              pages: data.pages.map((page) => ({
                ...page,
                nodes: page.nodes.map((node) =>
                  ids.has(node.id) ? applyAction(node, input) : node,
                ),
              })),
            }
          : data,
      );
      return { previous, previousCount };
    },
    onError: (_error, _input, context) => {
      for (const [key, data] of context?.previous ?? []) queryClient.setQueryData(key, data);
      queryClient.setQueryData(
        queryKeyNotifications.unreadCount(workspaceId),
        context?.previousCount,
      );
    },
    onSettled: () =>
      queryClient.invalidateQueries({ queryKey: queryKeyNotifications.all(workspaceId) }),
  });
}

export function useMarkAllNotificationsReadMutation(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => call(workspaceId, "mark_all_notifications_read", {}),
    meta: { errorTitle: "Could not mark all as read" },
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: queryKeyNotifications.all(workspaceId) }),
  });
}

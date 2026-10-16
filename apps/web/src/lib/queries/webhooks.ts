import {
  infiniteQueryOptions,
  queryOptions,
  useMutation,
  useQueryClient,
} from "@tanstack/react-query";

import type { Webhook, WebhookDelivery } from "@/api/ops.gen";
import { call, callAll, type OpArgs } from "@/lib/server/client";

export type TWebhook = Webhook;
export type TWebhookDelivery = WebhookDelivery;

export const queryKeyWebhooks = {
  all: (workspaceId: string) => [workspaceId, "webhooks"] as const,
  list: (workspaceId: string) => [workspaceId, "webhooks", "list"] as const,
  deliveries: (workspaceId: string, id: string) =>
    [workspaceId, "webhooks", "deliveries", id] as const,
};

export const webhooksQuery = (workspaceId: string) =>
  queryOptions({
    queryKey: queryKeyWebhooks.list(workspaceId),
    queryFn: () => callAll(workspaceId, "list_webhooks", {}),
  });

const DELIVERIES_PAGE_SIZE = 20;

/** A webhook's deliveries, newest first, a page at a time. */
export const webhookDeliveriesQuery = (workspaceId: string, id: string) =>
  infiniteQueryOptions({
    queryKey: queryKeyWebhooks.deliveries(workspaceId, id),
    queryFn: ({ pageParam }) =>
      call(workspaceId, "list_webhook_deliveries", {
        id,
        cursor: pageParam,
        limit: DELIVERIES_PAGE_SIZE,
      }),
    initialPageParam: null as string | null,
    getNextPageParam: (page) => (page.pageInfo.hasNextPage ? page.pageInfo.endCursor : null),
    refetchInterval: (query) =>
      query.state.data?.pages[0]?.nodes.some((delivery) => delivery.nextAttemptAt) ? 5000 : false,
  });

function useInvalidateWebhooks(workspaceId: string) {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: queryKeyWebhooks.all(workspaceId) });
}

export function useSaveWebhookMutation(workspaceId: string) {
  const invalidate = useInvalidateWebhooks(workspaceId);
  return useMutation({
    mutationFn: (args: OpArgs<"save_webhook">) => call(workspaceId, "save_webhook", args),
    meta: { errorTitle: "Could not save webhook" },
    onSuccess: invalidate,
  });
}

export function useDeleteWebhookMutation(workspaceId: string) {
  const invalidate = useInvalidateWebhooks(workspaceId);
  return useMutation({
    mutationFn: (id: string) => call(workspaceId, "delete_webhook", { id }),
    meta: { errorTitle: "Could not delete webhook" },
    onSuccess: invalidate,
  });
}

export function useRotateWebhookSecretMutation(workspaceId: string) {
  return useMutation({
    mutationFn: (id: string) => call(workspaceId, "rotate_webhook_secret", { id }),
    meta: { errorTitle: "Could not rotate the secret" },
  });
}

export function useTestWebhookMutation(workspaceId: string) {
  const invalidate = useInvalidateWebhooks(workspaceId);
  return useMutation({
    mutationFn: (id: string) => call(workspaceId, "test_webhook", { id }),
    meta: { errorTitle: "Could not send a test" },
    onSettled: invalidate,
  });
}

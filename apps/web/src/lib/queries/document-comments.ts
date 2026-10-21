import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";

import { queryKeyComments } from "@/lib/queries/comments";
import { queryKeyDocuments } from "@/lib/queries/documents";
import { DETAIL_REFETCH_INTERVAL_MS } from "@/lib/queries/issues";
import { call, callAll } from "@/lib/server/client";

export const queryKeyDocumentComments = {
  list: (workspaceId: string, documentId: string) =>
    [...queryKeyComments.all(workspaceId), "document", documentId] as const,
};

/** A document's comment threads, oldest first, each with its replies. */
export const documentCommentsQuery = (workspaceId: string, documentId: string) =>
  queryOptions({
    queryKey: queryKeyDocumentComments.list(workspaceId, documentId),
    queryFn: () => callAll(workspaceId, "list_comments", { documentId }),
    refetchInterval: DETAIL_REFETCH_INTERVAL_MS,
  });

export type TSuggestionDecision = "accept" | "reject";

/** Accepting changes the document, so it and its versions load again. */
export function useDecideSuggestionMutation(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, decision }: { id: string; decision: TSuggestionDecision }) =>
      call(workspaceId, decision === "accept" ? "accept_suggestion" : "reject_suggestion", { id }),
    meta: { errorTitle: "Could not decide the suggestion" },
    onSuccess: (_comment, { decision }) =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeyComments.all(workspaceId) }),
        decision === "accept" &&
          queryClient.invalidateQueries({ queryKey: queryKeyDocuments.all(workspaceId) }),
      ]),
  });
}

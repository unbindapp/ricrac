import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";

import { slugIdFromParam } from "@/lib/helpers/entity-slug";
import { call, callAll, type OpArgs } from "@/lib/server/client";

export type TDocumentListArgs = Omit<OpArgs<"list_documents">, "cursor" | "limit">;

export const DOCUMENT_TITLE_MAX_LENGTH = 255;
export const UNTITLED_DOCUMENT = "Untitled";

export const queryKeyDocuments = {
  all: (workspaceId: string) => [workspaceId, "documents"] as const,
  list: (workspaceId: string, args: TDocumentListArgs) =>
    [workspaceId, "documents", "list", args] as const,
  detail: (workspaceId: string, slugId: string) =>
    [workspaceId, "documents", "detail", slugId] as const,
  versions: (workspaceId: string, documentId: string) =>
    [workspaceId, "documents", "versions", documentId] as const,
  version: (workspaceId: string, versionId: number) =>
    [workspaceId, "documents", "version", versionId] as const,
};

export const documentsQuery = (workspaceId: string, args: TDocumentListArgs = {}) =>
  queryOptions({
    queryKey: queryKeyDocuments.list(workspaceId, args),
    queryFn: () => callAll(workspaceId, "list_documents", args),
  });

/** A document by its link slug, slug id or id; cached by slug id. */
export const documentQuery = (workspaceId: string, document: string) => {
  const slugId = slugIdFromParam(document);
  return queryOptions({
    queryKey: queryKeyDocuments.detail(workspaceId, slugId),
    queryFn: () => call(workspaceId, "get_document", { id: slugId }),
  });
};

/** Newest first. */
export const documentVersionsQuery = (workspaceId: string, documentId: string) =>
  queryOptions({
    queryKey: queryKeyDocuments.versions(workspaceId, documentId),
    queryFn: () => call(workspaceId, "list_document_versions", { id: documentId }),
  });

export const documentVersionQuery = (workspaceId: string, versionId: number) =>
  queryOptions({
    queryKey: queryKeyDocuments.version(workspaceId, versionId),
    queryFn: () => call(workspaceId, "get_document_version", { id: String(versionId) }),
    staleTime: Infinity,
  });

/** Documents are listed on projects and initiatives too, so their pages refresh. */
function useInvalidateDocuments(workspaceId: string) {
  const queryClient = useQueryClient();
  return () =>
    Promise.all(
      ["documents", "projects", "initiatives"].map((area) =>
        queryClient.invalidateQueries({ queryKey: [workspaceId, area] }),
      ),
    );
}

export function useSaveDocumentMutation(workspaceId: string) {
  const queryClient = useQueryClient();
  const invalidate = useInvalidateDocuments(workspaceId);
  return useMutation({
    mutationFn: (args: OpArgs<"save_document">) => call(workspaceId, "save_document", args),
    meta: { errorTitle: "Could not save document" },
    onSuccess: (document) => {
      queryClient.setQueryData(queryKeyDocuments.detail(workspaceId, document.slugId), document);
      return invalidate();
    },
  });
}

export type TDocumentLifecycleAction = "trash" | "restore";

export function useDocumentLifecycleMutation(workspaceId: string) {
  const invalidate = useInvalidateDocuments(workspaceId);
  return useMutation({
    mutationFn: ({ id, action }: { id: string; action: TDocumentLifecycleAction }) =>
      call(workspaceId, action === "trash" ? "delete_document" : "restore_document", { id }),
    meta: { errorTitle: "Could not change the document" },
    onSuccess: invalidate,
  });
}

import { useNavigate } from "@tanstack/react-router";

import { useWorkspace } from "@/components/workspace/workspace-provider";
import { entitySlug } from "@/lib/helpers/entity-slug";
import { UNTITLED_DOCUMENT, useSaveDocumentMutation } from "@/lib/queries/documents";

/** Where a document lives: exactly one of these, by id (or key for a team). */
export type TDocumentParent =
  { project: string } | { initiative: string } | { team: string } | { issue: string };

/** Creates an untitled document under a parent and opens it. */
export function useCreateDocument() {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const { mutateAsync, isPending } = useSaveDocumentMutation(workspace.id);
  const create = async (parent: TDocumentParent, title = UNTITLED_DOCUMENT) => {
    const document = await mutateAsync({ title, ...parent });
    await navigate({
      to: "/$workspace/document/$slug",
      params: { workspace: workspace.urlKey, slug: entitySlug(document) },
    });
  };
  return {
    create: (parent: TDocumentParent, title?: string) =>
      void create(parent, title).catch(() => undefined),
    isPending,
  };
}

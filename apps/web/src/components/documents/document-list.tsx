import { useQuery } from "@tanstack/react-query";
import type { ReactNode } from "react";

import { DocumentRow } from "@/components/documents/document-row";
import {
  type TDocumentParent,
  useCreateDocument,
} from "@/components/documents/use-create-document";
import { PlusIcon } from "@/components/icons";
import { ErrorCard } from "@/components/layout/error-card";
import { List } from "@/components/layout/list";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { SectionTitle } from "@/components/layout/section-title";
import { Button } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { documentsQuery, type TDocumentListArgs } from "@/lib/queries/documents";

type TProps = {
  args: TDocumentListArgs;
  showParent?: boolean;
  /** Shown when there are no documents; null shows nothing. */
  empty: ReactNode;
  skeletonRows?: number;
};

export function DocumentList({ args, showParent, empty, skeletonRows }: TProps) {
  const { workspace } = useWorkspace();
  const { data, isPending, error, refetch } = useQuery(documentsQuery(workspace.id, args));
  if (isPending) return <ListSkeleton rows={skeletonRows} />;
  if (error) return <ErrorCard message={error.message} onRetry={() => void refetch()} />;
  if (data.length === 0) return empty;
  return (
    <List>
      {data.map((document) => (
        <DocumentRow key={document.id} document={document} showParent={showParent} />
      ))}
    </List>
  );
}

/** The documents section of a project or initiative page; gone when empty for readers. */
export function ParentDocuments({ parent }: { parent: TDocumentParent }) {
  const { workspace, canEdit } = useWorkspace();
  const { data } = useQuery(documentsQuery(workspace.id, parent));
  const { create, isPending } = useCreateDocument();
  if (!canEdit && (data?.length ?? 0) === 0) return null;
  return (
    <section aria-label="Documents" className="flex w-full flex-col gap-3">
      <SectionTitle
        title="Documents"
        actions={
          canEdit && (
            <Button size="sm" variant="ghost" isPending={isPending} onClick={() => create(parent)}>
              <PlusIcon className="size-4" />
              New document
            </Button>
          )
        }
      />
      <DocumentList args={parent} showParent={false} empty={null} skeletonRows={2} />
    </section>
  );
}

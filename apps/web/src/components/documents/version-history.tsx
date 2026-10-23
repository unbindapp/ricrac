import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import type { DocumentDetail } from "@/api/ops.gen";
import { HistoryIcon } from "@/components/icons";
import { EmptyState } from "@/components/layout/empty-state";
import { ErrorCard } from "@/components/layout/error-card";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { MarkdownView } from "@/components/shared/markdown-view";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { ScrollArea } from "@/components/ui/scroll-area";
import { toast } from "@/components/ui/toast";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { formatDateTime, formatRelative } from "@/lib/helpers/dates";
import {
  documentVersionQuery,
  documentVersionsQuery,
  useSaveDocumentMutation,
} from "@/lib/queries/documents";

type TProps = {
  document: DocumentDetail;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** After a version is restored, so the editor shows it. */
  onRestored: () => void;
};

/** Earlier versions of a document: read one, and bring it back. */
export function VersionHistoryDialog({ document, open, onOpenChange, onRestored }: TProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className="h-160 max-h-full w-5xl max-w-full"
        classNameInnerWrapper="min-h-0 flex-1"
      >
        <DialogHeader>
          <DialogTitle>Version history</DialogTitle>
          <DialogDescription>
            A version is kept before each round of edits. Restoring one keeps the current text as a
            version, so it can be undone.
          </DialogDescription>
        </DialogHeader>
        {open && (
          <VersionBrowser
            document={document}
            onRestored={() => {
              onOpenChange(false);
              onRestored();
            }}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function VersionBrowser({
  document,
  onRestored,
}: {
  document: DocumentDetail;
  onRestored: () => void;
}) {
  const { workspace, canEdit } = useWorkspace();
  const versions = useQuery(documentVersionsQuery(workspace.id, document.id));
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const selected = selectedId ?? versions.data?.[0]?.id ?? null;

  if (versions.isPending) return <ListSkeleton rows={4} />;
  if (versions.error) {
    return <ErrorCard message={versions.error.message} onRetry={() => void versions.refetch()} />;
  }
  if (versions.data.length === 0) {
    return (
      <EmptyState
        icon={HistoryIcon}
        title="No earlier versions yet"
        description="Versions appear once the document is edited again later."
      />
    );
  }
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4 md:flex-row">
      <div className="flex max-h-48 shrink-0 flex-col rounded-lg border md:max-h-none md:w-56">
        <ScrollArea className="min-h-0 flex-1">
          <div
            role="listbox"
            aria-label="Versions"
            className="dank:gap-0.5 dank:p-2 flex flex-col p-1"
          >
            {versions.data.map((version) => (
              <Button
                key={version.id}
                role="option"
                aria-selected={version.id === selected}
                variant={version.id === selected ? "secondary" : "ghost"}
                size="sm"
                forceMinSize={false}
                className="dank:text-m3-on-surface dank:aria-selected:text-m3-on-secondary-container dank:rounded-xs dank:aria-selected:rounded-lg dank:px-4 dank:min-h-18 dank:justify-center h-auto flex-col items-start gap-0.5 py-2 text-left"
                onClick={() => setSelectedId(version.id)}
              >
                <span
                  className="dank:type-body-large text-sm"
                  title={formatDateTime(version.createdAt)}
                >
                  {formatRelative(version.createdAt)}
                </span>
                <span className="text-muted-foreground dank:type-body-medium dank:group-aria-selected/button:text-m3-on-secondary-container text-xs font-normal">
                  {version.author?.name ?? "Someone"}
                </span>
              </Button>
            ))}
          </div>
        </ScrollArea>
      </div>
      {selected !== null && (
        <VersionPreview
          documentId={document.id}
          versionId={selected}
          canRestore={canEdit && !document.trashed}
          onRestored={onRestored}
        />
      )}
    </div>
  );
}

function VersionPreview({
  documentId,
  versionId,
  canRestore,
  onRestored,
}: {
  documentId: string;
  versionId: number;
  canRestore: boolean;
  onRestored: () => void;
}) {
  const { workspace } = useWorkspace();
  const { data, isPending, error, refetch } = useQuery(
    documentVersionQuery(workspace.id, versionId),
  );
  const { mutateAsync: saveDocument, isPending: isRestoring } = useSaveDocumentMutation(
    workspace.id,
  );
  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-3">
      {isPending && <ListSkeleton rows={6} />}
      {error && <ErrorCard message={error.message} onRetry={() => void refetch()} />}
      {data && (
        <>
          <div className="flex items-center gap-2">
            <h3 className="dank:type-title-medium min-w-0 flex-1 truncate text-base font-semibold">
              {data.title}
            </h3>
            {canRestore && (
              <Button
                size="sm"
                isPending={isRestoring}
                onClick={() =>
                  void saveDocument({ id: documentId, restoreVersion: data.id })
                    .then(() => {
                      toast.add({ type: "success", title: "Version restored" });
                      onRestored();
                    })
                    .catch(() => undefined)
                }
              >
                Restore this version
              </Button>
            )}
          </div>
          <ScrollArea className="min-h-0 flex-1 rounded-lg border">
            <div className="p-4">
              {data.content.trim() ? (
                <MarkdownView markdown={data.content} />
              ) : (
                <p className="text-muted-foreground text-sm">This version was empty.</p>
              )}
            </div>
          </ScrollArea>
        </>
      )}
    </div>
  );
}

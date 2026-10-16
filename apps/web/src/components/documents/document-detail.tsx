import { useQuery } from "@tanstack/react-query";
import { FileTextIcon, FolderInputIcon } from "lucide-react";
import { useState } from "react";

import type { DocumentDetail as TDocumentDetail } from "@/api/ops.gen";
import { FavoriteButton } from "@/components/favorites/favorite-button";
import { useDocumentParent } from "@/components/documents/document-parent";
import { DocumentParentPicker } from "@/components/documents/document-parent-picker";
import { LifecycleBanner } from "@/components/documents/lifecycle-banner";
import { MarkdownBody } from "@/components/documents/markdown-body";
import { VersionHistoryDialog } from "@/components/documents/version-history";
import { Breadcrumb, type TBreadcrumbItem } from "@/components/layout/breadcrumb";
import { ErrorCard } from "@/components/layout/error-card";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { EntityNotFound } from "@/components/navigation/not-found-template";
import { EntityMenu } from "@/components/projects/entity-menu";
import { CopyLinkAction, HeaderActions } from "@/components/shared/detail-actions";
import { IconPicker } from "@/components/shared/icon-picker";
import { InlineText } from "@/components/shared/inline-text";
import { RelativeTime } from "@/components/shared/relative-time";
import { Button } from "@/components/ui/button";
import { toast } from "@/components/ui/toast";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import {
  DOCUMENT_TITLE_MAX_LENGTH,
  documentQuery,
  type TDocumentLifecycleAction,
  useDocumentLifecycleMutation,
  useSaveDocumentMutation,
} from "@/lib/queries/documents";
import { isApiError } from "@/lib/server/errors";

export function DocumentDetail({ slug }: { slug: string }) {
  const { workspace } = useWorkspace();
  const { data, isPending, error, refetch } = useQuery(documentQuery(workspace.id, slug));
  if (isPending) return <ListSkeleton rows={4} />;
  if (error) {
    if (isApiError(error, 404)) {
      return (
        <EntityNotFound
          Icon={FileTextIcon}
          description="Document not found. It was deleted, or you cannot see it."
        />
      );
    }
    return <ErrorCard message={error.message} onRetry={() => void refetch()} />;
  }
  return <DocumentDetailContent key={data.id} document={data} />;
}

const lifecycleToasts: Record<TDocumentLifecycleAction, string> = {
  trash: "Document moved to trash",
  restore: "Document restored",
};

function DocumentDetailContent({ document }: { document: TDocumentDetail }) {
  const { workspace, canEdit } = useWorkspace();
  const { mutate: save, mutateAsync: saveAsync } = useSaveDocumentMutation(workspace.id);
  const { mutateAsync: changeLifecycle, isPending: isChanging } = useDocumentLifecycleMutation(
    workspace.id,
  );
  const parent = useDocumentParent(document, workspace.urlKey);
  const [isHistoryOpen, setIsHistoryOpen] = useState(false);
  const [revision, setRevision] = useState(0);
  const editor = document.updatedBy ?? document.creator;
  const isEditable = canEdit && !document.trashed;

  const runLifecycle = (action: TDocumentLifecycleAction) =>
    void changeLifecycle({ id: document.id, action })
      .then(() => toast.add({ type: "success", title: lifecycleToasts[action] }))
      .catch(() => undefined);

  const breadcrumb: TBreadcrumbItem[] = [
    parent
      ? { label: parent.label, icon: parent.icon, link: parent.link }
      : {
          label: "Documents",
          link: { to: "/$workspace/documents", params: { workspace: workspace.urlKey } },
        },
    { label: document.title },
  ];

  return (
    <div className="flex w-full flex-col gap-6">
      <div className="flex w-full min-w-0 items-center gap-2">
        <div className="min-w-0 flex-1">
          <Breadcrumb items={breadcrumb} />
        </div>
        <HeaderActions>
          <FavoriteButton kind="document" entityId={document.id} />
          <CopyLinkAction url={document.url} />
          {canEdit && (
            <EntityMenu
              noun="document"
              isTrashed={document.trashed}
              onTrash={(trash) => runLifecycle(trash ? "trash" : "restore")}
              onShowHistory={() => setIsHistoryOpen(true)}
            />
          )}
        </HeaderActions>
      </div>
      <LifecycleBanner
        entity="document"
        state={document.trashed ? "trashed" : null}
        isPending={isChanging}
        onRestore={canEdit ? () => runLifecycle("restore") : undefined}
      />
      <div className="flex flex-col gap-2">
        <div className="flex min-w-0 items-center gap-2">
          <IconPicker
            value={document.icon}
            color={document.color}
            disabled={!isEditable}
            label="Document icon"
            onChange={(icon) => save({ id: document.id, icon })}
          />
          <InlineText
            value={document.title}
            label="Document title"
            maxLength={DOCUMENT_TITLE_MAX_LENGTH}
            readOnly={!isEditable}
            onSave={(title) => save({ id: document.id, title })}
          />
        </div>
        <div className="text-muted-foreground flex flex-wrap items-center gap-2 text-xs">
          {editor && (
            <span>
              Edited <RelativeTime value={document.updatedAt} /> by {editor.name}
            </span>
          )}
          {document.versionCount > 0 && (
            <Button
              variant="ghost"
              size="sm"
              forceMinSize={false}
              className="text-muted-foreground h-6 px-1.5 text-xs font-normal"
              onClick={() => setIsHistoryOpen(true)}
            >
              {document.versionCount} earlier version{document.versionCount === 1 ? "" : "s"}
            </Button>
          )}
          {isEditable && (
            <DocumentParentPicker
              label="Move document"
              value={currentParent(document)}
              onChange={(next) =>
                void saveAsync({ id: document.id, ...next })
                  .then(() => toast.add({ type: "success", title: "Document moved" }))
                  .catch(() => undefined)
              }
              trigger={
                <Button
                  variant="ghost"
                  size="sm"
                  forceMinSize={false}
                  className="text-muted-foreground h-6 px-1.5 text-xs font-normal"
                >
                  <FolderInputIcon className="size-3.5" />
                  Move
                </Button>
              }
            />
          )}
        </div>
      </div>
      <MarkdownBody
        key={`${document.id}:${revision}`}
        label="Document content"
        placeholder="Start writing…"
        initialValue={document.content ?? ""}
        readOnly={!isEditable}
        onSave={(content) => saveAsync({ id: document.id, content })}
      />
      <VersionHistoryDialog
        document={document}
        open={isHistoryOpen}
        onOpenChange={setIsHistoryOpen}
        onRestored={() => setRevision((value) => value + 1)}
      />
    </div>
  );
}

function currentParent(document: TDocumentDetail) {
  if (document.project) return { project: document.project.id };
  if (document.initiative) return { initiative: document.initiative.id };
  if (document.team && !document.cycle) return { team: document.team.id };
  return null;
}

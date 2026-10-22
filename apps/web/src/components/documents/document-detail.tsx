import { useQuery } from "@tanstack/react-query";
import { useLocation, useNavigate } from "@tanstack/react-router";
import { useEffect, useMemo, useRef, useState } from "react";
import { FileTextIcon, FolderInputIcon, MessageSquareTextIcon } from "lucide-react";

import type { DocumentDetail as TDocumentDetail } from "@/api/ops.gen";
import {
  DocumentAnnotations,
  type TCommentDraft,
} from "@/components/documents/document-annotations";
import { DocumentComments } from "@/components/documents/document-comments";
import { linkedThreadId, threadAnnotations } from "@/components/documents/document-threads";
import { FavoriteButton } from "@/components/favorites/favorite-button";
import { useDocumentParent } from "@/components/documents/document-parent";
import { DocumentParentPicker } from "@/components/documents/document-parent-picker";
import { AutosaveMarkdown } from "@/components/shared/autosave-markdown";
import { VersionHistoryDialog } from "@/components/documents/version-history";
import { PageContainer } from "@/components/layout/page-container";
import { PageBar } from "@/components/layout/page-header";
import { SplitView } from "@/components/layout/split-view";
import { Breadcrumb, type TBreadcrumbItem } from "@/components/layout/breadcrumb";
import { ErrorCard } from "@/components/layout/error-card";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { EntityNotFound } from "@/components/navigation/not-found-template";
import { EntityMenu, toastTrashed } from "@/components/projects/entity-menu";
import { useDeviceSize } from "@/components/providers/device-size-provider";
import { CopyLinkAction, HeaderActions } from "@/components/shared/detail-actions";
import { Annotations } from "@/components/shared/editor/annotations";
import { IconPicker } from "@/components/shared/icon-picker";
import { InlineText } from "@/components/shared/inline-text";
import { RelativeTime } from "@/components/shared/relative-time";
import { Button } from "@/components/ui/button";
import { Drawer, DrawerContent, DrawerTitle } from "@/components/ui/drawer";
import { toast } from "@/components/ui/toast";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import {
  DOCUMENT_TITLE_MAX_LENGTH,
  documentQuery,
  useDocumentLifecycleMutation,
  useSaveDocumentMutation,
} from "@/lib/queries/documents";
import { documentCommentsQuery } from "@/lib/queries/document-comments";
import { isMissing } from "@/lib/queries/projects";

export function DocumentDetail({ slug }: { slug: string }) {
  const { workspace } = useWorkspace();
  const { data, isPending, error, refetch } = useQuery(documentQuery(workspace.id, slug));
  if (isPending) {
    return (
      <PageContainer width="narrow">
        <ListSkeleton rows={4} />
      </PageContainer>
    );
  }
  if (error) {
    return (
      <PageContainer width="narrow">
        {isMissing(error) ? (
          <EntityNotFound
            Icon={FileTextIcon}
            description="Document not found. It was deleted, or you cannot see it."
          />
        ) : (
          <ErrorCard message={error.message} onRetry={() => void refetch()} />
        )}
      </PageContainer>
    );
  }
  return <DocumentDetailContent key={data.id} document={data} />;
}

function DocumentDetailContent({ document }: { document: TDocumentDetail }) {
  const { workspace, canEdit } = useWorkspace();
  const { mutate: save, mutateAsync: saveAsync } = useSaveDocumentMutation(workspace.id);
  const { mutateAsync: changeLifecycle } = useDocumentLifecycleMutation(workspace.id);
  const navigate = useNavigate();
  const parent = useDocumentParent(document, workspace.urlKey);
  const [isHistoryOpen, setIsHistoryOpen] = useState(false);
  const [revision, setRevision] = useState(0);
  const editor = document.updatedBy ?? document.creator;
  const comments = useDocumentComments(document.id);

  const trash = () =>
    void changeLifecycle({ id: document.id, action: "trash" })
      .then(async () => {
        await navigate(
          parent?.link ?? { to: "/$workspace/documents", params: { workspace: workspace.urlKey } },
        );
        toastTrashed(
          "Document moved to trash",
          () =>
            void changeLifecycle({ id: document.id, action: "restore" })
              .then(() => toast.add({ type: "success", title: "Document restored" }))
              .catch(() => undefined),
        );
      })
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

  const main = (
    <div className="flex w-full flex-col gap-6">
      <PageBar>
        <div className="min-w-0 flex-1">
          <Breadcrumb items={breadcrumb} />
        </div>
        <HeaderActions>
          <CommentsToggle count={comments.openCount} onToggle={comments.toggle} />
          <FavoriteButton kind="document" entityId={document.id} />
          <CopyLinkAction url={document.url} />
          {canEdit && (
            <EntityMenu
              noun="document"
              onTrash={trash}
              onShowHistory={() => setIsHistoryOpen(true)}
            />
          )}
        </HeaderActions>
      </PageBar>
      <div className="flex flex-col gap-2">
        <div className="flex min-w-0 items-center gap-2">
          <IconPicker
            value={document.icon}
            color={document.color}
            disabled={!canEdit}
            label="Document icon"
            onChange={(icon) => save({ id: document.id, icon })}
          />
          <InlineText
            value={document.title}
            label="Document title"
            maxLength={DOCUMENT_TITLE_MAX_LENGTH}
            readOnly={!canEdit}
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
          {canEdit && (
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
      <AutosaveMarkdown
        key={`${document.id}:${revision}`}
        label="Document content"
        placeholder="Start writing…"
        value={document.content ?? ""}
        readOnly={!canEdit}
        onSave={async (after, before) => {
          const saved = await saveAsync({ id: document.id, edit: { before, after } });
          return saved.content ?? "";
        }}
        addons={{
          extensions: comments.extensions,
          render: (current) => (
            <DocumentAnnotations
              editor={current}
              annotations={comments.annotations}
              activeId={comments.highlightedId}
              canSuggest={canEdit}
              onAnnotate={comments.startDraft}
            />
          ),
        }}
      />
      <VersionHistoryDialog
        document={document}
        open={isHistoryOpen}
        onOpenChange={setIsHistoryOpen}
        onRestored={() => setRevision((value) => value + 1)}
      />
    </div>
  );

  const panel = (onClose: () => void) => (
    <DocumentComments
      documentId={document.id}
      content={document.content ?? ""}
      draft={comments.draft}
      onDraftDone={() => comments.setDraft(null)}
      activeId={comments.activeId}
      onActivate={comments.setActiveId}
      onHover={comments.setHoverId}
      onAccepted={() => setRevision((value) => value + 1)}
      onClose={onClose}
    />
  );

  return (
    <>
      <SplitView
        main={<PageContainer width="narrow">{main}</PageContainer>}
        aside={comments.isAsideShown ? panel(() => comments.setIsAsideOpen(false)) : undefined}
      />
      {!comments.hasAside && (
        <Drawer
          direction="right"
          open={comments.isDrawerOpen}
          onOpenChange={comments.setIsDrawerOpen}
        >
          <DrawerContent className="w-96 max-w-full">
            <DrawerTitle className="sr-only">Comments</DrawerTitle>
            {panel(() => comments.setIsDrawerOpen(false))}
          </DrawerContent>
        </Drawer>
      )}
    </>
  );
}

/** The comments panel's state: a column on wide screens, a drawer otherwise. */
function useDocumentComments(documentId: string) {
  const { workspace } = useWorkspace();
  const { size } = useDeviceSize();
  const hasAside = size === "xl";
  const hash = useLocation({ select: (location) => location.hash });
  const [isAsideOpen, setIsAsideOpen] = useState(true);
  const [isDrawerOpen, setIsDrawerOpen] = useState(() => hash.startsWith("comment-"));
  const [draft, setDraft] = useState<TCommentDraft | null>(null);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [hoverId, setHoverId] = useState<string | null>(null);
  const { data: threads } = useQuery(documentCommentsQuery(workspace.id, documentId));
  const annotations = useMemo(() => threadAnnotations(threads ?? []), [threads]);
  const linkedId = useMemo(() => linkedThreadId(threads ?? [], hash), [threads, hash]);

  const show = () => (hasAside ? setIsAsideOpen(true) : setIsDrawerOpen(true));
  const activate = (id: string) => {
    setActiveId(id);
    show();
  };
  const activateRef = useRef(activate);
  useEffect(() => {
    activateRef.current = activate;
  });
  const [extensions] = useState(() => [Annotations.configure({ onActivate: activateRef })]);

  return {
    hasAside,
    isAsideShown: hasAside && isAsideOpen,
    setIsAsideOpen,
    isDrawerOpen,
    setIsDrawerOpen,
    toggle: () => (hasAside ? setIsAsideOpen((open) => !open) : setIsDrawerOpen((open) => !open)),
    openCount: threads?.filter((thread) => !thread.resolvedAt).length ?? 0,
    draft,
    setDraft,
    startDraft: (next: TCommentDraft) => {
      setDraft(next);
      show();
    },
    activeId: activeId ?? linkedId,
    setActiveId,
    setHoverId,
    highlightedId: hoverId ?? activeId ?? linkedId,
    annotations,
    extensions,
  };
}

function CommentsToggle({ count, onToggle }: { count: number; onToggle: () => void }) {
  return (
    <SimpleTooltip content="Comments">
      <Button
        variant="ghost"
        size="sm"
        aria-label={count > 0 ? `Comments, ${count} open` : "Comments"}
        onClick={onToggle}
      >
        <MessageSquareTextIcon className="size-4" />
        {count > 0 && <span className="text-xs">{count}</span>}
      </Button>
    </SimpleTooltip>
  );
}

function currentParent(document: TDocumentDetail) {
  if (document.project) return { project: document.project.id };
  if (document.initiative) return { initiative: document.initiative.id };
  if (document.team && !document.cycle) return { team: document.team.id };
  return null;
}

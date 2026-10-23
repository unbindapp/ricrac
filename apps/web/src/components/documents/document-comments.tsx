import { useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";

import type { TCommentDraft } from "@/components/documents/document-annotations";
import {
  DocumentCommentComposer,
  SuggestionComposer,
} from "@/components/documents/document-comment-composer";
import { DocumentThreadCard } from "@/components/documents/document-thread-card";
import { groupThreads } from "@/components/documents/document-threads";
import { ChevronRightIcon, MessageSquareTextIcon, XIcon } from "@/components/icons";
import { ListStates } from "@/components/layout/list-states";
import { Button } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { documentCommentsQuery } from "@/lib/queries/document-comments";

type TProps = {
  documentId: string;
  /** The document's Markdown, which orders the threads by their quotes. */
  content: string;
  draft: TCommentDraft | null;
  onDraftDone: () => void;
  activeId: string | null;
  onActivate: (id: string) => void;
  onHover: (id: string | null) => void;
  onAccepted: () => void;
  onClose?: () => void;
};

export function DocumentComments({
  documentId,
  content,
  draft,
  onDraftDone,
  activeId,
  onActivate,
  onHover,
  onAccepted,
  onClose,
}: TProps) {
  const { workspace } = useWorkspace();
  const { data, isPending, error, refetch } = useQuery(
    documentCommentsQuery(workspace.id, documentId),
  );
  const { open, resolved } = useMemo(() => groupThreads(data ?? [], content), [data, content]);
  const [isResolvedOpen, setIsResolvedOpen] = useState(false);
  const showsResolved = isResolvedOpen || resolved.some((thread) => thread.id === activeId);

  const card = (thread: (typeof open)[number]) => (
    <DocumentThreadCard
      key={thread.id}
      thread={thread}
      documentId={documentId}
      isActive={thread.id === activeId}
      onActivate={onActivate}
      onHover={onHover}
      onAccepted={onAccepted}
    />
  );

  return (
    <section aria-label="Comments" className="bg-background flex min-h-0 w-full flex-1 flex-col">
      <div className="flex h-11 shrink-0 items-center gap-2 border-b px-4">
        <h2 className="dank:type-title-medium min-w-0 flex-1 text-base font-semibold">Comments</h2>
        {onClose && (
          <Button variant="ghost" size="icon" aria-label="Close comments" onClick={onClose}>
            <XIcon className="size-4" />
          </Button>
        )}
      </div>
      <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto p-4">
        {draft ? (
          <DraftCard
            key={`${draft.suggest}:${draft.quotedText}`}
            documentId={documentId}
            draft={draft}
            onDone={onDraftDone}
          />
        ) : (
          <DocumentCommentComposer
            documentId={documentId}
            label="Comment"
            placeholder="Comment on the document…"
          />
        )}
        <ListStates
          isPending={isPending}
          error={error}
          onRetry={() => void refetch()}
          isEmpty={open.length === 0 && resolved.length === 0}
          skeletonRows={3}
          empty={{
            icon: MessageSquareTextIcon,
            title: "No comments yet",
            description: "Select text in the document to comment on it.",
          }}
        >
          <div className="flex flex-col gap-3">
            {open.map(card)}
            {resolved.length > 0 && (
              <Button
                variant="ghost"
                size="sm"
                className="text-muted-foreground self-start font-normal"
                aria-expanded={showsResolved}
                onClick={() => setIsResolvedOpen(!showsResolved)}
              >
                <ChevronRightIcon
                  data-open={showsResolved || undefined}
                  className="size-3.5 transition-transform data-open:rotate-90"
                />
                Resolved ({resolved.length})
              </Button>
            )}
            {showsResolved && resolved.map(card)}
          </div>
        </ListStates>
      </div>
    </section>
  );
}

function DraftCard({
  documentId,
  draft,
  onDone,
}: {
  documentId: string;
  draft: TCommentDraft;
  onDone: () => void;
}) {
  return (
    <div className="bg-card dank:rounded-md dank:p-4 flex flex-col gap-2 rounded-lg border p-3">
      <blockquote className="text-muted-foreground line-clamp-3 border-l-2 pl-3 text-sm break-words">
        {draft.quotedText}
      </blockquote>
      {draft.suggest ? (
        <SuggestionComposer documentId={documentId} quotedText={draft.quotedText} onDone={onDone} />
      ) : (
        <DocumentCommentComposer
          documentId={documentId}
          quotedText={draft.quotedText}
          label="Comment"
          placeholder="Add a comment…"
          autoFocus
          onDone={onDone}
          onCancel={onDone}
        />
      )}
    </div>
  );
}

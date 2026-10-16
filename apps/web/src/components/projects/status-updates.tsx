import { useQuery } from "@tanstack/react-query";
import { MessageSquareIcon, PencilIcon, PenLineIcon, Trash2Icon } from "lucide-react";
import { useEffect, useState } from "react";
import { useLocation } from "@tanstack/react-router";

import type { Comment, StatusUpdate } from "@/api/ops.gen";
import { ErrorCard } from "@/components/layout/error-card";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { SectionTitle } from "@/components/layout/section-title";
import { HealthBadge } from "@/components/projects/health-badge";
import { PersonAvatar, useMentionUsers } from "@/components/projects/people";
import { StatusUpdateComposer } from "@/components/projects/status-update-composer";
import { UpdateReactions } from "@/components/projects/update-reactions";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { MarkdownEditor } from "@/components/shared/markdown-editor";
import { MarkdownView } from "@/components/shared/markdown-view";
import { RelativeTime } from "@/components/shared/relative-time";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { meQuery } from "@/lib/queries/me";
import {
  statusUpdateCommentsQuery,
  statusUpdatesQuery,
  type TStatusUpdateParent,
  type THealth,
  useDeleteStatusUpdateCommentMutation,
  useDeleteStatusUpdateMutation,
  useSaveStatusUpdateCommentMutation,
  useSaveStatusUpdateMutation,
  useStatusUpdateReactionMutation,
} from "@/lib/queries/projects";

const SHOWN_UPDATES = 3;

function useMeId() {
  return useQuery(meQuery).data?.id;
}

type TProps = {
  parent: TStatusUpdateParent;
  /** project or initiative */
  noun: string;
  health: THealth | null;
};

/** Status updates of a project or initiative: the latest sets its health. */
export function StatusUpdatesSection({ parent, noun, health }: TProps) {
  const { workspace, canEdit } = useWorkspace();
  const { data, isPending, error, refetch } = useQuery(statusUpdatesQuery(workspace.id, parent));
  const { mutateAsync: saveUpdate } = useSaveStatusUpdateMutation(workspace.id);
  const [isWriting, setIsWriting] = useState(false);
  const [showAll, setShowAll] = useState(false);
  const updates = [...(data ?? [])].sort((a, b) => b.createdAt.localeCompare(a.createdAt));
  const linkedId = useLinkedUpdateId();
  const isLinkedHidden = updates.findIndex((update) => update.id === linkedId) >= SHOWN_UPDATES;
  const shown = showAll || isLinkedHidden ? updates : updates.slice(0, SHOWN_UPDATES);
  useEffect(() => {
    if (!linkedId || !data) return;
    document.getElementById(`update-${linkedId}`)?.scrollIntoView({ block: "center" });
  }, [linkedId, data]);
  const hidden = updates.length - shown.length;

  if (!canEdit && !isPending && !error && updates.length === 0) return null;

  return (
    <section aria-label="Updates" className="flex w-full flex-col gap-3">
      <SectionTitle
        title="Updates"
        actions={
          canEdit &&
          !isWriting && (
            <Button variant="ghost" size="sm" onClick={() => setIsWriting(true)}>
              <PenLineIcon className="size-4" />
              Write update
            </Button>
          )
        }
      />
      {isWriting && (
        <StatusUpdateComposer
          initialHealth={health ?? "onTrack"}
          onCancel={() => setIsWriting(false)}
          onSubmit={async (value) => {
            await saveUpdate({ ...parent, ...value });
            setIsWriting(false);
          }}
        />
      )}
      {isPending && <ListSkeleton rows={2} />}
      {error && <ErrorCard message={error.message} onRetry={() => void refetch()} />}
      {!isPending && !error && updates.length === 0 && !isWriting && (
        <p className="text-muted-foreground text-sm">
          Post how the {noun} is going; the latest update sets its health.
        </p>
      )}
      {shown.map((update) => (
        <StatusUpdateCard key={update.id} update={update} isLinked={update.id === linkedId} />
      ))}
      {hidden > 0 && (
        <Button variant="ghost" size="sm" className="self-start" onClick={() => setShowAll(true)}>
          Show {hidden} older update{hidden === 1 ? "" : "s"}
        </Button>
      )}
    </section>
  );
}

/** The update a link points at, as #update-<id> (the inbox links them so). */
function useLinkedUpdateId() {
  const hash = useLocation({ select: (location) => location.hash });
  return hash.startsWith("update-") ? hash.slice("update-".length) : null;
}

function StatusUpdateCard({ update, isLinked }: { update: StatusUpdate; isLinked: boolean }) {
  const { workspace, canEdit, isAdmin } = useWorkspace();
  const meId = useMeId();
  const [isEditing, setIsEditing] = useState(false);
  const [showComments, setShowComments] = useState(false);
  const { mutateAsync: saveUpdate } = useSaveStatusUpdateMutation(workspace.id);
  const { mutateAsync: deleteUpdate } = useDeleteStatusUpdateMutation(workspace.id);
  const { mutate: react } = useStatusUpdateReactionMutation(workspace.id);
  const isAuthor = Boolean(meId && update.user?.id === meId);

  if (isEditing) {
    return (
      <StatusUpdateComposer
        initialBody={update.body}
        initialHealth={update.health}
        submitLabel="Save"
        onCancel={() => setIsEditing(false)}
        onSubmit={async (value) => {
          await saveUpdate({ id: update.id, ...value });
          setIsEditing(false);
        }}
      />
    );
  }

  return (
    <Card
      id={`update-${update.id}`}
      data-linked={isLinked || undefined}
      className="data-linked:ring-primary/5-10 flex scroll-mt-4 flex-col gap-3 p-4 data-linked:ring-2"
    >
      <div className="flex min-w-0 items-center gap-2 text-sm">
        {update.user && <PersonAvatar user={update.user} />}
        <span className="truncate font-medium">{update.user?.name ?? "Someone"}</span>
        <HealthBadge health={update.health} />
        <span className="text-muted-foreground shrink-0 text-xs">
          <RelativeTime value={update.createdAt} />
          {update.editedAt && " · edited"}
        </span>
        <span className="flex-1" />
        {canEdit && isAuthor && (
          <SimpleTooltip content="Edit update">
            <Button
              variant="ghost"
              size="icon"
              aria-label="Edit update"
              onClick={() => setIsEditing(true)}
            >
              <PencilIcon className="size-4" />
            </Button>
          </SimpleTooltip>
        )}
        {canEdit && (isAuthor || isAdmin) && (
          <ConfirmDialog
            trigger={
              <Button variant="ghost" size="icon" aria-label="Delete update">
                <Trash2Icon className="size-4" />
              </Button>
            }
            title="Delete update"
            description="Its comments go with it. The health falls back to the update before it."
            confirmLabel="Delete update"
            onConfirm={() => deleteUpdate(update.id)}
          />
        )}
      </div>
      <MarkdownView markdown={update.body} />
      <div className="flex flex-wrap items-center gap-2">
        <UpdateReactions
          reactions={update.reactions}
          meId={meId}
          disabled={!canEdit}
          onToggle={(emoji, reacted) => react({ updateId: update.id, emoji, reacted })}
        />
        <span className="flex-1" />
        {(update.commentCount > 0 || canEdit) && (
          <Button
            variant="ghost"
            size="sm"
            aria-expanded={showComments}
            onClick={() => setShowComments((value) => !value)}
          >
            <MessageSquareIcon className="size-4" />
            {update.commentCount > 0
              ? `${update.commentCount} comment${update.commentCount === 1 ? "" : "s"}`
              : "Comment"}
          </Button>
        )}
      </div>
      {showComments && <UpdateComments updateId={update.id} />}
    </Card>
  );
}

function UpdateComments({ updateId }: { updateId: string }) {
  const { workspace, canEdit } = useWorkspace();
  const { data, isPending, error, refetch } = useQuery(
    statusUpdateCommentsQuery(workspace.id, updateId),
  );
  const threads = data ?? [];
  return (
    <div className="flex flex-col gap-3 border-t pt-3">
      {isPending && <ListSkeleton rows={2} />}
      {error && <ErrorCard message={error.message} onRetry={() => void refetch()} />}
      {threads.map((thread) => (
        <div key={thread.id} className="flex flex-col gap-3">
          <UpdateComment comment={thread} />
          {thread.replies.map((reply) => (
            <div key={reply.id} className="pl-8">
              <UpdateComment comment={reply} />
            </div>
          ))}
        </div>
      ))}
      {canEdit && <CommentComposer updateId={updateId} />}
    </div>
  );
}

function UpdateComment({ comment }: { comment: Comment }) {
  const { workspace, canEdit, isAdmin } = useWorkspace();
  const meId = useMeId();
  const { mutateAsync: deleteComment } = useDeleteStatusUpdateCommentMutation(workspace.id);
  const canDelete = canEdit && (isAdmin || comment.user?.id === meId);
  return (
    <div className="group/comment flex min-w-0 gap-2">
      {comment.user && <PersonAvatar user={comment.user} />}
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <div className="flex items-center gap-2 text-sm">
          <span className="truncate font-medium">{comment.user?.name ?? "Someone"}</span>
          <span className="text-muted-foreground text-xs">
            <RelativeTime value={comment.createdAt} />
          </span>
          <span className="flex-1" />
          {canDelete && (
            <ConfirmDialog
              trigger={
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label="Delete comment"
                  className="opacity-0 group-hover/comment:opacity-100 focus-visible:opacity-100"
                >
                  <Trash2Icon className="size-4" />
                </Button>
              }
              title="Delete comment"
              confirmLabel="Delete comment"
              onConfirm={() => deleteComment(comment.id)}
            />
          )}
        </div>
        <MarkdownView markdown={comment.body} />
      </div>
    </div>
  );
}

function CommentComposer({ updateId }: { updateId: string }) {
  const { workspace } = useWorkspace();
  const users = useMentionUsers();
  const [body, setBody] = useState("");
  const [editorKey, setEditorKey] = useState(0);
  const { mutateAsync: saveComment, isPending } = useSaveStatusUpdateCommentMutation(workspace.id);
  const submit = async () => {
    if (!body.trim() || isPending) return;
    await saveComment({ statusUpdateId: updateId, body: body.trim() }).catch(() => undefined);
    setBody("");
    setEditorKey((key) => key + 1);
  };
  return (
    <div className="flex flex-col gap-2">
      <MarkdownEditor
        key={editorKey}
        framed
        value={body}
        onChange={setBody}
        users={users}
        label="Comment"
        placeholder="Leave a comment…"
        onSubmit={() => void submit()}
      />
      <Button
        size="sm"
        className="self-end"
        disabled={!body.trim()}
        isPending={isPending}
        onClick={() => void submit()}
      >
        Comment
      </Button>
    </div>
  );
}

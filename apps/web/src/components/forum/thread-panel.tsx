import { useQuery } from "@tanstack/react-query";
import {
  CircleCheckIcon,
  CircleDotIcon,
  SplitIcon,
  Trash2Icon,
  UndoIcon,
  XIcon,
} from "lucide-react";
import { useState } from "react";

import type { DiscussionThreadDetail } from "@/api/ops.gen";
import { ChatComposer } from "@/components/forum/chat-message";
import { ChatMessages } from "@/components/forum/chat-messages";
import { IssueLinkPicker } from "@/components/forum/issue-link-picker";
import { THREAD_KINDS, threadKindOf, threadKinds } from "@/components/forum/thread-kinds";
import { useCanRewrite } from "@/components/forum/use-can-rewrite";
import { PickerTrigger } from "@/components/issues/pickers";
import { PropertyPicker } from "@/components/issues/property-picker";
import { ChatLayout } from "@/components/layout/chat-layout";
import { EmptyState } from "@/components/layout/empty-state";
import { ErrorCard } from "@/components/layout/error-card";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { PanelBar } from "@/components/layout/panel-bar";
import { AutosaveMarkdown } from "@/components/shared/autosave-markdown";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { CopyLinkAction, HeaderAction, HeaderActions } from "@/components/shared/detail-actions";
import { InlineText } from "@/components/shared/inline-text";
import { RelativeTime } from "@/components/shared/relative-time";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { toast } from "@/components/ui/toast";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { messagePlaceholder } from "@/lib/helpers/chat";
import { useHotkeyScope } from "@/lib/hotkeys/use-hotkey-scope";
import {
  DISCUSSION_TITLE_MAX_LENGTH,
  discussionThreadQuery,
  type TThreadKind,
  useDeleteDiscussionThreadMutation,
  useSaveDiscussionThreadMutation,
} from "@/lib/queries/discussions";
import { isMissing } from "@/lib/queries/projects";

type TProps = { threadId: string; onClose: () => void };

export function ThreadPanel({ threadId, onClose }: TProps) {
  const { workspace } = useWorkspace();
  const { data, isPending, error, refetch } = useQuery(
    discussionThreadQuery(workspace.id, threadId),
  );
  useHotkeyScope([{ hotkey: "Escape", callback: onClose }]);
  return (
    <section
      aria-label={data ? `Thread ${data.title}` : "Thread"}
      className="bg-background flex min-h-0 w-full flex-1 flex-col"
    >
      <ThreadPanelBody
        thread={data}
        isPending={isPending}
        error={error}
        onRetry={() => void refetch()}
        onClose={onClose}
      />
    </section>
  );
}

function ThreadPanelBody({
  thread,
  isPending,
  error,
  onRetry,
  onClose,
}: {
  thread: DiscussionThreadDetail | undefined;
  isPending: boolean;
  error: Error | null;
  onRetry: () => void;
  onClose: () => void;
}) {
  if (isPending) {
    return (
      <div className="p-4">
        <ListSkeleton rows={4} />
      </div>
    );
  }
  if (error && isMissing(error)) {
    return (
      <EmptyState
        icon={SplitIcon}
        title="Thread not found"
        description="It was deleted, or you cannot see it."
        action={
          <Button variant="outline" size="sm" onClick={onClose}>
            Close
          </Button>
        }
      />
    );
  }
  if (error || !thread) {
    return (
      <div className="p-4">
        <ErrorCard message={error?.message ?? "Could not load the thread"} onRetry={onRetry} />
      </div>
    );
  }
  return <ThreadContent key={thread.id} thread={thread} onClose={onClose} />;
}

function ThreadContent({
  thread,
  onClose,
}: {
  thread: DiscussionThreadDetail;
  onClose: () => void;
}) {
  const { workspace, canEdit } = useWorkspace();
  const canRewrite = useCanRewrite(thread.creator);
  const [isDeleting, setIsDeleting] = useState(false);
  const { mutate: save, mutateAsync: saveAsync } = useSaveDiscussionThreadMutation(workspace.id);
  const { mutateAsync: deleteThread } = useDeleteDiscussionThreadMutation(workspace.id);
  const isResolved = Boolean(thread.resolvedAt);

  const remove = async () => {
    onClose();
    await deleteThread(thread.id);
    toast.add({ type: "success", title: "Thread deleted" });
  };

  const header = (
    <PanelBar>
      <div className="flex min-w-0 flex-1 items-center gap-2">
        <ThreadKindPicker
          kind={thread.kind}
          disabled={!canEdit}
          onChange={(kind) => save({ id: thread.id, kind })}
        />
        <ThreadStatusBadge thread={thread} />
      </div>
      <HeaderActions>
        {canEdit && (
          <Button
            variant="outline"
            size="sm"
            onClick={() => save({ id: thread.id, resolved: !isResolved })}
          >
            {isResolved ? <UndoIcon className="size-4" /> : <CircleCheckIcon className="size-4" />}
            {isResolved ? "Reopen" : "Resolve"}
          </Button>
        )}
        <CopyLinkAction url={thread.url} />
        {canRewrite && (
          <HeaderAction label="Delete thread" onClick={() => setIsDeleting(true)}>
            <Trash2Icon className="size-4" />
          </HeaderAction>
        )}
        <HeaderAction label="Close thread" onClick={onClose}>
          <XIcon className="size-4" />
        </HeaderAction>
      </HeaderActions>
    </PanelBar>
  );

  return (
    <>
      <ChatLayout
        header={header}
        composer={
          canEdit && (
            <ChatComposer
              target={{ discussionThreadId: thread.id }}
              placeholder={messagePlaceholder(thread.title)}
            />
          )
        }
      >
        <div className="flex flex-col gap-2">
          <InlineText
            value={thread.title}
            label="Thread title"
            maxLength={DISCUSSION_TITLE_MAX_LENGTH}
            readOnly={!canRewrite}
            onSave={(title) => save({ id: thread.id, title })}
          />
          <div className="text-muted-foreground flex flex-wrap items-center gap-2 text-xs">
            {thread.creator && (
              <span>
                Started by {thread.creator.name} <RelativeTime value={thread.createdAt} />
              </span>
            )}
            {thread.resolvedAt && (
              <span>
                Resolved
                {thread.resolvingUser && ` by ${thread.resolvingUser.name}`}{" "}
                <RelativeTime value={thread.resolvedAt} />
              </span>
            )}
          </div>
          {(canEdit || thread.issue) && (
            <div className="flex">
              <IssueLinkPicker
                issue={thread.issue}
                disabled={!canEdit}
                onChange={(issue) => save({ id: thread.id, issue: issue?.id ?? null })}
              />
            </div>
          )}
        </div>
        <AutosaveMarkdown
          label="Thread body"
          placeholder="Add the details…"
          value={thread.body}
          readOnly={!canRewrite}
          onSave={async (after, before) => {
            const saved = await saveAsync({ id: thread.id, edit: { before, after } });
            return saved.body;
          }}
        />
        <ChatMessages
          target={{ discussionThreadId: thread.id }}
          empty={{ title: "No messages yet", description: "Talk this thread through here." }}
        />
      </ChatLayout>
      <ConfirmDialog
        open={isDeleting}
        onOpenChange={setIsDeleting}
        title="Delete thread"
        description="The thread and its comments are deleted for everyone."
        confirmLabel="Delete thread"
        onConfirm={remove}
      />
    </>
  );
}

function ThreadStatusBadge({ thread }: { thread: DiscussionThreadDetail }) {
  if (thread.resolvedAt) {
    return (
      <Badge variant="success">
        <CircleCheckIcon />
        Resolved
      </Badge>
    );
  }
  return (
    <Badge variant="process">
      <CircleDotIcon />
      Open
    </Badge>
  );
}

function ThreadKindPicker({
  kind,
  disabled,
  onChange,
}: {
  kind: string;
  disabled: boolean;
  onChange: (kind: TThreadKind) => void;
}) {
  const current = threadKindOf(kind);
  const { label, icon: Icon } = threadKinds[current];
  return (
    <PropertyPicker
      label="Change kind"
      options={THREAD_KINDS.map((known) => {
        const option = threadKinds[known];
        return {
          value: known,
          label: option.label,
          icon: <option.icon className="text-muted-foreground size-4" />,
        };
      })}
      value={[current]}
      disabled={disabled}
      onChange={([next]) => {
        const picked = THREAD_KINDS.find((known) => known === next);
        if (picked && picked !== current) onChange(picked);
      }}
      trigger={
        <PickerTrigger
          appearance="compact"
          label={`Kind: ${label}`}
          icon={<Icon className="text-muted-foreground size-4" />}
          text={label}
        />
      }
    />
  );
}

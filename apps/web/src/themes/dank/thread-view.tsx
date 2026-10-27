import { useQuery } from "@tanstack/react-query";

import type { DiscussionThreadDetail } from "@/api/ops.gen";
import { ChatComposer } from "@/components/forum/chat-message";
import { ChatMessages } from "@/components/forum/chat-messages";
import { IssueLinkPicker } from "@/components/forum/issue-link-picker";
import { threadKindOf } from "@/components/forum/thread-kinds";
import { ThreadKindPicker } from "@/components/forum/thread-panel";
import { useCanRewrite } from "@/components/forum/use-can-rewrite";
import { CircleCheckIcon, CircleIcon, SplitIcon, UndoIcon } from "@/components/icons";
import { ChatLayout } from "@/components/layout/chat-layout";
import { EmptyState } from "@/components/layout/empty-state";
import { ErrorCard } from "@/components/layout/error-card";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { AutosaveMarkdown } from "@/components/shared/autosave-markdown";
import { InlineText } from "@/components/shared/inline-text";
import { Button } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useHotkeyScope } from "@/lib/hotkeys/use-hotkey-scope";
import {
  DISCUSSION_TITLE_MAX_LENGTH,
  discussionThreadQuery,
  useSaveDiscussionThreadMutation,
} from "@/lib/queries/discussions";
import { isMissing } from "@/lib/queries/projects";
import { Byline } from "@/themes/dank/byline";

/** A thread in place of the post and its chat: its own header over its messages, Escape steps back. */
export function DankThreadView({ threadId, onBack }: { threadId: string; onBack: () => void }) {
  const { workspace } = useWorkspace();
  const { data, isPending, error, refetch } = useQuery(
    discussionThreadQuery(workspace.id, threadId),
  );
  useHotkeyScope([{ hotkey: "Escape", callback: onBack }]);
  if (isPending) {
    return (
      <ChatLayout>
        <ListSkeleton rows={4} />
      </ChatLayout>
    );
  }
  if (error && isMissing(error)) {
    return (
      <EmptyState
        icon={SplitIcon}
        title="Thread not found"
        description="It was deleted, or you cannot see it."
      />
    );
  }
  if (error) {
    return (
      <ChatLayout>
        <ErrorCard message={error.message} onRetry={() => void refetch()} />
      </ChatLayout>
    );
  }
  return <ThreadView thread={data} />;
}

function ThreadView({ thread }: { thread: DiscussionThreadDetail }) {
  const { workspace, canEdit } = useWorkspace();
  const canRewrite = useCanRewrite(thread.creator);
  const { mutate: save, mutateAsync: saveAsync } = useSaveDiscussionThreadMutation(workspace.id);
  const isResolved = Boolean(thread.resolvedAt);
  return (
    <div data-slot="discussion-chat">
      <ChatLayout
        composer={
          canEdit && (
            <ChatComposer
              target={{ discussionThreadId: thread.id }}
              placeholder="Reply in thread"
            />
          )
        }
      >
        <header data-slot="thread-head">
          <div data-slot="thread-head-top">
            <span data-slot="thread-kind-chip" data-kind={threadKindOf(thread.kind)}>
              <ThreadKindPicker
                kind={thread.kind}
                disabled={!canEdit}
                onChange={(kind) => save({ id: thread.id, kind })}
              />
            </span>
            <span data-slot="status-chip" data-resolved={isResolved || undefined}>
              {isResolved ? <CircleCheckIcon /> : <CircleIcon />}
              {isResolved ? "Resolved" : "Open"}
            </span>
            {canEdit && (
              <Button
                size="sm"
                data-slot="resolve-button"
                data-resolved={isResolved || undefined}
                onClick={() => save({ id: thread.id, resolved: !isResolved })}
              >
                {isResolved ? <UndoIcon /> : <CircleCheckIcon />}
                {isResolved ? "Reopen" : "Resolve"}
              </Button>
            )}
          </div>
          <div data-m3-title data-slot="thread-title">
            <InlineText
              value={thread.title}
              label="Thread title"
              maxLength={DISCUSSION_TITLE_MAX_LENGTH}
              readOnly={!canRewrite}
              onSave={(title) => save({ id: thread.id, title })}
            />
          </div>
          <Byline user={thread.creator} at={thread.createdAt} />
          {(canEdit || thread.issue) && (
            <div className="flex">
              <IssueLinkPicker
                issue={thread.issue}
                disabled={!canEdit}
                onChange={(issue) => save({ id: thread.id, issue: issue?.id ?? null })}
              />
            </div>
          )}
          {(thread.body || canRewrite) && (
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
          )}
        </header>
        <ChatMessages
          target={{ discussionThreadId: thread.id }}
          empty={{ title: "No replies yet", description: "Start the conversation below." }}
        />
      </ChatLayout>
    </div>
  );
}

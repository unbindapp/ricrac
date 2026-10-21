import { MessageSquareReplyIcon, XIcon } from "lucide-react";

import { ChatComposer, ChatDayDivider, ChatMessage } from "@/components/forum/chat-message";
import { ChatSkeleton, useChatMessages } from "@/components/forum/chat-messages";
import { userName } from "@/components/issues/grouping";
import { ChatLayout } from "@/components/layout/chat-layout";
import { EmptyState } from "@/components/layout/empty-state";
import { ErrorCard } from "@/components/layout/error-card";
import { PanelBar } from "@/components/layout/panel-bar";
import { HeaderAction } from "@/components/shared/detail-actions";
import { Button } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { chatRows } from "@/lib/helpers/chat";
import { useHotkeyScope } from "@/lib/hotkeys/use-hotkey-scope";

type TProps = { discussionId: string; messageId: string; onClose: () => void };

/** A message of the discussion's chat and the replies in its thread. */
export function MessageThreadPanel({ discussionId, messageId, onClose }: TProps) {
  const { canEdit } = useWorkspace();
  const target = { discussionId };
  const { threads, isPending, isComplete, error, refetch } = useChatMessages(target);
  const root = threads.find((thread) => thread.id === messageId);
  useHotkeyScope([{ hotkey: "Escape", callback: onClose }]);

  const header = (
    <PanelBar>
      <MessageSquareReplyIcon className="text-muted-foreground size-4" />
      <span className="min-w-0 flex-1 truncate text-sm font-medium">
        Thread{root?.user && ` with ${userName(root.user)}`}
      </span>
      <HeaderAction label="Close thread" onClick={onClose}>
        <XIcon className="size-4" />
      </HeaderAction>
    </PanelBar>
  );

  return (
    <section
      aria-label="Message thread"
      className="bg-background flex min-h-0 w-full flex-1 flex-col"
    >
      <ChatLayout
        header={header}
        composer={
          root &&
          canEdit && (
            <ChatComposer
              target={target}
              parentId={root.id}
              placeholder={`Reply to ${root.user ? userName(root.user) : "the message"}…`}
            />
          )
        }
      >
        {threadContent()}
      </ChatLayout>
    </section>
  );

  function threadContent() {
    if (error) return <ErrorCard message={error.message} onRetry={() => void refetch()} />;
    if (!root && (isPending || !isComplete)) return <ChatSkeleton />;
    if (!root) {
      return (
        <EmptyState
          icon={MessageSquareReplyIcon}
          title="Message not found"
          description="It was deleted, or it isn't in this discussion."
          action={
            <Button variant="outline" size="sm" onClick={onClose}>
              Close
            </Button>
          }
        />
      );
    }
    const count = root.replies.length;
    return (
      <div className="flex flex-col">
        <ChatMessage message={root} />
        <p className="text-muted-foreground border-b pt-4 pb-2 text-xs font-medium">
          {count === 0 ? "No replies yet" : `${count} ${count === 1 ? "reply" : "replies"}`}
        </p>
        <ol aria-label="Replies" className="flex flex-col">
          {chatRows(root.replies).map((row) => (
            <li key={row.key} className="flex flex-col">
              {row.kind === "day" ? (
                <ChatDayDivider date={row.date} />
              ) : (
                <ChatMessage message={row.message} isContinuation={row.isContinuation} />
              )}
            </li>
          ))}
        </ol>
      </div>
    );
  }
}

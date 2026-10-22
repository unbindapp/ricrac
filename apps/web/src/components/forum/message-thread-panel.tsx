import { ChatComposer, ChatMessage } from "@/components/forum/chat-message";
import { ChatSkeleton, ReplyList, useChatMessages } from "@/components/forum/chat-messages";
import { ArrowLeftIcon, MessageSquareReplyIcon } from "@/components/icons";
import { userName } from "@/components/issues/grouping";
import { ChatLayout } from "@/components/layout/chat-layout";
import { EmptyState } from "@/components/layout/empty-state";
import { ErrorCard } from "@/components/layout/error-card";
import { PanelBar } from "@/components/layout/panel-bar";
import { HeaderAction } from "@/components/shared/detail-actions";
import { Button } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useHotkeyScope } from "@/lib/hotkeys/use-hotkey-scope";

type TProps = { discussionId: string; messageId: string; onBack: () => void };

/** A message of the discussion's chat and the replies in its thread. */
export function MessageThreadPanel({ discussionId, messageId, onBack }: TProps) {
  const { canEdit } = useWorkspace();
  const target = { discussionId };
  const { threads, isPending, isComplete, error, refetch } = useChatMessages(target);
  const root = threads.find((thread) => thread.id === messageId);
  useHotkeyScope([{ hotkey: "Escape", callback: onBack }]);

  const header = (
    <PanelBar>
      <HeaderAction label="Back to the main chat" onClick={onBack}>
        <ArrowLeftIcon className="size-4" />
      </HeaderAction>
      <MessageSquareReplyIcon className="text-muted-foreground size-4" />
      <span className="min-w-0 flex-1 truncate text-sm font-medium">
        Replies{root?.user && ` to ${userName(root.user)}`}
        <span className="text-muted-foreground font-normal"> in the main chat</span>
      </span>
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
            <Button variant="outline" size="sm" onClick={onBack}>
              <ArrowLeftIcon className="size-4" />
              Back to the main chat
            </Button>
          }
        />
      );
    }
    const count = root.replies.length;
    return (
      <div className="flex flex-col">
        <ChatMessage message={root} />
        <p className="text-muted-foreground pt-4 text-xs font-medium">
          {count === 0 ? "No replies yet" : `${count} ${count === 1 ? "reply" : "replies"}`}
        </p>
        <ReplyList replies={root.replies} since={root.createdAt} />
      </div>
    );
  }
}

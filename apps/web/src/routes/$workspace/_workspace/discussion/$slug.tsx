import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { zodValidator } from "@tanstack/zod-adapter";
import { useLocalStorage } from "usehooks-ts";
import { z } from "zod";

import { DiscussionDetail } from "@/components/forum/discussion-detail";
import { MainChatPanel } from "@/components/forum/main-chat";
import { MessageThreadPanel } from "@/components/forum/message-thread-panel";
import { ThreadPanel } from "@/components/forum/thread-panel";
import { ThreadSwitcher } from "@/components/forum/thread-switcher";
import { MessagesSquareIcon } from "@/components/icons";
import { ErrorCard } from "@/components/layout/error-card";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { PageContainer } from "@/components/layout/page-container";
import { SplitView } from "@/components/layout/split-view";
import { EntityNotFound } from "@/components/navigation/not-found-template";
import { useDeviceSize } from "@/components/providers/device-size-provider";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { discussionQuery } from "@/lib/queries/discussions";
import { isMissing } from "@/lib/queries/projects";
import { useUiTheme } from "@/themes/ui-theme-provider";

const searchSchema = z.object({
  /** The thread open beside the discussion. */
  thread: z.string().optional(),
  /** The message whose replies are open beside the discussion. */
  message: z.string().optional(),
});

export const Route = createFileRoute("/$workspace/_workspace/discussion/$slug")({
  validateSearch: zodValidator(searchSchema),
  component: DiscussionSlugPage,
});

function DiscussionSlugPage() {
  const { slug } = Route.useParams();
  const { thread, message } = Route.useSearch();
  const navigate = Route.useNavigate();
  const { workspace } = useWorkspace();
  const { size } = useDeviceSize();
  const { threadsInChat } = useUiTheme();
  const [isChatShown, setIsChatShown] = useLocalStorage("discussion-chat-shown", true);
  const { data, isPending, error, refetch } = useQuery(discussionQuery(workspace.id, slug));
  const hasPanel = size === "lg" || size === "xl";
  const isAsideShown = isChatShown || Boolean(thread || message);
  const backToMainChat = () =>
    void navigate({
      search: (prev) => ({ ...prev, thread: undefined, message: undefined }),
      replace: true,
    });
  const toggleChat = () => {
    if (!isAsideShown) return setIsChatShown(true);
    setIsChatShown(false);
    backToMainChat();
  };
  const openMessage = (id: string) =>
    void navigate({
      search: (prev) => ({ ...prev, thread: undefined, message: id }),
      replace: true,
    });

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
            Icon={MessagesSquareIcon}
            description="Discussion not found. It was deleted, or you cannot see it."
          />
        ) : (
          <ErrorCard message={error.message} onRetry={() => void refetch()} />
        )}
      </PageContainer>
    );
  }

  const threadSwitcher = threadsInChat && <ThreadSwitcher discussion={data} threadId={thread} />;
  const threadPanel = thread ? (
    <ThreadPanel
      key={thread}
      threadId={thread}
      threadSwitcher={threadSwitcher}
      onBack={backToMainChat}
    />
  ) : message ? (
    <MessageThreadPanel
      key={message}
      discussionId={data.id}
      messageId={message}
      threadSwitcher={threadSwitcher}
      onBack={backToMainChat}
    />
  ) : null;
  const detail = (chat: "inline" | "aside" | "hidden") => (
    <DiscussionDetail
      key={data.id}
      discussion={data}
      chat={chat}
      onToggleChat={toggleChat}
      threadSwitcher={threadSwitcher}
      openThreadId={thread}
      openMessageId={message}
      onOpenMessage={openMessage}
    />
  );
  const mainChat = (
    <MainChatPanel
      discussion={data}
      openMessageId={message}
      onOpenMessage={openMessage}
      threadSwitcher={threadSwitcher}
    />
  );

  if (!hasPanel) return threadPanel ?? detail("inline");
  if (!isAsideShown) return <SplitView main={detail("hidden")} />;
  return <SplitView main={detail("aside")} aside={threadPanel ?? mainChat} />;
}

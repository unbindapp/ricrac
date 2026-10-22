import { useState } from "react";
import { MessagesSquareIcon } from "lucide-react";

import type { Comment, DiscussionDetail } from "@/api/ops.gen";
import { ChatComposer } from "@/components/forum/chat-message";
import { ChatMessages } from "@/components/forum/chat-messages";
import { NewThreadDialog } from "@/components/forum/new-thread-dialog";
import { commentExcerpt } from "@/components/inbox/notifications";
import { userName } from "@/components/issues/grouping";
import { ChatLayout } from "@/components/layout/chat-layout";
import { PanelBar } from "@/components/layout/panel-bar";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { messagePlaceholder, shorten } from "@/lib/helpers/chat";

const THREAD_TITLE_FROM_MESSAGE_LENGTH = 80;

type TProps = {
  discussion: DiscussionDetail;
  openMessageId?: string;
  onOpenMessage: (messageId: string) => void;
};

/** The discussion's own chat, beside the topic. */
export function MainChatPanel({ discussion, openMessageId, onOpenMessage }: TProps) {
  const { canEdit } = useWorkspace();
  const count = discussion.commentCount;
  const header = (
    <PanelBar>
      <MessagesSquareIcon className="text-muted-foreground size-4" />
      <span className="text-sm font-medium">Main chat</span>
      <span className="text-muted-foreground text-xs tabular-nums">
        {count} {count === 1 ? "message" : "messages"}
      </span>
    </PanelBar>
  );
  return (
    <section aria-label="Main chat" className="bg-background flex min-h-0 w-full flex-1 flex-col">
      <ChatLayout
        header={header}
        composer={canEdit && <MainChatComposer discussion={discussion} />}
      >
        <MainChatMessages
          discussion={discussion}
          openMessageId={openMessageId}
          onOpenMessage={onOpenMessage}
        />
      </ChatLayout>
    </section>
  );
}

export function MainChatComposer({ discussion }: { discussion: DiscussionDetail }) {
  return (
    <ChatComposer
      target={{ discussionId: discussion.id }}
      placeholder={messagePlaceholder(discussion.title)}
    />
  );
}

export function MainChatMessages({ discussion, openMessageId, onOpenMessage }: TProps) {
  const [splitFrom, setSplitFrom] = useState<Comment | null>(null);
  return (
    <>
      <ChatMessages
        target={{ discussionId: discussion.id }}
        threads={{ openId: openMessageId, onOpen: onOpenMessage }}
        onNewThread={setSplitFrom}
        empty={{
          title: "No messages yet",
          description: "Say what you think. Reply in a thread to keep a side topic together.",
        }}
      />
      <NewThreadDialog
        key={splitFrom?.id}
        discussion={discussion}
        open={Boolean(splitFrom)}
        onOpenChange={(open) => {
          if (!open) setSplitFrom(null);
        }}
        defaults={splitFrom ? threadFromMessage(splitFrom) : undefined}
      />
    </>
  );
}

function threadFromMessage(message: Comment) {
  const title = shorten(commentExcerpt(message.body), THREAD_TITLE_FROM_MESSAGE_LENGTH);
  const author = message.user ? userName(message.user) : "A deleted user";
  const quoted = message.body
    .split("\n")
    .map((line) => `> ${line}`.trimEnd())
    .join("\n");
  return { title, body: `${author} wrote:\n\n${quoted}\n\n` };
}

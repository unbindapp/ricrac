import type { ReactNode } from "react";

import type { DiscussionDetail } from "@/api/ops.gen";
import {
  DiscussionActions,
  DiscussionBody,
  DiscussionMeta,
  DiscussionTitle,
} from "@/components/forum/discussion-detail";
import { XIcon } from "@/components/icons";
import { ChatLeadContext } from "@/components/layout/chat-layout";
import { HeaderAction } from "@/components/shared/detail-actions";

type TProps = {
  discussion: DiscussionDetail;
  /** The main chat, or the thread open in its place. */
  chat: ReactNode;
  onClose: () => void;
};

/** A discussion in one panel: its post opens the chat and scrolls with it. */
export function DiscussionPanel({ discussion, chat, onClose }: TProps) {
  const body = discussion.body ?? "";
  const cover =
    discussion.thumbnailUrl && !body.includes(discussion.thumbnailUrl)
      ? discussion.thumbnailUrl
      : null;
  const post = (
    <article aria-label="Post" data-slot="discussion-post">
      <DiscussionMeta discussion={discussion} tagChips />
      {cover && <img data-slot="discussion-cover" src={cover} alt="" />}
      <DiscussionBody discussion={discussion} />
    </article>
  );
  return (
    <section aria-label={discussion.title} data-slot="discussion-panel">
      <div className="flex items-start gap-2">
        <div className="min-w-0 flex-1">
          <DiscussionTitle discussion={discussion} />
        </div>
        <DiscussionActions discussion={discussion} />
        <HeaderAction label="Back to the forum" onClick={onClose}>
          <XIcon className="size-4" />
        </HeaderAction>
      </div>
      <ChatLeadContext value={post}>
        <div data-slot="discussion-chat">{chat}</div>
      </ChatLeadContext>
    </section>
  );
}

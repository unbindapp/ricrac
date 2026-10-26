import type { ReactNode } from "react";

import type { DiscussionDetail } from "@/api/ops.gen";
import {
  DiscussionActions,
  DiscussionBody,
  DiscussionMeta,
  DiscussionTitle,
} from "@/components/forum/discussion-detail";
import { HashIcon, XIcon } from "@/components/icons";
import { ChatLeadContext } from "@/components/layout/chat-layout";
import { HeaderAction } from "@/components/shared/detail-actions";
import { channelName } from "@/lib/helpers/chat";

type TProps = {
  discussion: DiscussionDetail;
  /** The main chat, or the thread open in its place. */
  chat: ReactNode;
  threadSwitcher?: ReactNode;
  onClose: () => void;
};

/** A discussion in one panel: its post opens the chat and scrolls with it. */
export function DiscussionPanel({ discussion, chat, threadSwitcher, onClose }: TProps) {
  const body = discussion.body ?? "";
  const cover =
    discussion.thumbnailUrl && !body.includes(discussion.thumbnailUrl)
      ? discussion.thumbnailUrl
      : null;
  const post = (
    <article aria-label="Post" data-slot="discussion-post">
      <DiscussionTitle discussion={discussion} />
      <DiscussionMeta discussion={discussion} tagChips />
      {cover && <img data-slot="discussion-cover" src={cover} alt="" />}
      <DiscussionBody discussion={discussion} />
    </article>
  );
  return (
    <section aria-label={discussion.title} data-slot="discussion-panel">
      <div data-slot="discussion-bar">
        <span data-slot="discussion-channel">
          <HashIcon className="size-6 shrink-0" />
          <span className="truncate">{channelName(discussion.title)}</span>
        </span>
        <div role="group" aria-label="Discussion actions" data-slot="button-group">
          {threadSwitcher}
          <DiscussionActions discussion={discussion} grouped />
          <HeaderAction label="Back to the forum" onClick={onClose}>
            <XIcon className="size-4" />
          </HeaderAction>
        </div>
      </div>
      <ChatLeadContext value={post}>
        <div data-slot="discussion-chat">{chat}</div>
      </ChatLeadContext>
    </section>
  );
}

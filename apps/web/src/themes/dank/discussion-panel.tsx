import type { ReactNode } from "react";

import type { DiscussionDetail } from "@/api/ops.gen";
import {
  DiscussionActions,
  DiscussionBody,
  DiscussionHeading,
} from "@/components/forum/discussion-detail";
import { XIcon } from "@/components/icons";
import { HeaderAction } from "@/components/shared/detail-actions";

type TProps = {
  discussion: DiscussionDetail;
  /** The main chat, or the thread open in its place. */
  chat: ReactNode;
  onClose: () => void;
};

/** A discussion's topic over its chat, in one panel. */
export function DiscussionPanel({ discussion, chat, onClose }: TProps) {
  const body = discussion.body ?? "";
  const cover =
    discussion.thumbnailUrl && !body.includes(discussion.thumbnailUrl)
      ? discussion.thumbnailUrl
      : null;
  return (
    <section aria-label={discussion.title} data-slot="discussion-panel">
      <div data-slot="discussion-post">
        <div className="flex items-start gap-2">
          <div className="min-w-0 flex-1">
            <DiscussionHeading discussion={discussion} />
          </div>
          <DiscussionActions discussion={discussion} />
          <HeaderAction label="Back to the forum" onClick={onClose}>
            <XIcon className="size-4" />
          </HeaderAction>
        </div>
        {cover && <img data-slot="discussion-cover" src={cover} alt="" />}
        <DiscussionBody discussion={discussion} />
      </div>
      <div data-slot="discussion-chat">{chat}</div>
    </section>
  );
}

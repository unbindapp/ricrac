import type { CSSProperties, HTMLAttributes, MouseEvent, Ref } from "react";

import type { Discussion } from "@/api/ops.gen";
import { DiscussionCounts, discussionCover } from "@/components/forum/discussion-card";
import { DiscussionTagBadges } from "@/components/forum/discussion-tags";
import { LockIcon, MessagesSquareIcon } from "@/components/icons";
import { cardClassName, cardOverlayClassName } from "@/components/issues/issue-card";
import { PersonAvatar } from "@/components/projects/people";
import { cn } from "@/components/ui/utils";

const MAX_TAGS = 2;

type TProps = {
  discussion: Discussion;
  isDragging?: boolean;
  isOverlay?: boolean;
  onClick?: (event: MouseEvent) => void;
  ref?: Ref<HTMLDivElement>;
  style?: CSSProperties;
  dragProps?: HTMLAttributes<HTMLDivElement>;
};

export function DiscussionBoardCard({
  discussion,
  isDragging,
  isOverlay,
  onClick,
  ref,
  style,
  dragProps,
}: TProps) {
  const cover = discussionCover(discussion);
  const KindIcon = discussion.closedAt ? LockIcon : MessagesSquareIcon;
  return (
    <div
      ref={ref}
      style={style}
      {...dragProps}
      role="listitem"
      aria-label={`Discussion ${discussion.title}`}
      data-dragging={isDragging || undefined}
      onClick={onClick}
      className={cn(cardClassName, isOverlay && cardOverlayClassName)}
    >
      {cover?.kind === "image" && (
        <div className="bg-muted aspect-video overflow-hidden rounded-md">
          <img src={cover.url} alt="" loading="lazy" className="size-full object-cover" />
        </div>
      )}
      <div className="flex items-start gap-2">
        <KindIcon className="text-muted-foreground mt-0.5 size-4 shrink-0" />
        <p className="dank:type-title-medium min-w-0 flex-1">{discussion.title}</p>
      </div>
      <div className="text-muted-foreground flex min-w-0 flex-wrap items-center gap-1.5 text-xs">
        <DiscussionTagBadges tags={discussion.tags} max={MAX_TAGS} />
        {discussion.creator && <PersonAvatar user={discussion.creator} size="xs" />}
        <DiscussionCounts discussion={discussion} />
      </div>
    </div>
  );
}

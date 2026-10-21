import { Link } from "@tanstack/react-router";
import { LockIcon, MessageSquareIcon, MessagesSquareIcon, PinIcon, SplitIcon } from "lucide-react";

import type { Discussion } from "@/api/ops.gen";
import { PersonAvatar } from "@/components/projects/people";
import { RelativeTime } from "@/components/shared/relative-time";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { entitySlug } from "@/lib/helpers/entity-slug";

export function DiscussionRow({ discussion }: { discussion: Discussion }) {
  const { workspace } = useWorkspace();
  return (
    <Link
      role="listitem"
      to="/$workspace/discussion/$slug"
      params={{ workspace: workspace.urlKey, slug: entitySlug(discussion) }}
      className="has-hover:hover:bg-muted focus-visible:bg-muted flex h-9 w-full items-center gap-3 border-b px-3 text-sm outline-hidden group-data-flush/list:h-11 group-data-flush/list:px-6 last:border-b-0"
    >
      <DiscussionIcon discussion={discussion} />
      <span className="min-w-0 flex-1 truncate font-medium">{discussion.title}</span>
      {discussion.team && (
        <span className="text-muted-foreground hidden max-w-40 shrink-0 truncate text-xs sm:block">
          {discussion.team.name}
        </span>
      )}
      <SimpleTooltip
        content={`${discussion.openThreadCount} open of ${discussion.threadCount} thread${discussion.threadCount === 1 ? "" : "s"}`}
      >
        <span className="text-muted-foreground hidden w-14 shrink-0 items-center gap-1.5 text-xs tabular-nums sm:flex">
          <SplitIcon className="size-3.5" />
          {discussion.openThreadCount}/{discussion.threadCount}
        </span>
      </SimpleTooltip>
      <SimpleTooltip
        content={`${discussion.commentCount} comment${discussion.commentCount === 1 ? "" : "s"}`}
      >
        <span className="text-muted-foreground flex w-10 shrink-0 items-center gap-1.5 text-xs tabular-nums">
          <MessageSquareIcon className="size-3.5" />
          {discussion.commentCount}
        </span>
      </SimpleTooltip>
      <span className="flex w-6 shrink-0 justify-center">
        {discussion.creator && (
          <SimpleTooltip content={`Started by ${discussion.creator.name}`}>
            <span className="flex">
              <PersonAvatar user={discussion.creator} size="xs" />
            </span>
          </SimpleTooltip>
        )}
      </span>
      <span className="text-muted-foreground w-20 shrink-0 text-right text-xs">
        <RelativeTime value={discussion.lastActivityAt} />
      </span>
    </Link>
  );
}

function DiscussionIcon({ discussion }: { discussion: Discussion }) {
  if (discussion.pinnedAt) {
    return (
      <SimpleTooltip content="Pinned">
        <span className="flex shrink-0">
          <PinIcon aria-label="Pinned" className="text-process size-4" />
        </span>
      </SimpleTooltip>
    );
  }
  if (discussion.closedAt) {
    return (
      <SimpleTooltip content="Closed">
        <span className="flex shrink-0">
          <LockIcon aria-label="Closed" className="text-muted-foreground size-4" />
        </span>
      </SimpleTooltip>
    );
  }
  return <MessagesSquareIcon className="text-muted-foreground size-4 shrink-0" />;
}

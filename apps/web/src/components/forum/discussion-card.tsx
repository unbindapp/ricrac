import { Link } from "@tanstack/react-router";
import type { ReactNode } from "react";
import { LockIcon, MessageSquareIcon, MessagesSquareIcon, PinIcon, SplitIcon } from "lucide-react";

import type { Discussion } from "@/api/ops.gen";
import { DiscussionTagBadges } from "@/components/forum/discussion-tags";
import { commentExcerpt } from "@/components/inbox/notifications";
import { PersonAvatar } from "@/components/projects/people";
import { RelativeTime } from "@/components/shared/relative-time";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { entitySlug } from "@/lib/helpers/entity-slug";
import { firstEmbed } from "@/lib/helpers/media";

const SKELETON_CARDS = 6;

export function DiscussionGrid({ children }: { children: ReactNode }) {
  return (
    <div role="list" className="grid w-full grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
      {children}
    </div>
  );
}

export function DiscussionGridSkeleton() {
  return (
    <div
      aria-busy
      aria-label="Loading"
      className="grid w-full grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3"
    >
      {Array.from({ length: SKELETON_CARDS }, (_, index) => (
        <div key={index} className="bg-card flex flex-col overflow-hidden rounded-lg border">
          <div className="bg-muted aspect-video border-b" />
          <div className="flex flex-col gap-3 p-4">
            <Skeleton className="h-4 w-16 rounded-full" />
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="h-3 w-1/2" />
          </div>
        </div>
      ))}
    </div>
  );
}

export function DiscussionCard({ discussion }: { discussion: Discussion }) {
  const { workspace } = useWorkspace();
  return (
    <Link
      role="listitem"
      to="/$workspace/discussion/$slug"
      params={{ workspace: workspace.urlKey, slug: entitySlug(discussion) }}
      className="bg-card has-hover:hover:bg-card-hover has-hover:hover:border-muted-most-foreground focus-visible:ring-ring flex min-w-0 flex-col overflow-hidden rounded-lg border outline-hidden transition-colors focus-visible:ring-1"
    >
      <DiscussionPreview body={discussion.body ?? ""} thumbnailUrl={discussion.thumbnailUrl} />
      <div className="flex flex-1 flex-col gap-3 p-4">
        <DiscussionTags discussion={discussion} />
        <h2 className="line-clamp-2 text-base leading-snug font-semibold">{discussion.title}</h2>
        <div className="text-muted-foreground mt-auto flex min-w-0 items-center gap-3 text-xs">
          {discussion.creator && (
            <span className="flex min-w-0 items-center gap-1.5">
              <PersonAvatar user={discussion.creator} size="xs" />
              <span className="text-foreground truncate font-medium">
                {discussion.creator.name}
              </span>
            </span>
          )}
          <RelativeTime value={discussion.lastActivityAt} />
          <span className="ml-auto flex shrink-0 items-center gap-3 tabular-nums">
            <SimpleTooltip
              content={`${discussion.openThreadCount} open of ${discussion.threadCount} thread${discussion.threadCount === 1 ? "" : "s"}`}
            >
              <span className="flex items-center gap-1.5">
                <SplitIcon className="size-3.5" />
                {discussion.openThreadCount}/{discussion.threadCount}
              </span>
            </SimpleTooltip>
            <SimpleTooltip
              content={`${discussion.commentCount} comment${discussion.commentCount === 1 ? "" : "s"}`}
            >
              <span className="flex items-center gap-1.5">
                <MessageSquareIcon className="size-3.5" />
                {discussion.commentCount}
              </span>
            </SimpleTooltip>
          </span>
        </div>
      </div>
    </Link>
  );
}

const MAX_CARD_TAGS = 3;

function DiscussionPreview({ body, thumbnailUrl }: { body: string; thumbnailUrl: string | null }) {
  const embed = thumbnailUrl ? { url: thumbnailUrl, kind: "image" as const } : firstEmbed(body);
  if (embed?.kind === "image") {
    return (
      <div className="bg-muted aspect-video overflow-hidden border-b">
        <img src={embed.url} alt="" loading="lazy" className="size-full object-cover" />
      </div>
    );
  }
  if (embed?.kind === "video") {
    return (
      <div className="bg-muted aspect-video overflow-hidden border-b">
        <video
          src={embed.url}
          muted
          preload="metadata"
          className="pointer-events-none size-full object-cover"
        />
      </div>
    );
  }
  const excerpt = commentExcerpt(body);
  if (!excerpt) {
    return (
      <div className="bg-muted text-muted-most-foreground flex aspect-video items-center justify-center border-b">
        <MessagesSquareIcon className="size-5" />
      </div>
    );
  }
  return (
    <div className="bg-muted aspect-video overflow-hidden border-b p-4">
      <p className="text-muted-foreground line-clamp-5 text-sm">{excerpt}</p>
    </div>
  );
}

function DiscussionTags({ discussion }: { discussion: Discussion }) {
  return (
    <div className="flex min-w-0 flex-wrap items-center gap-1.5">
      {discussion.pinnedAt && (
        <Badge variant="process">
          <PinIcon />
          Pinned
        </Badge>
      )}
      {discussion.closedAt ? (
        <Badge variant="muted">
          <LockIcon />
          Closed
        </Badge>
      ) : (
        <Badge variant="success">Open</Badge>
      )}
      <DiscussionTagBadges tags={discussion.tags} max={MAX_CARD_TAGS} />
    </div>
  );
}

import { Link } from "@tanstack/react-router";
import type { ReactNode } from "react";

import type { Discussion } from "@/api/ops.gen";
import { DiscussionTagBadges } from "@/components/forum/discussion-tags";
import {
  ExternalLinkIcon,
  LinkIcon,
  LockIcon,
  LockOpenIcon,
  MessageSquareIcon,
  MessagesSquareIcon,
  PinIcon,
  PinOffIcon,
  SplitIcon,
} from "@/components/icons";
import { commentExcerpt } from "@/components/inbox/notifications";
import { PersonAvatar } from "@/components/projects/people";
import { RelativeTime } from "@/components/shared/relative-time";
import { Badge } from "@/components/ui/badge";
import { ContextMenu, ContextMenuContent, ContextMenuTrigger } from "@/components/ui/context-menu";
import { DropdownMenuGroup, DropdownMenuItem } from "@/components/ui/dropdown-menu";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "@/components/ui/toast";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { entitySlug } from "@/lib/helpers/entity-slug";
import { firstEmbed } from "@/lib/helpers/media";
import { useCopyToClipboard } from "@/lib/hooks/use-copy";
import { useSaveDiscussionMutation } from "@/lib/queries/discussions";

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
        <div
          key={index}
          className="bg-card dank:rounded-md flex flex-col overflow-hidden rounded-lg border"
        >
          <div className="bg-muted aspect-video border-b" />
          <div className="flex flex-col gap-3 p-4">
            <Skeleton className="dank:h-8 dank:rounded-sm h-4 w-16 rounded-full" />
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="h-3 w-1/2" />
          </div>
        </div>
      ))}
    </div>
  );
}

export function DiscussionCard({ discussion }: { discussion: Discussion }) {
  const { workspace, canEdit } = useWorkspace();
  const { copyToClipboard } = useCopyToClipboard();
  const { mutate: save } = useSaveDiscussionMutation(workspace.id);
  const isPinned = Boolean(discussion.pinnedAt);
  const isClosed = Boolean(discussion.closedAt);
  return (
    <ContextMenu>
      <ContextMenuTrigger
        render={
          <Link
            role="listitem"
            to="/$workspace/discussion/$slug"
            params={{ workspace: workspace.urlKey, slug: entitySlug(discussion) }}
            className="bg-card has-hover:hover:bg-card-hover has-hover:hover:border-muted-most-foreground data-popup-open:border-muted-most-foreground focus-visible:ring-ring dank:relative dank:state-layer dank:rounded-md dank:has-hover:hover:bg-card dank:has-hover:hover:border-border dank:data-popup-open:border-border dank:has-hover:hover:shadow-elevation-1 dank:focus-visible:ring-0 dank:transition-shadow flex min-w-0 flex-col overflow-hidden rounded-lg border outline-hidden transition-colors focus-visible:ring-1"
          />
        }
      >
        <DiscussionPreview body={discussion.body ?? ""} thumbnailUrl={discussion.thumbnailUrl} />
        <div className="flex flex-1 flex-col gap-3 p-4">
          <DiscussionTags discussion={discussion} />
          <h2 className="dank:type-title-medium line-clamp-2 text-base leading-snug font-semibold">
            {discussion.title}
          </h2>
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
      </ContextMenuTrigger>
      <ContextMenuContent className="w-56">
        <DropdownMenuGroup>
          <DropdownMenuItem onClick={() => window.open(discussion.url, "_blank", "noopener")}>
            <ExternalLinkIcon className="size-4" />
            Open in new tab
          </DropdownMenuItem>
          {canEdit && (
            <DropdownMenuItem onClick={() => save({ id: discussion.id, pinned: !isPinned })}>
              {isPinned ? <PinOffIcon className="size-4" /> : <PinIcon className="size-4" />}
              {isPinned ? "Unpin" : "Pin to the top"}
            </DropdownMenuItem>
          )}
          {canEdit && (
            <DropdownMenuItem onClick={() => save({ id: discussion.id, closed: !isClosed })}>
              {isClosed ? <LockOpenIcon className="size-4" /> : <LockIcon className="size-4" />}
              {isClosed ? "Reopen discussion" : "Close discussion"}
            </DropdownMenuItem>
          )}
          <DropdownMenuItem
            onClick={() => {
              copyToClipboard(discussion.url);
              toast.add({ type: "success", title: "Link copied" });
            }}
          >
            <LinkIcon className="size-4" />
            Copy link
          </DropdownMenuItem>
        </DropdownMenuGroup>
      </ContextMenuContent>
    </ContextMenu>
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
    <div className="dank:gap-2 flex min-w-0 flex-wrap items-center gap-1.5">
      {discussion.pinnedAt && (
        <Badge variant="process" data-m3-chip="assist">
          <PinIcon />
          Pinned
        </Badge>
      )}
      {discussion.closedAt ? (
        <Badge variant="muted" data-m3-chip="assist">
          <LockIcon />
          Closed
        </Badge>
      ) : (
        <Badge variant="success" data-m3-chip="assist">
          Open
        </Badge>
      )}
      <DiscussionTagBadges tags={discussion.tags} max={MAX_CARD_TAGS} />
    </div>
  );
}

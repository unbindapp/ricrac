import { useEffect, useRef } from "react";

import { ArchiveIcon, ArchiveRestoreIcon } from "@/components/icons";
import {
  NotificationTargetIcon,
  notificationTypeIcon,
} from "@/components/inbox/notification-icons";
import {
  commentExcerpt,
  isSnoozed,
  notificationSentence,
  type TNotificationGroup,
} from "@/components/inbox/notifications";
import { useNow } from "@/components/providers/now-provider";
import { Avatar } from "@/components/shared/avatar";
import { RelativeTime } from "@/components/shared/relative-time";
import { Button } from "@/components/ui/button";
import { cn } from "@/components/ui/utils";

type TProps = {
  group: TNotificationGroup;
  isSelected: boolean;
  isArchived: boolean;
  onSelect: (group: TNotificationGroup) => void;
  onToggleArchive: (group: TNotificationGroup) => void;
};

export function NotificationRow({
  group,
  isSelected,
  isArchived,
  onSelect,
  onToggleArchive,
}: TProps) {
  const now = useNow();
  const ref = useRef<HTMLDivElement>(null);
  const { latest, target } = group;
  const isUnread = group.unreadIds.length > 0;
  const { actor, text } = notificationSentence(latest);
  const TypeIcon = notificationTypeIcon(latest.type);
  const excerpt = latest.comment ? commentExcerpt(latest.comment.body) : "";
  const snoozedUntil = isSnoozed(latest, now) ? latest.snoozedUntil : null;
  const more = group.notifications.length - 1;

  useEffect(() => {
    if (isSelected) ref.current?.scrollIntoView({ block: "nearest" });
  }, [isSelected]);

  return (
    <div
      ref={ref}
      role="option"
      tabIndex={-1}
      aria-selected={isSelected}
      aria-label={`${isUnread ? "Unread: " : ""}${target.title}`}
      data-selected={isSelected || undefined}
      onClick={() => onSelect(group)}
      className="group/row has-hover:hover:bg-accent/60 data-selected:bg-accent flex w-full cursor-default items-center gap-3 border-b px-3 py-2 text-sm last:border-b-0"
    >
      <span
        aria-hidden
        className={cn("size-2 shrink-0 rounded-full", isUnread ? "bg-process" : "bg-transparent")}
      />
      {latest.actor ? (
        <span className="dank:*:size-10 dank:*:text-base flex shrink-0">
          <Avatar id={latest.actor.id} name={actor ?? latest.actor.email} />
        </span>
      ) : (
        <span className="bg-muted text-muted-foreground dank:size-10 dank:bg-m3-primary-container dank:text-m3-on-primary-container flex size-6 shrink-0 items-center justify-center rounded-full">
          <TypeIcon className="dank:size-5 size-3.5" />
        </span>
      )}
      <div className="dank:gap-0 flex min-w-0 flex-1 flex-col gap-0.5">
        <div className="flex min-w-0 items-center gap-1.5">
          <NotificationTargetIcon target={target} />
          {target.kind === "issue" && (
            <span className="text-muted-foreground shrink-0 font-mono text-xs">
              {target.identifier}
            </span>
          )}
          <span className={cn("truncate", isUnread ? "font-medium" : "text-muted-foreground")}>
            {target.title}
          </span>
        </div>
        <div
          data-slot="list-supporting"
          className="text-muted-foreground flex min-w-0 items-center gap-1.5 text-xs"
        >
          {latest.actor && <TypeIcon className="size-3.5 shrink-0" />}
          <span className="truncate">
            {actor && <span className="text-foreground font-medium">{actor} </span>}
            {text}
            {excerpt && `: ${excerpt}`}
          </span>
        </div>
      </div>
      <div className="text-muted-foreground dank:type-label-small flex shrink-0 flex-col items-end gap-0.5 text-xs has-hover:group-hover/row:hidden">
        <span>
          {snoozedUntil ? (
            <>
              Back <RelativeTime value={snoozedUntil} />
            </>
          ) : (
            <RelativeTime value={latest.createdAt} />
          )}
        </span>
        {more > 0 && <span>+{more} more</span>}
      </div>
      <Button
        variant="ghost"
        size="icon"
        aria-label={isArchived ? "Move to inbox" : "Archive"}
        className="hidden has-hover:group-hover/row:flex"
        onClick={(event) => {
          event.stopPropagation();
          onToggleArchive(group);
        }}
      >
        {isArchived ? (
          <ArchiveRestoreIcon className="size-4" />
        ) : (
          <ArchiveIcon className="size-4" />
        )}
      </Button>
    </div>
  );
}

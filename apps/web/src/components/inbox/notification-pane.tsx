import {
  ArchiveIcon,
  ArchiveRestoreIcon,
  ArrowUpRightIcon,
  InboxIcon,
  MailIcon,
  MailOpenIcon,
} from "lucide-react";
import { useState } from "react";

import {
  NotificationTargetIcon,
  notificationTypeIcon,
} from "@/components/inbox/notification-icons";
import { NotificationIssue } from "@/components/inbox/notification-target";
import {
  commentExcerpt,
  isSnoozed,
  notificationLink,
  notificationSentence,
  type TNotificationGroup,
} from "@/components/inbox/notifications";
import { SnoozeMenu } from "@/components/inbox/snooze-menu";
import { EmptyState } from "@/components/layout/empty-state";
import { useNow } from "@/components/providers/now-provider";
import { Avatar } from "@/components/shared/avatar";
import { RelativeTime } from "@/components/shared/relative-time";
import { Button, LinkButton } from "@/components/ui/button";
import { Kbd } from "@/components/ui/kbd";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspace } from "@/components/workspace/workspace-provider";

type TProps = {
  group: TNotificationGroup | undefined;
  isArchived: boolean;
  isSnoozeOpen: boolean;
  onSnoozeOpenChange: (open: boolean) => void;
  onSnooze: (group: TNotificationGroup, until: Date) => void;
  onUnsnooze: (group: TNotificationGroup) => void;
  onToggleRead: (group: TNotificationGroup) => void;
  onToggleArchive: (group: TNotificationGroup) => void;
};

const targetLabels = {
  issue: "Open issue",
  project: "Open project",
  initiative: "Open initiative",
  document: "Open document",
  discussion: "Open discussion",
  none: "Open",
};

export function NotificationPane({
  group,
  isArchived,
  isSnoozeOpen,
  onSnoozeOpenChange,
  onSnooze,
  onUnsnooze,
  onToggleRead,
  onToggleArchive,
}: TProps) {
  const { workspace } = useWorkspace();
  const now = useNow();
  if (!group) {
    return (
      <section
        aria-label="Notification"
        className="flex min-h-0 w-full flex-1 flex-col justify-center"
      >
        <EmptyState
          icon={InboxIcon}
          title="No notification selected"
          description="Pick one from the list, or move through it with J and K."
        />
      </section>
    );
  }
  const { target } = group;
  const isUnread = group.unreadIds.length > 0;
  const link = notificationLink(group.latest, workspace.urlKey);
  return (
    <section
      aria-label={target.title}
      className="bg-background flex min-h-0 w-full flex-1 flex-col"
    >
      <div className="flex h-12 shrink-0 items-center gap-2 border-b px-3">
        <div className="flex min-w-0 flex-1 items-center gap-1.5 text-sm">
          <NotificationTargetIcon target={target} />
          {target.kind === "issue" && (
            <span className="text-muted-foreground shrink-0 font-mono text-xs">
              {target.identifier}
            </span>
          )}
          <span className="truncate font-medium">{target.title}</span>
        </div>
        <ActionTooltip label={isUnread ? "Mark read" : "Mark unread"} hotkey="U">
          <Button
            variant="ghost"
            size="icon"
            aria-label={isUnread ? "Mark read" : "Mark unread"}
            onClick={() => onToggleRead(group)}
          >
            {isUnread ? <MailOpenIcon className="size-4" /> : <MailIcon className="size-4" />}
          </Button>
        </ActionTooltip>
        <SnoozeMenu
          open={isSnoozeOpen}
          onOpenChange={onSnoozeOpenChange}
          onSnooze={(until) => onSnooze(group, until)}
          onUnsnooze={isSnoozed(group.latest, now) ? () => onUnsnooze(group) : undefined}
        />
        <ActionTooltip label={isArchived ? "Move to inbox" : "Archive"} hotkey="E">
          <Button
            variant="ghost"
            size="icon"
            aria-label={isArchived ? "Move to inbox" : "Archive"}
            onClick={() => onToggleArchive(group)}
          >
            {isArchived ? (
              <ArchiveRestoreIcon className="size-4" />
            ) : (
              <ArchiveIcon className="size-4" />
            )}
          </Button>
        </ActionTooltip>
        {link && (
          <ActionTooltip label={targetLabels[target.kind]} hotkey="Enter">
            <LinkButton
              {...link}
              variant="ghost"
              size="icon"
              aria-label={targetLabels[target.kind]}
            >
              <ArrowUpRightIcon className="size-4" />
            </LinkButton>
          </ActionTooltip>
        )}
      </div>
      <div className="flex min-h-0 flex-1 flex-col overflow-y-auto">
        <NotificationEvents group={group} />
        {target.kind === "issue" ? (
          <NotificationIssue identifier={target.identifier} />
        ) : (
          link && (
            <div className="flex px-4 pb-4">
              <LinkButton {...link} variant="outline" size="sm">
                <NotificationTargetIcon target={target} />
                {targetLabels[target.kind]}
              </LinkButton>
            </div>
          )
        )}
      </div>
    </section>
  );
}

function ActionTooltip({
  label,
  hotkey,
  children,
}: {
  label: string;
  hotkey: string;
  children: React.ReactElement;
}) {
  return (
    <SimpleTooltip
      content={
        <span className="flex items-center gap-2">
          {label}
          <Kbd>{hotkey}</Kbd>
        </span>
      }
    >
      {children}
    </SimpleTooltip>
  );
}

const SHOWN_EVENTS = 5;

/** What happened to the target, newest first, each leading to its comment or update. */
function NotificationEvents({ group }: { group: TNotificationGroup }) {
  const { workspace } = useWorkspace();
  const [showAll, setShowAll] = useState(false);
  const shown = showAll ? group.notifications : group.notifications.slice(0, SHOWN_EVENTS);
  const hidden = group.notifications.length - shown.length;
  return (
    <ol aria-label="What happened" className="flex flex-col gap-3 border-b p-4">
      {shown.map((notification) => {
        const { actor, text } = notificationSentence(notification);
        const TypeIcon = notificationTypeIcon(notification.type);
        const link = notificationLink(notification, workspace.urlKey);
        const excerpt = notification.comment ? commentExcerpt(notification.comment.body) : "";
        return (
          <li key={notification.id} className="flex items-start gap-2 text-sm">
            {notification.actor ? (
              <Avatar
                id={notification.actor.id}
                name={actor ?? notification.actor.email}
                size="sm"
              />
            ) : (
              <span className="bg-muted text-muted-foreground flex size-5 shrink-0 items-center justify-center rounded-full">
                <TypeIcon className="size-3.5" />
              </span>
            )}
            <div className="flex min-w-0 flex-1 flex-col gap-1">
              <p className="flex min-w-0 items-center gap-1.5">
                <span className="min-w-0 truncate">
                  {actor && <span className="font-medium">{actor} </span>}
                  <span className={actor ? "text-muted-foreground" : undefined}>{text}</span>
                </span>
                {notification.readAt === null && (
                  <span aria-label="Unread" className="bg-process size-1.5 shrink-0 rounded-full" />
                )}
                <span className="text-muted-foreground ml-auto shrink-0 text-xs">
                  <RelativeTime value={notification.createdAt} />
                </span>
              </p>
              {excerpt && link && (
                <LinkButton
                  {...link}
                  variant="ghost"
                  size="sm"
                  forceMinSize={false}
                  className="bg-muted/60 text-muted-foreground h-auto justify-start py-1.5 text-left font-normal whitespace-normal"
                >
                  <span className="line-clamp-2">{excerpt}</span>
                </LinkButton>
              )}
            </div>
          </li>
        );
      })}
      {hidden > 0 && (
        <li>
          <Button variant="ghost" size="sm" onClick={() => setShowAll(true)}>
            Show {hidden} earlier
          </Button>
        </li>
      )}
    </ol>
  );
}

import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useEffect, useMemo, useRef, useState } from "react";

import {
  AlarmClockIcon,
  ArchiveIcon,
  CheckCheckIcon,
  EllipsisIcon,
  InboxIcon,
  MailIcon,
} from "@/components/icons";
import { NotificationPane } from "@/components/inbox/notification-pane";
import { NotificationRow } from "@/components/inbox/notification-row";
import {
  groupNotifications,
  notificationLink,
  type TNotificationGroup,
} from "@/components/inbox/notifications";
import { ListStates } from "@/components/layout/list-states";
import { PageContainer } from "@/components/layout/page-container";
import { PageHeader } from "@/components/layout/page-header";
import { SplitView } from "@/components/layout/split-view";
import { useDeviceSize } from "@/components/providers/device-size-provider";
import { useNow } from "@/components/providers/now-provider";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { toast } from "@/components/ui/toast";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { FilterMenu } from "@/components/shared/filter-menu";
import { formatDateTime } from "@/lib/helpers/dates";
import { useHotkeyScope } from "@/lib/hotkeys/use-hotkey-scope";
import {
  notificationsQuery,
  type TInboxFilter,
  type TNotificationAction,
  unreadCountQuery,
  useMarkAllNotificationsReadMutation,
  useMarkNotificationsMutation,
} from "@/lib/queries/notifications";

export const INBOX_FILTERS = ["inbox", "unread", "snoozed", "archived"] as const;

const filterLabels: Record<TInboxFilter, string> = {
  inbox: "All",
  unread: "Unread",
  snoozed: "Snoozed",
  archived: "Archived",
};

const emptyStates = {
  inbox: {
    icon: InboxIcon,
    title: "You're all caught up",
    description: "Assignments, mentions and comments on what you follow show up here.",
  },
  unread: {
    icon: MailIcon,
    title: "No unread notifications yet",
    description: "Everything in your inbox has been read.",
  },
  snoozed: {
    icon: AlarmClockIcon,
    title: "No snoozed notifications yet",
    description: "Snooze one with H and it comes back unread when the time comes.",
  },
  archived: {
    icon: ArchiveIcon,
    title: "No archived notifications yet",
    description: "Archive one with E once you're done with it.",
  },
} satisfies Record<TInboxFilter, unknown>;

/** Enter on a focused button or link presses it instead of opening the row. */
function isOnControl(target: EventTarget | null) {
  return target instanceof Element && target.closest("button, a, [role=menuitem]") !== null;
}

type TProps = {
  filter: TInboxFilter;
  selectedKey: string | undefined;
  onFilterChange: (filter: TInboxFilter) => void;
  onSelect: (key: string | undefined) => void;
};

export function InboxView({ filter, selectedKey, onFilterChange, onSelect }: TProps) {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const now = useNow();
  const { size } = useDeviceSize();
  const hasPane = size === "lg" || size === "xl";
  const [isSnoozeOpen, setIsSnoozeOpen] = useState(false);

  const list = useInfiniteQuery(notificationsQuery(workspace.id, filter));
  const { data: unread = 0 } = useQuery(unreadCountQuery(workspace.id));
  const groups = useMemo(
    () =>
      groupNotifications(
        list.data?.pages.flatMap((page) => page.nodes) ?? [],
        filter,
        now,
        selectedKey,
      ),
    [list.data, filter, now, selectedKey],
  );
  const selectedIndex = groups.findIndex((group) => group.key === selectedKey);
  const selected = selectedIndex >= 0 ? groups[selectedIndex] : undefined;
  const isArchivedFilter = filter === "archived";

  const { mutate: mark } = useMarkNotificationsMutation(workspace.id);
  const { mutate: markAllRead, isPending: isMarkingAll } = useMarkAllNotificationsReadMutation(
    workspace.id,
  );
  const markGroup = (group: TNotificationGroup, action: TNotificationAction, until?: string) =>
    mark({ ids: group.notifications.map((n) => n.id), action, until });

  const readOnOpen = useRef<string | null>(null);
  useEffect(() => {
    if (!hasPane || !selected || readOnOpen.current === selected.key) return;
    readOnOpen.current = selected.key;
    if (selected.unreadIds.length > 0) mark({ ids: selected.unreadIds, action: "read" });
  }, [hasPane, selected, mark]);

  const selectNeighbourOf = (group: TNotificationGroup) => {
    if (group.key !== selectedKey) return;
    const index = groups.findIndex((item) => item.key === group.key);
    onSelect((groups[index + 1] ?? groups[index - 1])?.key);
  };

  const openTarget = (group: TNotificationGroup) => {
    const link = notificationLink(group.latest, workspace.urlKey);
    if (group.unreadIds.length > 0) mark({ ids: group.unreadIds, action: "read" });
    if (link) void navigate(link);
  };

  const select = (group: TNotificationGroup) => {
    if (hasPane) {
      onSelect(group.key);
      return;
    }
    openTarget(group);
  };

  const toggleRead = (group: TNotificationGroup) => {
    if (group.unreadIds.length > 0) {
      mark({ ids: group.unreadIds, action: "read" });
      return;
    }
    readOnOpen.current = group.key;
    mark({ ids: [group.latest.id], action: "unread" });
  };

  const toggleArchive = (group: TNotificationGroup) => {
    selectNeighbourOf(group);
    if (isArchivedFilter) {
      markGroup(group, "unarchive");
      return;
    }
    markGroup(group, "archive");
    toast.add({
      type: "success",
      title: "Archived",
      data: {
        action: (
          <Button variant="outline" size="sm" onClick={() => markGroup(group, "unarchive")}>
            Undo
          </Button>
        ),
      },
    });
  };

  const snooze = (group: TNotificationGroup, until: Date) => {
    selectNeighbourOf(group);
    markGroup(group, "snooze", until.toISOString());
    toast.add({ type: "success", title: "Snoozed", description: `Until ${formatDateTime(until)}` });
  };

  const unsnooze = (group: TNotificationGroup) => {
    if (filter === "snoozed") selectNeighbourOf(group);
    markGroup(group, "unsnooze");
  };

  const archiveRead = () => {
    const read = groups.filter((group) => group.unreadIds.length === 0);
    if (read.length === 0) return;
    if (selected && read.includes(selected)) onSelect(undefined);
    mark({ ids: read.flatMap((group) => group.notifications.map((n) => n.id)), action: "archive" });
    toast.add({ type: "success", title: `Archived ${read.length} read` });
  };

  const move = (step: number) => {
    if (groups.length === 0) return;
    const next = Math.min(
      Math.max(selectedIndex < 0 ? 0 : selectedIndex + step, 0),
      groups.length - 1,
    );
    onSelect(groups[next].key);
    if (next >= groups.length - 3 && list.hasNextPage && !list.isFetchingNextPage) {
      void list.fetchNextPage();
    }
  };

  useHotkeyScope(
    [
      { hotkey: "J", callback: () => move(1) },
      { hotkey: "K", callback: () => move(-1) },
      { hotkey: "ArrowDown", callback: () => move(1) },
      { hotkey: "ArrowUp", callback: () => move(-1) },
      {
        hotkey: "E",
        callback: () => selected && toggleArchive(selected),
        enabled: Boolean(selected),
      },
      { hotkey: "U", callback: () => selected && toggleRead(selected), enabled: Boolean(selected) },
      { hotkey: "H", callback: () => setIsSnoozeOpen(true), enabled: Boolean(selected) },
      {
        hotkey: "Enter",
        callback: (event) => {
          if (!selected || isOnControl(event.target)) return;
          openTarget(selected);
        },
        enabled: Boolean(selected),
      },
      { hotkey: "Escape", callback: () => onSelect(undefined), enabled: Boolean(selected) },
    ],
    { enabled: !isSnoozeOpen },
  );

  const main = (
    <PageContainer>
      <PageHeader
        title="Inbox"
        icon={<InboxIcon />}
        actions={
          <>
            <Button
              variant="outline"
              size="sm"
              disabled={unread === 0}
              isPending={isMarkingAll}
              onClick={() => markAllRead()}
            >
              <CheckCheckIcon className="size-4" />
              Mark all read
            </Button>
            <DropdownMenu>
              <DropdownMenuTrigger
                render={
                  <Button variant="ghost" size="icon" aria-label="More inbox actions">
                    <EllipsisIcon className="size-4" />
                  </Button>
                }
              />
              <DropdownMenuContent align="end">
                <DropdownMenuItem disabled={isArchivedFilter} onClick={archiveRead}>
                  <ArchiveIcon className="size-4" />
                  Archive all read
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </>
        }
        toolbar={
          <FilterMenu
            label="Show"
            value={filter}
            options={INBOX_FILTERS.map((value) => ({
              value,
              label: filterLabels[value],
              suffix: value === "unread" && unread > 0 && (
                <Badge variant="muted">{unread > 99 ? "99+" : unread}</Badge>
              ),
            }))}
            onChange={onFilterChange}
          />
        }
      />
      <ListStates
        isPending={list.isPending}
        error={list.error}
        onRetry={() => void list.refetch()}
        isEmpty={groups.length === 0 && !list.hasNextPage}
        empty={emptyStates[filter]}
      >
        <div className="flex w-full flex-col gap-2">
          <div
            role="listbox"
            aria-label="Notifications"
            className="bg-card flex w-full flex-col overflow-hidden rounded-lg border"
          >
            {groups.map((group) => (
              <NotificationRow
                key={group.key}
                group={group}
                isSelected={group.key === selectedKey}
                isArchived={isArchivedFilter}
                onSelect={select}
                onToggleArchive={toggleArchive}
              />
            ))}
          </div>
          {list.hasNextPage && (
            <Button
              variant="ghost"
              size="sm"
              className="self-center"
              isPending={list.isFetchingNextPage}
              onClick={() => void list.fetchNextPage()}
            >
              Load older
            </Button>
          )}
        </div>
      </ListStates>
    </PageContainer>
  );

  return (
    <SplitView
      main={main}
      aside={
        hasPane ? (
          <NotificationPane
            group={selected}
            isArchived={isArchivedFilter}
            isSnoozeOpen={isSnoozeOpen}
            onSnoozeOpenChange={setIsSnoozeOpen}
            onSnooze={snooze}
            onUnsnooze={unsnooze}
            onToggleRead={toggleRead}
            onToggleArchive={toggleArchive}
          />
        ) : undefined
      }
    />
  );
}

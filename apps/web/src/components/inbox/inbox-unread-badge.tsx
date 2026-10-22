import { useQuery } from "@tanstack/react-query";

import { useWorkspace } from "@/components/workspace/workspace-provider";
import { unreadCountQuery } from "@/lib/queries/notifications";

export function useInboxUnreadCount() {
  const { workspace } = useWorkspace();
  const { data: unread = 0 } = useQuery(unreadCountQuery(workspace.id));
  return unread;
}

export function InboxUnreadBadge() {
  const unread = useInboxUnreadCount();
  if (unread === 0) return null;
  return (
    <span aria-label={`${unread} unread`} className="text-muted-foreground text-xs tabular-nums">
      {unread > 99 ? "99+" : unread}
    </span>
  );
}

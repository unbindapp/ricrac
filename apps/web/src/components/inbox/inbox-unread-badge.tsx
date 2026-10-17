import { useQuery } from "@tanstack/react-query";

import { Badge } from "@/components/ui/badge";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { unreadCountQuery } from "@/lib/queries/notifications";

/** How many unread notifications wait, beside the inbox in the sidebar. */
export function InboxUnreadBadge() {
  const { workspace } = useWorkspace();
  const { data: unread = 0 } = useQuery(unreadCountQuery(workspace.id));
  if (unread === 0) return null;
  return (
    <Badge variant="muted" aria-label={`${unread} unread`}>
      {unread > 99 ? "99+" : unread}
    </Badge>
  );
}

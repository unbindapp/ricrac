import { useQuery } from "@tanstack/react-query";

import type { UserRef } from "@/api/ops.gen";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { meQuery } from "@/lib/queries/me";

/** Only the author and admins may change a discussion's or thread's title, body or team. */
export function useCanRewrite(creator: UserRef | null) {
  const { canEdit, isAdmin } = useWorkspace();
  const { data: me } = useQuery(meQuery);
  if (!canEdit) return false;
  return isAdmin || Boolean(me && creator?.id === me.id);
}

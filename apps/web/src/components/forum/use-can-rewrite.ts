import { useQuery } from "@tanstack/react-query";

import type { UserRef } from "@/api/ops.gen";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { meQuery } from "@/lib/queries/me";

/** Only the author and admins may change a discussion's or thread's title or body. */
export function useCanRewrite(creator: UserRef | null) {
  const { canEdit, isAdmin } = useWorkspace();
  const { data: me } = useQuery(meQuery);
  if (!canEdit) return false;
  return isAdmin || Boolean(me && creator?.id === me.id);
}

/** Only the person who started a discussion closes or reopens it; admins when that person is gone. */
export function useCanClose(creator: UserRef | null) {
  const { canEdit, isAdmin } = useWorkspace();
  const { data: me } = useQuery(meQuery);
  if (!canEdit) return false;
  if (!creator) return isAdmin;
  return Boolean(me && creator.id === me.id);
}

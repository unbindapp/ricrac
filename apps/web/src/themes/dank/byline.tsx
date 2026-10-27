import type { ReactNode } from "react";

import type { UserRef } from "@/api/ops.gen";
import { userName } from "@/components/issues/grouping";
import { PersonAvatar } from "@/components/projects/people";
import { RelativeTime } from "@/components/shared/relative-time";

/** Who started it and when, then whatever follows on the same row. */
export function Byline({
  user,
  at,
  children,
}: {
  user: UserRef | null;
  at: string;
  children?: ReactNode;
}) {
  return (
    <div data-slot="byline">
      {user && <PersonAvatar user={user} size="sm" />}
      <span>
        Started by <b>{user ? userName(user) : "a deleted user"}</b>
      </span>
      <RelativeTime value={at} />
      {children}
    </div>
  );
}

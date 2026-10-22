import type { ReactNode } from "react";

import { LockIcon } from "@/components/icons";
import { EmptyState } from "@/components/layout/empty-state";
import { useWorkspace } from "@/components/workspace/workspace-provider";

type TProps = { access: "members" | "admins"; children: ReactNode };

/** Settings only members, or only admins, may change. */
export function SettingsGate({ access, children }: TProps) {
  const { canEdit, isAdmin } = useWorkspace();
  if (access === "members" && !canEdit) {
    return (
      <EmptyState
        icon={LockIcon}
        title="Members only"
        description="Guests can't change workspace settings."
      />
    );
  }
  if (access === "admins" && !isAdmin) {
    return (
      <EmptyState
        icon={LockIcon}
        title="Admins only"
        description="Ask a workspace admin to change these settings."
      />
    );
  }
  return children;
}

import { createFileRoute, Outlet } from "@tanstack/react-router";
import { LockIcon } from "lucide-react";

import { EmptyState } from "@/components/layout/empty-state";
import { useWorkspace } from "@/components/workspace/workspace-provider";

export const Route = createFileRoute("/$workspace/_workspace/settings/_workspace-settings")({
  component: WorkspaceSettingsGate,
});

function WorkspaceSettingsGate() {
  const { canEdit } = useWorkspace();
  if (canEdit) return <Outlet />;
  return (
    <EmptyState
      icon={LockIcon}
      title="Members only"
      description="Guests can't change workspace settings."
    />
  );
}

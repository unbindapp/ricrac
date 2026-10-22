import { createFileRoute, Outlet } from "@tanstack/react-router";

import { LockIcon } from "@/components/icons/symbols.gen";
import { EmptyState } from "@/components/layout/empty-state";
import { useWorkspace } from "@/components/workspace/workspace-provider";

export const Route = createFileRoute("/$workspace/_workspace/settings/_workspace-settings/_admin")({
  component: AdminSettingsGate,
});

function AdminSettingsGate() {
  const { isAdmin } = useWorkspace();
  if (isAdmin) return <Outlet />;
  return (
    <EmptyState
      icon={LockIcon}
      title="Admins only"
      description="Ask a workspace admin to change these settings."
    />
  );
}

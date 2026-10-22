import { createFileRoute, Outlet } from "@tanstack/react-router";

import { SettingsGate } from "@/components/settings/settings-gate";

export const Route = createFileRoute("/$workspace/_workspace/settings/_workspace-settings")({
  component: WorkspaceSettingsGate,
});

function WorkspaceSettingsGate() {
  return (
    <SettingsGate access="members">
      <Outlet />
    </SettingsGate>
  );
}

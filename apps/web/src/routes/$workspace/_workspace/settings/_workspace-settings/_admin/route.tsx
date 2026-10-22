import { createFileRoute, Outlet } from "@tanstack/react-router";

import { SettingsGate } from "@/components/settings/settings-gate";

export const Route = createFileRoute("/$workspace/_workspace/settings/_workspace-settings/_admin")({
  component: AdminSettingsGate,
});

function AdminSettingsGate() {
  return (
    <SettingsGate access="admins">
      <Outlet />
    </SettingsGate>
  );
}

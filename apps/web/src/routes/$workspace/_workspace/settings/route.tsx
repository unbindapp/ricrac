import { createFileRoute, Outlet, redirect } from "@tanstack/react-router";

import { SettingsLayout } from "@/components/layout/settings-layout";
import { useSettingsGroups } from "@/components/settings/settings-groups";
import { settingsWindowHref } from "@/lib/stores/settings-return";
import { workspaceQuery } from "@/lib/queries/workspaces";
import { uiThemeOf } from "@/themes/registry";

export const Route = createFileRoute("/$workspace/_workspace/settings")({
  beforeLoad: async ({ context: { queryClient, workspaceSummary }, location, params, preload }) => {
    if (preload) return;
    const workspace = await queryClient.ensureQueryData(workspaceQuery(workspaceSummary.id));
    if (!uiThemeOf(workspace.theme).SettingsWindow) return;
    const settingsPath = location.pathname.slice(params.workspace.length + 1);
    throw redirect({
      href: settingsWindowHref(params.workspace, settingsPath),
      mask: { to: location.pathname },
    });
  },
  component: SettingsRoute,
});

function SettingsRoute() {
  const groups = useSettingsGroups();
  return (
    <SettingsLayout title="Settings" groups={groups}>
      <Outlet />
    </SettingsLayout>
  );
}

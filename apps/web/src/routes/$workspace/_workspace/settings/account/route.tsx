import { createFileRoute, Outlet } from "@tanstack/react-router";

import { BlocksIcon, KeySquareIcon, UserIcon } from "@/components/icons/symbols.gen";
import { SettingsLayout, type TSettingsTab } from "@/components/layout/settings-layout";
import { SettingsSectionSwitcher } from "@/components/settings/settings-section-switcher";
import { useWorkspace } from "@/components/workspace/workspace-provider";

export const Route = createFileRoute("/$workspace/_workspace/settings/account")({
  component: AccountSettingsLayout,
});

function AccountSettingsLayout() {
  const { workspace } = useWorkspace();
  const params = { workspace: workspace.urlKey };
  const tabs: TSettingsTab[] = [
    {
      label: "Profile",
      description: "Name, picture, notifications, theme",
      icon: UserIcon,
      link: { to: "/$workspace/settings/account", params },
      exact: true,
    },
    {
      label: "API keys",
      description: "Keys for scripts, CI and agents",
      icon: KeySquareIcon,
      link: { to: "/$workspace/settings/account/api-keys", params },
    },
    {
      label: "Connected apps",
      description: "MCP server and apps you allowed",
      icon: BlocksIcon,
      link: { to: "/$workspace/settings/account/connected-apps", params },
    },
  ];
  return (
    <SettingsLayout
      title="Account"
      switcher={<SettingsSectionSwitcher current="account" />}
      groups={[tabs]}
    >
      <Outlet />
    </SettingsLayout>
  );
}

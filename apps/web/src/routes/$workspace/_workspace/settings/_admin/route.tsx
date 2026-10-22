import { createFileRoute, Outlet } from "@tanstack/react-router";

import {
  AppWindowIcon,
  BoxesIcon,
  BuildingIcon,
  DownloadIcon,
  FileStackIcon,
  LockIcon,
  PlugIcon,
  RocketIcon,
  SettingsIcon,
  TagIcon,
  TagsIcon,
  TriangleAlertIcon,
  UsersIcon,
  UsersRoundIcon,
  WebhookIcon,
} from "@/components/icons/symbols.gen";
import { EmptyState } from "@/components/layout/empty-state";
import { SettingsLayout, type TSettingsTab } from "@/components/layout/settings-layout";
import { SettingsSectionSwitcher } from "@/components/settings/settings-section-switcher";
import { useWorkspace } from "@/components/workspace/workspace-provider";

export const Route = createFileRoute("/$workspace/_workspace/settings/_admin")({
  component: WorkspaceSettingsLayout,
});

function WorkspaceSettingsLayout() {
  const { workspace, isAdmin } = useWorkspace();
  const switcher = <SettingsSectionSwitcher current={{ kind: "workspace" }} />;
  if (!isAdmin) {
    return (
      <SettingsLayout title="Workspace" switcher={switcher} groups={[]}>
        <EmptyState
          icon={LockIcon}
          title="Admins only"
          description="Ask a workspace admin to change these settings."
        />
      </SettingsLayout>
    );
  }
  const params = { workspace: workspace.urlKey };
  const groups: TSettingsTab[][] = [
    [
      {
        label: "General",
        description: "Name, URL and logo",
        icon: SettingsIcon,
        link: { to: "/$workspace/settings", params },
        exact: true,
      },
      {
        label: "Members",
        description: "People, roles and invitations",
        icon: UsersIcon,
        link: { to: "/$workspace/settings/members", params },
      },
      {
        label: "Teams",
        description: "Create and archive teams",
        icon: UsersRoundIcon,
        link: { to: "/$workspace/settings/teams", params },
      },
    ],
    [
      {
        label: "Labels",
        description: "Issue labels every team shares",
        icon: TagIcon,
        link: { to: "/$workspace/settings/labels", params },
      },
      {
        label: "Project statuses",
        description: "What a project moves through",
        icon: BoxesIcon,
        link: { to: "/$workspace/settings/project-statuses", params },
      },
      {
        label: "Project labels",
        description: "Labels for projects",
        icon: TagsIcon,
        link: { to: "/$workspace/settings/project-labels", params },
      },
      {
        label: "Templates",
        description: "Starting points for new work",
        icon: FileStackIcon,
        link: { to: "/$workspace/settings/templates", params },
      },
    ],
    [
      {
        label: "Customers",
        description: "Customer statuses and tiers",
        icon: BuildingIcon,
        link: { to: "/$workspace/settings/customers", params },
      },
      {
        label: "Releases",
        description: "Pipelines and their stages",
        icon: RocketIcon,
        link: { to: "/$workspace/settings/releases", params },
      },
    ],
    [
      {
        label: "Integrations",
        description: "GitHub and other tools",
        icon: PlugIcon,
        link: { to: "/$workspace/settings/integrations", params },
      },
      {
        label: "Webhooks",
        description: "Send changes to other services",
        icon: WebhookIcon,
        link: { to: "/$workspace/settings/webhooks", params },
      },
      {
        label: "Import",
        description: "Issues from another tracker",
        icon: DownloadIcon,
        link: { to: "/$workspace/settings/import", params },
      },
      {
        label: "OAuth apps",
        description: "Your own apps that sign in with OAuth",
        icon: AppWindowIcon,
        link: { to: "/$workspace/settings/oauth-apps", params },
      },
    ],
    [
      {
        label: "Danger zone",
        description: "Leave or delete the workspace",
        icon: TriangleAlertIcon,
        link: { to: "/$workspace/settings/danger", params },
        destructive: true,
      },
    ],
  ];
  return (
    <SettingsLayout title="Workspace" switcher={switcher} groups={groups}>
      <Outlet />
    </SettingsLayout>
  );
}

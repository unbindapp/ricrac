import { createFileRoute, Outlet } from "@tanstack/react-router";
import {
  AppWindowIcon,
  ArchiveIcon,
  BlocksIcon,
  BoxesIcon,
  BuildingIcon,
  DownloadIcon,
  FileStackIcon,
  GitPullRequestIcon,
  KeySquareIcon,
  PlugIcon,
  RefreshCwIcon,
  RocketIcon,
  RulerIcon,
  SettingsIcon,
  TagIcon,
  TagsIcon,
  TriangleAlertIcon,
  TriangleIcon,
  UserIcon,
  UsersIcon,
  WebhookIcon,
  WorkflowIcon,
} from "lucide-react";

import {
  SettingsLayout,
  type TSettingsTab,
  type TSettingsTabGroup,
} from "@/components/layout/settings-layout";
import { useWorkspace } from "@/components/workspace/workspace-provider";

export const Route = createFileRoute("/$workspace/_workspace/settings")({
  component: SettingsRoute,
});

type TWorkspaceTab = TSettingsTab & { isAdminOnly?: boolean };

function accountTabs(workspace: string): TSettingsTab[] {
  const params = { workspace };
  return [
    {
      label: "Profile",
      icon: UserIcon,
      link: { to: "/$workspace/settings/account", params },
      exact: true,
    },
    {
      label: "API keys",
      icon: KeySquareIcon,
      link: { to: "/$workspace/settings/account/api-keys", params },
    },
    {
      label: "Connected apps",
      icon: BlocksIcon,
      link: { to: "/$workspace/settings/account/connected-apps", params },
    },
  ];
}

function workspaceTabs(workspace: string): TWorkspaceTab[] {
  const params = { workspace };
  return [
    {
      label: "General",
      icon: SettingsIcon,
      link: { to: "/$workspace/settings", params },
      exact: true,
    },
    {
      label: "Members",
      icon: UsersIcon,
      link: { to: "/$workspace/settings/members", params },
      isAdminOnly: true,
    },
    {
      label: "Workflow",
      icon: WorkflowIcon,
      link: { to: "/$workspace/settings/workflow", params },
    },
    { label: "Labels", icon: TagIcon, link: { to: "/$workspace/settings/labels", params } },
    {
      label: "Templates",
      icon: FileStackIcon,
      link: { to: "/$workspace/settings/templates", params },
    },
    { label: "Estimates", icon: RulerIcon, link: { to: "/$workspace/settings/estimates", params } },
    { label: "Triage", icon: TriangleIcon, link: { to: "/$workspace/settings/triage", params } },
    { label: "Cycles", icon: RefreshCwIcon, link: { to: "/$workspace/settings/cycles", params } },
    {
      label: "Auto close & archive",
      icon: ArchiveIcon,
      link: { to: "/$workspace/settings/auto-archive", params },
    },
    {
      label: "Project statuses",
      icon: BoxesIcon,
      link: { to: "/$workspace/settings/project-statuses", params },
      isAdminOnly: true,
    },
    {
      label: "Project labels",
      icon: TagsIcon,
      link: { to: "/$workspace/settings/project-labels", params },
      isAdminOnly: true,
    },
    {
      label: "Customers",
      icon: BuildingIcon,
      link: { to: "/$workspace/settings/customers", params },
      isAdminOnly: true,
    },
    {
      label: "Releases",
      icon: RocketIcon,
      link: { to: "/$workspace/settings/releases", params },
      isAdminOnly: true,
    },
    {
      label: "Integrations",
      icon: PlugIcon,
      link: { to: "/$workspace/settings/integrations", params },
      isAdminOnly: true,
    },
    {
      label: "Pull requests",
      icon: GitPullRequestIcon,
      link: { to: "/$workspace/settings/github", params },
    },
    {
      label: "Webhooks",
      icon: WebhookIcon,
      link: { to: "/$workspace/settings/webhooks", params },
      isAdminOnly: true,
    },
    {
      label: "Import",
      icon: DownloadIcon,
      link: { to: "/$workspace/settings/import", params },
      isAdminOnly: true,
    },
    {
      label: "OAuth apps",
      icon: AppWindowIcon,
      link: { to: "/$workspace/settings/oauth-apps", params },
      isAdminOnly: true,
    },
    {
      label: "Danger zone",
      icon: TriangleAlertIcon,
      link: { to: "/$workspace/settings/danger", params },
      isAdminOnly: true,
    },
  ];
}

function SettingsRoute() {
  const { workspace, canEdit, isAdmin } = useWorkspace();
  const groups: TSettingsTabGroup[] = [{ title: "Account", tabs: accountTabs(workspace.urlKey) }];
  if (canEdit) {
    groups.push({
      title: "Workspace",
      tabs: workspaceTabs(workspace.urlKey).filter((tab) => isAdmin || !tab.isAdminOnly),
    });
  }
  return (
    <SettingsLayout title="Settings" groups={groups}>
      <Outlet />
    </SettingsLayout>
  );
}

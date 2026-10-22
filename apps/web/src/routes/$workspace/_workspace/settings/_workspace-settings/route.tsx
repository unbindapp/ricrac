import { createFileRoute, Outlet } from "@tanstack/react-router";

import {
  AppWindowIcon,
  ArchiveIcon,
  BoxesIcon,
  BuildingIcon,
  DownloadIcon,
  FileStackIcon,
  GitPullRequestIcon,
  LockIcon,
  PlugIcon,
  RefreshCwIcon,
  RocketIcon,
  RulerIcon,
  SettingsIcon,
  TagIcon,
  TagsIcon,
  TriangleAlertIcon,
  TriangleIcon,
  UsersIcon,
  WebhookIcon,
  WorkflowIcon,
} from "@/components/icons/symbols.gen";
import { EmptyState } from "@/components/layout/empty-state";
import { SettingsLayout, type TSettingsTab } from "@/components/layout/settings-layout";
import { SettingsSectionSwitcher } from "@/components/settings/settings-section-switcher";
import { useWorkspace } from "@/components/workspace/workspace-provider";

export const Route = createFileRoute("/$workspace/_workspace/settings/_workspace-settings")({
  component: WorkspaceSettingsLayout,
});

type TWorkspaceTab = TSettingsTab & { isAdminOnly?: boolean };

function workspaceTabGroups(workspace: string): TWorkspaceTab[][] {
  const params = { workspace };
  return [
    [
      {
        label: "General",
        description: "Name, URL, logo and issue prefix",
        icon: SettingsIcon,
        link: { to: "/$workspace/settings", params },
        exact: true,
      },
      {
        label: "Members",
        description: "People, roles and invitations",
        icon: UsersIcon,
        link: { to: "/$workspace/settings/members", params },
        isAdminOnly: true,
      },
    ],
    [
      {
        label: "Workflow",
        description: "The statuses issues move through",
        icon: WorkflowIcon,
        link: { to: "/$workspace/settings/workflow", params },
      },
      {
        label: "Labels",
        description: "Issue labels and groups",
        icon: TagIcon,
        link: { to: "/$workspace/settings/labels", params },
      },
      {
        label: "Templates",
        description: "Starting points for new work",
        icon: FileStackIcon,
        link: { to: "/$workspace/settings/templates", params },
      },
      {
        label: "Estimates",
        description: "How issues are sized",
        icon: RulerIcon,
        link: { to: "/$workspace/settings/estimates", params },
      },
      {
        label: "Triage",
        description: "Review incoming issues",
        icon: TriangleIcon,
        link: { to: "/$workspace/settings/triage", params },
      },
      {
        label: "Cycles",
        description: "Schedule and automation",
        icon: RefreshCwIcon,
        link: { to: "/$workspace/settings/cycles", params },
      },
      {
        label: "Auto close & archive",
        description: "Keep the backlog tidy",
        icon: ArchiveIcon,
        link: { to: "/$workspace/settings/auto-archive", params },
      },
    ],
    [
      {
        label: "Project statuses",
        description: "What a project moves through",
        icon: BoxesIcon,
        link: { to: "/$workspace/settings/project-statuses", params },
        isAdminOnly: true,
      },
      {
        label: "Project labels",
        description: "Labels for projects",
        icon: TagsIcon,
        link: { to: "/$workspace/settings/project-labels", params },
        isAdminOnly: true,
      },
      {
        label: "Customers",
        description: "Customer statuses and tiers",
        icon: BuildingIcon,
        link: { to: "/$workspace/settings/customers", params },
        isAdminOnly: true,
      },
      {
        label: "Releases",
        description: "Pipelines and their stages",
        icon: RocketIcon,
        link: { to: "/$workspace/settings/releases", params },
        isAdminOnly: true,
      },
    ],
    [
      {
        label: "Integrations",
        description: "GitHub and other tools",
        icon: PlugIcon,
        link: { to: "/$workspace/settings/integrations", params },
        isAdminOnly: true,
      },
      {
        label: "Pull requests",
        description: "Move issues as pull requests change",
        icon: GitPullRequestIcon,
        link: { to: "/$workspace/settings/github", params },
      },
      {
        label: "Webhooks",
        description: "Send changes to other services",
        icon: WebhookIcon,
        link: { to: "/$workspace/settings/webhooks", params },
        isAdminOnly: true,
      },
      {
        label: "Import",
        description: "Issues from another tracker",
        icon: DownloadIcon,
        link: { to: "/$workspace/settings/import", params },
        isAdminOnly: true,
      },
      {
        label: "OAuth apps",
        description: "Your own apps that sign in with OAuth",
        icon: AppWindowIcon,
        link: { to: "/$workspace/settings/oauth-apps", params },
        isAdminOnly: true,
      },
    ],
    [
      {
        label: "Danger zone",
        description: "Leave or delete the workspace",
        icon: TriangleAlertIcon,
        link: { to: "/$workspace/settings/danger", params },
        destructive: true,
        isAdminOnly: true,
      },
    ],
  ];
}

function WorkspaceSettingsLayout() {
  const { workspace, isAdmin, canEdit } = useWorkspace();
  const switcher = <SettingsSectionSwitcher current="workspace" />;
  if (!canEdit) {
    return (
      <SettingsLayout title="Workspace" switcher={switcher} groups={[]}>
        <EmptyState
          icon={LockIcon}
          title="Members only"
          description="Guests can't change workspace settings."
        />
      </SettingsLayout>
    );
  }
  const groups = workspaceTabGroups(workspace.urlKey)
    .map((tabs) => tabs.filter((tab) => isAdmin || !tab.isAdminOnly))
    .filter((tabs) => tabs.length > 0);
  return (
    <SettingsLayout title="Workspace" switcher={switcher} groups={groups}>
      <Outlet />
    </SettingsLayout>
  );
}

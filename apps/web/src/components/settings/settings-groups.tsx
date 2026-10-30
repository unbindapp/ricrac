import { useMemo } from "react";

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
  MailIcon,
  PlugIcon,
  RefreshCwIcon,
  RocketIcon,
  RulerIcon,
  SettingsIcon,
  SparklesIcon,
  TagIcon,
  TagsIcon,
  TriangleAlertIcon,
  TriangleIcon,
  UserIcon,
  UsersIcon,
  WebhookIcon,
  WorkflowIcon,
} from "@/components/icons";
import { useManagesAssistant } from "@/components/assistant/use-assistant-available";
import { type TSettingsTab, type TSettingsTabGroup } from "@/components/layout/settings-layout";
import { useWorkspace } from "@/components/workspace/workspace-provider";

type TWorkspaceTab = TSettingsTab & { isAdminOnly?: boolean; isAssistantOnly?: boolean };

function accountTabs(workspace: string): TSettingsTab[] {
  const params = { workspace };
  return [
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
    {
      label: "Invitations",
      description: "Invite people, join other workspaces",
      icon: MailIcon,
      link: { to: "/$workspace/settings/account/invitations", params },
    },
  ];
}

function workspaceTabs(workspace: string): TWorkspaceTab[] {
  const params = { workspace };
  return [
    {
      label: "General",
      description: "Name, URL, logo and issue prefix",
      section: "workspace",
      icon: SettingsIcon,
      link: { to: "/$workspace/settings", params },
      exact: true,
    },
    {
      label: "Members",
      description: "People, roles and invitations",
      section: "workspace",
      icon: UsersIcon,
      link: { to: "/$workspace/settings/members", params },
      isAdminOnly: true,
    },
    {
      label: "Claude",
      description: "Who can ask Claude here",
      section: "workspace",
      icon: SparklesIcon,
      link: { to: "/$workspace/settings/claude", params },
      isAdminOnly: true,
      isAssistantOnly: true,
    },
    {
      label: "Workflow",
      description: "The statuses issues move through",
      section: "issues",
      icon: WorkflowIcon,
      link: { to: "/$workspace/settings/workflow", params },
    },
    {
      label: "Labels",
      description: "Issue labels and groups",
      section: "issues",
      icon: TagIcon,
      link: { to: "/$workspace/settings/labels", params },
    },
    {
      label: "Templates",
      description: "Starting points for new work",
      section: "issues",
      icon: FileStackIcon,
      link: { to: "/$workspace/settings/templates", params },
    },
    {
      label: "Estimates",
      description: "How issues are sized",
      section: "issues",
      icon: RulerIcon,
      link: { to: "/$workspace/settings/estimates", params },
    },
    {
      label: "Triage",
      description: "Review incoming issues",
      section: "issues",
      icon: TriangleIcon,
      link: { to: "/$workspace/settings/triage", params },
    },
    {
      label: "Cycles",
      description: "Schedule and automation",
      section: "issues",
      icon: RefreshCwIcon,
      link: { to: "/$workspace/settings/cycles", params },
    },
    {
      label: "Auto close & archive",
      description: "Keep the backlog tidy",
      section: "issues",
      icon: ArchiveIcon,
      link: { to: "/$workspace/settings/auto-archive", params },
    },
    {
      label: "Project statuses",
      description: "What a project moves through",
      section: "projects",
      icon: BoxesIcon,
      link: { to: "/$workspace/settings/project-statuses", params },
      isAdminOnly: true,
    },
    {
      label: "Project labels",
      description: "Labels for projects",
      section: "projects",
      icon: TagsIcon,
      link: { to: "/$workspace/settings/project-labels", params },
      isAdminOnly: true,
    },
    {
      label: "Customers",
      description: "Customer statuses and tiers",
      section: "projects",
      icon: BuildingIcon,
      link: { to: "/$workspace/settings/customers", params },
      isAdminOnly: true,
    },
    {
      label: "Releases",
      description: "Pipelines and their stages",
      section: "projects",
      icon: RocketIcon,
      link: { to: "/$workspace/settings/releases", params },
      isAdminOnly: true,
    },
    {
      label: "Integrations",
      description: "GitHub and other tools",
      section: "connections",
      icon: PlugIcon,
      link: { to: "/$workspace/settings/integrations", params },
      isAdminOnly: true,
    },
    {
      label: "Pull requests",
      description: "Move issues as pull requests change",
      section: "connections",
      icon: GitPullRequestIcon,
      link: { to: "/$workspace/settings/github", params },
    },
    {
      label: "Webhooks",
      description: "Send changes to other services",
      section: "connections",
      icon: WebhookIcon,
      link: { to: "/$workspace/settings/webhooks", params },
      isAdminOnly: true,
    },
    {
      label: "Import",
      description: "Issues from another tracker",
      section: "connections",
      icon: DownloadIcon,
      link: { to: "/$workspace/settings/import", params },
      isAdminOnly: true,
    },
    {
      label: "OAuth apps",
      description: "Your own apps that sign in with OAuth",
      section: "connections",
      icon: AppWindowIcon,
      link: { to: "/$workspace/settings/oauth-apps", params },
      isAdminOnly: true,
    },
    {
      label: "Danger zone",
      description: "Leave or delete the workspace",
      section: "danger",
      destructive: true,
      icon: TriangleAlertIcon,
      link: { to: "/$workspace/settings/danger", params },
      isAdminOnly: true,
    },
  ];
}

/** The settings tabs the caller may open, in their groups. */
export function useSettingsGroups() {
  const { workspace, canEdit, isAdmin } = useWorkspace();
  const managesAssistant = useManagesAssistant();
  return useMemo(() => {
    const groups: TSettingsTabGroup[] = [
      { title: "Account", icon: UserIcon, tabs: accountTabs(workspace.urlKey) },
    ];
    if (!canEdit) return groups;
    groups.push({
      title: "Workspace",
      icon: BuildingIcon,
      tabs: workspaceTabs(workspace.urlKey).filter(
        (tab) => (isAdmin || !tab.isAdminOnly) && (managesAssistant || !tab.isAssistantOnly),
      ),
    });
    return groups;
  }, [workspace.urlKey, canEdit, isAdmin, managesAssistant]);
}

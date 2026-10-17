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
} from "lucide-react";

import { EmptyState } from "@/components/layout/empty-state";
import { PageContainer } from "@/components/layout/page-container";
import { SettingsLayout, type TSettingsTab } from "@/components/layout/settings-layout";
import { useWorkspace } from "@/components/workspace/workspace-provider";

export const Route = createFileRoute("/$workspace/_workspace/settings/_admin")({
  component: WorkspaceSettingsLayout,
});

function WorkspaceSettingsLayout() {
  const { workspace, isAdmin } = useWorkspace();
  if (!isAdmin) {
    return (
      <PageContainer width="narrow">
        <EmptyState
          icon={LockIcon}
          title="Admins only"
          description="Ask a workspace admin to change these settings."
        />
      </PageContainer>
    );
  }
  const params = { workspace: workspace.urlKey };
  const tabs: TSettingsTab[] = [
    {
      label: "General",
      icon: SettingsIcon,
      link: { to: "/$workspace/settings", params },
      exact: true,
    },
    { label: "Members", icon: UsersIcon, link: { to: "/$workspace/settings/members", params } },
    { label: "Teams", icon: UsersRoundIcon, link: { to: "/$workspace/settings/teams", params } },
    { label: "Labels", icon: TagIcon, link: { to: "/$workspace/settings/labels", params } },
    {
      label: "Project statuses",
      icon: BoxesIcon,
      link: { to: "/$workspace/settings/project-statuses", params },
    },
    {
      label: "Project labels",
      icon: TagsIcon,
      link: { to: "/$workspace/settings/project-labels", params },
    },
    {
      label: "Templates",
      icon: FileStackIcon,
      link: { to: "/$workspace/settings/templates", params },
    },
    {
      label: "Customers",
      icon: BuildingIcon,
      link: { to: "/$workspace/settings/customers", params },
    },
    {
      label: "Releases",
      icon: RocketIcon,
      link: { to: "/$workspace/settings/releases", params },
    },
    {
      label: "Integrations",
      icon: PlugIcon,
      link: { to: "/$workspace/settings/integrations", params },
    },
    { label: "Webhooks", icon: WebhookIcon, link: { to: "/$workspace/settings/webhooks", params } },
    { label: "Import", icon: DownloadIcon, link: { to: "/$workspace/settings/import", params } },
    {
      label: "OAuth apps",
      icon: AppWindowIcon,
      link: { to: "/$workspace/settings/oauth-apps", params },
    },
    {
      label: "Danger zone",
      icon: TriangleAlertIcon,
      link: { to: "/$workspace/settings/danger", params },
    },
  ];
  return (
    <SettingsLayout title="Workspace" tabs={tabs}>
      <Outlet />
    </SettingsLayout>
  );
}

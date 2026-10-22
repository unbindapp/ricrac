import { createFileRoute, Outlet } from "@tanstack/react-router";
import type { ReactNode } from "react";

import {
  ArchiveIcon,
  FileStackIcon,
  GitPullRequestIcon,
  LockIcon,
  RefreshCwIcon,
  RulerIcon,
  SettingsIcon,
  TagIcon,
  TriangleAlertIcon,
  TriangleIcon,
  UsersIcon,
  WorkflowIcon,
} from "@/components/icons/symbols.gen";
import { EmptyState } from "@/components/layout/empty-state";
import { ErrorCard } from "@/components/layout/error-card";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { SettingsLayout, type TSettingsTab } from "@/components/layout/settings-layout";
import { SettingsSectionSwitcher } from "@/components/settings/settings-section-switcher";
import { useTeams } from "@/components/workspace/teams-provider";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { EntityNotFound } from "@/components/navigation/not-found-template";

export const Route = createFileRoute("/$workspace/_workspace/settings/team/$key")({
  component: TeamSettingsLayout,
});

function TeamSettingsLayout() {
  const { key } = Route.useParams();
  const { workspace, canEdit } = useWorkspace();
  const { teamByKey, isPending, error, refetch } = useTeams();
  const team = teamByKey(key);

  if (!canEdit) {
    return (
      <TeamSettingsNotice teamKey={key}>
        <EmptyState
          icon={LockIcon}
          title="Members only"
          description="Guests can't change team settings."
        />
      </TeamSettingsNotice>
    );
  }
  if (isPending) {
    return (
      <TeamSettingsNotice teamKey={key}>
        <ListSkeleton />
      </TeamSettingsNotice>
    );
  }
  if (error) {
    return (
      <TeamSettingsNotice teamKey={key}>
        <ErrorCard message={error.message} onRetry={refetch} />
      </TeamSettingsNotice>
    );
  }
  if (!team) {
    return (
      <TeamSettingsNotice teamKey={key}>
        <EntityNotFound
          Icon={UsersIcon}
          description="There is no team with this key in this workspace."
        />
      </TeamSettingsNotice>
    );
  }

  const params = { workspace: workspace.urlKey, key: team.key };
  const groups: TSettingsTab[][] = [
    [
      {
        label: "General",
        description: "Name, key and visibility",
        icon: SettingsIcon,
        link: { to: "/$workspace/settings/team/$key", params },
        exact: true,
      },
      {
        label: "Workflow",
        description: "The statuses issues move through",
        icon: WorkflowIcon,
        link: { to: "/$workspace/settings/team/$key/workflow", params },
      },
      {
        label: "Cycles",
        description: "Schedule and automation",
        icon: RefreshCwIcon,
        link: { to: "/$workspace/settings/team/$key/cycles", params },
      },
      {
        label: "Estimates",
        description: "How issues are sized",
        icon: RulerIcon,
        link: { to: "/$workspace/settings/team/$key/estimates", params },
      },
      {
        label: "Triage",
        description: "Review incoming issues",
        icon: TriangleIcon,
        link: { to: "/$workspace/settings/team/$key/triage", params },
      },
    ],
    [
      {
        label: "Members",
        description: "People in this team",
        icon: UsersIcon,
        link: { to: "/$workspace/settings/team/$key/members", params },
      },
      {
        label: "Labels",
        description: "Labels of this team",
        icon: TagIcon,
        link: { to: "/$workspace/settings/team/$key/labels", params },
      },
      {
        label: "Templates",
        description: "Starting points for new issues",
        icon: FileStackIcon,
        link: { to: "/$workspace/settings/team/$key/templates", params },
      },
    ],
    [
      {
        label: "GitHub",
        description: "Pull request automation",
        icon: GitPullRequestIcon,
        link: { to: "/$workspace/settings/team/$key/github", params },
      },
      {
        label: "Auto close & archive",
        description: "Keep the backlog tidy",
        icon: ArchiveIcon,
        link: { to: "/$workspace/settings/team/$key/auto-archive", params },
      },
    ],
    [
      {
        label: "Danger zone",
        description: "Leave or delete the team",
        icon: TriangleAlertIcon,
        link: { to: "/$workspace/settings/team/$key/danger", params },
        destructive: true,
      },
    ],
  ];
  return (
    <SettingsLayout
      title={team.name}
      switcher={<SettingsSectionSwitcher current={{ kind: "team", key: team.key }} />}
      groups={groups}
    >
      <Outlet />
    </SettingsLayout>
  );
}

function TeamSettingsNotice({ teamKey, children }: { teamKey: string; children: ReactNode }) {
  return (
    <SettingsLayout
      title={teamKey}
      switcher={<SettingsSectionSwitcher current={{ kind: "team", key: teamKey }} />}
      groups={[]}
    >
      {children}
    </SettingsLayout>
  );
}

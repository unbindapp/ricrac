import { createFileRoute } from "@tanstack/react-router";

import { LabelManager } from "@/components/labels/label-manager";
import { SettingsPage } from "@/components/layout/settings-page";
import { useRequiredTeamFromRoute } from "@/components/workspace/use-team-from-route";
import { useWorkspace } from "@/components/workspace/workspace-provider";

export const Route = createFileRoute("/$workspace/_workspace/settings/team/$key/labels")({
  component: TeamLabelsSettingsPage,
});

function TeamLabelsSettingsPage() {
  const { canEdit } = useWorkspace();
  const team = useRequiredTeamFromRoute();
  return (
    <SettingsPage
      title="Labels"
      description={`Labels only ${team.name} uses, next to the workspace labels every team shares.`}
    >
      <LabelManager team={team} canEdit={canEdit} title={`${team.name} labels`} />
    </SettingsPage>
  );
}

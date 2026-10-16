import { createFileRoute } from "@tanstack/react-router";

import { SettingsPage } from "@/components/layout/settings-page";
import { TemplateManager } from "@/components/templates/template-manager";
import { useRequiredTeamFromRoute } from "@/components/workspace/use-team-from-route";
import { useWorkspace } from "@/components/workspace/workspace-provider";

export const Route = createFileRoute("/$workspace/_workspace/settings/team/$key/templates")({
  component: TeamTemplatesSettingsPage,
});

function TeamTemplatesSettingsPage() {
  const { canEdit } = useWorkspace();
  const team = useRequiredTeamFromRoute();
  return (
    <SettingsPage
      title="Templates"
      description={`Templates only ${team.name} uses. Templates for every team are in workspace settings.`}
    >
      <TemplateManager team={team} canEdit={canEdit} />
    </SettingsPage>
  );
}

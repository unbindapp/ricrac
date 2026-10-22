import { createFileRoute } from "@tanstack/react-router";

import { TagIcon } from "@/components/icons/symbols.gen";
import { LabelManager } from "@/components/labels/label-manager";
import { PageContainer } from "@/components/layout/page-container";
import { PageHeader } from "@/components/layout/page-header";
import { teamCrumb } from "@/components/workspace/team-crumb";
import { useRequiredTeamFromRoute } from "@/components/workspace/use-team-from-route";
import { useWorkspace } from "@/components/workspace/workspace-provider";

export const Route = createFileRoute("/$workspace/_workspace/team/$key/labels")({
  component: TeamLabelsPage,
});

function TeamLabelsPage() {
  const { workspace, canEdit } = useWorkspace();
  const team = useRequiredTeamFromRoute();
  return (
    <PageContainer width="narrow">
      <PageHeader
        title="Labels"
        icon={<TagIcon />}
        breadcrumb={[teamCrumb(team, workspace.urlKey), { label: "Labels" }]}
      />
      <LabelManager team={team} canEdit={canEdit} title={`${team.name} labels`} />
      <LabelManager
        team={null}
        canEdit={canEdit}
        title="Workspace labels"
        description="Shared by every team."
      />
    </PageContainer>
  );
}

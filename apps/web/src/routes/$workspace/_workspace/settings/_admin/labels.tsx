import { createFileRoute } from "@tanstack/react-router";

import { LabelManager } from "@/components/labels/label-manager";
import { SettingsPage } from "@/components/layout/settings-page";
import { useWorkspace } from "@/components/workspace/workspace-provider";

export const Route = createFileRoute("/$workspace/_workspace/settings/_admin/labels")({
  component: LabelsSettingsPage,
});

function LabelsSettingsPage() {
  const { canEdit } = useWorkspace();
  return (
    <SettingsPage
      title="Labels"
      description="Workspace labels are shared by every team. Teams add their own in team settings."
    >
      <LabelManager team={null} canEdit={canEdit} title="Workspace labels" />
    </SettingsPage>
  );
}

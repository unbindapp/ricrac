import { createFileRoute } from "@tanstack/react-router";

import { LabelManager } from "@/components/labels/label-manager";
import { SettingsPage } from "@/components/layout/settings-page";
import { useWorkspace } from "@/components/workspace/workspace-provider";

export const Route = createFileRoute("/$workspace/_workspace/settings/_workspace-settings/labels")({
  component: LabelsSettingsPage,
});

function LabelsSettingsPage() {
  const { canEdit } = useWorkspace();
  return (
    <SettingsPage title="Labels" description="Labels and groups that sort issues.">
      <LabelManager canEdit={canEdit} title="Issue labels" />
    </SettingsPage>
  );
}

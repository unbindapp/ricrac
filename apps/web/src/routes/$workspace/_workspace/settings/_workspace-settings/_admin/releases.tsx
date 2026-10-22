import { createFileRoute } from "@tanstack/react-router";

import { SettingsPage } from "@/components/layout/settings-page";
import { NewPipelineButton, PipelineSettings } from "@/components/releases/pipeline-settings";

export const Route = createFileRoute(
  "/$workspace/_workspace/settings/_workspace-settings/_admin/releases",
)({
  component: ReleaseSettingsPage,
});

function ReleaseSettingsPage() {
  return (
    <SettingsPage
      title="Releases"
      description="Pipelines and the stages their releases move through. Drag stages to reorder them."
      actions={<NewPipelineButton />}
    >
      <PipelineSettings />
    </SettingsPage>
  );
}

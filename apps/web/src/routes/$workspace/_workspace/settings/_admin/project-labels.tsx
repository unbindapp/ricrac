import { createFileRoute } from "@tanstack/react-router";

import { SettingsPage } from "@/components/layout/settings-page";
import { ProjectLabelManager } from "@/components/projects/project-label-manager";

export const Route = createFileRoute("/$workspace/_workspace/settings/_admin/project-labels")({
  component: ProjectLabelsSettingsPage,
});

function ProjectLabelsSettingsPage() {
  return (
    <SettingsPage
      title="Project labels"
      description="Labels for projects, apart from issue labels."
    >
      <ProjectLabelManager />
    </SettingsPage>
  );
}

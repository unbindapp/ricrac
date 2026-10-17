import { createFileRoute } from "@tanstack/react-router";

import { SettingsPage } from "@/components/layout/settings-page";
import { TemplateManager } from "@/components/templates/template-manager";
import { useWorkspace } from "@/components/workspace/workspace-provider";

export const Route = createFileRoute("/$workspace/_workspace/settings/_admin/templates")({
  component: TemplatesSettingsPage,
});

function TemplatesSettingsPage() {
  const { canEdit } = useWorkspace();
  return (
    <SettingsPage
      title="Templates"
      description="What new issues, projects and documents start from, for every team or one."
    >
      <TemplateManager team={null} canEdit={canEdit} />
    </SettingsPage>
  );
}

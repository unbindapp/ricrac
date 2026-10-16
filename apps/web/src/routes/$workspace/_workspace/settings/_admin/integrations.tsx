import { createFileRoute } from "@tanstack/react-router";

import { GithubErrorNotice } from "@/components/integrations/github-error-notice";
import { GithubSettings } from "@/components/integrations/github-settings";
import { SettingsPage } from "@/components/layout/settings-page";

export const Route = createFileRoute("/$workspace/_workspace/settings/_admin/integrations")({
  component: IntegrationsSettingsPage,
});

function IntegrationsSettingsPage() {
  return (
    <SettingsPage title="Integrations" description="Connect Ricrac to the tools your team uses.">
      <GithubErrorNotice title="GitHub isn't connected" />
      <GithubSettings />
    </SettingsPage>
  );
}

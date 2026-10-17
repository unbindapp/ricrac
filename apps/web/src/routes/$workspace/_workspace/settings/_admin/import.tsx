import { createFileRoute } from "@tanstack/react-router";

import { ImportFormat } from "@/components/import/import-format";
import { IssueImport } from "@/components/import/issue-import";
import { SettingsPage } from "@/components/layout/settings-page";

export const Route = createFileRoute("/$workspace/_workspace/settings/_admin/import")({
  component: ImportSettingsPage,
});

function ImportSettingsPage() {
  return (
    <SettingsPage
      title="Import"
      description="Move issues from another tracker into a team, as a JSON file."
    >
      <IssueImport />
      <ImportFormat />
    </SettingsPage>
  );
}

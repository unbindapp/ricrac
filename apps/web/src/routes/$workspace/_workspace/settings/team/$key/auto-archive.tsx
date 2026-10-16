import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";

import { SettingsPage } from "@/components/layout/settings-page";
import { SettingsSection } from "@/components/layout/settings-section";
import { useTeamDetailFromRoute } from "@/components/workspace/use-team-from-route";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { submitHandler, useAppForm } from "@/lib/hooks/use-app-form";
import { teamStatusesQuery, useSaveTeamMutation } from "@/lib/queries/teams";

export const Route = createFileRoute("/$workspace/_workspace/settings/team/$key/auto-archive")({
  component: TeamAutoArchiveSettingsPage,
});

const NEVER = "never";
const monthItems = [
  { value: NEVER, label: "Never" },
  ...[1, 3, 6, 9, 12].map((n) => ({
    value: String(n),
    label: `After ${n} ${n === 1 ? "month" : "months"}`,
  })),
];

const toMonths = (value: string) => (value === NEVER ? null : Number(value));
const fromMonths = (months: number | null) => (months === null ? NEVER : String(months));

function TeamAutoArchiveSettingsPage() {
  const team = useTeamDetailFromRoute();
  const { workspace } = useWorkspace();
  const { data: statuses = [] } = useQuery(teamStatusesQuery(workspace.id, team.id));
  const { mutateAsync: saveTeam } = useSaveTeamMutation(workspace.id);
  const canceledItems = statuses
    .filter((status) => status.type === "canceled")
    .map((status) => ({ value: status.id, label: status.name }));

  const form = useAppForm({
    defaultValues: {
      autoCloseMonths: fromMonths(team.autoCloseMonths),
      autoCloseState: team.autoCloseState?.id ?? "",
      autoArchiveMonths: fromMonths(team.autoArchiveMonths),
      autoCloseParentIssues: team.autoCloseParentIssues,
      autoCloseChildIssues: team.autoCloseChildIssues,
    },
    onSubmit: async ({ value, formApi }) => {
      await saveTeam({
        id: team.id,
        autoCloseMonths: toMonths(value.autoCloseMonths),
        autoCloseState: value.autoCloseState || null,
        autoArchiveMonths: toMonths(value.autoArchiveMonths),
        autoCloseParentIssues: value.autoCloseParentIssues,
        autoCloseChildIssues: value.autoCloseChildIssues,
      });
      formApi.reset(value);
    },
  });

  return (
    <SettingsPage title="Auto close & archive" description="Keep the backlog and history tidy.">
      <SettingsSection
        title="Automation"
        onSubmit={submitHandler(form)}
        footer={
          <form.AppForm>
            <form.SubmitButton size="sm" requireDirty>
              Save
            </form.SubmitButton>
          </form.AppForm>
        }
      >
        <form.AppField
          name="autoCloseMonths"
          children={(field) => (
            <field.SelectField
              label="Auto close"
              description="Close issues that haven't been updated for this long."
              items={monthItems}
            />
          )}
        />
        <form.AppField
          name="autoCloseState"
          children={(field) => (
            <field.SelectField
              label="Closed status"
              description="The canceled status auto-closed issues move to."
              placeholder="First canceled status"
              items={canceledItems}
            />
          )}
        />
        <form.AppField
          name="autoArchiveMonths"
          children={(field) => (
            <field.SelectField
              label="Auto archive"
              description="Archive completed and canceled issues after this long."
              items={monthItems}
            />
          )}
        />
        <form.AppField
          name="autoCloseParentIssues"
          children={(field) => (
            <field.SwitchField
              label="Complete parents"
              description="A parent issue is completed when all its sub-issues are done."
            />
          )}
        />
        <form.AppField
          name="autoCloseChildIssues"
          children={(field) => (
            <field.SwitchField
              label="Close sub-issues"
              description="Sub-issues are closed when their parent is closed."
            />
          )}
        />
      </SettingsSection>
    </SettingsPage>
  );
}

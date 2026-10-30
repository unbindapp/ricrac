import { useQuery } from "@tanstack/react-query";

import type { TeamDetail } from "@/api/ops.gen";
import { ErrorCard } from "@/components/layout/error-card";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { SettingsSection } from "@/components/layout/settings-section";
import { useWorkspaceTeam } from "@/components/workspace/teams-provider";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { submitHandler, useAppForm } from "@/lib/hooks/use-app-form";
import { teamQuery, useSaveTeamMutation } from "@/lib/queries/teams";

/** How the team's issues and synced repositories meet: new issues on GitHub, issue fields. */
export function GithubTeamSettings() {
  const { workspace } = useWorkspace();
  const teamId = useWorkspaceTeam()?.id ?? "";
  const { data, isPending, error, refetch } = useQuery({
    ...teamQuery(workspace.id, teamId),
    enabled: Boolean(teamId),
  });
  if (isPending) return <ListSkeleton rows={2} />;
  if (error) return <ErrorCard message={error.message} onRetry={() => void refetch()} />;
  return <GithubTeamForm team={data} />;
}

function GithubTeamForm({ team }: { team: TeamDetail }) {
  const { workspace } = useWorkspace();
  const { mutateAsync: saveTeam } = useSaveTeamMutation(workspace.id);
  const form = useAppForm({
    defaultValues: {
      githubOpenIssues: team.githubOpenIssues,
      githubSyncFields: team.githubSyncFields,
    },
    onSubmit: async ({ value, formApi }) => {
      await saveTeam({ id: team.id, ...value });
      formApi.reset(value);
    },
  });
  return (
    <SettingsSection
      title="Issues on GitHub"
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
        name="githubOpenIssues"
        children={(field) => (
          <field.SwitchField
            label="Open new issues on GitHub"
            description="New issues open in a synced repository too, as their author. Each issue can still pick another repository or stay off GitHub."
          />
        )}
      />
      <form.AppField
        name="githubSyncFields"
        children={(field) => (
          <field.SwitchField
            label="Sync priority, estimate, due date and cycle with GitHub issue fields"
            description="Uses the organization's issue fields named Priority, Estimate, Due date (or Target date) and Cycle."
          />
        )}
      />
    </SettingsSection>
  );
}

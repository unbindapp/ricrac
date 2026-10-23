import { useQuery } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";

import type { TeamDetail } from "@/api/ops.gen";
import { SettingsPage } from "@/components/layout/settings-page";
import { SettingsSection } from "@/components/layout/settings-section";
import { StateIcon } from "@/components/shared/state-icon";
import { useSettingsTeamDetail } from "@/components/settings/use-settings-team";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { submitHandler, type TSelectFieldItem, useAppForm } from "@/lib/hooks/use-app-form";
import { githubIntegrationQuery } from "@/lib/queries/github";
import { useSaveTeamMutation } from "@/lib/queries/teams";

export const Route = createFileRoute("/$workspace/_workspace/settings/_workspace-settings/github")({
  component: PullRequestSettingsPage,
});

const DEFAULT_STATE = "default";

function stateItems(team: TeamDetail, fallbackType: "started" | "completed"): TSelectFieldItem[] {
  const fallback = team.states.find((state) => state.type === fallbackType);
  return [
    {
      value: DEFAULT_STATE,
      label: fallback ? `Default (${fallback.name})` : "Default (no change)",
    },
    ...team.states.map((state) => ({
      value: state.id,
      label: state.name,
      icon: <StateIcon type={state.type} color={state.color} name={state.name} />,
    })),
  ];
}

function PullRequestSettingsPage() {
  const team = useSettingsTeamDetail();
  const { workspace } = useWorkspace();
  const { mutateAsync: saveTeam } = useSaveTeamMutation(workspace.id);
  const toInput = (value: string) => (value === DEFAULT_STATE ? null : value);
  const form = useAppForm({
    defaultValues: {
      opened: team.githubPrOpenedState?.id ?? DEFAULT_STATE,
      merged: team.githubPrMergedState?.id ?? DEFAULT_STATE,
    },
    onSubmit: async ({ value, formApi }) => {
      await saveTeam({
        id: team.id,
        githubPrOpenedState: toInput(value.opened),
        githubPrMergedState: toInput(value.merged),
      });
      formApi.reset(value);
    },
  });

  return (
    <SettingsPage title="Pull requests" description="Move issues when their pull requests change.">
      <SettingsSection
        title="Pull request automation"
        description="An opened pull request or a pushed branch that mentions an issue moves it when nobody started it yet. A merged pull request that fixes, closes or resolves an issue moves it unless it is completed or canceled."
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
          name="opened"
          children={(field) => (
            <field.SelectField
              label="Pull request opened"
              description={`For issues in triage, backlog or unstarted statuses, when a pull request or a branch like ${team.key.toLowerCase()}-123 mentions them.`}
              items={stateItems(team, "started")}
            />
          )}
        />
        <form.AppField
          name="merged"
          children={(field) => (
            <field.SelectField
              label="Pull request merged"
              description={`For issues the pull request fixes, like "Fixes ${team.key}-123", or a commit on the default branch does.`}
              items={stateItems(team, "completed")}
            />
          )}
        />
      </SettingsSection>
      <ConnectionHint />
    </SettingsPage>
  );
}

function ConnectionHint() {
  const { workspace, isAdmin } = useWorkspace();
  const { data } = useQuery(githubIntegrationQuery(workspace.id));
  if (!data || data.installations.length > 0) return null;
  const text = data.configured
    ? "GitHub isn't connected to this workspace yet, so nothing moves issues until it is."
    : "This server has no GitHub App set up, so nothing moves issues until it has.";
  return (
    <p className="text-muted-foreground text-sm">
      {text}{" "}
      {isAdmin && data.configured && (
        <Link
          to="/$workspace/settings/integrations"
          params={{ workspace: workspace.urlKey }}
          className="text-foreground dank:text-m3-primary underline-offset-4 has-hover:hover:underline"
        >
          Connect GitHub
        </Link>
      )}
    </p>
  );
}

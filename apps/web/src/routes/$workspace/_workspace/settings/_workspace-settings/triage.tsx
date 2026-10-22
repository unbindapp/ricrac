import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";

import { SettingsPage } from "@/components/layout/settings-page";
import { SettingsSection } from "@/components/layout/settings-section";
import { Avatar } from "@/components/shared/avatar";
import { CheckboxList } from "@/components/shared/checkbox-list";
import { useSettingsTeamDetail } from "@/components/settings/use-settings-team";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { submitHandler, useAppForm } from "@/lib/hooks/use-app-form";
import { useSaveTeamMutation } from "@/lib/queries/teams";
import { usersQuery } from "@/lib/queries/users";

export const Route = createFileRoute("/$workspace/_workspace/settings/_workspace-settings/triage")({
  component: TriageSettingsPage,
});

const actionItems = [
  { value: "none", label: "Nothing" },
  { value: "notify", label: "Notify the responsible people" },
  { value: "assign", label: "Assign to the person on duty" },
];

const NO_ROTATION = "none";
const rotationItems = [
  { value: NO_ROTATION, label: "Everyone at once" },
  ...[1, 2, 3, 4].map((n) => ({
    value: String(n),
    label: `Take turns, ${n} ${n === 1 ? "week" : "weeks"} each`,
  })),
];

function TriageSettingsPage() {
  const team = useSettingsTeamDetail();
  const { workspace } = useWorkspace();
  const { data: users = [] } = useQuery(usersQuery(workspace.id));
  const { mutateAsync: saveTeam } = useSaveTeamMutation(workspace.id);
  const form = useAppForm({
    defaultValues: {
      triageEnabled: team.triageEnabled,
      requirePriorityToLeaveTriage: team.requirePriorityToLeaveTriage,
      triageAction: team.triageAction as string,
      triageResponsible: team.triageResponsible.map((user) => user.id),
      triageRotationWeeks: team.triageRotationWeeks
        ? String(team.triageRotationWeeks)
        : NO_ROTATION,
    },
    onSubmit: async ({ value, formApi }) => {
      await saveTeam({
        id: team.id,
        triageEnabled: value.triageEnabled,
        requirePriorityToLeaveTriage: value.requirePriorityToLeaveTriage,
        triageAction: value.triageAction,
        triageResponsible: value.triageResponsible,
        triageRotationWeeks:
          value.triageRotationWeeks === NO_ROTATION ? null : Number(value.triageRotationWeeks),
      });
      formApi.reset(value);
    },
  });

  return (
    <SettingsPage
      title="Triage"
      description="Review incoming issues before they reach the backlog."
    >
      <SettingsSection
        title="Triage"
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
          name="triageEnabled"
          children={(field) => (
            <field.SwitchField
              label="Enable triage"
              description="Issues from integrations land in Triage first. Turning it off moves issues in triage to the default status."
            />
          )}
        />
        <form.Subscribe selector={(state) => state.values.triageEnabled}>
          {(enabled) => (
            <>
              <form.AppField
                name="requirePriorityToLeaveTriage"
                children={(field) => (
                  <field.SwitchField
                    label="Require a priority"
                    description="Issues need a priority before they leave triage."
                    disabled={!enabled}
                  />
                )}
              />
              <form.AppField
                name="triageAction"
                children={(field) => (
                  <field.SelectField
                    label="When an issue arrives"
                    items={actionItems}
                    disabled={!enabled}
                  />
                )}
              />
              <form.Field
                name="triageResponsible"
                children={(field) => (
                  <div className="flex flex-col gap-1.5">
                    <p className="text-sm font-medium">Responsible</p>
                    <CheckboxList
                      label="Responsible"
                      emptyText="There's nobody in the workspace yet."
                      items={users.map((user) => ({
                        value: user.id,
                        label: user.name,
                        description: user.email,
                        icon: (
                          <Avatar id={user.id} name={user.name} src={user.avatarUrl} size="xs" />
                        ),
                      }))}
                      value={field.state.value}
                      onChange={field.handleChange}
                    />
                  </div>
                )}
              />
              <form.AppField
                name="triageRotationWeeks"
                children={(field) => (
                  <field.SelectField
                    label="Rotation"
                    description="With a rotation, the responsible people take turns in the order you picked them."
                    items={rotationItems}
                    disabled={!enabled}
                  />
                )}
              />
            </>
          )}
        </form.Subscribe>
      </SettingsSection>
    </SettingsPage>
  );
}

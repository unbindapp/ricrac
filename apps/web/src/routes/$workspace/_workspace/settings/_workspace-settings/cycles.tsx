import { createFileRoute } from "@tanstack/react-router";

import { SettingsPage } from "@/components/layout/settings-page";
import { SettingsSection } from "@/components/layout/settings-section";
import { useSettingsTeamDetail } from "@/components/settings/use-settings-team";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { submitHandler, useAppForm } from "@/lib/hooks/use-app-form";
import { useSaveTeamMutation } from "@/lib/queries/teams";

export const Route = createFileRoute("/$workspace/_workspace/settings/_workspace-settings/cycles")({
  component: CyclesSettingsPage,
});

const numbered = (from: number, to: number, label: (n: number) => string) =>
  Array.from({ length: to - from + 1 }, (_, i) => from + i).map((n) => ({
    value: String(n),
    label: label(n),
  }));

const weeks = (n: number) => `${n} ${n === 1 ? "week" : "weeks"}`;
const durationItems = numbered(1, 8, weeks);
const cooldownItems = numbered(0, 4, (n) => (n === 0 ? "No cooldown" : weeks(n)));
const upcomingItems = numbered(1, 8, (n) => `${n} ${n === 1 ? "cycle" : "cycles"}`);
const dayItems = ["Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"].map(
  (label, index) => ({ value: String((index + 1) % 7), label }),
);

function CyclesSettingsPage() {
  const team = useSettingsTeamDetail();
  const { workspace } = useWorkspace();
  const { mutateAsync: saveTeam } = useSaveTeamMutation(workspace.id);

  const form = useAppForm({
    defaultValues: {
      cyclesEnabled: team.cyclesEnabled,
      cycleDurationWeeks: String(team.cycleDurationWeeks),
      cycleCooldownWeeks: String(team.cycleCooldownWeeks),
      cycleStartDay: String(team.cycleStartDay),
      upcomingCycleCount: String(team.upcomingCycleCount),
      cycleRollover: team.cycleRollover,
      cycleAutoAddStarted: team.cycleAutoAddStarted,
      cycleAutoAddCompleted: team.cycleAutoAddCompleted,
    },
    onSubmit: async ({ value, formApi }) => {
      await saveTeam({
        id: team.id,
        cyclesEnabled: value.cyclesEnabled,
        cycleDurationWeeks: Number(value.cycleDurationWeeks),
        cycleCooldownWeeks: Number(value.cycleCooldownWeeks),
        cycleStartDay: Number(value.cycleStartDay),
        upcomingCycleCount: Number(value.upcomingCycleCount),
        cycleRollover: value.cycleRollover,
        cycleAutoAddStarted: value.cycleAutoAddStarted,
        cycleAutoAddCompleted: value.cycleAutoAddCompleted,
      });
      formApi.reset(value);
    },
  });

  return (
    <SettingsPage title="Cycles" description="Time boxed iterations.">
      <SettingsSection
        title="Cycles"
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
          name="cyclesEnabled"
          children={(field) => (
            <field.SwitchField
              label="Enable cycles"
              description="New cycles are created automatically on the schedule below."
            />
          )}
        />
        <form.Subscribe selector={(state) => state.values.cyclesEnabled}>
          {(enabled) => (
            <>
              <form.AppField
                name="cycleDurationWeeks"
                children={(field) => (
                  <field.SelectField label="Duration" items={durationItems} disabled={!enabled} />
                )}
              />
              <form.AppField
                name="cycleCooldownWeeks"
                children={(field) => (
                  <field.SelectField
                    label="Cooldown"
                    description="A break between cycles."
                    items={cooldownItems}
                    disabled={!enabled}
                  />
                )}
              />
              <form.AppField
                name="cycleStartDay"
                children={(field) => (
                  <field.SelectField label="Start day" items={dayItems} disabled={!enabled} />
                )}
              />
              <form.AppField
                name="upcomingCycleCount"
                children={(field) => (
                  <field.SelectField
                    label="Upcoming cycles"
                    description="How many cycles are planned ahead."
                    items={upcomingItems}
                    disabled={!enabled}
                  />
                )}
              />
              <form.AppField
                name="cycleRollover"
                children={(field) => (
                  <field.SwitchField
                    label="Move unfinished issues"
                    description="Issues that are not done move to the next cycle when a cycle ends."
                    disabled={!enabled}
                  />
                )}
              />
              <form.AppField
                name="cycleAutoAddStarted"
                children={(field) => (
                  <field.SwitchField
                    label="Add started issues"
                    description="An issue that moves to a started status joins the current cycle."
                    disabled={!enabled}
                  />
                )}
              />
              <form.AppField
                name="cycleAutoAddCompleted"
                children={(field) => (
                  <field.SwitchField
                    label="Add completed issues"
                    description="An issue completed outside a cycle joins the current one."
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

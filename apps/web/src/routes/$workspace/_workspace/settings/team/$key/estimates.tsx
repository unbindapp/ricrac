import { createFileRoute } from "@tanstack/react-router";

import { SettingsPage } from "@/components/layout/settings-page";
import { SettingsSection } from "@/components/layout/settings-section";
import { EstimateBadge } from "@/components/shared/estimate-badge";
import { useTeamDetailFromRoute } from "@/components/workspace/use-team-from-route";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { estimateOptions, estimateTypeLabels, type TEstimateType } from "@/lib/helpers/estimates";
import { submitHandler, useAppForm } from "@/lib/hooks/use-app-form";
import { useSaveTeamMutation } from "@/lib/queries/teams";

export const Route = createFileRoute("/$workspace/_workspace/settings/team/$key/estimates")({
  component: TeamEstimatesSettingsPage,
});

const typeItems = (Object.keys(estimateTypeLabels) as TEstimateType[]).map((type) => ({
  value: type,
  label: estimateTypeLabels[type],
}));

function TeamEstimatesSettingsPage() {
  const team = useTeamDetailFromRoute();
  const { workspace } = useWorkspace();
  const { mutateAsync: saveTeam } = useSaveTeamMutation(workspace.id);

  const form = useAppForm({
    defaultValues: {
      estimateType: team.estimateType,
      estimateAllowZero: team.estimateAllowZero,
      estimateExtended: team.estimateExtended,
    },
    onSubmit: async ({ value, formApi }) => {
      await saveTeam({ id: team.id, ...value });
      formApi.reset(value);
    },
  });

  return (
    <SettingsPage title="Estimates" description="How this team sizes its issues.">
      <SettingsSection
        title="Estimates"
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
          name="estimateType"
          children={(field) => <field.SelectField label="Scale" items={typeItems} />}
        />
        <form.Subscribe selector={(state) => state.values}>
          {(values) => (
            <>
              <form.AppField
                name="estimateAllowZero"
                children={(field) => (
                  <field.SwitchField
                    label="Allow zero"
                    description="Lets an issue be estimated at zero."
                    disabled={values.estimateType === "notUsed"}
                  />
                )}
              />
              <form.AppField
                name="estimateExtended"
                children={(field) => (
                  <field.SwitchField
                    label="Extended scale"
                    description="Adds two larger values to the scale."
                    disabled={values.estimateType === "notUsed"}
                  />
                )}
              />
              <EstimatePreview
                type={values.estimateType}
                allowZero={values.estimateAllowZero}
                extended={values.estimateExtended}
              />
            </>
          )}
        </form.Subscribe>
      </SettingsSection>
    </SettingsPage>
  );
}

function EstimatePreview({
  type,
  allowZero,
  extended,
}: {
  type: TEstimateType;
  allowZero: boolean;
  extended: boolean;
}) {
  const options = estimateOptions({ type, allowZero, extended });
  if (options.length === 0) {
    return <p className="text-muted-foreground text-sm">Estimates are turned off for this team.</p>;
  }
  return (
    <div className="flex flex-wrap items-center gap-2">
      {options.map((value) => (
        <EstimateBadge key={value} value={value} type={type} />
      ))}
    </div>
  );
}

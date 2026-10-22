import { useQueryClient } from "@tanstack/react-query";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { z } from "zod";

import { SettingsPage } from "@/components/layout/settings-page";
import { SettingsSection } from "@/components/layout/settings-section";
import { useSettingsTeamDetail } from "@/components/settings/use-settings-team";
import { WorkspaceIcon } from "@/components/workspace/workspace-icon";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { sanitizeTeamKey } from "@/lib/helpers/identifiers";
import { timezoneOptions } from "@/lib/helpers/timezones";
import { submitHandler, useAppForm } from "@/lib/hooks/use-app-form";
import { TeamFormSchema, useSaveTeamMutation } from "@/lib/queries/teams";
import { useUploadAsset } from "@/lib/queries/uploads";
import { useUpdateWorkspaceMutation, WorkspaceFormSchema } from "@/lib/queries/workspaces";

export const Route = createFileRoute("/$workspace/_workspace/settings/_workspace-settings/")({
  component: GeneralSettingsPage,
});

const workspaceFormSchema = WorkspaceFormSchema.extend({
  logoUrl: z.string(),
});

const issueFormSchema = TeamFormSchema.pick({ key: true }).extend({
  timezone: z.string(),
});

const timezones = timezoneOptions();

function GeneralSettingsPage() {
  const { isAdmin } = useWorkspace();
  return (
    <SettingsPage
      title="General"
      description={
        isAdmin
          ? "Name, URL and logo of this workspace, and how its issues are named."
          : "How issues are named."
      }
    >
      {isAdmin && <WorkspaceSection />}
      <IssuesSection />
    </SettingsPage>
  );
}

function WorkspaceSection() {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const { mutateAsync: update } = useUpdateWorkspaceMutation(workspace.id);
  const uploadImage = useUploadAsset(workspace.id);

  const form = useAppForm({
    defaultValues: {
      name: workspace.name,
      urlKey: workspace.urlKey,
      logoUrl: workspace.logoUrl ?? "",
    },
    validators: { onChange: workspaceFormSchema },
    onSubmit: async ({ value, formApi }) => {
      const parsed = workspaceFormSchema.parse(value);
      const updated = await update({
        name: parsed.name,
        urlKey: parsed.urlKey,
        logoUrl: parsed.logoUrl || null,
      });
      formApi.reset({
        name: updated.name,
        urlKey: updated.urlKey,
        logoUrl: updated.logoUrl ?? "",
      });
      if (updated.urlKey === workspace.urlKey) return;
      await navigate({ to: "/$workspace/settings", params: { workspace: updated.urlKey } });
    },
  });

  return (
    <SettingsSection
      title="Workspace"
      onSubmit={submitHandler(form)}
      footer={
        <form.AppForm>
          <form.SubmitButton size="sm" requireDirty>
            Save
          </form.SubmitButton>
        </form.AppForm>
      }
    >
      <form.AppField name="name" children={(field) => <field.TextField label="Name" />} />
      <form.AppField
        name="urlKey"
        children={(field) => (
          <field.TextField
            label="URL"
            description={`${window.location.host}/${field.state.value || "…"}. Old links stop working when it changes.`}
            transform={(value) => value.toLowerCase()}
          />
        )}
      />
      <form.AppField
        name="logoUrl"
        children={(field) => (
          <field.ImageField
            label="Logo"
            description="A square image. Without one, the first letter of the name shows."
            upload={uploadImage}
            preview={(logoUrl) => (
              <form.Subscribe selector={(state) => state.values.name}>
                {(name) => (
                  <WorkspaceIcon name={name || workspace.name} logoUrl={logoUrl} size="lg" />
                )}
              </form.Subscribe>
            )}
          />
        )}
      />
    </SettingsSection>
  );
}

function IssuesSection() {
  const team = useSettingsTeamDetail();
  const { workspace } = useWorkspace();
  const queryClient = useQueryClient();
  const { mutateAsync: saveTeam } = useSaveTeamMutation(workspace.id);

  const form = useAppForm({
    defaultValues: { key: team.key, timezone: team.timezone || "UTC" },
    validators: { onChange: issueFormSchema },
    onSubmit: async ({ value, formApi }) => {
      const parsed = issueFormSchema.parse(value);
      const saved = await saveTeam({ id: team.id, key: parsed.key, timezone: parsed.timezone });
      formApi.reset({ key: saved.key, timezone: value.timezone });
      if (saved.key === team.key) return;
      await queryClient.invalidateQueries({ queryKey: [workspace.id] });
    },
  });

  return (
    <SettingsSection
      title="Issues"
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
        name="key"
        children={(field) => (
          <field.TextField
            label="Issue prefix"
            description={`Issues are named like ${field.state.value || "…"}-123. Changing it renames every issue; old identifiers keep working.`}
            transform={sanitizeTeamKey}
            className="font-mono"
          />
        )}
      />
      <form.AppField
        name="timezone"
        children={(field) => (
          <field.SelectField
            label="Timezone"
            description="Cycles start and end at midnight in this timezone."
            items={timezones}
          />
        )}
      />
    </SettingsSection>
  );
}

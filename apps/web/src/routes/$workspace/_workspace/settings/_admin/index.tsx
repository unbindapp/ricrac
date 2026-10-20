import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { z } from "zod";

import { SettingsPage } from "@/components/layout/settings-page";
import { SettingsSection } from "@/components/layout/settings-section";
import { WorkspaceIcon } from "@/components/workspace/workspace-icon";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { submitHandler, useAppForm } from "@/lib/hooks/use-app-form";
import { useUploadAsset } from "@/lib/queries/uploads";
import { useUpdateWorkspaceMutation, WorkspaceFormSchema } from "@/lib/queries/workspaces";

export const Route = createFileRoute("/$workspace/_workspace/settings/_admin/")({
  component: WorkspaceGeneralSettingsPage,
});

const formSchema = WorkspaceFormSchema.extend({
  logoUrl: z.string(),
});

function WorkspaceGeneralSettingsPage() {
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
    validators: { onChange: formSchema },
    onSubmit: async ({ value, formApi }) => {
      const parsed = formSchema.parse(value);
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
    <SettingsPage title="General" description="Name, URL and logo of this workspace.">
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
    </SettingsPage>
  );
}

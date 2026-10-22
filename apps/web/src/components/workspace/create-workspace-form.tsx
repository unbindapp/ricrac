import type { WorkspaceSummary } from "@/api/account.gen";
import { deriveTeamKey, sanitizeTeamKey, slugify } from "@/lib/helpers/identifiers";
import { submitHandler, useAppForm } from "@/lib/hooks/use-app-form";
import { TeamFormSchema } from "@/lib/queries/teams";
import { useCreateWorkspaceMutation, WorkspaceFormSchema } from "@/lib/queries/workspaces";

const CreateWorkspaceSchema = WorkspaceFormSchema.extend({
  issuePrefix: TeamFormSchema.shape.key,
});

export function CreateWorkspaceForm({
  onCreated,
}: {
  onCreated: (workspace: WorkspaceSummary) => void;
}) {
  const { mutateAsync: create } = useCreateWorkspaceMutation();
  const form = useAppForm({
    defaultValues: { name: "", urlKey: "", issuePrefix: "" },
    validators: { onSubmit: CreateWorkspaceSchema },
    onSubmit: async ({ value }) => {
      const { name, urlKey, issuePrefix } = CreateWorkspaceSchema.parse(value);
      const workspace = await create({ name, urlKey, teamName: name, teamKey: issuePrefix });
      onCreated(workspace);
    },
  });

  return (
    <form className="flex w-full flex-col gap-4" onSubmit={submitHandler(form)} noValidate>
      <form.AppField
        name="name"
        listeners={{
          onChange: ({ value }) => {
            if (!form.getFieldMeta("urlKey")?.isDirty) {
              form.setFieldValue("urlKey", slugify(value), { dontUpdateMeta: true });
            }
            if (!form.getFieldMeta("issuePrefix")?.isDirty) {
              form.setFieldValue("issuePrefix", deriveTeamKey(value), { dontUpdateMeta: true });
            }
          },
        }}
        children={(field) => <field.TextField label="Name" placeholder="Acme" autoFocus />}
      />
      <form.AppField
        name="urlKey"
        children={(field) => (
          <field.TextField
            label="URL"
            description={`${window.location.host}/${field.state.value || "…"}`}
            transform={(value) => value.toLowerCase()}
          />
        )}
      />
      <form.AppField
        name="issuePrefix"
        children={(field) => (
          <field.TextField
            label="Issue prefix"
            description={`Issues are named like ${field.state.value || "…"}-123.`}
            transform={sanitizeTeamKey}
            className="font-mono"
          />
        )}
      />
      <form.AppForm>
        <form.SubmitButton className="w-full">Create workspace</form.SubmitButton>
      </form.AppForm>
    </form>
  );
}

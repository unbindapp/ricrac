import type { WorkspaceSummary } from "@/api/account.gen";
import { slugify } from "@/lib/helpers/identifiers";
import { submitHandler, useAppForm } from "@/lib/hooks/use-app-form";
import { useCreateWorkspaceMutation, WorkspaceFormSchema } from "@/lib/queries/workspaces";

export function CreateWorkspaceForm({
  onCreated,
}: {
  onCreated: (workspace: WorkspaceSummary) => void;
}) {
  const { mutateAsync: create } = useCreateWorkspaceMutation();
  const form = useAppForm({
    defaultValues: { name: "", urlKey: "" },
    validators: { onSubmit: WorkspaceFormSchema },
    onSubmit: async ({ value }) => {
      const workspace = await create(WorkspaceFormSchema.parse(value));
      onCreated(workspace);
    },
  });

  return (
    <form className="flex w-full flex-col gap-4" onSubmit={submitHandler(form)} noValidate>
      <form.AppField
        name="name"
        listeners={{
          onChange: ({ value }) => {
            if (form.getFieldMeta("urlKey")?.isDirty) return;
            form.setFieldValue("urlKey", slugify(value), { dontUpdateMeta: true });
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
      <form.AppForm>
        <form.SubmitButton className="w-full">Create workspace</form.SubmitButton>
      </form.AppForm>
    </form>
  );
}

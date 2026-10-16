import { type ReactElement, useState } from "react";
import { z } from "zod";

import type { ProjectStatus } from "@/api/ops.gen";
import { projectStatusTypeLabels } from "@/components/projects/project-status-icon";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { DEFAULT_PROJECT_STATUS_COLORS } from "@/lib/helpers/colors";
import { submitHandler, useAppForm } from "@/lib/hooks/use-app-form";
import { type TProjectStatusType, useSaveProjectStatusMutation } from "@/lib/queries/projects";

const formSchema = z.object({
  name: z.string().trim().min(1, "Name is required.").max(64, "Keep the name under 64 characters."),
  color: z.string(),
  description: z.string().trim().max(500, "Keep the description under 500 characters."),
});

type TProps = { trigger: ReactElement } & (
  { type: TProjectStatusType; status?: never } | { status: ProjectStatus; type?: never }
);

export function ProjectStatusDialog(props: TProps) {
  const [isOpen, setIsOpen] = useState(false);
  const title = props.status
    ? "Edit status"
    : `Add ${projectStatusTypeLabels[props.type].toLowerCase()} status`;
  return (
    <Dialog open={isOpen} onOpenChange={setIsOpen}>
      <DialogTrigger render={props.trigger} />
      <DialogContent className="w-md max-w-full">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        {isOpen && <ProjectStatusForm {...props} onDone={() => setIsOpen(false)} />}
      </DialogContent>
    </Dialog>
  );
}

function ProjectStatusForm(props: TProps & { onDone: () => void }) {
  const { workspace } = useWorkspace();
  const { mutateAsync: saveStatus } = useSaveProjectStatusMutation(workspace.id);
  const { status } = props;
  const form = useAppForm({
    defaultValues: status
      ? { name: status.name, color: status.color, description: status.description ?? "" }
      : { name: "", color: DEFAULT_PROJECT_STATUS_COLORS[props.type], description: "" },
    validators: { onSubmit: formSchema },
    onSubmit: async ({ value }) => {
      const parsed = formSchema.parse(value);
      const fields = { ...parsed, description: parsed.description || null };
      await saveStatus(status ? { id: status.id, ...fields } : { type: props.type, ...fields });
      props.onDone();
    },
  });
  return (
    <form className="flex flex-col gap-4" onSubmit={submitHandler(form)} noValidate>
      <div className="flex items-start gap-4">
        <form.AppField name="color" children={(field) => <field.ColorField label="Color" />} />
        <div className="min-w-0 flex-1">
          <form.AppField
            name="name"
            children={(field) => <field.TextField label="Name" autoFocus />}
          />
        </div>
      </div>
      <form.AppField
        name="description"
        children={(field) => <field.TextareaField label="Description" rows={2} />}
      />
      <DialogFooter>
        <DialogClose
          render={
            <Button type="button" variant="outline" size="sm">
              Cancel
            </Button>
          }
        />
        <form.AppForm>
          <form.SubmitButton size="sm">{status ? "Save" : "Add status"}</form.SubmitButton>
        </form.AppForm>
      </DialogFooter>
    </form>
  );
}

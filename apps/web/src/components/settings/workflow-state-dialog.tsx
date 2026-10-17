import { ReactElement, useState } from "react";
import { z } from "zod";

import type { IssueStatus } from "@/api/ops.gen";
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
import { type TStateType, stateTypeLabels } from "@/components/shared/state-icon";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { DEFAULT_STATE_COLORS } from "@/lib/helpers/colors";
import { submitHandler, useAppForm } from "@/lib/hooks/use-app-form";
import { useSaveIssueStatusMutation } from "@/lib/queries/teams";

export const defaultStateColors: Record<TStateType, string> = DEFAULT_STATE_COLORS;

const formSchema = z.object({
  name: z.string().trim().min(1, "Name is required.").max(64, "Keep the name under 64 characters."),
  color: z.string(),
  description: z.string().max(255, "Keep the description under 255 characters."),
});

type TProps = {
  trigger: ReactElement;
} & (
  | { mode: "create"; teamId: string; type: TStateType; state?: never }
  | { mode: "edit"; state: IssueStatus; teamId?: never; type?: never }
);

export function WorkflowStateDialog(props: TProps) {
  const [isOpen, setIsOpen] = useState(false);
  const title =
    props.mode === "create"
      ? `Add ${stateTypeLabels[props.type].toLowerCase()} status`
      : "Edit status";
  return (
    <Dialog open={isOpen} onOpenChange={setIsOpen}>
      <DialogTrigger render={props.trigger} />
      <DialogContent className="w-md max-w-full">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        {isOpen && <WorkflowStateForm {...props} onDone={() => setIsOpen(false)} />}
      </DialogContent>
    </Dialog>
  );
}

function WorkflowStateForm(props: TProps & { onDone: () => void }) {
  const { workspace } = useWorkspace();
  const { mutateAsync: saveStatus } = useSaveIssueStatusMutation(workspace.id);
  const form = useAppForm({
    defaultValues:
      props.mode === "edit"
        ? {
            name: props.state.name,
            color: props.state.color,
            description: props.state.description ?? "",
          }
        : { name: "", color: defaultStateColors[props.type], description: "" },
    validators: { onSubmit: formSchema },
    onSubmit: async ({ value }) => {
      const parsed = formSchema.parse(value);
      const fields = { ...parsed, description: parsed.description.trim() || null };
      if (props.mode === "edit") {
        await saveStatus({ id: props.state.id, ...fields });
        props.onDone();
        return;
      }
      await saveStatus({ team: props.teamId, type: props.type, ...fields });
      props.onDone();
    },
  });
  return (
    <form className="flex flex-col gap-4" onSubmit={submitHandler(form)} noValidate>
      <form.AppField name="color" children={(field) => <field.ColorField label="Color" />} />
      <form.AppField name="name" children={(field) => <field.TextField label="Name" autoFocus />} />
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
          <form.SubmitButton size="sm">
            {props.mode === "edit" ? "Save" : "Add status"}
          </form.SubmitButton>
        </form.AppForm>
      </DialogFooter>
    </form>
  );
}

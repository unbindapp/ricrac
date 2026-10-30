import { type ReactElement, useState } from "react";
import { z } from "zod";

import type { Milestone } from "@/api/ops.gen";
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
import { submitHandler, useAppForm } from "@/lib/hooks/use-app-form";
import { useSaveMilestoneMutation } from "@/lib/queries/milestones";

const milestoneSchema = z.object({
  name: z.string().trim().min(1, "Name is required.").max(80, "Keep the name under 80 characters."),
  description: z.string().trim().max(5000, "Keep the description under 5000 characters."),
  targetDate: z.string().regex(/^(\d{4}-\d{2}-\d{2})?$/, "Pick a date."),
});

type TProps = {
  trigger?: ReactElement;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
} & ({ milestone: Milestone; projectId?: never } | { milestone?: never; projectId?: string });

/** Edits a milestone, or adds one to the project, or to the workspace without a project. */
export function MilestoneDialog(props: TProps) {
  const [internalOpen, setInternalOpen] = useState(false);
  const isOpen = props.open ?? internalOpen;
  const setOpen = (open: boolean) => {
    setInternalOpen(open);
    props.onOpenChange?.(open);
  };
  return (
    <Dialog open={isOpen} onOpenChange={setOpen}>
      {props.trigger && <DialogTrigger render={props.trigger} />}
      <DialogContent className="w-md max-w-full">
        <DialogHeader>
          <DialogTitle>{dialogTitle(props)}</DialogTitle>
        </DialogHeader>
        {isOpen && <MilestoneForm {...props} onDone={() => setOpen(false)} />}
      </DialogContent>
    </Dialog>
  );
}

function dialogTitle({ milestone, projectId }: TProps) {
  if (milestone) return "Edit milestone";
  return projectId ? "Add milestone" : "New milestone";
}

function submitLabel({ milestone, projectId }: TProps) {
  if (milestone) return "Save";
  return projectId ? "Add milestone" : "Create milestone";
}

function MilestoneForm(props: TProps & { onDone: () => void }) {
  const { workspace } = useWorkspace();
  const { mutateAsync: saveMilestone } = useSaveMilestoneMutation(workspace.id);
  const { milestone } = props;
  const form = useAppForm({
    defaultValues: {
      name: milestone?.name ?? "",
      description: milestone?.description ?? "",
      targetDate: milestone?.targetDate ?? "",
    },
    validators: { onSubmit: milestoneSchema },
    onSubmit: async ({ value }) => {
      const parsed = milestoneSchema.parse(value);
      const fields = {
        name: parsed.name,
        description: parsed.description || null,
        targetDate: parsed.targetDate || null,
      };
      await saveMilestone(
        milestone ? { id: milestone.id, ...fields } : { project: props.projectId, ...fields },
      );
      props.onDone();
    },
  });
  return (
    <form className="flex flex-col gap-4" onSubmit={submitHandler(form)} noValidate>
      <form.AppField
        name="name"
        children={(field) => <field.TextField label="Name" placeholder="Beta" autoFocus />}
      />
      <form.AppField
        name="targetDate"
        children={(field) => <field.TextField label="Target date" type="date" />}
      />
      <form.AppField
        name="description"
        children={(field) => (
          <field.TextareaField
            label="Description"
            placeholder="What is done when this milestone is reached?"
            rows={3}
          />
        )}
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
          <form.SubmitButton size="sm">{submitLabel(props)}</form.SubmitButton>
        </form.AppForm>
      </DialogFooter>
    </form>
  );
}

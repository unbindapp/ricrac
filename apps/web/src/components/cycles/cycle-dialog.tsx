import { addDays, format } from "date-fns";
import { z } from "zod";

import type { Cycle } from "@/api/ops.gen";
import { cycleLastDay } from "@/components/cycles/cycle-status";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { submitHandler, useAppForm } from "@/lib/hooks/use-app-form";
import { cycleName, useSaveCycleMutation } from "@/lib/queries/cycles";

const datePattern = /^\d{4}-\d{2}-\d{2}$/;

const cycleFormSchema = z
  .object({
    name: z.string().trim().max(80, "Keep the name under 80 characters."),
    description: z.string().trim().max(2000, "Keep the description under 2000 characters."),
    startsOn: z.string().regex(datePattern, "Pick a start date."),
    lastDay: z.string().regex(datePattern, "Pick the last day."),
  })
  .refine((value) => value.lastDay >= value.startsOn, {
    message: "The cycle ends on or after its first day.",
    path: ["lastDay"],
  });

const day = (date: Date) => format(date, "yyyy-MM-dd");

export type TCycleDialogTarget =
  { mode: "create"; startsOn: Date; weeks: number } | { mode: "edit"; cycle: Cycle };

type TProps = {
  team: { id: string };
  target: TCycleDialogTarget | null;
  onClose: () => void;
};

export function CycleDialog({ team, target, onClose }: TProps) {
  return (
    <Dialog open={target !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="w-md max-w-full">
        <DialogHeader>
          <DialogTitle>
            {target?.mode === "edit" ? `Edit ${cycleName(target.cycle)}` : "New cycle"}
          </DialogTitle>
          {target?.mode === "create" && (
            <DialogDescription>
              Cycles are planned on their own; add one for a break from the schedule.
            </DialogDescription>
          )}
        </DialogHeader>
        {target && <CycleForm team={team} target={target} onDone={onClose} />}
      </DialogContent>
    </Dialog>
  );
}

function CycleForm({
  team,
  target,
  onDone,
}: {
  team: TProps["team"];
  target: TCycleDialogTarget;
  onDone: () => void;
}) {
  const { workspace } = useWorkspace();
  const { mutateAsync: saveCycle } = useSaveCycleMutation(workspace.id);
  const cycle = target.mode === "edit" ? target.cycle : null;
  const form = useAppForm({
    defaultValues:
      target.mode === "edit"
        ? {
            name: target.cycle.name ?? "",
            description: target.cycle.description ?? "",
            startsOn: day(new Date(target.cycle.startsAt)),
            lastDay: day(cycleLastDay(target.cycle)),
          }
        : {
            name: "",
            description: "",
            startsOn: day(target.startsOn),
            lastDay: day(addDays(target.startsOn, 7 * target.weeks - 1)),
          },
    validators: { onSubmit: cycleFormSchema },
    onSubmit: async ({ value }) => {
      const parsed = cycleFormSchema.parse(value);
      const fields = {
        name: parsed.name || null,
        description: parsed.description || null,
        startsAt: parsed.startsOn,
        endsAt: day(addDays(new Date(`${parsed.lastDay}T00:00:00`), 1)),
      };
      if (cycle) await saveCycle({ id: cycle.id, ...fields });
      else await saveCycle({ team: team.id, ...fields });
      onDone();
    },
  });

  return (
    <form className="flex flex-col gap-4" onSubmit={submitHandler(form)} noValidate>
      <form.AppField
        name="name"
        children={(field) => (
          <field.TextField
            label="Name"
            placeholder={cycle ? `Cycle ${cycle.number}` : "Optional"}
            autoFocus
            autoComplete="off"
          />
        )}
      />
      <div className="grid grid-cols-2 gap-4">
        <form.AppField
          name="startsOn"
          children={(field) => <field.TextField label="First day" type="date" />}
        />
        <form.AppField
          name="lastDay"
          children={(field) => <field.TextField label="Last day" type="date" />}
        />
      </div>
      <form.AppField
        name="description"
        children={(field) => (
          <field.TextareaField label="Goal" rows={3} placeholder="What this cycle is for" />
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
          <form.SubmitButton size="sm">{cycle ? "Save" : "Create cycle"}</form.SubmitButton>
        </form.AppForm>
      </DialogFooter>
    </form>
  );
}

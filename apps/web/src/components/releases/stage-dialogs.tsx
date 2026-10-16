import { type ReactElement, useState } from "react";
import { z } from "zod";

import type { ReleasePipeline, ReleaseStage } from "@/api/ops.gen";
import { ReleaseStageIcon } from "@/components/releases/release-stage-icon";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { DEFAULT_STATE_COLORS } from "@/lib/helpers/colors";
import { STAGE_TYPES, stageTypeLabels, type TStageType } from "@/lib/helpers/releases";
import { submitHandler, useAppForm } from "@/lib/hooks/use-app-form";
import { useDeleteStageMutation, useSaveStageMutation } from "@/lib/queries/releases";

const defaultColors: Record<TStageType, string> = {
  planned: DEFAULT_STATE_COLORS.unstarted,
  started: DEFAULT_STATE_COLORS.started,
  completed: DEFAULT_STATE_COLORS.completed,
  canceled: DEFAULT_STATE_COLORS.canceled,
};

const typeDescriptions: Record<TStageType, string> = {
  planned: "Releases being planned. Moving back here clears their start and release times.",
  started: "Releases being built or rolled out; they get a start time.",
  completed: "Shipped releases; they get a release time.",
  canceled: "Releases that won't ship.",
};

const formSchema = z.object({
  name: z.string().trim().min(1, "Name is required.").max(64, "Keep the name under 64 characters."),
  type: z.enum(["planned", "started", "completed", "canceled"]),
  color: z.string(),
});

type TProps = {
  trigger: ReactElement;
  pipeline: ReleasePipeline;
  stage?: ReleaseStage;
};

export function StageDialog({ trigger, pipeline, stage }: TProps) {
  const [isOpen, setIsOpen] = useState(false);
  return (
    <Dialog open={isOpen} onOpenChange={setIsOpen}>
      <DialogTrigger render={trigger} />
      <DialogContent className="w-md max-w-full">
        <DialogHeader>
          <DialogTitle>{stage ? "Edit stage" : `Add a stage to ${pipeline.name}`}</DialogTitle>
        </DialogHeader>
        {isOpen && <StageForm pipeline={pipeline} stage={stage} onDone={() => setIsOpen(false)} />}
      </DialogContent>
    </Dialog>
  );
}

function StageForm({
  pipeline,
  stage,
  onDone,
}: {
  pipeline: ReleasePipeline;
  stage?: ReleaseStage;
  onDone: () => void;
}) {
  const { workspace } = useWorkspace();
  const { mutateAsync: save } = useSaveStageMutation(workspace.id);
  const form = useAppForm({
    defaultValues: {
      name: stage?.name ?? "",
      type: stage?.type ?? ("started" as TStageType),
      color: stage?.color ?? defaultColors.started,
    },
    validators: { onSubmit: formSchema },
    onSubmit: async ({ value }) => {
      const parsed = formSchema.parse(value);
      if (stage) {
        await save({ id: stage.id, ...parsed });
        onDone();
        return;
      }
      await save({ pipeline: pipeline.id, ...parsed });
      onDone();
    },
  });
  const typeItems = STAGE_TYPES.map((type) => ({
    value: type,
    label: stageTypeLabels[type],
    icon: <ReleaseStageIcon type={type} />,
  }));
  return (
    <form className="flex flex-col gap-4" onSubmit={submitHandler(form)} noValidate>
      <form.AppField name="color" children={(field) => <field.ColorField label="Color" />} />
      <form.AppField name="name" children={(field) => <field.TextField label="Name" autoFocus />} />
      <form.Subscribe selector={(state) => state.values.type}>
        {(type) => (
          <form.AppField
            name="type"
            listeners={{
              onChange: ({ value }) => {
                if (!stage) form.setFieldValue("color", defaultColors[value]);
              },
            }}
            children={(field) => (
              <field.SelectField
                label="Kind"
                description={typeDescriptions[type]}
                items={typeItems}
              />
            )}
          />
        )}
      </form.Subscribe>
      <DialogFooter>
        <DialogClose
          render={
            <Button type="button" variant="outline" size="sm">
              Cancel
            </Button>
          }
        />
        <form.AppForm>
          <form.SubmitButton size="sm">{stage ? "Save" : "Add stage"}</form.SubmitButton>
        </form.AppForm>
      </DialogFooter>
    </form>
  );
}

export function DeleteStageDialog({
  trigger,
  pipeline,
  stage,
}: {
  trigger: ReactElement;
  pipeline: ReleasePipeline;
  stage: ReleaseStage;
}) {
  const { workspace } = useWorkspace();
  const { mutateAsync: remove } = useDeleteStageMutation(workspace.id);
  const others = (pipeline.stages ?? []).filter((item) => item.id !== stage.id);
  const fallback = others.find((item) => item.type === stage.type) ?? others[0];
  const [moveTo, setMoveTo] = useState(fallback?.id ?? "");
  const target = others.find((item) => item.id === moveTo);
  return (
    <ConfirmDialog
      trigger={trigger}
      title={`Delete ${stage.name}`}
      description="Releases in this stage move to the one you pick."
      confirmLabel="Delete stage"
      confirmDisabled={others.length === 0}
      onConfirm={() => remove({ id: stage.id, moveReleasesTo: moveTo || null })}
    >
      {others.length === 0 ? (
        <p className="text-muted-foreground text-sm">A pipeline needs at least one stage.</p>
      ) : (
        <div className="flex flex-col gap-1.5">
          <p className="text-sm font-medium">Move its releases to</p>
          <Select
            items={others.map((item) => ({ value: item.id, label: item.name }))}
            value={moveTo}
            onValueChange={(value) => setMoveTo(value ?? "")}
          >
            <SelectTrigger aria-label="Move its releases to">
              <SelectValue>
                {target && (
                  <>
                    <ReleaseStageIcon type={target.type} color={target.color} />
                    <span className="truncate">{target.name}</span>
                  </>
                )}
              </SelectValue>
            </SelectTrigger>
            <SelectContent>
              {others.map((item) => (
                <SelectItem key={item.id} value={item.id}>
                  <ReleaseStageIcon type={item.type} color={item.color} />
                  <span className="truncate">{item.name}</span>
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      )}
    </ConfirmDialog>
  );
}

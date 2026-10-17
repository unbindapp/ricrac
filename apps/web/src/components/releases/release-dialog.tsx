import { useNavigate } from "@tanstack/react-router";
import { type ReactElement, useState } from "react";
import { z } from "zod";

import type { Release, ReleasePipeline } from "@/api/ops.gen";
import { ReleaseStageIcon } from "@/components/releases/release-stage-icon";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { submitHandler, type TSelectFieldItem, useAppForm } from "@/lib/hooks/use-app-form";
import { useSaveReleaseMutation } from "@/lib/queries/releases";

const optionalDay = z
  .string()
  .refine((value) => value === "" || /^\d{4}-\d{2}-\d{2}$/.test(value), {
    message: "Pick a date.",
  });

const formSchema = z
  .object({
    name: z
      .string()
      .trim()
      .min(1, "Name is required.")
      .max(120, "Keep the name under 120 characters."),
    version: z.string().trim().max(64, "Keep the version under 64 characters."),
    stage: z.string(),
    startDate: optionalDay,
    targetDate: optionalDay,
    commitSha: z
      .string()
      .trim()
      .refine((value) => value === "" || /^[0-9a-f]{7,40}$/i.test(value), {
        message: "Use a commit SHA: 7 to 40 hexadecimal characters.",
      }),
  })
  .refine((value) => !value.startDate || !value.targetDate || value.startDate <= value.targetDate, {
    message: "The target date comes after the start.",
    path: ["targetDate"],
  });

type TProps = {
  trigger: ReactElement;
  pipeline: ReleasePipeline;
  /** Edits this release; without it, the dialog creates one in the pipeline. */
  release?: Release;
};

export function ReleaseDialog({ trigger, pipeline, release }: TProps) {
  const [isOpen, setIsOpen] = useState(false);
  return (
    <Dialog open={isOpen} onOpenChange={setIsOpen}>
      <DialogTrigger render={trigger} />
      <DialogContent className="w-lg max-w-full">
        <DialogHeader>
          <DialogTitle>{release ? "Edit release" : "New release"}</DialogTitle>
          <DialogDescription>
            {release
              ? `In ${pipeline.name}.`
              : `A release of ${pipeline.name}. Add issues to it from its page.`}
          </DialogDescription>
        </DialogHeader>
        {isOpen && (
          <ReleaseForm pipeline={pipeline} release={release} onDone={() => setIsOpen(false)} />
        )}
      </DialogContent>
    </Dialog>
  );
}

function ReleaseForm({
  pipeline,
  release,
  onDone,
}: {
  pipeline: ReleasePipeline;
  release?: Release;
  onDone: () => void;
}) {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const { mutateAsync: save } = useSaveReleaseMutation(workspace.id);
  const stages = [...(pipeline.stages ?? [])].sort((a, b) => a.position - b.position);
  const firstPlanned = stages.find((stage) => stage.type === "planned") ?? stages[0];
  const form = useAppForm({
    defaultValues: {
      name: release?.name ?? "",
      version: release?.version ?? "",
      stage: release?.stage?.id ?? firstPlanned?.id ?? "",
      startDate: release?.startDate ?? "",
      targetDate: release?.targetDate ?? "",
      commitSha: release?.commitSha ?? "",
    },
    validators: { onSubmit: formSchema },
    onSubmit: async ({ value }) => {
      const fields = {
        name: value.name.trim(),
        version: value.version.trim() || null,
        stage: value.stage || null,
        startDate: value.startDate || null,
        targetDate: value.targetDate || null,
        commitSha: value.commitSha.trim() || null,
      };
      if (release) {
        await save({ id: release.id, ...fields });
        onDone();
        return;
      }
      const created = await save({ pipeline: pipeline.id, ...fields });
      onDone();
      void navigate({
        to: "/$workspace/release/$slug",
        params: { workspace: workspace.urlKey, slug: created.slugId },
      });
    },
  });
  const stageItems: TSelectFieldItem[] = stages.map((stage) => ({
    value: stage.id,
    label: stage.name,
    icon: <ReleaseStageIcon type={stage.type} color={stage.color} />,
  }));

  return (
    <form className="flex flex-col gap-4" onSubmit={submitHandler(form)} noValidate>
      <div className="grid grid-cols-3 gap-4">
        <div className="col-span-2">
          <form.AppField
            name="name"
            children={(field) => (
              <field.TextField
                label="Name"
                placeholder="Spring launch"
                autoComplete="off"
                autoFocus
              />
            )}
          />
        </div>
        <form.AppField
          name="version"
          children={(field) => (
            <field.TextField label="Version" placeholder="1.4.0" autoComplete="off" />
          )}
        />
      </div>
      <form.AppField
        name="stage"
        children={(field) => <field.SelectField label="Stage" items={stageItems} />}
      />
      <div className="grid grid-cols-2 gap-4">
        <form.AppField
          name="startDate"
          children={(field) => <field.TextField label="Start date" type="date" />}
        />
        <form.AppField
          name="targetDate"
          children={(field) => <field.TextField label="Target date" type="date" />}
        />
      </div>
      <form.AppField
        name="commitSha"
        children={(field) => (
          <field.TextField
            label="Commit"
            placeholder="a1b2c3d"
            description="Optional: the commit the release was built from."
            autoComplete="off"
            className="font-mono"
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
          <form.SubmitButton size="sm">{release ? "Save" : "Create release"}</form.SubmitButton>
        </form.AppForm>
      </DialogFooter>
    </form>
  );
}

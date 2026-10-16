import { useNavigate } from "@tanstack/react-router";
import { type ReactElement, useState } from "react";
import { z } from "zod";

import type { ReleasePipeline } from "@/api/ops.gen";
import { CheckboxList } from "@/components/shared/checkbox-list";
import { TeamIcon } from "@/components/shared/team-icon";
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
import { useTeams } from "@/components/workspace/teams-provider";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { submitHandler, useAppForm } from "@/lib/hooks/use-app-form";
import { useSavePipelineMutation } from "@/lib/queries/releases";

const formSchema = z.object({
  name: z.string().trim().min(1, "Name is required.").max(80, "Keep the name under 80 characters."),
  type: z.enum(["continuous", "scheduled"]),
  isProduction: z.boolean(),
  teams: z.array(z.string()),
});

const typeItems = [
  { value: "continuous", label: "Continuous: every deploy is a release" },
  { value: "scheduled", label: "Scheduled: releases are planned ahead" },
];

type TProps = {
  trigger: ReactElement;
  /** Edits this pipeline; without it, the dialog creates one. */
  pipeline?: ReleasePipeline;
};

export function PipelineDialog({ trigger, pipeline }: TProps) {
  const [isOpen, setIsOpen] = useState(false);
  return (
    <Dialog open={isOpen} onOpenChange={setIsOpen}>
      <DialogTrigger render={trigger} />
      <DialogContent className="w-lg max-w-full">
        <DialogHeader>
          <DialogTitle>{pipeline ? "Edit pipeline" : "New release pipeline"}</DialogTitle>
          <DialogDescription>
            {pipeline
              ? "Its stages are edited in the list below it."
              : "It starts with the stages Planned, In Progress, Released and Canceled."}
          </DialogDescription>
        </DialogHeader>
        {isOpen && <PipelineForm pipeline={pipeline} onDone={() => setIsOpen(false)} />}
      </DialogContent>
    </Dialog>
  );
}

function PipelineForm({ pipeline, onDone }: { pipeline?: ReleasePipeline; onDone: () => void }) {
  const { workspace } = useWorkspace();
  const { teams } = useTeams();
  const navigate = useNavigate();
  const { mutateAsync: save } = useSavePipelineMutation(workspace.id);
  const form = useAppForm({
    defaultValues: {
      name: pipeline?.name ?? "",
      type: pipeline?.type ?? ("scheduled" as "continuous" | "scheduled"),
      isProduction: pipeline?.isProduction ?? true,
      teams: pipeline?.teams?.map((team) => team.id) ?? [],
    },
    validators: { onSubmit: formSchema },
    onSubmit: async ({ value }) => {
      const parsed = formSchema.parse(value);
      const fields = {
        name: parsed.name,
        type: parsed.type,
        isProduction: parsed.isProduction,
        setTeams: parsed.teams,
      };
      if (pipeline) {
        await save({ id: pipeline.id, ...fields });
        onDone();
        return;
      }
      const created = await save(fields);
      onDone();
      void navigate({
        to: "/$workspace/releases",
        params: { workspace: workspace.urlKey },
        search: { pipeline: created.slugId },
      });
    },
  });
  return (
    <form className="flex flex-col gap-4" onSubmit={submitHandler(form)} noValidate>
      <form.AppField
        name="name"
        children={(field) => (
          <field.TextField label="Name" placeholder="Web app" autoComplete="off" autoFocus />
        )}
      />
      <form.AppField
        name="type"
        children={(field) => <field.SelectField label="How it ships" items={typeItems} />}
      />
      <form.AppField
        name="isProduction"
        children={(field) => (
          <field.SwitchField
            label="Deploys to production"
            description="Production pipelines come first on the releases page."
          />
        )}
      />
      <form.Field
        name="teams"
        children={(field) => (
          <div className="flex flex-col gap-1.5">
            <p className="text-sm font-medium">Teams</p>
            <p className="text-muted-foreground text-xs">
              Only people who see one of these teams see the pipeline. Leave empty for everyone.
            </p>
            <CheckboxList
              label="Teams"
              emptyText="There are no teams yet."
              items={teams.map((team) => ({
                value: team.id,
                label: team.name,
                icon: <TeamIcon teamKey={team.key} icon={team.icon} color={team.color} size="sm" />,
              }))}
              value={field.state.value}
              onChange={field.handleChange}
            />
          </div>
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
          <form.SubmitButton size="sm">{pipeline ? "Save" : "Create pipeline"}</form.SubmitButton>
        </form.AppForm>
      </DialogFooter>
    </form>
  );
}

import { useState } from "react";
import { z } from "zod";

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
import { Tabs, TabsList, TabsTab } from "@/components/ui/tabs";
import {
  conditionsToFilter,
  type TCondition,
  type TFilterModel,
} from "@/components/views/view-filter";
import { ViewFilterBar } from "@/components/views/view-filter-bar";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { submitHandler, useAppForm } from "@/lib/hooks/use-app-form";
import { type TFilter, type TView, useSaveViewMutation } from "@/lib/queries/views";

const viewFormSchema = z.object({
  name: z.string().trim().min(1, "Name is required.").max(80, "Keep the name under 80 characters."),
  description: z.string().trim().max(500, "Keep the description under 500 characters."),
  icon: z.string(),
  color: z.string(),
  shared: z.boolean(),
});

/** A new view: what it lists and, when saved from a list, the filter it starts from. */
export type TViewDraft = {
  model: TFilterModel;
  filter?: TFilter;
  display?: Record<string, unknown>;
};

export type TViewDialogTarget =
  { mode: "create"; draft: TViewDraft } | { mode: "edit"; view: TView };

type TProps = {
  target: TViewDialogTarget | null;
  onClose: () => void;
  onSaved?: (view: TView) => void;
};

export function ViewDialog({ target, onClose, onSaved }: TProps) {
  return (
    <Dialog open={target !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="w-lg max-w-full">
        <DialogHeader>
          <DialogTitle>{target?.mode === "edit" ? "Edit view" : "New view"}</DialogTitle>
          {target?.mode === "create" && (
            <DialogDescription>
              A saved filter that stays up to date. Only you see it unless you share it.
            </DialogDescription>
          )}
        </DialogHeader>
        {target && (
          <ViewForm
            target={target}
            onDone={(view) => {
              onClose();
              onSaved?.(view);
            }}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function ViewForm({
  target,
  onDone,
}: {
  target: TViewDialogTarget;
  onDone: (view: TView) => void;
}) {
  const { workspace } = useWorkspace();
  const { mutateAsync: saveView } = useSaveViewMutation(workspace.id);
  const view = target.mode === "edit" ? target.view : null;
  const draft = target.mode === "create" ? target.draft : null;
  const [model, setModel] = useState<TFilterModel>(
    view?.model === "project" ? "project" : (draft?.model ?? "issue"),
  );
  const [conditions, setConditions] = useState<TCondition[]>([]);

  const form = useAppForm({
    defaultValues: {
      name: view?.name ?? "",
      description: view?.description ?? "",
      icon: view?.icon ?? "",
      color: view?.color ?? "",
      shared: view?.shared ?? false,
    },
    validators: { onSubmit: viewFormSchema },
    onSubmit: async ({ value }) => {
      const parsed = viewFormSchema.parse(value);
      const fields = {
        name: parsed.name,
        description: parsed.description || null,
        icon: parsed.icon || null,
        color: parsed.color || null,
        shared: parsed.shared,
      };
      if (view) {
        onDone(await saveView({ id: view.id, ...fields }));
        return;
      }
      const filter = draft?.filter ?? conditionsToFilter(conditions);
      onDone(await saveView({ ...fields, model, filter, display: draft?.display }));
    },
  });

  return (
    <form className="flex flex-col gap-4" onSubmit={submitHandler(form)} noValidate>
      {!view && !draft?.filter && (
        <Tabs
          value={model}
          onValueChange={(value) => {
            setModel(value as TFilterModel);
            setConditions([]);
          }}
        >
          <TabsList variant="segmented">
            <TabsTab value="issue">Issues</TabsTab>
            <TabsTab value="project">Projects</TabsTab>
          </TabsList>
        </Tabs>
      )}
      <form.AppField
        name="name"
        children={(field) => (
          <field.TextField
            label="Name"
            autoFocus
            autoComplete="off"
            placeholder={model === "project" ? "Projects at risk" : "Urgent bugs"}
          />
        )}
      />
      <form.AppField
        name="description"
        children={(field) => (
          <field.TextareaField label="Description" rows={2} placeholder="Optional" />
        )}
      />
      <div className="grid grid-cols-2 gap-4">
        <form.AppField name="icon" children={(field) => <field.IconField label="Icon" />} />
        <form.AppField name="color" children={(field) => <field.ColorField label="Color" />} />
      </div>
      {!view && !draft?.filter && (
        <div className="flex flex-col gap-1.5">
          <span className="text-sm font-medium">Filters</span>
          <div className="flex min-h-9 items-center rounded-md border px-2 py-1">
            <ViewFilterBar model={model} conditions={conditions} onChange={setConditions} />
          </div>
          <p className="text-muted-foreground text-xs">You can change them later on the view.</p>
        </div>
      )}
      <form.AppField
        name="shared"
        children={(field) => (
          <field.SwitchField
            label="Share with the workspace"
            description="Everyone sees shared views; admins can change them too."
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
          <form.SubmitButton size="sm">{view ? "Save" : "Create view"}</form.SubmitButton>
        </form.AppForm>
      </DialogFooter>
    </form>
  );
}

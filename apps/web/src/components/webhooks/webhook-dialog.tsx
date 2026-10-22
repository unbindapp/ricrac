import type { AnyFieldApi } from "@tanstack/react-form";

import type { SavedWebhook } from "@/api/ops.gen";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  RESOURCE_GROUPS,
  resourceLabel,
  toSaveArgs,
  webhookFormSchema,
} from "@/components/webhooks/webhook-form";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { submitHandler, useAppForm, useFieldError } from "@/lib/hooks/use-app-form";
import { type TWebhook, useSaveWebhookMutation } from "@/lib/queries/webhooks";

type TProps = {
  /** The webhook to edit, "new" to create one, null to stay closed. */
  target: TWebhook | "new" | null;
  onClose: () => void;
  onCreated?: (webhook: SavedWebhook) => void;
};

export function WebhookDialog({ target, onClose, onCreated }: TProps) {
  return (
    <Dialog open={target !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="w-xl max-w-full">
        <DialogHeader>
          <DialogTitle>{target === "new" ? "New webhook" : "Edit webhook"}</DialogTitle>
          <DialogDescription>
            Ricrac sends a signed JSON POST to the URL for every change it listens to.
          </DialogDescription>
        </DialogHeader>
        {target && (
          <WebhookForm
            webhook={target === "new" ? null : target}
            onDone={(saved) => {
              onClose();
              if (target === "new") onCreated?.(saved);
            }}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function WebhookForm({
  webhook,
  onDone,
}: {
  webhook: TWebhook | null;
  onDone: (saved: SavedWebhook) => void;
}) {
  const { workspace } = useWorkspace();
  const { mutateAsync: saveWebhook } = useSaveWebhookMutation(workspace.id);
  const form = useAppForm({
    defaultValues: {
      url: webhook?.url ?? "",
      label: webhook?.label ?? "",
      resourceTypes: webhook?.resourceTypes ?? ["Issue", "Comment"],
      enabled: webhook?.enabled ?? true,
    },
    validators: { onSubmit: webhookFormSchema },
    onSubmit: async ({ value }) => {
      const args = toSaveArgs(webhookFormSchema.parse(value));
      onDone(await saveWebhook(webhook ? { id: webhook.id, ...args } : args));
    },
  });

  return (
    <form className="flex flex-col gap-4" onSubmit={submitHandler(form)} noValidate>
      <form.AppField
        name="url"
        children={(field) => (
          <field.TextField
            label="URL"
            type="url"
            placeholder="https://example.com/hooks/ricrac"
            autoFocus={!webhook}
            autoComplete="off"
          />
        )}
      />
      <form.AppField
        name="label"
        children={(field) => (
          <field.TextField label="Label" placeholder="Optional" autoComplete="off" />
        )}
      />
      <form.Field name="resourceTypes" children={(field) => <ResourceTypesField field={field} />} />
      {webhook && (
        <form.AppField
          name="enabled"
          children={(field) => (
            <field.SwitchField
              label="Enabled"
              description="A disabled webhook receives nothing; its queued deliveries are given up."
            />
          )}
        />
      )}
      <DialogFooter>
        <DialogClose
          render={
            <Button type="button" variant="outline" size="sm">
              Cancel
            </Button>
          }
        />
        <form.AppForm>
          <form.SubmitButton size="sm">{webhook ? "Save" : "Create webhook"}</form.SubmitButton>
        </form.AppForm>
      </DialogFooter>
    </form>
  );
}

function ResourceTypesField({ field }: { field: AnyFieldApi }) {
  const { error } = useFieldError(field);
  const value: string[] = field.state.value;
  const toggle = (type: string, checked: boolean) => {
    field.handleChange(checked ? [...value, type] : value.filter((item) => item !== type));
    field.handleBlur();
  };
  return (
    <fieldset className="flex flex-col gap-1.5">
      <legend className="pb-1.5 text-sm font-medium">Receives</legend>
      <div className="grid grid-cols-2 gap-x-4 gap-y-3 rounded-md border p-3">
        {RESOURCE_GROUPS.map((group) => (
          <div key={group.label} className="flex flex-col gap-1.5">
            <span className="text-muted-foreground text-xs font-medium">{group.label}</span>
            {group.types.map((type) => (
              <label key={type} className="flex cursor-pointer items-center gap-2 text-sm">
                <Checkbox
                  checked={value.includes(type)}
                  onCheckedChange={(checked) => toggle(type, checked)}
                />
                {resourceLabel(type)}
              </label>
            ))}
          </div>
        ))}
      </div>
      {error && (
        <p role="alert" className="text-destructive text-xs">
          {error}
        </p>
      )}
    </fieldset>
  );
}

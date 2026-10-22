import { useState } from "react";
import { z } from "zod";
import { PlusIcon } from "lucide-react";

import type { CreatedAPIKey } from "@/api/account.gen";
import { WorkspaceAccessList } from "@/components/api-key/access-fields";
import { ApiKeyCreatedDialog } from "@/components/api-key/api-key-created-dialog";
import {
  accessFormShape,
  accessItems,
  hasPickedWorkspace,
  pickWorkspaceMessage,
  scopeLevelDescriptions,
  scopeLevelItems,
  scopesOf,
  type TAccess,
  type TScopeLevel,
} from "@/components/api-key/helpers";
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
import { submitHandler, useAppForm } from "@/lib/hooks/use-app-form";
import { useCreateApiKeyMutation } from "@/lib/queries/api-keys";

const formSchema = z
  .object({
    label: z
      .string()
      .trim()
      .min(1, "Label is required.")
      .max(80, "Keep the label under 80 characters."),
    scopeLevel: z.enum(["read", "write", "admin"]),
    ...accessFormShape,
  })
  .refine(hasPickedWorkspace, pickWorkspaceMessage);

export function CreateApiKeyDialog({ takenLabels }: { takenLabels: string[] }) {
  const [isOpen, setIsOpen] = useState(false);
  const [created, setCreated] = useState<CreatedAPIKey | null>(null);
  return (
    <>
      <Dialog open={isOpen} onOpenChange={setIsOpen}>
        <DialogTrigger
          render={
            <Button size="sm">
              <PlusIcon className="size-4" />
              Create key
            </Button>
          }
        />
        <DialogContent className="w-lg max-w-full">
          <DialogHeader>
            <DialogTitle>Create API key</DialogTitle>
            <DialogDescription>
              Scripts and agents use Ricrac as you with it, never doing more than you can.
            </DialogDescription>
          </DialogHeader>
          {isOpen && (
            <CreateApiKeyForm
              takenLabels={takenLabels}
              onCreated={(key) => {
                setIsOpen(false);
                setCreated(key);
              }}
            />
          )}
        </DialogContent>
      </Dialog>
      <ApiKeyCreatedDialog created={created} onClose={() => setCreated(null)} />
    </>
  );
}

function CreateApiKeyForm({
  takenLabels,
  onCreated,
}: {
  takenLabels: string[];
  onCreated: (key: CreatedAPIKey) => void;
}) {
  const { mutateAsync: createKey } = useCreateApiKeyMutation();
  const form = useAppForm({
    defaultValues: {
      label: "",
      scopeLevel: "write" as TScopeLevel,
      access: "all" as TAccess,
      workspaceIds: [] as string[],
    },
    validators: { onSubmit: formSchema },
    onSubmit: async ({ value }) => {
      const created = await createKey({
        label: value.label.trim(),
        scopes: scopesOf(value.scopeLevel),
        workspaceIds: value.access === "all" ? undefined : value.workspaceIds,
      });
      onCreated(created);
    },
  });

  return (
    <form className="flex flex-col gap-4" onSubmit={submitHandler(form)} noValidate>
      <form.AppField
        name="label"
        validators={{
          onSubmit: ({ value }) =>
            takenLabels.includes(value.trim())
              ? "You already have a key with this label."
              : undefined,
        }}
        children={(field) => (
          <field.TextField
            label="Label"
            description="Something that tells you where the key is used."
            placeholder="CI pipeline"
            autoFocus
          />
        )}
      />
      <form.Subscribe selector={(state) => state.values.scopeLevel}>
        {(level) => (
          <form.AppField
            name="scopeLevel"
            children={(field) => (
              <field.SelectField
                label="Permissions"
                description={scopeLevelDescriptions[level]}
                items={scopeLevelItems}
              />
            )}
          />
        )}
      </form.Subscribe>
      <form.AppField
        name="access"
        children={(field) => <field.SelectField label="Workspaces" items={accessItems} />}
      />
      <form.Subscribe selector={(state) => state.values.access}>
        {(access) => (
          <form.Field
            name="workspaceIds"
            children={(field) => <WorkspaceAccessList field={field} access={access} />}
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
          <form.SubmitButton size="sm">Create key</form.SubmitButton>
        </form.AppForm>
      </DialogFooter>
    </form>
  );
}

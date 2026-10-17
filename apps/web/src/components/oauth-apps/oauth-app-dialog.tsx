import { ReactElement, useState } from "react";
import { z } from "zod";

import type { OAuthApp, OAuthAppSecret, SaveOAuthApp } from "@/api/account.gen";
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
import { submitHandler, useAppForm } from "@/lib/hooks/use-app-form";
import { useCreateOAuthAppMutation, useUpdateOAuthAppMutation } from "@/lib/queries/oauth-apps";

const formSchema = z.object({
  name: z.string().trim().min(1, "Name is required.").max(80, "Keep the name under 80 characters."),
  description: z.string(),
  homepageUrl: z.union([z.literal(""), z.string().trim().url("Enter an http(s) URL.")]),
  redirectUris: z
    .string()
    .refine((value) => splitLines(value).length > 0, "Add at least one callback URL.")
    .refine((value) => splitLines(value).length <= 10, "Add at most 10 callback URLs."),
});

type TValues = z.infer<typeof formSchema>;

function splitLines(value: string) {
  return value
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean);
}

function toInput(value: TValues): SaveOAuthApp {
  return {
    name: value.name.trim(),
    description: value.description.trim() || null,
    homepageUrl: value.homepageUrl.trim() || null,
    redirectUris: splitLines(value.redirectUris),
  };
}

/** Creates an app (no `app`) or edits one; a new app's secret goes to onCreated. */
export function OAuthAppDialog({
  app,
  trigger,
  onCreated,
}: {
  app?: OAuthApp;
  trigger: ReactElement;
  onCreated?: (created: OAuthAppSecret) => void;
}) {
  const [isOpen, setIsOpen] = useState(false);
  return (
    <Dialog open={isOpen} onOpenChange={setIsOpen}>
      <DialogTrigger render={trigger} />
      <DialogContent className="w-lg max-w-full">
        <DialogHeader>
          <DialogTitle>{app ? `Edit ${app.name}` : "Create OAuth app"}</DialogTitle>
          <DialogDescription>
            People in this workspace sign in to it with Ricrac, and it works as them.
          </DialogDescription>
        </DialogHeader>
        {isOpen && (
          <OAuthAppForm
            app={app}
            onDone={(created) => {
              setIsOpen(false);
              if (created) onCreated?.(created);
            }}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function OAuthAppForm({
  app,
  onDone,
}: {
  app?: OAuthApp;
  onDone: (created?: OAuthAppSecret) => void;
}) {
  const { workspace } = useWorkspace();
  const { mutateAsync: create } = useCreateOAuthAppMutation(workspace.id);
  const { mutateAsync: update } = useUpdateOAuthAppMutation(workspace.id);
  const form = useAppForm({
    defaultValues: {
      name: app?.name ?? "",
      description: app?.description ?? "",
      homepageUrl: app?.homepageUrl ?? "",
      redirectUris: app?.redirectUris.join("\n") ?? "",
    },
    validators: { onSubmit: formSchema },
    onSubmit: async ({ value }) => {
      const input = toInput(value);
      if (app) {
        await update({ id: app.id, ...input });
        onDone();
        return;
      }
      onDone(await create(input));
    },
  });

  return (
    <form className="flex flex-col gap-4" onSubmit={submitHandler(form)} noValidate>
      <form.AppField
        name="name"
        children={(field) => <field.TextField label="Name" placeholder="Deploy bot" autoFocus />}
      />
      <form.AppField
        name="description"
        children={(field) => (
          <field.TextField label="Description" description="Shown when people connect it." />
        )}
      />
      <form.AppField
        name="homepageUrl"
        children={(field) => <field.TextField label="Homepage" placeholder="https://example.com" />}
      />
      <form.AppField
        name="redirectUris"
        children={(field) => (
          <field.TextareaField
            label="Callback URLs"
            description="One per line: https, http on localhost, or an app scheme."
            placeholder="https://example.com/oauth/callback"
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
          <form.SubmitButton size="sm">{app ? "Save" : "Create app"}</form.SubmitButton>
        </form.AppForm>
      </DialogFooter>
    </form>
  );
}

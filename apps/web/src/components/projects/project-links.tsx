import { LinkIcon, PlusIcon, XIcon } from "lucide-react";
import { useState } from "react";
import { z } from "zod";

import type { EntityLink } from "@/api/ops.gen";
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
import { submitHandler, useAppForm } from "@/lib/hooks/use-app-form";

const linkSchema = z.object({
  url: z
    .string()
    .trim()
    .url("Enter a full URL such as https://example.com.")
    .refine((value) => /^https?:\/\//i.test(value), "Use an http or https URL."),
  title: z.string().trim().max(120, "Keep the title under 120 characters."),
});

type TProps = {
  links: EntityLink[];
  canEdit: boolean;
  onAdd: (link: { url: string; title: string }) => Promise<unknown>;
  onRemove: (url: string) => void;
};

function hostOf(url: string) {
  try {
    return new URL(url).host.replace(/^www\./, "");
  } catch {
    return url;
  }
}

/** Links of a project or initiative: specs, designs, dashboards. */
export function EntityLinks({ links, canEdit, onAdd, onRemove }: TProps) {
  if (!canEdit && links.length === 0) return null;
  return (
    <div className="flex w-full min-w-0 flex-col gap-1">
      <div className="flex h-8 items-center gap-2">
        <span className="text-muted-foreground flex-1 text-sm">Links</span>
        {canEdit && <AddLinkDialog onAdd={onAdd} />}
      </div>
      {links.map((link) => (
        <div
          key={link.id}
          className="group/link has-hover:hover:bg-accent flex h-8 min-w-0 items-center gap-2 rounded-md px-2 text-sm"
        >
          <LinkIcon className="text-muted-foreground size-3.5 shrink-0" />
          <a
            href={link.url}
            target="_blank"
            rel="noreferrer"
            title={link.url}
            className="min-w-0 flex-1 truncate"
          >
            {link.label || hostOf(link.url)}
          </a>
          {canEdit && (
            <Button
              variant="ghost"
              size="icon"
              forceMinSize={false}
              aria-label={`Remove link ${link.label || link.url}`}
              className="size-6 opacity-0 group-hover/link:opacity-100 focus-visible:opacity-100"
              onClick={() => onRemove(link.url)}
            >
              <XIcon className="size-3.5" />
            </Button>
          )}
        </div>
      ))}
    </div>
  );
}

function AddLinkDialog({ onAdd }: { onAdd: TProps["onAdd"] }) {
  const [isOpen, setIsOpen] = useState(false);
  return (
    <Dialog open={isOpen} onOpenChange={setIsOpen}>
      <DialogTrigger
        render={
          <Button variant="ghost" size="icon" aria-label="Add link">
            <PlusIcon className="size-4" />
          </Button>
        }
      />
      <DialogContent className="w-md max-w-full">
        <DialogHeader>
          <DialogTitle>Add link</DialogTitle>
        </DialogHeader>
        {isOpen && <AddLinkForm onAdd={onAdd} onDone={() => setIsOpen(false)} />}
      </DialogContent>
    </Dialog>
  );
}

function AddLinkForm({ onAdd, onDone }: { onAdd: TProps["onAdd"]; onDone: () => void }) {
  const form = useAppForm({
    defaultValues: { url: "", title: "" },
    validators: { onSubmit: linkSchema },
    onSubmit: async ({ value }) => {
      const parsed = linkSchema.parse(value);
      await onAdd({ url: parsed.url, title: parsed.title || hostOf(parsed.url) });
      onDone();
    },
  });
  return (
    <form className="flex flex-col gap-4" onSubmit={submitHandler(form)} noValidate>
      <form.AppField
        name="url"
        children={(field) => <field.TextField label="URL" placeholder="https://" autoFocus />}
      />
      <form.AppField
        name="title"
        children={(field) => <field.TextField label="Title" placeholder="Optional" />}
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
          <form.SubmitButton size="sm">Add link</form.SubmitButton>
        </form.AppForm>
      </DialogFooter>
    </form>
  );
}

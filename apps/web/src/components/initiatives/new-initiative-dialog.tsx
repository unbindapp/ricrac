import { useNavigate } from "@tanstack/react-router";
import { type ReactElement, useState } from "react";

import { InitiativeParentPicker } from "@/components/initiatives/initiative-parent-picker";
import { InitiativeStatusPicker } from "@/components/initiatives/initiative-status";
import {
  PersonPicker,
  toUserRef,
  useMentionUsers,
  useWorkspaceUserList,
} from "@/components/projects/people";
import { fromResolution, type TResolution } from "@/components/projects/project-dates";
import { PlannedDatePicker } from "@/components/projects/project-pickers";
import { MarkdownField } from "@/components/shared/markdown-editor";
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
import { useEditorUploads } from "@/lib/queries/uploads";
import { entitySlug } from "@/lib/helpers/entity-slug";
import { submitHandler, useAppForm } from "@/lib/hooks/use-app-form";
import {
  InitiativeFormSchema,
  type TInitiativeStatus,
  useSaveInitiativeMutation,
} from "@/lib/queries/initiatives";

type TProps = {
  trigger: ReactElement;
  /** Creates a sub-initiative of this one. */
  parentId?: string;
};

export function NewInitiativeDialog({ trigger, parentId }: TProps) {
  const [isOpen, setIsOpen] = useState(false);
  return (
    <Dialog open={isOpen} onOpenChange={setIsOpen}>
      <DialogTrigger render={trigger} />
      <DialogContent className="w-2xl max-w-full">
        <DialogHeader>
          <DialogTitle>{parentId ? "New sub-initiative" : "New initiative"}</DialogTitle>
        </DialogHeader>
        {isOpen && <NewInitiativeForm parentId={parentId} onDone={() => setIsOpen(false)} />}
      </DialogContent>
    </Dialog>
  );
}

function NewInitiativeForm({ parentId, onDone }: { parentId?: string; onDone: () => void }) {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const { users } = useWorkspaceUserList();
  const mentionUsers = useMentionUsers();
  const uploads = useEditorUploads(workspace.id);
  const { mutateAsync: saveInitiative } = useSaveInitiativeMutation(workspace.id);
  const [description, setDescription] = useState("");
  const [status, setStatus] = useState<TInitiativeStatus>("planned");
  const [ownerId, setOwnerId] = useState<string | null>(null);
  const [parent, setParent] = useState<string | null>(parentId ?? null);
  const [target, setTarget] = useState<{ date: string | null; resolution: TResolution }>({
    date: null,
    resolution: "day",
  });
  const owner = users.find((user) => user.id === ownerId);

  const form = useAppForm({
    defaultValues: { name: "", summary: "" },
    validators: { onSubmit: InitiativeFormSchema },
    onSubmit: async ({ value }) => {
      const parsed = InitiativeFormSchema.parse(value);
      const initiative = await saveInitiative({
        name: parsed.name,
        summary: parsed.summary || undefined,
        description: description.trim() || undefined,
        status,
        owner: ownerId ?? undefined,
        parent: parent ?? undefined,
        targetDate: target.date ?? undefined,
        targetDateResolution: target.date ? fromResolution(target.resolution) : undefined,
      });
      onDone();
      await navigate({
        to: "/$workspace/initiative/$slug",
        params: { workspace: workspace.urlKey, slug: entitySlug(initiative) },
      });
    },
  });

  return (
    <form className="flex flex-col gap-4" onSubmit={submitHandler(form)} noValidate>
      <form.AppField name="name" children={(field) => <field.TextField label="Name" autoFocus />} />
      <form.AppField
        name="summary"
        children={(field) => <field.TextField label="Summary" placeholder="The goal in one line" />}
      />
      <MarkdownField
        label="Description"
        value={description}
        onChange={setDescription}
        users={mentionUsers}
        {...uploads.editorProps}
        placeholder="Why it matters and how you'll know it's done…"
      />
      <div className="flex flex-wrap items-center gap-2">
        <InitiativeStatusPicker appearance="compact" status={status} onChange={setStatus} />
        <PersonPicker
          role="Owner"
          appearance="compact"
          user={owner ? toUserRef(owner) : null}
          onChange={setOwnerId}
        />
        <PlannedDatePicker
          appearance="compact"
          label="Target date"
          edge="end"
          date={target.date}
          resolution={target.resolution}
          onChange={setTarget}
        />
        <InitiativeParentPicker appearance="compact" parentId={parent} onChange={setParent} />
      </div>
      <DialogFooter>
        <DialogClose
          render={
            <Button type="button" variant="outline" size="sm">
              Cancel
            </Button>
          }
        />
        <form.AppForm>
          <form.SubmitButton size="sm">Create initiative</form.SubmitButton>
        </form.AppForm>
      </DialogFooter>
    </form>
  );
}

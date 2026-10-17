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
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { colorFromId, hexColorPattern } from "@/lib/helpers/colors";
import { submitHandler, useAppForm } from "@/lib/hooks/use-app-form";
import { type TLabel, type TLabelGroupType, useSaveLabelMutation } from "@/lib/queries/labels";

const NO_GROUP = "none";

const labelFormSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, "Name is required.")
    .max(80, "Keep the name under 80 characters.")
    .refine((name) => !name.includes("/"), "Names can't contain a slash."),
  color: z.string().regex(hexColorPattern, "Pick a color."),
  description: z.string().trim().max(500, "Keep the description under 500 characters."),
  parent: z.string(),
  groupType: z.enum(["singleSelect", "multiSelect"]),
});

const groupTypeItems: { value: TLabelGroupType; label: string }[] = [
  { value: "singleSelect", label: "Single select: an issue takes one of its labels" },
  { value: "multiSelect", label: "Multi select: an issue takes any of its labels" },
];

export type TLabelDialogTarget =
  { mode: "create"; isGroup: boolean; parent?: string } | { mode: "edit"; label: TLabel };

type TProps = {
  /** What the dialog creates or edits; null keeps it closed. */
  target: TLabelDialogTarget | null;
  onClose: () => void;
  /** The team the label belongs to; null for a workspace label. */
  team: { id: string; name: string } | null;
  /** The groups a label can go in. */
  groups: TLabel[];
};

export function LabelDialog({ target, onClose, team, groups }: TProps) {
  const isGroup = target?.mode === "edit" ? target.label.isGroup : !!target?.isGroup;
  const noun = isGroup ? "group" : "label";
  const scope = team ? `Only ${team.name} uses it.` : "Every team can use it.";
  return (
    <Dialog open={target !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="w-md max-w-full">
        <DialogHeader>
          <DialogTitle>{target?.mode === "edit" ? `Edit ${noun}` : `New ${noun}`}</DialogTitle>
          {target?.mode === "create" && <DialogDescription>{scope}</DialogDescription>}
        </DialogHeader>
        {target && (
          <LabelForm
            target={target}
            team={team}
            groups={groups}
            isGroup={isGroup}
            onDone={onClose}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function LabelForm({
  target,
  team,
  groups,
  isGroup,
  onDone,
}: Omit<TProps, "target" | "onClose"> & {
  target: TLabelDialogTarget;
  isGroup: boolean;
  onDone: () => void;
}) {
  const { workspace } = useWorkspace();
  const { mutateAsync: saveLabel } = useSaveLabelMutation(workspace.id);
  const label = target.mode === "edit" ? target.label : null;
  const groupItems = [
    { value: NO_GROUP, label: "No group" },
    ...groups.map((group) => ({
      value: group.id,
      label: group.team || !team ? group.name : `${group.name} (workspace)`,
    })),
  ];
  const form = useAppForm({
    defaultValues: {
      name: label?.name ?? "",
      color: label?.color ?? colorFromId(crypto.randomUUID()),
      description: label?.description ?? "",
      parent: label?.parent?.id ?? (target.mode === "create" && target.parent) ?? NO_GROUP,
      groupType: label?.groupType ?? ("singleSelect" as TLabelGroupType),
    },
    validators: { onSubmit: labelFormSchema },
    onSubmit: async ({ value }) => {
      const parsed = labelFormSchema.parse(value);
      const fields = {
        name: parsed.name,
        color: parsed.color,
        description: parsed.description || null,
        ...(isGroup
          ? { groupType: parsed.groupType }
          : { parent: parsed.parent === NO_GROUP ? null : parsed.parent }),
      };
      if (label) await saveLabel({ id: label.id, ...fields });
      else await saveLabel({ ...fields, isGroup, team: team?.id ?? null });
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
            autoFocus
            placeholder={isGroup ? "Type" : "Bug"}
            autoComplete="off"
          />
        )}
      />
      <form.AppField name="color" children={(field) => <field.ColorField label="Color" />} />
      <form.AppField
        name="description"
        children={(field) => (
          <field.TextareaField label="Description" rows={2} placeholder="Optional" />
        )}
      />
      {isGroup ? (
        <form.AppField
          name="groupType"
          children={(field) => <field.SelectField label="Selection" items={groupTypeItems} />}
        />
      ) : (
        groupItems.length > 1 && (
          <form.AppField
            name="parent"
            children={(field) => <field.SelectField label="Group" items={groupItems} />}
          />
        )
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
          <form.SubmitButton size="sm">
            {label ? "Save" : isGroup ? "Create group" : "Create label"}
          </form.SubmitButton>
        </form.AppForm>
      </DialogFooter>
    </form>
  );
}

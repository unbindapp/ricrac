import { useQuery } from "@tanstack/react-query";
import {
  ArchiveIcon,
  ArchiveRestoreIcon,
  EllipsisIcon,
  FolderIcon,
  PencilIcon,
  PlusIcon,
  TagIcon,
  Trash2Icon,
} from "lucide-react";
import { type ReactElement, useState } from "react";
import { z } from "zod";

import type { ProjectLabel } from "@/api/ops.gen";
import { EmptyState } from "@/components/layout/empty-state";
import { ErrorCard } from "@/components/layout/error-card";
import { List } from "@/components/layout/list";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { SettingsSection } from "@/components/layout/settings-section";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { Badge } from "@/components/ui/badge";
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
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { PRESET_COLORS } from "@/lib/helpers/colors";
import { submitHandler, type TSelectFieldItem, useAppForm } from "@/lib/hooks/use-app-form";
import {
  projectLabelsQuery,
  type TProjectLabelAction,
  useProjectLabelActionMutation,
  useSaveProjectLabelMutation,
} from "@/lib/queries/projects";

const NO_GROUP = "__none__";

const labelSchema = z.object({
  name: z.string().trim().min(1, "Name is required.").max(64, "Keep the name under 64 characters."),
  color: z.string(),
  description: z.string().trim().max(500, "Keep the description under 500 characters."),
  parent: z.string(),
});

/** Labels for projects, separate from issue labels; groups hold related ones. */
export function ProjectLabelManager() {
  const { workspace } = useWorkspace();
  const { data, isPending, error, refetch } = useQuery(projectLabelsQuery(workspace.id));
  const labels = data ?? [];
  const groups = labels.filter((label) => label.isGroup);
  const loose = labels.filter((label) => !label.isGroup && !label.parent);

  return (
    <SettingsSection
      title="Project labels"
      description="Tag projects by theme, customer or quarter. Retired labels stay on projects but can't be added."
      actions={
        <>
          <ProjectLabelDialog
            isGroup
            groups={groups}
            trigger={
              <Button variant="ghost" size="sm">
                <FolderIcon className="size-4" />
                New group
              </Button>
            }
          />
          <ProjectLabelDialog
            groups={groups}
            trigger={
              <Button variant="outline" size="sm">
                <PlusIcon className="size-4" />
                New label
              </Button>
            }
          />
        </>
      }
    >
      {isPending && <ListSkeleton rows={4} />}
      {error && <ErrorCard message={error.message} onRetry={() => void refetch()} />}
      {!isPending && !error && labels.length === 0 && (
        <EmptyState icon={TagIcon} title="No project labels yet" />
      )}
      {labels.length > 0 && (
        <List>
          {groups.map((group) => (
            <div key={group.id} role="presentation" className="contents">
              <LabelRow label={group} groups={groups} />
              {labels
                .filter((label) => label.parent?.id === group.id)
                .map((label) => (
                  <LabelRow key={label.id} label={label} groups={groups} indent />
                ))}
            </div>
          ))}
          {loose.map((label) => (
            <LabelRow key={label.id} label={label} groups={groups} />
          ))}
        </List>
      )}
    </SettingsSection>
  );
}

function LabelRow({
  label,
  groups,
  indent,
}: {
  label: ProjectLabel;
  groups: ProjectLabel[];
  indent?: boolean;
}) {
  const { workspace } = useWorkspace();
  const { mutate: act, mutateAsync: actAsync } = useProjectLabelActionMutation(workspace.id);
  const [isDeleting, setIsDeleting] = useState(false);
  const run = (action: TProjectLabelAction) => act({ id: label.id, action });
  return (
    <div
      role="listitem"
      data-retired={Boolean(label.retiredAt) || undefined}
      className="data-retired:text-muted-foreground flex h-9 w-full items-center gap-2 border-b px-3 text-sm last:border-b-0"
    >
      <span
        data-indent={indent || undefined}
        className="flex min-w-0 flex-1 items-center gap-2 data-indent:pl-6"
      >
        {label.isGroup ? (
          <FolderIcon className="text-muted-foreground size-4 shrink-0" />
        ) : (
          <span
            aria-hidden
            className="size-2.5 shrink-0 rounded-full"
            style={{ backgroundColor: label.color }}
          />
        )}
        <span className="truncate font-medium">{label.name}</span>
        {label.description && (
          <span className="text-muted-foreground hidden truncate text-xs sm:inline">
            {label.description}
          </span>
        )}
        {label.retiredAt && <Badge variant="muted">Retired</Badge>}
      </span>
      {!label.isGroup && label.projectCount > 0 && (
        <span className="text-muted-foreground shrink-0 text-xs tabular-nums">
          {label.projectCount} project{label.projectCount === 1 ? "" : "s"}
        </span>
      )}
      <ProjectLabelDialog
        label={label}
        isGroup={label.isGroup}
        groups={groups}
        trigger={
          <Button variant="ghost" size="icon" aria-label={`Edit ${label.name}`}>
            <PencilIcon className="size-4" />
          </Button>
        }
      />
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button variant="ghost" size="icon" aria-label={`More actions for ${label.name}`}>
              <EllipsisIcon className="size-4" />
            </Button>
          }
        />
        <DropdownMenuContent align="end" className="w-44">
          {!label.isGroup && (
            <DropdownMenuItem onClick={() => run(label.retiredAt ? "restore" : "retire")}>
              {label.retiredAt ? (
                <ArchiveRestoreIcon className="size-4" />
              ) : (
                <ArchiveIcon className="size-4" />
              )}
              {label.retiredAt ? "Restore" : "Retire"}
            </DropdownMenuItem>
          )}
          <DropdownMenuItem className="text-destructive" onClick={() => setIsDeleting(true)}>
            <Trash2Icon className="size-4" />
            Delete
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <ConfirmDialog
        open={isDeleting}
        onOpenChange={setIsDeleting}
        title={`Delete ${label.name}`}
        description={
          label.isGroup
            ? "Its labels move out of the group."
            : "It comes off every project. Retire it instead to keep it on existing projects."
        }
        confirmLabel={label.isGroup ? "Delete group" : "Delete label"}
        onConfirm={() => actAsync({ id: label.id, action: "delete" })}
      />
    </div>
  );
}

function ProjectLabelDialog({
  trigger,
  label,
  isGroup = false,
  groups,
}: {
  trigger: ReactElement;
  label?: ProjectLabel;
  isGroup?: boolean;
  groups: ProjectLabel[];
}) {
  const [isOpen, setIsOpen] = useState(false);
  const noun = isGroup ? "group" : "label";
  return (
    <Dialog open={isOpen} onOpenChange={setIsOpen}>
      <DialogTrigger render={trigger} />
      <DialogContent className="w-md max-w-full">
        <DialogHeader>
          <DialogTitle>{label ? `Edit ${noun}` : `New ${noun}`}</DialogTitle>
        </DialogHeader>
        {isOpen && (
          <ProjectLabelForm
            label={label}
            isGroup={isGroup}
            groups={groups}
            onDone={() => setIsOpen(false)}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function ProjectLabelForm({
  label,
  isGroup,
  groups,
  onDone,
}: {
  label?: ProjectLabel;
  isGroup: boolean;
  groups: ProjectLabel[];
  onDone: () => void;
}) {
  const { workspace } = useWorkspace();
  const { mutateAsync: saveLabel } = useSaveProjectLabelMutation(workspace.id);
  const groupItems: TSelectFieldItem[] = [
    { value: NO_GROUP, label: "No group" },
    ...groups.map((group) => ({ value: group.id, label: group.name })),
  ];
  const form = useAppForm({
    defaultValues: {
      name: label?.name ?? "",
      color: label?.color ?? PRESET_COLORS[1],
      description: label?.description ?? "",
      parent: label?.parent?.id ?? NO_GROUP,
    },
    validators: { onSubmit: labelSchema },
    onSubmit: async ({ value }) => {
      const parsed = labelSchema.parse(value);
      const fields = {
        name: parsed.name,
        color: parsed.color,
        description: parsed.description || null,
        ...(isGroup ? {} : { parent: parsed.parent === NO_GROUP ? null : parsed.parent }),
      };
      await saveLabel(label ? { id: label.id, ...fields } : { isGroup, ...fields });
      onDone();
    },
  });
  return (
    <form className="flex flex-col gap-4" onSubmit={submitHandler(form)} noValidate>
      <div className="flex items-start gap-4">
        <form.AppField name="color" children={(field) => <field.ColorField label="Color" />} />
        <div className="min-w-0 flex-1">
          <form.AppField
            name="name"
            children={(field) => <field.TextField label="Name" autoFocus />}
          />
        </div>
      </div>
      <form.AppField
        name="description"
        children={(field) => <field.TextareaField label="Description" rows={2} />}
      />
      {!isGroup && groups.length > 0 && (
        <form.AppField
          name="parent"
          children={(field) => <field.SelectField label="Group" items={groupItems} />}
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
          <form.SubmitButton size="sm">
            {label ? "Save" : `Create ${isGroup ? "group" : "label"}`}
          </form.SubmitButton>
        </form.AppForm>
      </DialogFooter>
    </form>
  );
}

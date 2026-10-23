import { useState } from "react";

import {
  ArchiveIcon,
  ArchiveRestoreIcon,
  EllipsisIcon,
  PencilIcon,
  PlusIcon,
  Trash2Icon,
} from "@/components/icons";
import type { TLabelDialogTarget } from "@/components/labels/label-dialog";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { toast } from "@/components/ui/toast";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import {
  type TLabel,
  useDeleteLabelMutation,
  useRestoreLabelMutation,
  useRetireLabelMutation,
} from "@/lib/queries/labels";

type TProps = {
  label: TLabel;
  onOpen: (target: TLabelDialogTarget) => void;
};

function deleteDescription(label: TLabel) {
  const uses =
    label.issueCount === 0
      ? "No issue has it."
      : `It comes off ${label.issueCount} ${label.issueCount === 1 ? "issue" : "issues"}.`;
  if (label.isGroup) return `The group and its labels are deleted for good. ${uses}`;
  return `${uses} To keep it on those issues but stop new uses, retire it instead.`;
}

export function LabelRowMenu({ label, onOpen }: TProps) {
  const { workspace } = useWorkspace();
  const [isDeleting, setIsDeleting] = useState(false);
  const retire = useRetireLabelMutation(workspace.id);
  const restore = useRestoreLabelMutation(workspace.id);
  const { mutateAsync: deleteLabel } = useDeleteLabelMutation(workspace.id);
  const noun = label.isGroup ? "group" : "label";

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button
              variant="ghost"
              size="icon"
              forceMinSize={false}
              aria-label={`Actions for ${label.name}`}
              className="text-muted-foreground dank:mr-0 dank:size-10 -mr-1 size-7"
            >
              <EllipsisIcon className="size-4" />
            </Button>
          }
        />
        <DropdownMenuContent align="end" className="w-48">
          <DropdownMenuItem onClick={() => onOpen({ mode: "edit", label })}>
            <PencilIcon className="size-4" />
            Edit {noun}
          </DropdownMenuItem>
          {label.isGroup && !label.retiredAt && (
            <DropdownMenuItem
              onClick={() => onOpen({ mode: "create", isGroup: false, parent: label.id })}
            >
              <PlusIcon className="size-4" />
              Add label to group
            </DropdownMenuItem>
          )}
          {label.retiredAt ? (
            <DropdownMenuItem onClick={() => restore.mutate(label.id)}>
              <ArchiveRestoreIcon className="size-4" />
              Restore {noun}
            </DropdownMenuItem>
          ) : (
            <DropdownMenuItem
              onClick={() =>
                retire.mutate(label.id, {
                  onSuccess: () =>
                    toast.add({
                      type: "success",
                      title: `Retired ${label.name}`,
                      description: "It stays on its issues but can't be added anymore.",
                    }),
                })
              }
            >
              <ArchiveIcon className="size-4" />
              Retire {noun}
            </DropdownMenuItem>
          )}
          <DropdownMenuSeparator />
          <DropdownMenuItem className="text-destructive" onClick={() => setIsDeleting(true)}>
            <Trash2Icon className="size-4" />
            Delete {noun}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <ConfirmDialog
        open={isDeleting}
        onOpenChange={setIsDeleting}
        title={`Delete ${label.name}`}
        description={deleteDescription(label)}
        confirmLabel={`Delete ${noun}`}
        onConfirm={() => deleteLabel(label.id)}
      />
    </>
  );
}

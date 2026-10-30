import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import type { Milestone } from "@/api/ops.gen";
import {
  CircleCheckIcon,
  EllipsisIcon,
  PencilIcon,
  RotateCwIcon,
  Trash2Icon,
} from "@/components/icons";
import { MilestoneDialog } from "@/components/projects/milestone-dialog";
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
import { useDeleteMilestoneMutation, useSaveMilestoneMutation } from "@/lib/queries/milestones";

/** Edits, closes or reopens, and deletes a milestone; deleting goes back to the milestones. */
export function MilestoneMenu({ milestone }: { milestone: Milestone }) {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const [isEditing, setIsEditing] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const { mutate: save } = useSaveMilestoneMutation(workspace.id);
  const { mutateAsync: remove } = useDeleteMilestoneMutation(workspace.id);
  const isClosed = Boolean(milestone.closedAt);

  const setClosed = (closed: boolean) =>
    save(
      { id: milestone.id, closed },
      {
        onSuccess: () =>
          toast.add({
            type: "success",
            title: closed ? `Closed ${milestone.name}` : `Reopened ${milestone.name}`,
          }),
      },
    );

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button variant="ghost" size="icon" aria-label={`Actions for ${milestone.name}`}>
              <EllipsisIcon className="size-4" />
            </Button>
          }
        />
        <DropdownMenuContent align="end" className="w-48">
          <DropdownMenuItem onClick={() => setIsEditing(true)}>
            <PencilIcon className="size-4" />
            Edit milestone
          </DropdownMenuItem>
          {isClosed ? (
            <DropdownMenuItem onClick={() => setClosed(false)}>
              <RotateCwIcon className="size-4" />
              Reopen milestone
            </DropdownMenuItem>
          ) : (
            <DropdownMenuItem onClick={() => setClosed(true)}>
              <CircleCheckIcon className="size-4" />
              Close milestone
            </DropdownMenuItem>
          )}
          <DropdownMenuSeparator />
          <DropdownMenuItem className="text-destructive" onClick={() => setIsDeleting(true)}>
            <Trash2Icon className="size-4" />
            Delete milestone
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <MilestoneDialog milestone={milestone} open={isEditing} onOpenChange={setIsEditing} />
      <ConfirmDialog
        open={isDeleting}
        onOpenChange={setIsDeleting}
        title={`Delete ${milestone.name}`}
        description={deleteDescription(milestone)}
        confirmLabel="Delete milestone"
        onConfirm={async () => {
          await remove(milestone.id);
          toast.add({ type: "success", title: `Deleted ${milestone.name}` });
          await navigate({
            to: "/$workspace/milestones",
            params: { workspace: workspace.urlKey },
          });
        }}
      />
    </>
  );
}

function deleteDescription(milestone: Milestone) {
  const issues = "Its issues stay where they are, without a milestone.";
  switch (milestone.github.length) {
    case 0:
      return issues;
    case 1:
      return `${issues} Its milestone on GitHub is deleted too.`;
    default:
      return `${issues} Its milestones on GitHub are deleted too.`;
  }
}

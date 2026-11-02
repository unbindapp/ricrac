import { useState } from "react";

import { BoardDialog } from "@/components/boards/board-dialog";
import { EllipsisIcon, LinkIcon, PencilIcon, Trash2Icon } from "@/components/icons";
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
import { type TBoard, useDeleteBoardMutation } from "@/lib/queries/boards";

type TProps = {
  board: TBoard;
  onDeleted?: () => void;
};

export function BoardMenu({ board, onDeleted }: TProps) {
  const { workspace, canEdit } = useWorkspace();
  const [dialog, setDialog] = useState<"edit" | "delete" | null>(null);
  const { mutateAsync: deleteBoard } = useDeleteBoardMutation(workspace.id);
  const close = () => setDialog(null);

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button variant="ghost" size="icon" aria-label={`Actions for ${board.name}`}>
              <EllipsisIcon className="size-4" />
            </Button>
          }
        />
        <DropdownMenuContent align="end" className="w-52">
          {canEdit && (
            <DropdownMenuItem onClick={() => setDialog("edit")}>
              <PencilIcon className="size-4" />
              Edit board
            </DropdownMenuItem>
          )}
          <DropdownMenuItem
            onClick={() =>
              void navigator.clipboard
                .writeText(board.url)
                .then(() => toast.add({ type: "success", title: "Link copied" }))
            }
          >
            <LinkIcon className="size-4" />
            Copy link
          </DropdownMenuItem>
          {canEdit && (
            <>
              <DropdownMenuSeparator />
              <DropdownMenuItem className="text-destructive" onClick={() => setDialog("delete")}>
                <Trash2Icon className="size-4" />
                Delete board
              </DropdownMenuItem>
            </>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
      <BoardDialog target={dialog === "edit" ? { mode: "edit", board } : null} onClose={close} />
      <ConfirmDialog
        open={dialog === "delete"}
        onOpenChange={(open) => !open && close()}
        title={`Delete ${board.name}`}
        description="The board is deleted for everyone in the workspace. Its issues stay as they are."
        confirmLabel="Delete board"
        onConfirm={async () => {
          await deleteBoard(board.id);
          onDeleted?.();
        }}
      />
    </>
  );
}

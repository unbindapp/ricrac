import { useQuery } from "@tanstack/react-query";

import { KanbanIcon } from "@/components/icons";
import {
  DropdownMenuItem,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
} from "@/components/ui/dropdown-menu";
import { toast } from "@/components/ui/toast";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { boardsQuery, type TBoardSubject, useAddToBoardMutation } from "@/lib/queries/boards";

/** A submenu of the workspace's boards, each with its columns, that puts the subject there. */
export function AddToBoardMenu({ subject }: { subject: TBoardSubject }) {
  return (
    <DropdownMenuSub>
      <DropdownMenuSubTrigger>
        <KanbanIcon className="size-4" />
        Add to board
      </DropdownMenuSubTrigger>
      <DropdownMenuSubContent className="w-56">
        <BoardItems subject={subject} />
      </DropdownMenuSubContent>
    </DropdownMenuSub>
  );
}

function BoardItems({ subject }: { subject: TBoardSubject }) {
  const { workspace } = useWorkspace();
  const { data: boards, isPending, error } = useQuery(boardsQuery(workspace.id));
  const { mutate: add } = useAddToBoardMutation(workspace.id);

  if (isPending) return <DropdownMenuItem disabled>Loading…</DropdownMenuItem>;
  if (error) return <DropdownMenuItem disabled>Could not load boards</DropdownMenuItem>;
  if (boards.length === 0) return <DropdownMenuItem disabled>No boards yet</DropdownMenuItem>;
  return boards.map((board) => (
    <DropdownMenuSub key={board.id}>
      <DropdownMenuSubTrigger>
        <span className="min-w-0 flex-1 truncate">{board.name}</span>
      </DropdownMenuSubTrigger>
      <DropdownMenuSubContent className="w-48">
        {board.columns.map((column) => (
          <DropdownMenuItem
            key={column.id}
            onClick={() =>
              add(
                { board, subject, column: column.id },
                {
                  onSuccess: () =>
                    toast.add({ type: "success", title: `Added to ${board.name}, ${column.name}` }),
                },
              )
            }
          >
            <span className="min-w-0 flex-1 truncate">{column.name}</span>
          </DropdownMenuItem>
        ))}
      </DropdownMenuSubContent>
    </DropdownMenuSub>
  ));
}

import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";

import { BoardMenu } from "@/components/boards/board-menu";
import { KanbanIcon, PlusIcon } from "@/components/icons";
import { List } from "@/components/layout/list";
import { ListStates } from "@/components/layout/list-states";
import { Button } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { boardParam, boardsQuery, type TBoard } from "@/lib/queries/boards";

export function BoardList({ onCreate }: { onCreate?: () => void }) {
  const { workspace } = useWorkspace();
  const { data, isPending, error, refetch } = useQuery(boardsQuery(workspace.id));
  const boards = data ?? [];

  return (
    <ListStates
      isPending={isPending}
      error={error}
      onRetry={() => void refetch()}
      isEmpty={boards.length === 0}
      empty={{
        icon: KanbanIcon,
        title: "No boards yet",
        description:
          "A board has columns of your own, like Ideas, Design and Ready, for issues at any status.",
        action: onCreate && (
          <Button size="sm" variant="outline" onClick={onCreate}>
            <PlusIcon className="size-4" />
            New board
          </Button>
        ),
      }}
    >
      <List>
        {boards.map((board) => (
          <BoardRow key={board.id} board={board} />
        ))}
      </List>
    </ListStates>
  );
}

function BoardRow({ board }: { board: TBoard }) {
  const { workspace } = useWorkspace();
  const cards = board.columns.reduce((sum, column) => sum + column.issueCount, 0);
  return (
    <div
      role="listitem"
      data-interactive
      className="has-hover:hover:bg-accent/50 flex h-11 w-full items-center gap-2 border-b px-3 text-sm last:border-b-0"
    >
      <KanbanIcon className="text-muted-foreground size-4 shrink-0" />
      <Link
        to="/$workspace/board/$board_id"
        params={{ workspace: workspace.urlKey, board_id: boardParam(board) }}
        className="dank:flex-col dank:items-stretch dank:gap-0 dank:before:absolute dank:before:inset-0 flex min-w-0 flex-1 items-baseline gap-2 outline-hidden focus-visible:underline"
      >
        <span className="dank:font-normal truncate font-medium">{board.name}</span>
        <span
          data-slot="list-supporting"
          className="text-muted-foreground dank:block hidden truncate text-xs sm:inline"
        >
          {board.description ?? board.columns.map((column) => column.name).join(" · ")}
        </span>
      </Link>
      <span className="text-muted-foreground text-xs">
        {cards} {cards === 1 ? "issue" : "issues"}
      </span>
      <BoardMenu board={board} />
    </div>
  );
}

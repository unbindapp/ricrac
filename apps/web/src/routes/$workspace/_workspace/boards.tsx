import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { BoardDialog } from "@/components/boards/board-dialog";
import { BoardList } from "@/components/boards/board-list";
import { KanbanIcon } from "@/components/icons";
import { CreateButton } from "@/components/layout/create-button";
import { PageContainer } from "@/components/layout/page-container";
import { PageHeader } from "@/components/layout/page-header";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { boardParam } from "@/lib/queries/boards";

export const Route = createFileRoute("/$workspace/_workspace/boards")({
  component: BoardsPage,
});

function BoardsPage() {
  const { workspace, canEdit } = useWorkspace();
  const navigate = useNavigate();
  const [isCreating, setIsCreating] = useState(false);
  const create = canEdit ? () => setIsCreating(true) : undefined;

  return (
    <PageContainer>
      <PageHeader
        title="Boards"
        icon={<KanbanIcon />}
        actions={create && <CreateButton label="New board" onClick={create} />}
      />
      <BoardList onCreate={create} />
      <BoardDialog
        target={isCreating ? { mode: "create" } : null}
        onClose={() => setIsCreating(false)}
        onSaved={(board) =>
          void navigate({
            to: "/$workspace/board/$board_id",
            params: { workspace: workspace.urlKey, board_id: boardParam(board) },
          })
        }
      />
    </PageContainer>
  );
}

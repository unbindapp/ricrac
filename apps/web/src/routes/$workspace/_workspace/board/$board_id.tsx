import { useQuery } from "@tanstack/react-query";
import { createFileRoute, useNavigate } from "@tanstack/react-router";

import { BoardColumns } from "@/components/boards/board-columns";
import { BoardMenu } from "@/components/boards/board-menu";
import { KanbanIcon } from "@/components/icons";
import { IssuePeekHost } from "@/components/issues/issue-peek";
import type { TBreadcrumbItem } from "@/components/layout/breadcrumb";
import { ErrorCard } from "@/components/layout/error-card";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { PageContainer } from "@/components/layout/page-container";
import { PageHeader } from "@/components/layout/page-header";
import { EntityNotFound } from "@/components/navigation/not-found-template";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { boardQuery } from "@/lib/queries/boards";
import { isApiError } from "@/lib/server/errors";

export const Route = createFileRoute("/$workspace/_workspace/board/$board_id")({
  component: BoardPage,
});

function BoardPage() {
  const { board_id } = Route.useParams();
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const { data: board, isPending, error, refetch } = useQuery(boardQuery(workspace.id, board_id));
  const breadcrumb: TBreadcrumbItem[] = [
    {
      label: "Boards",
      link: { to: "/$workspace/boards", params: { workspace: workspace.urlKey } },
    },
  ];

  if (board) {
    return (
      <IssuePeekHost>
        <PageContainer>
          <PageHeader
            title={board.name}
            icon={<KanbanIcon />}
            description={board.description ?? undefined}
            breadcrumb={breadcrumb}
            actions={
              <BoardMenu
                board={board}
                onDeleted={() =>
                  void navigate({
                    to: "/$workspace/boards",
                    params: { workspace: workspace.urlKey },
                  })
                }
              />
            }
          />
          <BoardColumns board={board} />
        </PageContainer>
      </IssuePeekHost>
    );
  }
  const isMissing = isApiError(error) && error.status >= 400 && error.status < 500;
  return (
    <PageContainer>
      <PageHeader title="Board" icon={<KanbanIcon />} breadcrumb={breadcrumb} />
      {isPending && <ListSkeleton />}
      {isMissing && (
        <EntityNotFound Icon={KanbanIcon} description="This board doesn't exist anymore." />
      )}
      {error && !isMissing && <ErrorCard message={error.message} onRetry={() => void refetch()} />}
    </PageContainer>
  );
}

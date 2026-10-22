import type { ReactNode } from "react";

import type { Team } from "@/api/ops.gen";
import { ErrorCard } from "@/components/layout/error-card";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { PageContainer } from "@/components/layout/page-container";
import { useTeams } from "@/components/workspace/teams-provider";

/** Renders a page once the workspace's team, which holds its issues, statuses and cycles, loads. */
export function WorkspaceTeamGate({ children }: { children: (team: Team) => ReactNode }) {
  const { team, isPending, error, refetch } = useTeams();
  if (isPending) {
    return (
      <PageContainer>
        <ListSkeleton />
      </PageContainer>
    );
  }
  if (error || !team) {
    return (
      <PageContainer>
        <ErrorCard
          message={error?.message ?? "This workspace has no issues set up."}
          onRetry={refetch}
        />
      </PageContainer>
    );
  }
  return children(team);
}

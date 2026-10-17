import { useQuery } from "@tanstack/react-query";
import { BoxIcon } from "lucide-react";
import { type ReactNode } from "react";

import type { ProjectDetail } from "@/api/ops.gen";
import { ErrorCard } from "@/components/layout/error-card";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { PageContainer } from "@/components/layout/page-container";
import { EntityNotFound } from "@/components/navigation/not-found-template";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { isMissing, projectQuery } from "@/lib/queries/projects";

export function ProjectLoader({
  slug,
  children,
}: {
  slug: string;
  children: (project: ProjectDetail) => ReactNode;
}) {
  const { workspace } = useWorkspace();
  const { data, isPending, error, refetch } = useQuery(projectQuery(workspace.id, slug));
  if (isPending) {
    return (
      <PageContainer>
        <ListSkeleton />
      </PageContainer>
    );
  }
  if (error && isMissing(error)) {
    return (
      <PageContainer>
        <EntityNotFound
          Icon={BoxIcon}
          description="Project not found. It was deleted, or you cannot see it."
        />
      </PageContainer>
    );
  }
  if (error) {
    return (
      <PageContainer>
        <ErrorCard message={error.message} onRetry={() => void refetch()} />
      </PageContainer>
    );
  }
  return children(data);
}

import { useQuery } from "@tanstack/react-query";
import { type ReactNode } from "react";

import type { ProjectDetail } from "@/api/ops.gen";
import { BoxIcon } from "@/components/icons";
import { DetailPlaceholder } from "@/components/layout/detail-placeholder";
import { ErrorCard } from "@/components/layout/error-card";
import { PageContainer } from "@/components/layout/page-container";
import { EntityNotFound } from "@/components/navigation/not-found-template";
import { ProjectTabs, type TProjectTab } from "@/components/projects/project-header";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { isMissing, projectQuery } from "@/lib/queries/projects";

export function ProjectLoader({
  slug,
  tab,
  children,
}: {
  slug: string;
  tab: TProjectTab;
  children: (project: ProjectDetail) => ReactNode;
}) {
  const { workspace } = useWorkspace();
  const { data, isPending, error, refetch } = useQuery(projectQuery(workspace.id, slug));
  if (isPending) {
    return (
      <PageContainer>
        <DetailPlaceholder
          breadcrumb={[
            {
              label: "Projects",
              link: { to: "/$workspace/projects", params: { workspace: workspace.urlKey } },
            },
          ]}
        >
          <ProjectTabs slug={slug} tab={tab} />
        </DetailPlaceholder>
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
  if (error && !data) {
    return (
      <PageContainer>
        <ErrorCard message={error.message} onRetry={() => void refetch()} />
      </PageContainer>
    );
  }
  return children(data);
}

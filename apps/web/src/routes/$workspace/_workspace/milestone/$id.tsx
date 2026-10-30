import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";

import type { MilestoneDetail } from "@/api/ops.gen";
import { CircleDotIcon, DiamondIcon } from "@/components/icons";
import { IssuesByState } from "@/components/issues/issues-by-state";
import { type TBreadcrumbItem } from "@/components/layout/breadcrumb";
import { ErrorCard } from "@/components/layout/error-card";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { PageContainer } from "@/components/layout/page-container";
import { PageHeader } from "@/components/layout/page-header";
import { SectionTitle } from "@/components/layout/section-title";
import { MilestoneMenu } from "@/components/milestones/milestone-menu";
import { GithubMilestoneLinks } from "@/components/milestones/milestone-row";
import {
  isMilestoneLate,
  MilestoneIcon,
  milestoneIssueCount,
} from "@/components/milestones/milestone-status";
import { EntityNotFound } from "@/components/navigation/not-found-template";
import { ProgressBar } from "@/components/projects/progress-bar";
import { formatProjectDate } from "@/components/projects/project-dates";
import { MarkdownView } from "@/components/shared/markdown-view";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { entitySlug } from "@/lib/helpers/entity-slug";
import { milestoneQuery } from "@/lib/queries/milestones";
import { isNotFoundError } from "@/lib/server/errors";

export const Route = createFileRoute("/$workspace/_workspace/milestone/$id")({
  component: MilestonePage,
});

function MilestonePage() {
  const { id } = Route.useParams();
  const { workspace, canEdit } = useWorkspace();
  const { data: milestone, isPending, error, refetch } = useQuery(milestoneQuery(workspace.id, id));

  const body = () => {
    if (isNotFoundError(error)) {
      return <EntityNotFound Icon={DiamondIcon} description="There is no such milestone." />;
    }
    if (error) return <ErrorCard message={error.message} onRetry={() => void refetch()} />;
    if (isPending) return <ListSkeleton />;
    return <MilestoneBody milestone={milestone} />;
  };

  return (
    <PageContainer>
      <PageHeader
        title={milestone?.name ?? "Milestone"}
        icon={
          milestone ? <MilestoneIcon milestone={milestone} className="size-4" /> : <DiamondIcon />
        }
        breadcrumb={useBreadcrumb(milestone)}
        actions={milestone && canEdit && <MilestoneMenu milestone={milestone} />}
      />
      {body()}
    </PageContainer>
  );
}

/** A project's milestone sits under its project, the workspace's own under Milestones. */
function useBreadcrumb(milestone: MilestoneDetail | undefined): TBreadcrumbItem[] {
  const { workspace } = useWorkspace();
  const params = { workspace: workspace.urlKey };
  const project = milestone?.project;
  if (!project) return [{ label: "Milestones", link: { to: "/$workspace/milestones", params } }];
  return [
    { label: "Projects", link: { to: "/$workspace/projects", params } },
    {
      label: project.name,
      link: { to: "/$workspace/project/$slug", params: { ...params, slug: entitySlug(project) } },
    },
  ];
}

function MilestoneBody({ milestone }: { milestone: MilestoneDetail }) {
  const isClosed = Boolean(milestone.closedAt);
  return (
    <>
      <Card className="flex w-full flex-col gap-4 p-4">
        <div className="flex w-full flex-wrap items-center gap-2">
          <Badge variant={isClosed ? "muted" : "process"}>{isClosed ? "Closed" : "Open"}</Badge>
          {milestone.targetDate && (
            <span
              data-late={isMilestoneLate(milestone) || undefined}
              className="text-muted-foreground data-late:text-destructive text-sm"
            >
              Target {formatProjectDate(milestone.targetDate)}
            </span>
          )}
          <GithubMilestoneLinks github={milestone.github} className="ml-auto flex-wrap" />
        </div>
        {milestone.description && <MarkdownView markdown={milestone.description} />}
        {milestone.issueCount > 0 && (
          <div className="flex w-full items-center gap-3">
            <ProgressBar
              className="flex-1"
              value={milestone.progress}
              label={`${milestone.name} progress`}
            />
            <span className="text-muted-foreground shrink-0 text-sm tabular-nums">
              {milestoneIssueCount(milestone)} done
            </span>
          </div>
        )}
      </Card>
      <section className="flex w-full flex-col gap-2">
        <SectionTitle title="Issues" />
        <IssuesByState
          issues={milestone.issues}
          empty={{
            icon: CircleDotIcon,
            title: "No issues in this milestone yet",
            description: "Add issues to the milestone from an issue's properties.",
          }}
        />
      </section>
    </>
  );
}

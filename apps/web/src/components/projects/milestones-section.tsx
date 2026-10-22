import { Link } from "@tanstack/react-router";

import type { Milestone, ProjectDetail } from "@/api/ops.gen";
import { DiamondIcon, PencilIcon, PlusIcon, Trash2Icon } from "@/components/icons/symbols.gen";
import { List } from "@/components/layout/list";
import { SectionTitle } from "@/components/layout/section-title";
import { MilestoneDialog } from "@/components/projects/milestone-dialog";
import { ProgressBar } from "@/components/projects/progress-bar";
import { formatProjectDate, isPastTarget } from "@/components/projects/project-dates";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { Button } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { entitySlug } from "@/lib/helpers/entity-slug";
import { useDeleteMilestoneMutation } from "@/lib/queries/projects";

export function MilestonesSection({ project }: { project: ProjectDetail }) {
  const { canEdit } = useWorkspace();
  if (!canEdit && project.milestones.length === 0) return null;
  return (
    <section aria-label="Milestones" className="flex w-full flex-col gap-3">
      <SectionTitle
        title="Milestones"
        actions={
          canEdit && (
            <MilestoneDialog
              projectId={project.id}
              trigger={
                <Button variant="ghost" size="sm">
                  <PlusIcon className="size-4" />
                  Add milestone
                </Button>
              }
            />
          )
        }
      />
      {project.milestones.length > 0 && (
        <List>
          {project.milestones.map((milestone) => (
            <MilestoneRow
              key={milestone.id}
              project={project}
              milestone={milestone}
              canEdit={canEdit}
            />
          ))}
        </List>
      )}
    </section>
  );
}

function MilestoneRow({
  project,
  milestone,
  canEdit,
}: {
  project: ProjectDetail;
  milestone: Milestone;
  canEdit: boolean;
}) {
  const { workspace } = useWorkspace();
  const { mutateAsync: deleteMilestone } = useDeleteMilestoneMutation(workspace.id);
  const open = milestone.issueCount - milestone.completedIssueCount - milestone.canceledIssueCount;
  const isDone = milestone.issueCount > 0 && open === 0;
  const isLate = !isDone && milestone.targetDate !== null && isPastTarget(milestone.targetDate);
  return (
    <div
      role="listitem"
      className="group/row has-hover:hover:bg-accent flex h-9 w-full items-center gap-3 border-b px-3 text-sm last:border-b-0"
    >
      <DiamondIcon
        filled={isDone}
        data-done={isDone || undefined}
        className="text-muted-foreground data-done:text-primary size-3.5 shrink-0"
      />
      <Link
        to="/$workspace/project/$slug/issues"
        params={{ workspace: workspace.urlKey, slug: entitySlug(project) }}
        search={{ milestone: milestone.id }}
        className="min-w-0 flex-1 truncate font-medium"
        title={milestone.description ?? undefined}
      >
        {milestone.name}
      </Link>
      <span className="text-muted-foreground hidden w-20 shrink-0 text-right text-xs tabular-nums sm:block">
        {milestone.issueCount > 0 &&
          `${milestone.completedIssueCount} / ${milestone.issueCount - milestone.canceledIssueCount}`}
      </span>
      <span
        data-late={isLate || undefined}
        className="text-muted-foreground data-late:text-destructive hidden w-24 shrink-0 truncate text-xs tabular-nums sm:block"
      >
        {milestone.targetDate && formatProjectDate(milestone.targetDate)}
      </span>
      <ProgressBar
        className="w-32 shrink-0"
        value={milestone.progress}
        label={`${milestone.name} progress`}
      />
      {canEdit && (
        <div className="flex shrink-0 items-center gap-1 opacity-0 group-focus-within/row:opacity-100 group-hover/row:opacity-100">
          <MilestoneDialog
            milestone={milestone}
            trigger={
              <Button variant="ghost" size="icon" aria-label={`Edit ${milestone.name}`}>
                <PencilIcon className="size-4" />
              </Button>
            }
          />
          <ConfirmDialog
            trigger={
              <Button variant="ghost" size="icon" aria-label={`Delete ${milestone.name}`}>
                <Trash2Icon className="size-4" />
              </Button>
            }
            title={`Delete ${milestone.name}`}
            description="Its issues stay in the project without a milestone."
            confirmLabel="Delete milestone"
            onConfirm={() => deleteMilestone(milestone.id)}
          />
        </div>
      )}
    </div>
  );
}

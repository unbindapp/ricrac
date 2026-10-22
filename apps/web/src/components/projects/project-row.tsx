import { Link } from "@tanstack/react-router";

import type { Project, ProjectStatusRef } from "@/api/ops.gen";
import { ChevronRightIcon, PlusIcon } from "@/components/icons/symbols.gen";
import { HealthIcon, healthLabels } from "@/components/projects/health-badge";
import { PersonAvatar } from "@/components/projects/people";
import { ProgressBar } from "@/components/projects/progress-bar";
import { formatProjectDate, isPastTarget } from "@/components/projects/project-dates";
import { ProjectStatusIcon } from "@/components/projects/project-status-icon";
import { PriorityIcon } from "@/components/shared/priority-icon";
import { ProjectIcon } from "@/components/shared/project-icon";
import { Button } from "@/components/ui/button";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { entitySlug } from "@/lib/helpers/entity-slug";

export function isOpenStatus(status: ProjectStatusRef) {
  return status.type !== "completed" && status.type !== "canceled";
}

/** The target date, red once it has passed on a project still open. */
export function TargetDate({
  project,
}: {
  project: Pick<Project, "targetDate" | "targetDateResolution" | "status">;
}) {
  if (!project.targetDate) return null;
  const isLate = isOpenStatus(project.status) && isPastTarget(project.targetDate);
  return (
    <span
      data-late={isLate || undefined}
      className="text-muted-foreground data-late:text-destructive truncate text-xs tabular-nums"
    >
      {formatProjectDate(project.targetDate, project.targetDateResolution)}
    </span>
  );
}

export function ProjectListHeader() {
  return (
    <div
      aria-hidden
      className="text-muted-foreground flex h-8 w-full items-center gap-3 border-b px-3 text-xs group-data-flush/list:px-6"
    >
      <span className="min-w-0 flex-1">Name</span>
      <span className="hidden w-24 shrink-0 md:block">Health</span>
      <span className="w-6 shrink-0" />
      <span className="w-10 shrink-0 text-center">Lead</span>
      <span className="hidden w-24 shrink-0 sm:block">Target</span>
      <span className="w-32 shrink-0">Progress</span>
    </div>
  );
}

export function ProjectRow({ project }: { project: Project }) {
  const { workspace } = useWorkspace();
  return (
    <Link
      role="listitem"
      to="/$workspace/project/$slug"
      params={{ workspace: workspace.urlKey, slug: entitySlug(project) }}
      className="has-hover:hover:bg-muted focus-visible:bg-muted flex h-9 w-full items-center gap-3 border-b px-3 text-sm outline-hidden group-data-flush/list:h-11 group-data-flush/list:px-6 last:border-b-0"
    >
      <span className="flex min-w-0 flex-1 items-center gap-2">
        <ProjectIcon icon={project.icon} color={project.color} />
        <span className="truncate font-medium">{project.name}</span>
        {project.summary && (
          <span className="text-muted-foreground hidden min-w-0 truncate text-xs lg:inline">
            {project.summary}
          </span>
        )}
      </span>
      <span className="hidden w-24 shrink-0 items-center gap-1.5 text-xs md:flex">
        {project.health && (
          <>
            <HealthIcon health={project.health} />
            <span className="text-muted-foreground truncate">{healthLabels[project.health]}</span>
          </>
        )}
      </span>
      <span className="flex w-6 shrink-0 justify-center">
        {project.priority > 0 && <PriorityIcon priority={project.priority} />}
      </span>
      <span className="flex w-10 shrink-0 justify-center">
        {project.lead && (
          <SimpleTooltip content={`Lead: ${project.lead.name}`}>
            <span className="flex">
              <PersonAvatar user={project.lead} size="xs" />
            </span>
          </SimpleTooltip>
        )}
      </span>
      <span className="hidden w-24 shrink-0 sm:flex">
        <TargetDate project={project} />
      </span>
      <span className="w-32 shrink-0">
        {project.issueCount > 0 && (
          <ProgressBar value={project.progress} label={`${project.name} progress`} />
        )}
      </span>
    </Link>
  );
}

type THeaderProps = {
  status: ProjectStatusRef;
  count: number;
  isCollapsed: boolean;
  onToggle: () => void;
  onCreate?: () => void;
};

export function ProjectGroupHeader({
  status,
  count,
  isCollapsed,
  onToggle,
  onCreate,
}: THeaderProps) {
  return (
    <div
      role="listitem"
      aria-label={`${status.name}, ${count} projects`}
      className="bg-muted flex h-9 w-full items-center gap-2 px-1.5 text-sm group-data-flush/list:px-4"
    >
      <Button
        variant="ghost"
        size="sm"
        forceMinSize={false}
        aria-expanded={!isCollapsed}
        onClick={onToggle}
        className="min-w-0 justify-start"
      >
        <ChevronRightIcon
          data-open={!isCollapsed || undefined}
          className="text-muted-foreground size-3.5 transition-transform data-open:rotate-90"
        />
        <ProjectStatusIcon type={status.type} color={status.color} />
        <span className="truncate">{status.name}</span>
        <span className="text-muted-foreground text-xs font-normal tabular-nums">{count}</span>
      </Button>
      <span className="flex-1" />
      {onCreate && (
        <SimpleTooltip content={`New project in ${status.name}`}>
          <Button
            variant="ghost"
            size="icon"
            aria-label={`New project in ${status.name}`}
            onClick={onCreate}
          >
            <PlusIcon className="size-4" />
          </Button>
        </SimpleTooltip>
      )}
    </div>
  );
}

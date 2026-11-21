import { Link } from "@tanstack/react-router";

import type { Project, ProjectStatusRef } from "@/api/ops.gen";
import { ChevronRightIcon, PlusIcon } from "@/components/icons";
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
      className="text-muted-foreground dux:border-0 dux:px-4 dux:type-label-medium dux:group-data-flush/list:px-4 flex h-8 w-full items-center gap-3 border-b px-3 text-xs"
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
      className="has-hover:hover:bg-muted focus-visible:bg-muted flex h-9 w-full items-center gap-3 border-b px-3 text-sm outline-hidden group-data-flush/list:h-11 last:border-b-0"
    >
      <span className="dux:gap-3 flex min-w-0 flex-1 items-center gap-2">
        <span className="dux:*:size-6 flex shrink-0">
          <ProjectIcon icon={project.icon} color={project.color} />
        </span>
        <span className="dux:flex-col dux:items-stretch dux:gap-0 flex min-w-0 items-center gap-2">
          <span className="dux:font-normal truncate font-medium">{project.name}</span>
          {project.summary && (
            <span
              data-slot="list-supporting"
              className="text-muted-foreground dux:block hidden min-w-0 truncate text-xs lg:inline"
            >
              {project.summary}
            </span>
          )}
        </span>
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
  const title = (
    <>
      <ProjectStatusIcon type={status.type} color={status.color} />
      <span className="truncate">{status.name}</span>
      <span className="text-muted-foreground dux:type-label-large text-xs font-normal tabular-nums">
        {count}
      </span>
    </>
  );
  return (
    <div className="bg-muted dux:h-12 dux:bg-transparent dux:px-0 dux:group-data-flush/list:px-0 flex h-9 w-full items-center gap-2 px-1.5 text-sm group-data-flush/list:px-1">
      <Button
        variant="ghost"
        size="sm"
        forceMinSize={false}
        aria-label={status.name}
        aria-expanded={!isCollapsed}
        onClick={onToggle}
        className="dux:size-10 dux:justify-center dux:p-0 min-w-0 justify-start"
      >
        <span className="contents">
          <ChevronRightIcon
            data-open={!isCollapsed || undefined}
            className="text-muted-foreground dux:size-6 size-3.5 transition-transform data-open:rotate-90"
          />
        </span>
        <span className="dux:hidden contents">{title}</span>
      </Button>
      <span className="dux:flex dux:type-title-small hidden min-w-0 items-center gap-2">
        {title}
      </span>
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

import {
  closestCorners,
  DndContext,
  type DragEndEvent,
  DragOverlay,
  type DragStartEvent,
  KeyboardSensor,
  PointerSensor,
  useDroppable,
  useSensor,
  useSensors,
} from "@dnd-kit/core";
import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { Link } from "@tanstack/react-router";
import { PlusIcon } from "lucide-react";
import { type CSSProperties, type HTMLAttributes, type ReactNode, type Ref, useState } from "react";

import type { Project } from "@/api/ops.gen";
import { sortOrderAtIndex } from "@/components/issues/sort-order";
import { ErrorCard } from "@/components/layout/error-card";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { HealthBadge } from "@/components/projects/health-badge";
import { useNewProjectStore } from "@/components/projects/new-project-store";
import { PersonAvatar } from "@/components/projects/people";
import { ProgressBar } from "@/components/projects/progress-bar";
import { type TProjectGroup } from "@/components/projects/project-groups";
import { useProjectList } from "@/components/projects/project-list";
import { TargetDate } from "@/components/projects/project-row";
import { ProjectStatusIcon } from "@/components/projects/project-status-icon";
import { PriorityIcon } from "@/components/shared/priority-icon";
import { ProjectIcon } from "@/components/shared/project-icon";
import { Button } from "@/components/ui/button";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { cn } from "@/components/ui/utils";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { entitySlug } from "@/lib/helpers/entity-slug";
import { type TProjectListArgs, useSaveProjectMutation } from "@/lib/queries/projects";

const DRAG_ACTIVATION_DISTANCE = 6;

type TCardData = { statusId: string; projectId: string };

type TProps = {
  filters: TProjectListArgs;
  query?: string;
  empty: ReactNode;
  noMatches?: ReactNode;
  createDefaults?: { teamId?: string; initiativeId?: string };
};

export function ProjectBoard({ filters, query, empty, noMatches = null, createDefaults }: TProps) {
  const { groups, projects, total, isPending, error, refetch } = useProjectList(filters, query);
  if (isPending) return <ListSkeleton rows={8} />;
  if (error) return <ErrorCard message={error.message} onRetry={refetch} />;
  if (total === 0) return empty;
  if (projects.length === 0) return noMatches;
  return <Board groups={groups} createDefaults={createDefaults} />;
}

function Board({
  groups,
  createDefaults,
}: {
  groups: TProjectGroup[];
  createDefaults?: TProps["createDefaults"];
}) {
  const { workspace, canEdit } = useWorkspace();
  const { mutate: saveProject } = useSaveProjectMutation(workspace.id);
  const [active, setActive] = useState<TCardData | null>(null);
  const [pending, setPending] = useState<{ projectId: string; statusId: string } | null>(null);
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: DRAG_ACTIVATION_DISTANCE } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );
  const shown = pending ? moveProject(groups, pending.projectId, pending.statusId) : groups;
  const activeProject = active
    ? shown.flatMap((group) => group.projects).find((project) => project.id === active.projectId)
    : undefined;

  const onDragStart = (event: DragStartEvent) =>
    setActive((event.active.data.current as TCardData | undefined) ?? null);

  const onDragEnd = (event: DragEndEvent) => {
    setActive(null);
    const source = event.active.data.current as TCardData | undefined;
    if (!source || !event.over) return;
    const overCard = event.over.data.current as Partial<TCardData> | undefined;
    const targetId = overCard?.statusId ?? String(event.over.id).replace(/^column::/, "");
    const sourceGroup = shown.find((group) => group.status.id === source.statusId);
    const targetGroup = shown.find((group) => group.status.id === targetId);
    const project = sourceGroup?.projects.find((item) => item.id === source.projectId);
    if (!sourceGroup || !targetGroup || !project) return;

    const isSameColumn = sourceGroup.status.id === targetGroup.status.id;
    const overIndex = overCard?.projectId
      ? targetGroup.projects.findIndex((item) => item.id === overCard.projectId)
      : targetGroup.projects.length;
    const fromIndex = targetGroup.projects.findIndex((item) => item.id === project.id);
    const reordered = isSameColumn
      ? arrayMove(targetGroup.projects, fromIndex, overIndex)
      : [
          ...targetGroup.projects.slice(0, overIndex),
          project,
          ...targetGroup.projects.slice(overIndex),
        ];
    const newIndex = reordered.findIndex((item) => item.id === project.id);
    if (isSameColumn && newIndex === fromIndex) return;
    const neighbors = reordered
      .filter((item) => item.id !== project.id)
      .map((item) => item.sortOrder);

    setPending({ projectId: project.id, statusId: targetGroup.status.id });
    saveProject(
      {
        id: project.id,
        sortOrder: sortOrderAtIndex(neighbors, newIndex),
        ...(isSameColumn ? {} : { state: targetGroup.status.id }),
      },
      { onSettled: () => setPending(null) },
    );
  };

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={closestCorners}
      onDragStart={onDragStart}
      onDragCancel={() => setActive(null)}
      onDragEnd={onDragEnd}
    >
      <div className="flex min-h-0 w-full min-w-0 flex-1 items-start gap-3 overflow-x-auto pb-2">
        {shown.map((group) => (
          <BoardColumn
            key={group.status.id}
            group={group}
            canEdit={canEdit}
            createDefaults={createDefaults}
          />
        ))}
      </div>
      <DragOverlay>
        {activeProject && <ProjectCard project={activeProject} isOverlay />}
      </DragOverlay>
    </DndContext>
  );
}

function moveProject(groups: TProjectGroup[], projectId: string, statusId: string) {
  const project = groups.flatMap((group) => group.projects).find((item) => item.id === projectId);
  if (!project) return groups;
  return groups.map((group) => {
    const rest = group.projects.filter((item) => item.id !== projectId);
    return { ...group, projects: group.status.id === statusId ? [...rest, project] : rest };
  });
}

function BoardColumn({
  group,
  canEdit,
  createDefaults,
}: {
  group: TProjectGroup;
  canEdit: boolean;
  createDefaults?: TProps["createDefaults"];
}) {
  const { setNodeRef, isOver } = useDroppable({ id: `column::${group.status.id}` });
  const openNewProject = useNewProjectStore((state) => state.open);
  if (group.projects.length === 0 && !isOver) {
    return (
      <section
        ref={setNodeRef}
        aria-label={`${group.status.name}, no projects`}
        className="bg-muted flex w-10 shrink-0 flex-col items-center gap-2 rounded-lg border py-3"
      >
        <ProjectStatusIcon type={group.status.type} color={group.status.color} />
        <span
          className="text-muted-foreground text-sm font-medium whitespace-nowrap"
          style={{ writingMode: "vertical-rl" }}
        >
          {group.status.name}
        </span>
      </section>
    );
  }
  return (
    <section
      aria-label={group.status.name}
      data-over={isOver || undefined}
      className="bg-muted data-over:border-primary/5-10 flex max-h-full w-72 shrink-0 flex-col rounded-lg border"
    >
      <header className="flex h-10 shrink-0 items-center gap-2 pr-1.5 pl-3 text-sm">
        <ProjectStatusIcon type={group.status.type} color={group.status.color} />
        <span className="min-w-0 truncate font-medium">{group.status.name}</span>
        <span className="text-muted-foreground text-xs tabular-nums">{group.projects.length}</span>
        <span className="flex-1" />
        {canEdit && (
          <SimpleTooltip content={`New project in ${group.status.name}`}>
            <Button
              variant="ghost"
              size="icon"
              aria-label={`New project in ${group.status.name}`}
              onClick={() => openNewProject({ ...createDefaults, statusId: group.status.id })}
            >
              <PlusIcon className="size-4" />
            </Button>
          </SimpleTooltip>
        )}
      </header>
      <SortableContext
        id={group.status.id}
        items={group.projects.map((project) => project.id)}
        strategy={verticalListSortingStrategy}
      >
        <div
          ref={setNodeRef}
          role="list"
          className="flex min-h-16 flex-col gap-2 overflow-y-auto px-2 pb-2"
        >
          {group.projects.map((project) => (
            <SortableProjectCard
              key={project.id}
              statusId={group.status.id}
              project={project}
              canEdit={canEdit}
            />
          ))}
        </div>
      </SortableContext>
    </section>
  );
}

function SortableProjectCard({
  statusId,
  project,
  canEdit,
}: {
  statusId: string;
  project: Project;
  canEdit: boolean;
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: project.id,
    data: { statusId, projectId: project.id } satisfies TCardData,
    disabled: !canEdit,
  });
  return (
    <ProjectCard
      ref={setNodeRef}
      project={project}
      isDragging={isDragging}
      style={{
        transform: transform ? `translate3d(${transform.x}px, ${transform.y}px, 0)` : undefined,
        transition,
      }}
      dragProps={{ ...attributes, ...listeners }}
    />
  );
}

function ProjectCard({
  project,
  isDragging,
  isOverlay,
  ref,
  style,
  dragProps,
}: {
  project: Project;
  isDragging?: boolean;
  isOverlay?: boolean;
  ref?: Ref<HTMLDivElement>;
  style?: CSSProperties;
  dragProps?: HTMLAttributes<HTMLDivElement>;
}) {
  const { workspace } = useWorkspace();
  const hasFooter = Boolean(project.health || project.targetDate || project.lead);
  return (
    <div
      ref={ref}
      style={style}
      {...dragProps}
      role="listitem"
      aria-label={project.name}
      data-dragging={isDragging || undefined}
      className={cn(
        "bg-card has-hover:hover:border-primary/4-10 focus-visible:ring-primary/8-10 relative flex w-full cursor-default flex-col gap-3 rounded-lg border p-3 text-sm outline-hidden focus-visible:ring-1 data-dragging:opacity-40",
        isOverlay && "shadow-lg",
      )}
    >
      <div className="flex min-w-0 items-center gap-2">
        <ProjectIcon icon={project.icon} color={project.color} />
        <Link
          to="/$workspace/project/$slug"
          params={{ workspace: workspace.urlKey, slug: entitySlug(project) }}
          className="min-w-0 flex-1 truncate font-medium after:absolute after:inset-0"
        >
          {project.name}
        </Link>
        {project.priority > 0 && <PriorityIcon priority={project.priority} />}
      </div>
      {project.summary && (
        <p className="text-muted-foreground line-clamp-2 text-xs">{project.summary}</p>
      )}
      {project.issueCount > 0 && (
        <ProgressBar value={project.progress} label={`${project.name} progress`} />
      )}
      {hasFooter && (
        <div className="flex min-w-0 items-center gap-2">
          <HealthBadge health={project.health} />
          <span className="min-w-0 flex-1 truncate">
            <TargetDate project={project} />
          </span>
          {project.lead && <PersonAvatar user={project.lead} size="xs" />}
        </div>
      )}
    </div>
  );
}

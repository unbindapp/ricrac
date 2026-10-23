import { useQuery } from "@tanstack/react-query";
import { type ReactNode, useState } from "react";

import type { Project } from "@/api/ops.gen";

import { ErrorCard } from "@/components/layout/error-card";
import { List } from "@/components/layout/list";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { useNewProjectStore } from "@/components/projects/new-project-store";
import {
  groupProjectsByStatus,
  inScope,
  matchesQuery,
  scopeArgs,
  type TProjectScope,
} from "@/components/projects/project-groups";
import { useProjectStatuses } from "@/components/projects/project-pickers";
import {
  isOpenStatus,
  ProjectGroupHeader,
  ProjectListHeader,
  ProjectRow,
} from "@/components/projects/project-row";
import { TrashedRow } from "@/components/projects/entity-menu";
import { ProjectIcon } from "@/components/shared/project-icon";
import { toast } from "@/components/ui/toast";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import {
  projectsQuery,
  type TProjectListArgs,
  useProjectLifecycleMutation,
} from "@/lib/queries/projects";

export function useProjectList(
  filters: TProjectListArgs,
  query = "",
  scope: TProjectScope = "active",
) {
  const { workspace } = useWorkspace();
  const projectsResult = useQuery(projectsQuery(workspace.id, { ...filters, ...scopeArgs(scope) }));
  const { statuses, isPending: statusesPending } = useProjectStatuses();
  const all = (projectsResult.data ?? []).filter((project) => inScope(project, scope));
  const projects = all.filter((project) => matchesQuery(project, query));
  return {
    groups: groupProjectsByStatus(statuses, projects),
    projects,
    total: all.length,
    isPending: projectsResult.isPending || statusesPending,
    error: projectsResult.error,
    refetch: () => void projectsResult.refetch(),
  };
}

type TProps = {
  filters: TProjectListArgs;
  query?: string;
  scope?: TProjectScope;
  /** Shown when there are no projects at all; null shows nothing. */
  empty: ReactNode;
  /** Shown when projects exist but none match the query. */
  noMatches?: ReactNode;
  /** Defaults for the project created from a group's + button. */
  createDefaults?: { initiativeId?: string };
};

export function ProjectList({
  filters,
  query,
  scope = "active",
  empty,
  noMatches = null,
  createDefaults,
}: TProps) {
  const { canEdit } = useWorkspace();
  const openNewProject = useNewProjectStore((state) => state.open);
  const { groups, projects, total, isPending, error, refetch } = useProjectList(
    filters,
    query,
    scope,
  );
  const [toggled, setToggled] = useState<string[]>([]);

  if (isPending) return <ListSkeleton />;
  if (error) return <ErrorCard message={error.message} onRetry={refetch} />;
  if (total === 0) return empty;
  if (projects.length === 0) return noMatches;
  if (scope === "trash") return <TrashedProjects projects={projects} />;

  const toggle = (statusId: string) =>
    setToggled((current) =>
      current.includes(statusId) ? current.filter((id) => id !== statusId) : [...current, statusId],
    );

  return (
    <List flush>
      <ProjectListHeader />
      {groups
        .filter((group) => group.projects.length > 0)
        .map((group) => {
          const closedByDefault = scope === "active" && !isOpenStatus(group.status);
          const isCollapsed = closedByDefault !== toggled.includes(group.status.id);
          return (
            <div key={group.status.id} role="presentation" className="contents">
              <ProjectGroupHeader
                status={group.status}
                count={group.projects.length}
                isCollapsed={isCollapsed}
                onToggle={() => toggle(group.status.id)}
                onCreate={
                  canEdit && scope === "active"
                    ? () => openNewProject({ ...createDefaults, statusId: group.status.id })
                    : undefined
                }
              />
              {!isCollapsed && (
                <div role="presentation" data-slot="list-group" className="contents">
                  {group.projects.map((project) => (
                    <ProjectRow key={project.id} project={project} />
                  ))}
                </div>
              )}
            </div>
          );
        })}
    </List>
  );
}

function TrashedProjects({ projects }: { projects: Project[] }) {
  const { workspace, canEdit } = useWorkspace();
  const { mutate: change, isPending, variables } = useProjectLifecycleMutation(workspace.id);
  const newestFirst = [...projects].sort((a, b) =>
    (b.trashedAt ?? "").localeCompare(a.trashedAt ?? ""),
  );
  return (
    <List flush>
      {newestFirst.map((project) => (
        <TrashedRow
          key={project.id}
          icon={<ProjectIcon icon={project.icon} color={project.color} />}
          name={project.name}
          trashedAt={project.trashedAt}
          isPending={isPending && variables?.id === project.id}
          onRestore={
            canEdit
              ? () =>
                  change(
                    { id: project.id, action: "restore" },
                    { onSuccess: () => toast.add({ type: "success", title: "Project restored" }) },
                  )
              : undefined
          }
        />
      ))}
    </List>
  );
}

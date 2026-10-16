import { useQuery } from "@tanstack/react-query";
import { type ReactNode, useState } from "react";

import { ErrorCard } from "@/components/layout/error-card";
import { List } from "@/components/layout/list";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { useNewProjectStore } from "@/components/projects/new-project-store";
import { groupProjectsByStatus, matchesQuery } from "@/components/projects/project-groups";
import { useProjectStatuses } from "@/components/projects/project-pickers";
import {
  isOpenStatus,
  ProjectGroupHeader,
  ProjectListHeader,
  ProjectRow,
} from "@/components/projects/project-row";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { projectsQuery, type TProjectListArgs } from "@/lib/queries/projects";

export function useProjectList(filters: TProjectListArgs, query = "") {
  const { workspace } = useWorkspace();
  const projectsResult = useQuery(projectsQuery(workspace.id, filters));
  const { statuses, isPending: statusesPending } = useProjectStatuses();
  const all = projectsResult.data ?? [];
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
  /** Shown when there are no projects at all; null shows nothing. */
  empty: ReactNode;
  /** Shown when projects exist but none match the query. */
  noMatches?: ReactNode;
  /** Defaults for the project created from a group's + button. */
  createDefaults?: { teamId?: string; initiativeId?: string };
};

export function ProjectList({ filters, query, empty, noMatches = null, createDefaults }: TProps) {
  const { canEdit } = useWorkspace();
  const openNewProject = useNewProjectStore((state) => state.open);
  const { groups, projects, total, isPending, error, refetch } = useProjectList(filters, query);
  const [toggled, setToggled] = useState<string[]>([]);

  if (isPending) return <ListSkeleton />;
  if (error) return <ErrorCard message={error.message} onRetry={refetch} />;
  if (total === 0) return empty;
  if (projects.length === 0) return noMatches;

  const toggle = (statusId: string) =>
    setToggled((current) =>
      current.includes(statusId) ? current.filter((id) => id !== statusId) : [...current, statusId],
    );

  return (
    <List>
      <ProjectListHeader />
      {groups
        .filter((group) => group.projects.length > 0)
        .map((group) => {
          const closedByDefault = !isOpenStatus(group.status);
          const isCollapsed = closedByDefault !== toggled.includes(group.status.id);
          return (
            <div key={group.status.id} role="presentation" className="contents">
              <ProjectGroupHeader
                status={group.status}
                count={group.projects.length}
                isCollapsed={isCollapsed}
                onToggle={() => toggle(group.status.id)}
                onCreate={
                  canEdit
                    ? () => openNewProject({ ...createDefaults, statusId: group.status.id })
                    : undefined
                }
              />
              {!isCollapsed &&
                group.projects.map((project) => <ProjectRow key={project.id} project={project} />)}
            </div>
          );
        })}
    </List>
  );
}

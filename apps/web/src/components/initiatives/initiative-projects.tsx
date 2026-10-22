import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";

import type { InitiativeDetail, InitiativeProject, SubInitiative } from "@/api/ops.gen";
import { PlusIcon } from "@/components/icons";
import { InitiativeStatusIcon } from "@/components/initiatives/initiative-status";
import { NewInitiativeDialog } from "@/components/initiatives/new-initiative-dialog";
import { List } from "@/components/layout/list";
import { SectionTitle } from "@/components/layout/section-title";
import { HealthIcon, healthLabels } from "@/components/projects/health-badge";
import { useNewProjectStore } from "@/components/projects/new-project-store";
import { PersonAvatar } from "@/components/projects/people";
import { ProgressBar } from "@/components/projects/progress-bar";
import { ProjectsPicker } from "@/components/projects/project-pickers";
import { TargetDate } from "@/components/projects/project-row";
import { ProjectStatusIcon } from "@/components/projects/project-status-icon";
import { ProjectIcon } from "@/components/shared/project-icon";
import { Button } from "@/components/ui/button";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { entitySlug } from "@/lib/helpers/entity-slug";
import { useSaveInitiativeMutation } from "@/lib/queries/initiatives";
import { projectsQuery } from "@/lib/queries/projects";

export function InitiativeProjects({ initiative }: { initiative: InitiativeDetail }) {
  const { workspace, canEdit } = useWorkspace();
  const { mutate: saveInitiative } = useSaveInitiativeMutation(workspace.id);
  const openNewProject = useNewProjectStore((state) => state.open);
  const current = initiative.projects.map((project) => project.id);
  if (!canEdit && current.length === 0) return null;
  return (
    <section aria-label="Projects" className="flex w-full flex-col gap-3">
      <SectionTitle
        title="Projects"
        actions={
          canEdit && (
            <>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => openNewProject({ initiativeId: initiative.id })}
              >
                <PlusIcon className="size-4" />
                New project
              </Button>
              <ProjectsPicker
                label="Projects in this initiative"
                value={current}
                onChange={(next) =>
                  saveInitiative({
                    id: initiative.id,
                    addProjects: next.filter((id) => !current.includes(id)),
                    removeProjects: current.filter((id) => !next.includes(id)),
                  })
                }
                trigger={
                  <Button variant="ghost" size="sm">
                    Add existing
                  </Button>
                }
              />
            </>
          )
        }
      />
      {initiative.projects.length > 0 && (
        <List>
          {initiative.projects.map((project) => (
            <InitiativeProjectRow key={project.id} project={project} />
          ))}
        </List>
      )}
    </section>
  );
}

function InitiativeProjectRow({ project }: { project: InitiativeProject }) {
  const { workspace } = useWorkspace();
  const { data } = useQuery(projectsQuery(workspace.id));
  const full = data?.find((item) => item.id === project.id);
  return (
    <Link
      role="listitem"
      to="/$workspace/project/$slug"
      params={{ workspace: workspace.urlKey, slug: entitySlug(project) }}
      className="has-hover:hover:bg-accent focus-visible:bg-accent flex h-9 w-full items-center gap-3 border-b px-3 text-sm outline-hidden last:border-b-0"
    >
      <span className="flex min-w-0 flex-1 items-center gap-2">
        <ProjectIcon icon={full?.icon} color={full?.color} />
        <span className="truncate font-medium">{project.name}</span>
      </span>
      <SimpleTooltip content={project.status.name}>
        <span className="flex shrink-0">
          <ProjectStatusIcon type={project.status.type} color={project.status.color} />
        </span>
      </SimpleTooltip>
      <span className="flex w-4 shrink-0 justify-center">
        {project.health && (
          <SimpleTooltip content={healthLabels[project.health]}>
            <span className="flex">
              <HealthIcon health={project.health} />
            </span>
          </SimpleTooltip>
        )}
      </span>
      <span className="flex w-6 shrink-0 justify-center">
        {project.lead && <PersonAvatar user={project.lead} size="xs" />}
      </span>
      <span className="hidden w-24 shrink-0 sm:flex">
        <TargetDate
          project={{
            targetDate: project.targetDate,
            targetDateResolution: full?.targetDateResolution ?? null,
            status: project.status,
          }}
        />
      </span>
      <span className="w-32 shrink-0">
        {(full?.issueCount ?? 0) > 0 && (
          <ProgressBar value={project.progress} label={`${project.name} progress`} />
        )}
      </span>
    </Link>
  );
}

export function SubInitiatives({ initiative }: { initiative: InitiativeDetail }) {
  const { canEdit } = useWorkspace();
  if (!canEdit && initiative.subInitiatives.length === 0) return null;
  return (
    <section aria-label="Sub-initiatives" className="flex w-full flex-col gap-3">
      <SectionTitle
        title="Sub-initiatives"
        actions={
          canEdit && (
            <NewInitiativeDialog
              parentId={initiative.id}
              trigger={
                <Button variant="ghost" size="sm">
                  <PlusIcon className="size-4" />
                  New sub-initiative
                </Button>
              }
            />
          )
        }
      />
      {initiative.subInitiatives.length > 0 && (
        <List>
          {initiative.subInitiatives.map((sub) => (
            <SubInitiativeRow key={sub.id} initiative={sub} />
          ))}
        </List>
      )}
    </section>
  );
}

function SubInitiativeRow({ initiative }: { initiative: SubInitiative }) {
  const { workspace } = useWorkspace();
  return (
    <Link
      role="listitem"
      to="/$workspace/initiative/$slug"
      params={{ workspace: workspace.urlKey, slug: entitySlug(initiative) }}
      className="has-hover:hover:bg-accent focus-visible:bg-accent flex h-9 w-full items-center gap-3 border-b px-3 text-sm outline-hidden last:border-b-0"
    >
      <InitiativeStatusIcon status={initiative.status} />
      <span className="min-w-0 flex-1 truncate font-medium">{initiative.name}</span>
      <span className="flex w-4 shrink-0 justify-center">
        {initiative.health && (
          <SimpleTooltip content={healthLabels[initiative.health]}>
            <span className="flex">
              <HealthIcon health={initiative.health} />
            </span>
          </SimpleTooltip>
        )}
      </span>
      <span className="flex w-6 shrink-0 justify-center">
        {initiative.owner && <PersonAvatar user={initiative.owner} size="xs" />}
      </span>
      <ProgressBar
        className="w-32 shrink-0"
        value={initiative.progress}
        label={`${initiative.name} progress`}
      />
    </Link>
  );
}

import type { OpArgs } from "@/lib/server/client";
import type { ProjectDetail } from "@/api/ops.gen";
import { PropertyRow } from "@/components/layout/property-row";
import { HealthBadge } from "@/components/projects/health-badge";
import { PeoplePicker, PersonPicker } from "@/components/projects/people";
import { fromResolution, toResolution } from "@/components/projects/project-dates";
import { ProjectDependencies } from "@/components/projects/project-dependencies";
import { EntityLinks } from "@/components/projects/project-links";
import {
  InitiativesPicker,
  PlannedDatePicker,
  ProjectLabelsPicker,
  ProjectPriorityPicker,
  ProjectStatusPicker,
  TeamsPicker,
} from "@/components/projects/project-pickers";
import { Separator } from "@/components/ui/separator";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useSaveProjectMutation } from "@/lib/queries/projects";

function hasRelations(project: ProjectDetail) {
  return Object.values(project.relations).some((related) => related.length > 0);
}

export function ProjectProperties({ project }: { project: ProjectDetail }) {
  const { workspace, canEdit } = useWorkspace();
  const { mutate: saveProject, mutateAsync: saveProjectAsync } = useSaveProjectMutation(
    workspace.id,
  );
  const update = (args: Omit<OpArgs<"save_project">, "id">) =>
    saveProject({ id: project.id, ...args });
  const disabled = !canEdit;

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-0.5">
        <PropertyRow label="Status">
          <ProjectStatusPicker
            status={project.status}
            disabled={disabled}
            onChange={(state) => update({ state })}
          />
        </PropertyRow>
        {project.health && (
          <PropertyRow label="Health">
            <span className="px-2">
              <HealthBadge health={project.health} />
            </span>
          </PropertyRow>
        )}
        <PropertyRow label="Priority">
          <ProjectPriorityPicker
            priority={project.priority}
            disabled={disabled}
            onChange={(priority) => update({ priority })}
          />
        </PropertyRow>
        <PropertyRow label="Lead">
          <PersonPicker
            role="Lead"
            user={project.lead}
            disabled={disabled}
            onChange={(lead) => update({ lead })}
          />
        </PropertyRow>
        <PropertyRow label="Members">
          <PeoplePicker
            people={project.members}
            disabled={disabled}
            onChange={(ids) => {
              const current = project.members.map((member) => member.id);
              update({
                addMembers: ids.filter((id) => !current.includes(id)),
                removeMembers: current.filter((id) => !ids.includes(id)),
              });
            }}
          />
        </PropertyRow>
        <PropertyRow label="Teams">
          <TeamsPicker
            teams={project.teams}
            disabled={disabled}
            onChange={(setTeams) => update({ setTeams })}
          />
        </PropertyRow>
        <PropertyRow label="Start">
          <PlannedDatePicker
            label="Start date"
            edge="start"
            date={project.startDate}
            resolution={toResolution(project.startDateResolution)}
            disabled={disabled}
            onChange={({ date, resolution }) =>
              update({
                startDate: date,
                startDateResolution: date ? fromResolution(resolution) : null,
              })
            }
          />
        </PropertyRow>
        <PropertyRow label="Target">
          <PlannedDatePicker
            label="Target date"
            edge="end"
            date={project.targetDate}
            resolution={toResolution(project.targetDateResolution)}
            disabled={disabled}
            onChange={({ date, resolution }) =>
              update({
                targetDate: date,
                targetDateResolution: date ? fromResolution(resolution) : null,
              })
            }
          />
        </PropertyRow>
        <PropertyRow label="Labels">
          <ProjectLabelsPicker
            labels={project.labels}
            disabled={disabled}
            onChange={(labels) => update({ labels })}
          />
        </PropertyRow>
        <PropertyRow label="Initiatives">
          <InitiativesPicker
            initiatives={project.initiatives}
            disabled={disabled}
            onChange={(setInitiatives) => update({ setInitiatives })}
          />
        </PropertyRow>
      </div>
      {(canEdit || project.links.length > 0 || hasRelations(project)) && <Separator />}
      <ProjectDependencies project={project} />
      <EntityLinks
        links={project.links}
        canEdit={canEdit}
        onAdd={(link) => saveProjectAsync({ id: project.id, links: [link] })}
        onRemove={(url) => update({ removeLinks: [url] })}
      />
    </div>
  );
}

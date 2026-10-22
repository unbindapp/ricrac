import { useQuery } from "@tanstack/react-query";
import { type ReactElement } from "react";

import type { TDocumentParent } from "@/components/documents/use-create-document";
import { InitiativeIcon } from "@/components/initiatives/initiative-status";
import { PropertyPicker, type TPickerOption } from "@/components/issues/property-picker";
import { ProjectIcon } from "@/components/shared/project-icon";
import { WorkspaceIcon } from "@/components/workspace/workspace-icon";
import { useWorkspaceTeam } from "@/components/workspace/teams-provider";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { initiativesQuery } from "@/lib/queries/initiatives";
import { projectsQuery } from "@/lib/queries/projects";

export function parentValue(parent: TDocumentParent | null) {
  if (!parent) return null;
  const [kind, id] = Object.entries(parent)[0] ?? [];
  return kind && id ? `${kind}:${id}` : null;
}

function parentFromValue(value: string): TDocumentParent | null {
  const [kind, id] = value.split(":");
  if (!id) return null;
  if (kind === "project") return { project: id };
  if (kind === "initiative") return { initiative: id };
  if (kind === "team") return { team: id };
  return null;
}

/** Picks where a document lives: the workspace itself, one of its projects or initiatives. */
export function DocumentParentPicker({
  label,
  value,
  onChange,
  trigger,
}: {
  label: string;
  value: TDocumentParent | null;
  onChange: (parent: TDocumentParent) => void;
  trigger: ReactElement;
}) {
  const { workspace } = useWorkspace();
  const team = useWorkspaceTeam();
  const projects = useQuery(projectsQuery(workspace.id));
  const initiatives = useQuery(initiativesQuery(workspace.id));
  const options: TPickerOption[] = [
    ...(team
      ? [
          {
            value: `team:${team.id}`,
            label: workspace.name,
            keywords: ["workspace"],
            icon: <WorkspaceIcon name={workspace.name} logoUrl={workspace.logoUrl} />,
          },
        ]
      : []),
    ...(projects.data ?? []).map((project) => ({
      value: `project:${project.id}`,
      label: project.name,
      group: "Projects",
      icon: <ProjectIcon icon={project.icon} color={project.color} />,
    })),
    ...(initiatives.data ?? []).map((initiative) => ({
      value: `initiative:${initiative.id}`,
      label: initiative.name,
      group: "Initiatives",
      icon: <InitiativeIcon initiative={initiative} />,
    })),
  ];
  const current = parentValue(value);
  return (
    <PropertyPicker
      label={label}
      searchPlaceholder="Find a project or initiative…"
      options={options}
      value={current ? [current] : []}
      isPending={projects.isPending || initiatives.isPending}
      onChange={([next]) => {
        const parent = next ? parentFromValue(next) : null;
        if (parent && next !== current) onChange(parent);
      }}
      trigger={trigger}
    />
  );
}

import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";

import type { ProjectDetail, ProjectRef } from "@/api/ops.gen";
import { PlusIcon, XIcon } from "@/components/icons";
import { PropertyPicker, type TPickerOption } from "@/components/issues/property-picker";
import { ProjectIcon } from "@/components/shared/project-icon";
import { Button } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { entitySlug } from "@/lib/helpers/entity-slug";
import { projectsQuery, useSaveProjectMutation } from "@/lib/queries/projects";

type TKind = keyof ProjectDetail["relations"];

const kinds: { kind: TKind; label: string; add: "blockedBy" | "blocks" | "relatedTo" }[] = [
  { kind: "blockedBy", label: "Blocked by", add: "blockedBy" },
  { kind: "blocks", label: "Blocking", add: "blocks" },
  { kind: "related", label: "Related to", add: "relatedTo" },
];

const removeArgs = {
  blockedBy: "removeBlockedBy",
  blocks: "removeBlocks",
  related: "removeRelatedTo",
} as const;

/** Projects this one waits on, holds up or relates to. */
export function ProjectDependencies({ project }: { project: ProjectDetail }) {
  const { workspace, canEdit } = useWorkspace();
  const { mutate: saveProject } = useSaveProjectMutation(workspace.id);
  const hasAny = kinds.some(({ kind }) => project.relations[kind].length > 0);
  if (!canEdit && !hasAny) return null;

  const change = (kind: TKind, add: boolean, projectId: string) => {
    const entry = kinds.find((item) => item.kind === kind);
    if (!entry) return;
    saveProject({ id: project.id, [add ? entry.add : removeArgs[kind]]: [projectId] });
  };

  return (
    <div className="flex w-full min-w-0 flex-col gap-1">
      <div className="flex h-8 items-center gap-2">
        <span className="text-muted-foreground flex-1 text-sm">Dependencies</span>
        {canEdit && <DependencyPicker project={project} onChange={change} />}
      </div>
      {kinds.map(({ kind, label }) =>
        project.relations[kind].length === 0 ? null : (
          <div key={kind} className="flex flex-col gap-0.5">
            <span className="text-muted-foreground px-2 text-xs">{label}</span>
            {project.relations[kind].map((related) => (
              <RelatedProject
                key={related.id}
                project={related}
                onRemove={canEdit ? () => change(kind, false, related.id) : undefined}
              />
            ))}
          </div>
        ),
      )}
    </div>
  );
}

function RelatedProject({ project, onRemove }: { project: ProjectRef; onRemove?: () => void }) {
  const { workspace } = useWorkspace();
  const { data } = useQuery(projectsQuery(workspace.id));
  const full = data?.find((item) => item.id === project.id);
  return (
    <div className="group/dep has-hover:hover:bg-accent flex h-8 min-w-0 items-center gap-2 rounded-md px-2 text-sm">
      <ProjectIcon icon={full?.icon} color={full?.color} />
      <Link
        to="/$workspace/project/$slug"
        params={{ workspace: workspace.urlKey, slug: entitySlug(project) }}
        className="min-w-0 flex-1 truncate"
      >
        {project.name}
      </Link>
      {onRemove && (
        <Button
          variant="ghost"
          size="icon"
          forceMinSize={false}
          aria-label={`Remove ${project.name}`}
          className="size-6 opacity-0 group-hover/dep:opacity-100 focus-visible:opacity-100"
          onClick={onRemove}
        >
          <XIcon className="size-3.5" />
        </Button>
      )}
    </div>
  );
}

function DependencyPicker({
  project,
  onChange,
}: {
  project: ProjectDetail;
  onChange: (kind: TKind, add: boolean, projectId: string) => void;
}) {
  const { workspace } = useWorkspace();
  const { data, isPending } = useQuery(projectsQuery(workspace.id));
  const others = (data ?? []).filter((item) => item.id !== project.id);
  const options: TPickerOption[] = kinds.flatMap(({ kind, label }) =>
    others.map((other) => ({
      value: `${kind}:${other.id}`,
      label: other.name,
      group: label,
      icon: <ProjectIcon icon={other.icon} color={other.color} />,
    })),
  );
  const value = kinds.flatMap(({ kind }) =>
    project.relations[kind].map((related) => `${kind}:${related.id}`),
  );
  return (
    <PropertyPicker
      label="Add dependency"
      searchPlaceholder="Find a project…"
      emptyText="No other projects"
      multiple
      align="end"
      options={options}
      value={value}
      isPending={isPending}
      onChange={(next) => {
        const added = next.find((item) => !value.includes(item));
        const removed = value.find((item) => !next.includes(item));
        const changed = added ?? removed;
        if (!changed) return;
        const [kind, id] = changed.split(":") as [TKind, string];
        onChange(kind, Boolean(added), id);
      }}
      trigger={
        <Button variant="ghost" size="icon" aria-label="Add dependency">
          <PlusIcon className="size-4" />
        </Button>
      }
    />
  );
}

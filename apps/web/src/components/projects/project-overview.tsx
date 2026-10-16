import type { ProjectDetail } from "@/api/ops.gen";
import { ParentDocuments } from "@/components/documents/document-list";
import { MarkdownBody } from "@/components/documents/markdown-body";
import { ArchivedBanner } from "@/components/projects/archived-banner";
import { MilestonesSection } from "@/components/projects/milestones-section";
import { ProjectHeaderBar, useProjectLifecycle } from "@/components/projects/project-header";
import { ProjectProgress } from "@/components/projects/project-progress";
import { ProjectProperties } from "@/components/projects/project-properties";
import { StatusUpdatesSection } from "@/components/projects/status-updates";
import { IconPicker } from "@/components/shared/icon-picker";
import { InlineText } from "@/components/shared/inline-text";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { projectNameMaxLength, useSaveProjectMutation } from "@/lib/queries/projects";

export function ProjectOverview({ project }: { project: ProjectDetail }) {
  const { workspace, canEdit } = useWorkspace();
  const { mutate: saveProject, mutateAsync: saveProjectAsync } = useSaveProjectMutation(
    workspace.id,
  );
  const lifecycle = useProjectLifecycle(project.id);

  return (
    <div className="flex w-full flex-col gap-6">
      <ProjectHeaderBar project={project} tab="overview" />
      <div className="flex w-full flex-col gap-6 lg:flex-row lg:items-start">
        <div className="mx-auto flex w-full max-w-3xl min-w-0 flex-1 flex-col gap-6">
          {project.archivedAt && (
            <ArchivedBanner
              noun="project"
              isPending={lifecycle.isPending}
              onUnarchive={canEdit ? () => lifecycle.run("unarchive") : undefined}
            />
          )}
          <div className="flex flex-col gap-2">
            <div className="flex min-w-0 items-center gap-2">
              <IconPicker
                value={project.icon}
                color={project.color}
                disabled={!canEdit}
                label="Project icon"
                onChange={(icon) => saveProject({ id: project.id, icon })}
              />
              <InlineText
                label="Project name"
                value={project.name}
                maxLength={projectNameMaxLength}
                readOnly={!canEdit}
                onSave={(name) => saveProject({ id: project.id, name })}
              />
            </div>
            {(canEdit || project.summary) && (
              <InlineText
                label="Project summary"
                variant="summary"
                value={project.summary ?? ""}
                maxLength={255}
                allowEmpty
                readOnly={!canEdit}
                placeholder="Add a one-line summary…"
                onSave={(summary) => saveProject({ id: project.id, summary: summary || null })}
              />
            )}
          </div>
          <ProjectProgress project={project} />
          <MarkdownBody
            key={project.id}
            label="Description"
            placeholder="Describe the plan: goals, scope, links…"
            initialValue={project.description ?? ""}
            readOnly={!canEdit}
            onSave={(description) => saveProjectAsync({ id: project.id, description })}
          />
          <MilestonesSection project={project} />
          <StatusUpdatesSection
            parent={{ project: project.id }}
            noun="project"
            health={project.health}
          />
          <ParentDocuments parent={{ project: project.id }} />
        </div>
        <aside aria-label="Properties" className="w-full shrink-0 lg:sticky lg:top-0 lg:w-80">
          <div className="bg-card rounded-lg border p-4">
            <ProjectProperties project={project} />
          </div>
        </aside>
      </div>
    </div>
  );
}

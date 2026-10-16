import { useNavigate } from "@tanstack/react-router";

import type { ProjectDetail } from "@/api/ops.gen";
import { FavoriteButton } from "@/components/favorites/favorite-button";
import { Breadcrumb, type TBreadcrumbItem } from "@/components/layout/breadcrumb";
import { EntityMenu, toastTrashed } from "@/components/projects/entity-menu";
import { CopyLinkAction, HeaderActions } from "@/components/shared/detail-actions";
import { ProjectIcon } from "@/components/shared/project-icon";
import { Tabs, TabsList, TabsTab } from "@/components/ui/tabs";
import { toast } from "@/components/ui/toast";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { entitySlug } from "@/lib/helpers/entity-slug";
import { type TProjectLifecycleAction, useProjectLifecycleMutation } from "@/lib/queries/projects";

export type TProjectTab = "overview" | "issues";

const lifecycleToasts: Record<TProjectLifecycleAction, string> = {
  archive: "Project archived",
  unarchive: "Project unarchived",
  trash: "Project moved to trash",
  restore: "Project restored",
};

export function useProjectLifecycle(projectId: string) {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const { mutateAsync, isPending } = useProjectLifecycleMutation(workspace.id);
  const run = (action: TProjectLifecycleAction) =>
    void mutateAsync({ id: projectId, action })
      .then(() => toast.add({ type: "success", title: lifecycleToasts[action] }))
      .catch(() => undefined);
  const trash = () =>
    void mutateAsync({ id: projectId, action: "trash" })
      .then(async () => {
        await navigate({ to: "/$workspace/projects", params: { workspace: workspace.urlKey } });
        toastTrashed("Project moved to trash", () => run("restore"));
      })
      .catch(() => undefined);
  return { run, trash, isPending };
}

function ProjectActions({ project }: { project: ProjectDetail }) {
  const { canEdit } = useWorkspace();
  const lifecycle = useProjectLifecycle(project.id);
  return (
    <HeaderActions>
      <FavoriteButton kind="project" entityId={project.id} />
      <CopyLinkAction url={project.url} />
      {canEdit && (
        <EntityMenu
          noun="project"
          isArchived={Boolean(project.archivedAt)}
          onArchive={(archive) => lifecycle.run(archive ? "archive" : "unarchive")}
          onTrash={lifecycle.trash}
        />
      )}
    </HeaderActions>
  );
}

function ProjectTabs({ project, tab }: { project: ProjectDetail; tab: TProjectTab }) {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const params = { workspace: workspace.urlKey, slug: entitySlug(project) };
  return (
    <Tabs
      value={tab}
      onValueChange={(value) =>
        void navigate(
          value === "issues"
            ? { to: "/$workspace/project/$slug/issues", params }
            : { to: "/$workspace/project/$slug", params },
        )
      }
    >
      <TabsList>
        <TabsTab value="overview">Overview</TabsTab>
        <TabsTab value="issues">
          Issues
          {project.issueCount > 0 && (
            <span className="text-muted-foreground text-xs font-normal tabular-nums">
              {project.issueCount}
            </span>
          )}
        </TabsTab>
      </TabsList>
    </Tabs>
  );
}

export function projectBreadcrumb(
  workspaceUrlKey: string,
  project: Pick<ProjectDetail, "name" | "icon" | "color">,
): TBreadcrumbItem[] {
  return [
    {
      label: "Projects",
      link: { to: "/$workspace/projects", params: { workspace: workspaceUrlKey } },
    },
    {
      label: project.name,
      icon: <ProjectIcon icon={project.icon} color={project.color} size="sm" />,
    },
  ];
}

export function ProjectHeaderBar({ project, tab }: { project: ProjectDetail; tab: TProjectTab }) {
  const { workspace } = useWorkspace();
  return (
    <div className="flex w-full flex-col gap-2">
      <div className="flex w-full min-w-0 items-center gap-2">
        <div className="min-w-0 flex-1">
          <Breadcrumb items={projectBreadcrumb(workspace.urlKey, project)} />
        </div>
        <ProjectActions project={project} />
      </div>
      <ProjectTabs project={project} tab={tab} />
    </div>
  );
}

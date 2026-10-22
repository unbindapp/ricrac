import { useState } from "react";
import { useLocalStorage } from "usehooks-ts";

import {
  BoxIcon,
  LayoutGridIcon,
  ListIcon,
  PlusIcon,
  SearchIcon,
} from "@/components/icons/symbols.gen";
import type { TBreadcrumbItem } from "@/components/layout/breadcrumb";
import { EmptyState } from "@/components/layout/empty-state";
import { PageContainer } from "@/components/layout/page-container";
import { PageHeader } from "@/components/layout/page-header";
import { PageToolbar } from "@/components/layout/page-toolbar";
import { useNewProjectStore } from "@/components/projects/new-project-store";
import { ProjectBoard } from "@/components/projects/project-board";
import type { TProjectScope } from "@/components/projects/project-groups";
import { ProjectList } from "@/components/projects/project-list";
import { ScopeEmpty, ScopeTabs } from "@/components/projects/scope-tabs";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { SimpleTooltip } from "@/components/ui/tooltip";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import type { TProjectListArgs } from "@/lib/queries/projects";

type TLayout = "list" | "board";

type TProps = {
  title: string;
  breadcrumb?: TBreadcrumbItem[];
  storageKey: string;
  filters: TProjectListArgs;
};

export function ProjectsView({ title, breadcrumb, storageKey, filters }: TProps) {
  const { canEdit } = useWorkspace();
  const openNewProject = useNewProjectStore((state) => state.open);
  const [layout, setLayout] = useLocalStorage<TLayout>(`projects-layout:${storageKey}`, "list");
  const [query, setQuery] = useState("");
  const [scope, setScope] = useState<TProjectScope>("active");
  const create = () => openNewProject();
  const empty =
    scope !== "active" ? (
      <ScopeEmpty scope={scope} noun="project" />
    ) : (
      <EmptyState
        icon={BoxIcon}
        title="No projects yet"
        description="Projects group issues toward a goal, with milestones and status updates."
        action={
          canEdit && (
            <Button size="sm" variant="outline" onClick={create}>
              <PlusIcon className="size-4" />
              New project
            </Button>
          )
        }
      />
    );
  const noMatches = (
    <EmptyState
      icon={SearchIcon}
      title="No matching projects"
      description={`Nothing matches “${query.trim()}”.`}
    />
  );
  const listProps = { filters, query, scope, empty, noMatches };
  const isBoard = layout === "board" && scope === "active";

  return (
    <PageContainer>
      <PageHeader
        title={title}
        icon={<BoxIcon />}
        breadcrumb={breadcrumb}
        actions={
          canEdit && (
            <Button onClick={create}>
              <PlusIcon className="size-4" />
              New project
            </Button>
          )
        }
        toolbar={
          <PageToolbar
            actions={
              <>
                <ScopeTabs scope={scope} onChange={setScope} />
                {scope === "active" && (
                  <div role="radiogroup" aria-label="Layout" className="flex items-center gap-1">
                    <SimpleTooltip content="List">
                      <Button
                        variant={layout === "list" ? "secondary" : "ghost"}
                        size="icon"
                        role="radio"
                        aria-checked={layout === "list"}
                        aria-label="List layout"
                        onClick={() => setLayout("list")}
                      >
                        <ListIcon className="size-4" />
                      </Button>
                    </SimpleTooltip>
                    <SimpleTooltip content="Board">
                      <Button
                        variant={layout === "board" ? "secondary" : "ghost"}
                        size="icon"
                        role="radio"
                        aria-checked={layout === "board"}
                        aria-label="Board layout"
                        onClick={() => setLayout("board")}
                      >
                        <LayoutGridIcon className="size-4" />
                      </Button>
                    </SimpleTooltip>
                  </div>
                )}
              </>
            }
          >
            <div className="relative w-full max-w-xs">
              <SearchIcon className="text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
              <Input
                aria-label="Filter projects"
                placeholder="Filter projects…"
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Escape") setQuery("");
                }}
                className="h-8 pl-8"
              />
            </div>
          </PageToolbar>
        }
      />
      {isBoard ? <ProjectBoard {...listProps} /> : <ProjectList {...listProps} />}
    </PageContainer>
  );
}

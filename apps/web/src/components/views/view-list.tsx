import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { BoxIcon, LayersIcon, PlusIcon } from "lucide-react";

import { FavoriteButton } from "@/components/favorites/favorite-button";
import { List } from "@/components/layout/list";
import { ListStates } from "@/components/layout/list-states";
import { SectionTitle } from "@/components/layout/section-title";
import { Avatar } from "@/components/shared/avatar";
import { EntityIcon } from "@/components/shared/entity-icon";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ViewMenu } from "@/components/views/view-menu";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { type TView, viewParam, viewsQuery } from "@/lib/queries/views";

export function ViewList({ onCreate }: { onCreate?: () => void }) {
  const { workspace } = useWorkspace();
  const { data, isPending, error, refetch } = useQuery(viewsQuery(workspace.id));
  const views = data ?? [];
  const sections = [
    { title: "Shared with the workspace", views: views.filter((view) => view.shared) },
    { title: "Only you", views: views.filter((view) => !view.shared) },
  ].filter((section) => section.views.length > 0);

  return (
    <ListStates
      isPending={isPending}
      error={error}
      onRetry={() => void refetch()}
      isEmpty={views.length === 0}
      empty={{
        icon: LayersIcon,
        title: "No views yet",
        description: "A view saves a filter of issues or projects and keeps it up to date.",
        action: onCreate && (
          <Button size="sm" variant="outline" onClick={onCreate}>
            <PlusIcon className="size-4" />
            New view
          </Button>
        ),
      }}
    >
      <div className="flex w-full flex-col gap-6">
        {sections.map((section) => (
          <section key={section.title} className="flex w-full flex-col gap-2">
            <SectionTitle title={section.title} />
            <List>
              {section.views.map((view) => (
                <ViewRow key={view.id} view={view} />
              ))}
            </List>
          </section>
        ))}
      </div>
    </ListStates>
  );
}

function ViewRow({ view }: { view: TView }) {
  const { workspace } = useWorkspace();
  const isProjects = view.model === "project";
  return (
    <div
      role="listitem"
      className="has-hover:hover:bg-accent/50 flex h-11 w-full items-center gap-2 border-b px-3 text-sm last:border-b-0"
    >
      <EntityIcon
        icon={view.icon}
        color={view.color}
        fallback={isProjects ? BoxIcon : LayersIcon}
      />
      <Link
        to="/$workspace/view/$view_id"
        params={{ workspace: workspace.urlKey, view_id: viewParam(view) }}
        className="flex min-w-0 flex-1 items-baseline gap-2 outline-hidden focus-visible:underline"
      >
        <span className="truncate font-medium">{view.name}</span>
        {view.description && (
          <span className="text-muted-foreground hidden truncate text-xs sm:inline">
            {view.description}
          </span>
        )}
      </Link>
      {isProjects && <Badge variant="muted">Projects</Badge>}
      {view.shared && view.owner && (
        <Avatar id={view.owner.id} name={view.owner.displayName} size="sm" />
      )}
      <FavoriteButton kind="customView" entityId={view.id} />
      <ViewMenu view={view} />
    </div>
  );
}

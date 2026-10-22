import { useQuery } from "@tanstack/react-query";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { BoxIcon, BracesIcon, GlobeIcon, LayersIcon, LockIcon } from "lucide-react";

import { FavoriteButton } from "@/components/favorites/favorite-button";
import type { TBreadcrumbItem } from "@/components/layout/breadcrumb";
import { EmptyState } from "@/components/layout/empty-state";
import { ErrorCard } from "@/components/layout/error-card";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { PageContainer } from "@/components/layout/page-container";
import { PageHeader } from "@/components/layout/page-header";
import { PageToolbar } from "@/components/layout/page-toolbar";
import { EntityNotFound } from "@/components/navigation/not-found-template";
import { EntityIcon } from "@/components/shared/entity-icon";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { SimpleTooltip } from "@/components/ui/tooltip";
import {
  conditionsToFilter,
  filterToConditions,
  type TCondition,
  type TFilterModel,
} from "@/components/views/view-filter";
import { ViewFilterBar } from "@/components/views/view-filter-bar";
import { ViewFilterJsonDialog } from "@/components/views/view-filter-json-dialog";
import { useCanEditView, ViewMenu } from "@/components/views/view-menu";
import { ViewIssues, ViewProjects } from "@/components/views/view-results";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { type TView, useSaveViewMutation, viewQuery } from "@/lib/queries/views";
import { isApiError } from "@/lib/server/errors";

export const Route = createFileRoute("/$workspace/_workspace/view/$view_id")({
  component: ViewPage,
});

function ViewPage() {
  const { view_id } = Route.useParams();
  const { workspace } = useWorkspace();
  const { data: view, isPending, error, refetch } = useQuery(viewQuery(workspace.id, view_id));
  const breadcrumb: TBreadcrumbItem[] = [
    { label: "Views", link: { to: "/$workspace/views", params: { workspace: workspace.urlKey } } },
  ];

  if (view) return <SavedView key={view.id} view={view} breadcrumb={breadcrumb} />;
  const isMissing = isApiError(error) && error.status >= 400 && error.status < 500;
  return (
    <PageContainer>
      <PageHeader title="View" icon={<LayersIcon />} breadcrumb={breadcrumb} />
      {isPending && <ListSkeleton />}
      {isMissing && (
        <EntityNotFound
          Icon={LayersIcon}
          description="This view doesn't exist anymore, or it's private to someone else."
        />
      )}
      {error && !isMissing && <ErrorCard message={error.message} onRetry={() => void refetch()} />}
    </PageContainer>
  );
}

function SavedView({ view, breadcrumb }: { view: TView; breadcrumb: TBreadcrumbItem[] }) {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const canEdit = useCanEditView(view);
  const save = useSaveViewMutation(workspace.id);
  const model: TFilterModel = view.model === "project" ? "project" : "issue";
  const saved = filterToConditions(view.filter, model);
  const [draft, setDraft] = useState<TCondition[] | null>(saved);
  const [isEditingJson, setIsEditingJson] = useState(false);
  const draftFilter = draft ? conditionsToFilter(draft) : view.filter;
  const isDirty =
    !!saved && JSON.stringify(draftFilter) !== JSON.stringify(conditionsToFilter(saved));

  const toolbar = (
    <PageToolbar
      actions={
        <>
          {isDirty && (
            <>
              <Button variant="ghost" size="sm" onClick={() => setDraft(saved)}>
                Reset
              </Button>
              {canEdit && (
                <Button
                  size="sm"
                  isPending={save.isPending}
                  onClick={() => save.mutate({ id: view.id, filter: draftFilter })}
                >
                  Save filters
                </Button>
              )}
            </>
          )}
          <SimpleTooltip
            content={view.shared ? "Shared with the workspace" : "Only you see this view"}
          >
            <Badge variant="outline">
              {view.shared ? <GlobeIcon /> : <LockIcon />}
              {view.shared ? "Shared" : "Private"}
            </Badge>
          </SimpleTooltip>
        </>
      }
    >
      {draft ? (
        <ViewFilterBar model={model} conditions={draft} onChange={setDraft} />
      ) : (
        <div className="flex min-w-0 items-center gap-2">
          <SimpleTooltip
            content={<pre className="text-xs">{JSON.stringify(view.filter, null, 2)}</pre>}
          >
            <Badge variant="muted">
              <BracesIcon />
              Custom filter
            </Badge>
          </SimpleTooltip>
          {canEdit && (
            <Button variant="ghost" size="sm" onClick={() => setIsEditingJson(true)}>
              Edit as JSON
            </Button>
          )}
        </div>
      )}
    </PageToolbar>
  );

  return (
    <PageContainer>
      <PageHeader
        title={view.name}
        icon={
          <EntityIcon
            icon={view.icon}
            color={view.color}
            fallback={model === "project" ? BoxIcon : LayersIcon}
            size="lg"
          />
        }
        description={view.description ?? undefined}
        breadcrumb={[...breadcrumb, { label: view.name }]}
        actions={
          <>
            <FavoriteButton kind="customView" entityId={view.id} />
            <ViewMenu
              view={view}
              onDeleted={() =>
                void navigate({ to: "/$workspace/views", params: { workspace: workspace.urlKey } })
              }
            />
          </>
        }
        toolbar={toolbar}
      />
      {view.model === "initiative" ? (
        <EmptyState
          icon={LayersIcon}
          title="Initiative views open in the API"
          description="This view lists initiatives; list_initiatives shows them."
        />
      ) : model === "project" ? (
        <ViewProjects filter={draftFilter} />
      ) : (
        <ViewIssues filter={draftFilter} />
      )}
      <ViewFilterJsonDialog
        view={isEditingJson ? view : null}
        onClose={() => setIsEditingJson(false)}
      />
    </PageContainer>
  );
}

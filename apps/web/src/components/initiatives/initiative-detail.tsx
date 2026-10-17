import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { TargetIcon } from "lucide-react";

import type { InitiativeDetail as TInitiativeDetail } from "@/api/ops.gen";
import { FavoriteButton } from "@/components/favorites/favorite-button";
import { ParentDocuments } from "@/components/documents/document-list";
import { AutosaveMarkdown } from "@/components/shared/autosave-markdown";
import { InitiativeProjects, SubInitiatives } from "@/components/initiatives/initiative-projects";
import { InitiativeProperties } from "@/components/initiatives/initiative-properties";
import { InitiativeIcon } from "@/components/initiatives/initiative-status";
import { Breadcrumb, type TBreadcrumbItem } from "@/components/layout/breadcrumb";
import { ErrorCard } from "@/components/layout/error-card";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { EntityNotFound } from "@/components/navigation/not-found-template";
import { ArchivedBanner } from "@/components/projects/archived-banner";
import { EntityMenu, toastTrashed } from "@/components/projects/entity-menu";
import { ProgressBar } from "@/components/projects/progress-bar";
import { StatusUpdatesSection } from "@/components/projects/status-updates";
import { CopyLinkAction, HeaderActions } from "@/components/shared/detail-actions";
import { IconPicker } from "@/components/shared/icon-picker";
import { InlineText } from "@/components/shared/inline-text";
import { toast } from "@/components/ui/toast";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { entitySlug } from "@/lib/helpers/entity-slug";
import {
  initiativeNameMaxLength,
  initiativeQuery,
  type TInitiativeLifecycleAction,
  useInitiativeLifecycleMutation,
  useSaveInitiativeMutation,
} from "@/lib/queries/initiatives";
import { isMissing } from "@/lib/queries/projects";

const lifecycleToasts: Record<TInitiativeLifecycleAction, string> = {
  archive: "Initiative archived",
  unarchive: "Initiative unarchived",
  trash: "Initiative moved to trash",
  restore: "Initiative restored",
};

function useInitiativeLifecycle(id: string) {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const { mutateAsync, isPending } = useInitiativeLifecycleMutation(workspace.id);
  const run = (action: TInitiativeLifecycleAction) =>
    void mutateAsync({ id, action })
      .then(() => toast.add({ type: "success", title: lifecycleToasts[action] }))
      .catch(() => undefined);
  const trash = () =>
    void mutateAsync({ id, action: "trash" })
      .then(async () => {
        await navigate({ to: "/$workspace/initiatives", params: { workspace: workspace.urlKey } });
        toastTrashed("Initiative moved to trash", () => run("restore"));
      })
      .catch(() => undefined);
  return { run, trash, isPending };
}

export function InitiativeDetail({ slug }: { slug: string }) {
  const { workspace } = useWorkspace();
  const { data, isPending, error, refetch } = useQuery(initiativeQuery(workspace.id, slug));
  if (isPending) return <ListSkeleton rows={4} />;
  if (error) {
    if (isMissing(error)) {
      return (
        <EntityNotFound
          Icon={TargetIcon}
          description="Initiative not found. It was deleted, or you cannot see it."
        />
      );
    }
    return <ErrorCard message={error.message} onRetry={() => void refetch()} />;
  }
  return <InitiativeDetailContent key={data.id} initiative={data} />;
}

function initiativeBreadcrumb(
  workspaceUrlKey: string,
  initiative: TInitiativeDetail,
): TBreadcrumbItem[] {
  const items: TBreadcrumbItem[] = [
    {
      label: "Initiatives",
      link: { to: "/$workspace/initiatives", params: { workspace: workspaceUrlKey } },
    },
  ];
  if (initiative.parent) {
    items.push({
      label: initiative.parent.name,
      link: {
        to: "/$workspace/initiative/$slug",
        params: { workspace: workspaceUrlKey, slug: entitySlug(initiative.parent) },
      },
    });
  }
  items.push({
    label: initiative.name,
    icon: <InitiativeIcon initiative={initiative} size="sm" />,
  });
  return items;
}

function InitiativeDetailContent({ initiative }: { initiative: TInitiativeDetail }) {
  const { workspace, canEdit } = useWorkspace();
  const { mutate: save, mutateAsync: saveAsync } = useSaveInitiativeMutation(workspace.id);
  const lifecycle = useInitiativeLifecycle(initiative.id);

  return (
    <div className="flex w-full flex-col gap-6">
      <div className="flex w-full min-w-0 items-center gap-2">
        <div className="min-w-0 flex-1">
          <Breadcrumb items={initiativeBreadcrumb(workspace.urlKey, initiative)} />
        </div>
        <HeaderActions>
          <FavoriteButton kind="initiative" entityId={initiative.id} />
          <CopyLinkAction url={initiative.url} />
          {canEdit && (
            <EntityMenu
              noun="initiative"
              isArchived={Boolean(initiative.archivedAt)}
              onArchive={(archive) => lifecycle.run(archive ? "archive" : "unarchive")}
              onTrash={lifecycle.trash}
            />
          )}
        </HeaderActions>
      </div>
      <div className="flex w-full flex-col gap-6 lg:flex-row lg:items-start lg:justify-between">
        <div className="flex w-full max-w-3xl min-w-0 flex-1 flex-col gap-6">
          {initiative.archivedAt && (
            <ArchivedBanner
              noun="initiative"
              isPending={lifecycle.isPending}
              onUnarchive={canEdit ? () => lifecycle.run("unarchive") : undefined}
            />
          )}
          <div className="flex flex-col gap-2">
            <div className="flex min-w-0 items-center gap-2">
              <IconPicker
                value={initiative.icon}
                color={initiative.color}
                disabled={!canEdit}
                label="Initiative icon"
                onChange={(icon) => save({ id: initiative.id, icon })}
              />
              <InlineText
                value={initiative.name}
                label="Initiative name"
                maxLength={initiativeNameMaxLength}
                readOnly={!canEdit}
                onSave={(name) => save({ id: initiative.id, name })}
              />
            </div>
            {(canEdit || initiative.summary) && (
              <InlineText
                label="Initiative summary"
                variant="summary"
                value={initiative.summary ?? ""}
                maxLength={255}
                allowEmpty
                readOnly={!canEdit}
                placeholder="Add a one-line summary…"
                onSave={(summary) => save({ id: initiative.id, summary: summary || null })}
              />
            )}
          </div>
          {initiative.projectCount > 0 && (
            <div className="bg-card flex flex-col gap-2 rounded-lg border p-4">
              <div className="flex items-baseline gap-2">
                <span className="text-base font-semibold tabular-nums">
                  {Math.round(initiative.progress * 100)}%
                </span>
                <span className="text-muted-foreground text-sm">
                  average progress of {initiative.projectCount} project
                  {initiative.projectCount === 1 ? "" : "s"}
                </span>
              </div>
              <ProgressBar
                value={initiative.progress}
                label="Initiative progress"
                showValue={false}
              />
            </div>
          )}
          <AutosaveMarkdown
            key={initiative.id}
            label="Description"
            placeholder="Why it matters and how you'll know it's done…"
            value={initiative.description ?? ""}
            readOnly={!canEdit}
            onSave={async (after, before) => {
              const saved = await saveAsync({ id: initiative.id, edit: { before, after } });
              return saved.description ?? "";
            }}
          />
          <InitiativeProjects initiative={initiative} />
          <SubInitiatives initiative={initiative} />
          <StatusUpdatesSection
            parent={{ initiative: initiative.id }}
            noun="initiative"
            health={initiative.health}
          />
          <ParentDocuments parent={{ initiative: initiative.id }} />
        </div>
        <aside aria-label="Properties" className="w-full shrink-0 lg:sticky lg:top-0 lg:w-80">
          <div className="bg-card rounded-lg border p-4">
            <InitiativeProperties initiative={initiative} />
          </div>
        </aside>
      </div>
    </div>
  );
}

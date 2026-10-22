import { useQuery } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";

import type { ReleaseDetail as TReleaseDetail, ReleasePipeline } from "@/api/ops.gen";
import {
  ArchiveIcon,
  EllipsisIcon,
  FileTextIcon,
  PencilIcon,
  PlusIcon,
  RocketIcon,
} from "@/components/icons";
import { PageBar } from "@/components/layout/page-header";
import { Breadcrumb } from "@/components/layout/breadcrumb";
import { ErrorCard } from "@/components/layout/error-card";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { PageContainer } from "@/components/layout/page-container";
import { PropertyRow } from "@/components/layout/property-row";
import { SectionTitle } from "@/components/layout/section-title";
import { EntityNotFound } from "@/components/navigation/not-found-template";
import { ReleaseDialog } from "@/components/releases/release-dialog";
import { ReleaseIssues } from "@/components/releases/release-issues";
import { ReleaseStagePicker } from "@/components/releases/release-stage-picker";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { InlineText } from "@/components/shared/inline-text";
import { MarkdownEditor } from "@/components/shared/markdown-editor";
import { RelativeTime } from "@/components/shared/relative-time";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { useUploadAsset } from "@/lib/queries/uploads";
import { formatDay, shortSha } from "@/lib/helpers/releases";
import {
  pipelinesQuery,
  releaseQuery,
  useDeleteReleaseMutation,
  useSaveReleaseMutation,
  useSaveReleaseNoteMutation,
} from "@/lib/queries/releases";
import { isNotFoundError } from "@/lib/server/errors";

export function ReleaseDetail({ slug }: { slug: string }) {
  const { workspace } = useWorkspace();
  const release = useQuery(releaseQuery(workspace.id, slug));
  const pipelines = useQuery(pipelinesQuery(workspace.id));
  if (release.isPending || pipelines.isPending) {
    return (
      <PageContainer>
        <ListSkeleton />
      </PageContainer>
    );
  }
  if (isNotFoundError(release.error)) {
    return (
      <PageContainer>
        <EntityNotFound
          Icon={RocketIcon}
          description="Release not found. It was deleted, or you cannot see its pipeline."
        />
      </PageContainer>
    );
  }
  const error = release.error ?? pipelines.error;
  if (error) {
    return (
      <PageContainer>
        <ErrorCard
          message={error.message}
          onRetry={() => void Promise.all([release.refetch(), pipelines.refetch()])}
        />
      </PageContainer>
    );
  }
  const pipeline = pipelines.data?.find((item) => item.id === release.data?.pipeline.id);
  if (!release.data || !pipeline) return null;
  return <ReleasePage release={release.data} pipeline={pipeline} />;
}

function ReleasePage({
  release,
  pipeline,
}: {
  release: TReleaseDetail;
  pipeline: ReleasePipeline;
}) {
  const { workspace, canEdit } = useWorkspace();
  const { mutate: save, mutateAsync: saveAsync } = useSaveReleaseMutation(workspace.id);
  const upload = useUploadAsset(workspace.id);
  const description = release.description ?? "";
  const params = { workspace: workspace.urlKey };

  return (
    <PageContainer>
      <PageBar>
        <div className="min-w-0 flex-1">
          <Breadcrumb
            items={[
              {
                label: "Releases",
                icon: <RocketIcon className="size-4" />,
                link: { to: "/$workspace/releases", params },
              },
              {
                label: pipeline.name,
                link: { to: "/$workspace/releases", params, search: { pipeline: pipeline.slugId } },
              },
              { label: release.name },
            ]}
          />
        </div>
        {canEdit && <ReleaseActions release={release} pipeline={pipeline} />}
      </PageBar>
      <div className="flex w-full flex-col gap-6 lg:flex-row">
        <div className="flex min-w-0 flex-1 flex-col gap-6">
          <div className="flex flex-col gap-2">
            <div className="flex min-w-0 items-center gap-2">
              <InlineText
                label="Release name"
                value={release.name}
                maxLength={120}
                readOnly={!canEdit}
                onSave={(name) => save({ id: release.id, name })}
              />
              {release.version && (
                <Badge variant="muted" className="font-mono">
                  {release.version}
                </Badge>
              )}
              {release.archivedAt && <Badge variant="muted">Archived</Badge>}
            </div>
            <MarkdownEditor
              key={release.id}
              label="Description"
              value={description}
              onUpload={upload}
              placeholder={canEdit ? "Add a description…" : "No description."}
              readOnly={!canEdit}
              autosave={{
                onSave: async (markdown) => {
                  const saved = await saveAsync({
                    id: release.id,
                    description: markdown.trim() || null,
                  });
                  return saved.description ?? "";
                },
              }}
            />
          </div>
          <ReleaseIssues release={release} />
          <ReleaseNotesSection release={release} />
        </div>
        <aside aria-label="Properties" className="w-full shrink-0 lg:w-72">
          <div className="flex flex-col gap-1 rounded-lg border p-4 lg:sticky lg:top-0">
            <ReleaseProperties release={release} pipeline={pipeline} />
          </div>
        </aside>
      </div>
    </PageContainer>
  );
}

function Value({ children, muted }: { children: ReactNode; muted?: boolean }) {
  return (
    <span
      data-muted={muted || undefined}
      className="data-muted:text-muted-foreground min-w-0 truncate px-2.5"
    >
      {children}
    </span>
  );
}

function ReleaseProperties({
  release,
  pipeline,
}: {
  release: TReleaseDetail;
  pipeline: ReleasePipeline;
}) {
  const { workspace, canEdit } = useWorkspace();
  const { mutate: save } = useSaveReleaseMutation(workspace.id);
  return (
    <>
      <PropertyRow label="Stage">
        <ReleaseStagePicker
          stages={pipeline.stages ?? []}
          value={release.stage}
          disabled={!canEdit}
          onChange={(stage) => save({ id: release.id, stage })}
        />
      </PropertyRow>
      <PropertyRow label="Pipeline">
        <Value>{pipeline.name}</Value>
      </PropertyRow>
      <PropertyRow label="Version">
        <Value muted={!release.version}>
          {release.version ? (
            <span className="font-mono text-xs">{release.version}</span>
          ) : (
            "Not set"
          )}
        </Value>
      </PropertyRow>
      <PropertyRow label="Start date">
        <Value muted={!release.startDate}>
          {release.startDate ? formatDay(release.startDate) : "Not set"}
        </Value>
      </PropertyRow>
      <PropertyRow label="Target date">
        <Value muted={!release.targetDate}>
          {release.targetDate ? formatDay(release.targetDate) : "Not set"}
        </Value>
      </PropertyRow>
      {release.startedAt && (
        <PropertyRow label="Started">
          <Value>
            <RelativeTime value={release.startedAt} mode="date" />
          </Value>
        </PropertyRow>
      )}
      {release.completedAt && (
        <PropertyRow label="Released">
          <Value>
            <RelativeTime value={release.completedAt} mode="date" />
          </Value>
        </PropertyRow>
      )}
      {release.canceledAt && (
        <PropertyRow label="Canceled">
          <Value>
            <RelativeTime value={release.canceledAt} mode="date" />
          </Value>
        </PropertyRow>
      )}
      <PropertyRow label="Commit">
        <Value muted={!release.commitSha}>
          {release.commitSha ? (
            <span className="font-mono text-xs" title={release.commitSha}>
              {shortSha(release.commitSha)}
            </span>
          ) : (
            "Not set"
          )}
        </Value>
      </PropertyRow>
      <PropertyRow label="Created">
        <Value muted>
          {release.creator ? `${release.creator.name}, ` : ""}
          <RelativeTime value={release.createdAt} />
        </Value>
      </PropertyRow>
    </>
  );
}

function ReleaseNotesSection({ release }: { release: TReleaseDetail }) {
  const { workspace, canEdit } = useWorkspace();
  const navigate = useNavigate();
  const { mutate: saveNote, isPending } = useSaveReleaseNoteMutation(workspace.id);
  const write = () =>
    saveNote(
      { pipeline: release.pipeline.id, releases: [release.id] },
      {
        onSuccess: (note) =>
          navigate({
            to: "/$workspace/release-note/$slug",
            params: { workspace: workspace.urlKey, slug: note.slugId },
          }),
      },
    );
  return (
    <section aria-label="Release notes" className="flex flex-col gap-3">
      <SectionTitle
        title="Release notes"
        actions={
          canEdit && (
            <Button variant="ghost" size="sm" isPending={isPending} onClick={write}>
              <PlusIcon className="size-4" />
              Write release notes
            </Button>
          )
        }
      />
      {release.releaseNotes.length === 0 ? (
        <p className="text-muted-foreground text-sm">No release notes cover this release yet.</p>
      ) : (
        <div role="list" className="flex w-full flex-col overflow-hidden rounded-lg border">
          {release.releaseNotes.map((note) => (
            <Link
              key={note.id}
              role="listitem"
              to="/$workspace/release-note/$slug"
              params={{ workspace: workspace.urlKey, slug: note.slugId }}
              className="has-hover:hover:bg-accent focus-visible:ring-primary/8-10 flex h-9 items-center gap-2 border-b px-3 text-sm outline-hidden last:border-b-0 focus-visible:ring-1 focus-visible:ring-inset"
            >
              <FileTextIcon className="text-muted-foreground size-4 shrink-0" />
              <span className="min-w-0 flex-1 truncate">{note.title}</span>
              <span className="text-muted-foreground shrink-0 text-xs">
                <RelativeTime value={note.createdAt} />
              </span>
            </Link>
          ))}
        </div>
      )}
    </section>
  );
}

function ReleaseActions({
  release,
  pipeline,
}: {
  release: TReleaseDetail;
  pipeline: ReleasePipeline;
}) {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const { mutateAsync: archive } = useDeleteReleaseMutation(workspace.id);
  const [isArchiveOpen, setIsArchiveOpen] = useState(false);
  return (
    <>
      <ReleaseDialog
        pipeline={pipeline}
        release={release}
        trigger={
          <Button variant="outline" size="sm">
            <PencilIcon className="size-4" />
            Edit
          </Button>
        }
      />
      {!release.archivedAt && (
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button variant="ghost" size="icon" aria-label="More actions">
                <EllipsisIcon className="size-4" />
              </Button>
            }
          />
          <DropdownMenuContent align="end" className="w-48">
            <DropdownMenuItem className="text-destructive" onClick={() => setIsArchiveOpen(true)}>
              <ArchiveIcon className="size-4" />
              Archive release
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      )}
      <ConfirmDialog
        open={isArchiveOpen}
        onOpenChange={setIsArchiveOpen}
        title={`Archive ${release.name}`}
        description="It leaves the releases list but stays readable and keeps its issues."
        confirmLabel="Archive release"
        onConfirm={async () => {
          await archive(release.id);
          await navigate({
            to: "/$workspace/releases",
            params: { workspace: workspace.urlKey },
            search: { pipeline: pipeline.slugId },
          });
        }}
      />
    </>
  );
}

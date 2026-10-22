import { useQuery } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";

import type { ReleaseNote } from "@/api/ops.gen";
import { FileTextIcon, PlusIcon, RocketIcon, Trash2Icon } from "@/components/icons";
import { PropertyPicker } from "@/components/issues/property-picker";
import { PageBar } from "@/components/layout/page-header";
import { Breadcrumb } from "@/components/layout/breadcrumb";
import { ErrorCard } from "@/components/layout/error-card";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { PageContainer } from "@/components/layout/page-container";
import { EntityNotFound } from "@/components/navigation/not-found-template";
import { ReleaseStageIcon } from "@/components/releases/release-stage-icon";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { InlineText } from "@/components/shared/inline-text";
import { MarkdownEditor } from "@/components/shared/markdown-editor";
import { RelativeTime } from "@/components/shared/relative-time";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import {
  releaseNoteQuery,
  releasesQuery,
  useDeleteReleaseNoteMutation,
  useSaveReleaseNoteMutation,
} from "@/lib/queries/releases";
import { useUploadAsset } from "@/lib/queries/uploads";
import { isNotFoundError } from "@/lib/server/errors";

export function ReleaseNoteDetail({ slug }: { slug: string }) {
  const { workspace } = useWorkspace();
  const { data, isPending, error, refetch } = useQuery(releaseNoteQuery(workspace.id, slug));
  if (isPending) {
    return (
      <PageContainer width="narrow">
        <ListSkeleton />
      </PageContainer>
    );
  }
  if (isNotFoundError(error)) {
    return (
      <PageContainer width="narrow">
        <EntityNotFound
          Icon={FileTextIcon}
          description="Release notes not found. They were deleted, or the link is wrong."
        />
      </PageContainer>
    );
  }
  if (error) {
    return (
      <PageContainer width="narrow">
        <ErrorCard message={error.message} onRetry={() => void refetch()} />
      </PageContainer>
    );
  }
  return <NotePage note={data} />;
}

function NotePage({ note }: { note: ReleaseNote }) {
  const { workspace, canEdit } = useWorkspace();
  const { mutate: save, mutateAsync: saveAsync } = useSaveReleaseNoteMutation(workspace.id);
  const upload = useUploadAsset(workspace.id);
  const content = note.content ?? "";
  const params = { workspace: workspace.urlKey };

  return (
    <PageContainer width="narrow">
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
                label: `${note.pipeline.name} release notes`,
                link: {
                  to: "/$workspace/releases",
                  params,
                  search: { pipeline: note.pipeline.slugId, tab: "notes" },
                },
              },
              { label: note.title },
            ]}
          />
        </div>
        {canEdit && <DeleteNote note={note} />}
      </PageBar>
      <div className="flex flex-col gap-3">
        <InlineText
          label="Title"
          value={note.title}
          maxLength={200}
          readOnly={!canEdit}
          onSave={(title) => save({ id: note.id, title })}
        />
        <CoveredReleases note={note} />
        <p className="text-muted-foreground text-xs">
          {note.creator ? `${note.creator.name} · ` : ""}updated{" "}
          <RelativeTime value={note.updatedAt} />
        </p>
      </div>
      <MarkdownEditor
        key={note.id}
        label="Release notes"
        value={content}
        onUpload={upload}
        placeholder={
          canEdit ? "What shipped, and why it matters to people…" : "Nothing written yet."
        }
        readOnly={!canEdit}
        autosave={{
          onSave: async (markdown) =>
            (await saveAsync({ id: note.id, content: markdown })).content ?? "",
        }}
      />
    </PageContainer>
  );
}

function CoveredReleases({ note }: { note: ReleaseNote }) {
  const { workspace, canEdit } = useWorkspace();
  const { data: releases = [], isPending } = useQuery(
    releasesQuery(workspace.id, note.pipeline.id),
  );
  const { mutate: save } = useSaveReleaseNoteMutation(workspace.id);
  const covered = note.releases.map((release) => release.id);
  return (
    <div className="flex flex-wrap items-center gap-2">
      {note.releases.map((release) => (
        <Link
          key={release.id}
          to="/$workspace/release/$slug"
          params={{ workspace: workspace.urlKey, slug: release.slugId }}
          className="outline-hidden focus-visible:underline"
        >
          <Badge variant="outline">
            <RocketIcon />
            {release.name}
            {release.version && (
              <span className="text-muted-foreground font-mono">{release.version}</span>
            )}
          </Badge>
        </Link>
      ))}
      {canEdit && (
        <PropertyPicker
          label="Releases covered"
          multiple
          isPending={isPending}
          emptyText="No releases in this pipeline yet"
          value={covered}
          onChange={(ids) => save({ id: note.id, releases: ids })}
          options={releases.map((release) => ({
            value: release.id,
            label: release.name,
            description: release.version ?? undefined,
            icon: release.stage ? (
              <ReleaseStageIcon type={release.stage.type} color={release.stage.color} />
            ) : undefined,
          }))}
          trigger={
            <Button variant="ghost" size="sm" className="text-muted-foreground">
              <PlusIcon className="size-4" />
              {covered.length === 0 ? "Pick the releases it covers" : "Change releases"}
            </Button>
          }
        />
      )}
    </div>
  );
}

function DeleteNote({ note }: { note: ReleaseNote }) {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const { mutateAsync: remove } = useDeleteReleaseNoteMutation(workspace.id);
  return (
    <ConfirmDialog
      trigger={
        <Button variant="ghost" size="icon" aria-label="Delete release notes">
          <Trash2Icon className="size-4" />
        </Button>
      }
      title={`Delete ${note.title}`}
      description="The releases they cover stay."
      confirmLabel="Delete release notes"
      onConfirm={async () => {
        await remove(note.id);
        await navigate({
          to: "/$workspace/releases",
          params: { workspace: workspace.urlKey },
          search: { pipeline: note.pipeline.slugId, tab: "notes" },
        });
      }}
    />
  );
}

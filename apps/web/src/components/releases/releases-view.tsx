import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";

import type { Release, ReleaseNote, ReleasePipeline } from "@/api/ops.gen";
import {
  ChevronDownIcon,
  CircleDotIcon,
  FileTextIcon,
  GitCommitHorizontalIcon,
  PlusIcon,
  RocketIcon,
  WorkflowIcon,
} from "@/components/icons/symbols.gen";
import { PropertyPicker } from "@/components/issues/property-picker";
import { EmptyState } from "@/components/layout/empty-state";
import { ErrorCard } from "@/components/layout/error-card";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { PageContainer } from "@/components/layout/page-container";
import { PageHeader } from "@/components/layout/page-header";
import { PageToolbar } from "@/components/layout/page-toolbar";
import { PipelineDialog } from "@/components/releases/pipeline-dialog";
import { ReleaseDialog } from "@/components/releases/release-dialog";
import { ReleaseStageIcon } from "@/components/releases/release-stage-icon";
import { Avatar } from "@/components/shared/avatar";
import { RelativeTime } from "@/components/shared/relative-time";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { groupByStage, releaseDateText, shortSha } from "@/lib/helpers/releases";
import {
  pipelinesQuery,
  releaseNotesQuery,
  releasesQuery,
  useSaveReleaseNoteMutation,
} from "@/lib/queries/releases";

export type TReleasesTab = "releases" | "notes";

type TProps = {
  pipeline: string | undefined;
  tab: TReleasesTab;
  onChange: (search: { pipeline?: string; tab?: TReleasesTab }) => void;
};

export function ReleasesView({ pipeline: pipelineSlug, tab, onChange }: TProps) {
  const { workspace, canEdit } = useWorkspace();
  const {
    data: pipelines = [],
    isPending,
    error,
    refetch,
  } = useQuery(pipelinesQuery(workspace.id));
  const pipeline =
    pipelines.find((item) => item.slugId === pipelineSlug) ??
    pipelines.find((item) => item.isProduction) ??
    pipelines[0];

  return (
    <PageContainer>
      <PageHeader
        title="Releases"
        icon={<RocketIcon />}
        actions={canEdit && pipeline && <HeaderAction pipeline={pipeline} tab={tab} />}
        toolbar={
          pipeline && (
            <PageToolbar
              actions={<TabSwitch tab={tab} onChange={(next) => onChange({ tab: next })} />}
            >
              <PipelineSwitch
                pipelines={pipelines}
                pipeline={pipeline}
                onChange={(slug) => onChange({ pipeline: slug })}
              />
            </PageToolbar>
          )
        }
      />
      {isPending && <ListSkeleton />}
      {!isPending && error && <ErrorCard message={error.message} onRetry={() => void refetch()} />}
      {!isPending && !error && !pipeline && (
        <EmptyState
          icon={WorkflowIcon}
          title="No release pipelines yet"
          description="A pipeline is how your work ships: continuously with every deploy, or in releases planned ahead. Releases move through its stages."
          action={
            canEdit && (
              <PipelineDialog
                trigger={
                  <Button size="sm" variant="outline">
                    <PlusIcon className="size-4" />
                    New pipeline
                  </Button>
                }
              />
            )
          }
        />
      )}
      {pipeline && tab === "releases" && <PipelineReleases pipeline={pipeline} />}
      {pipeline && tab === "notes" && <PipelineNotes pipeline={pipeline} />}
    </PageContainer>
  );
}

function HeaderAction({ pipeline, tab }: { pipeline: ReleasePipeline; tab: TReleasesTab }) {
  if (tab === "notes") return <NewNotesButton pipeline={pipeline} />;
  return (
    <ReleaseDialog
      pipeline={pipeline}
      trigger={
        <Button>
          <PlusIcon className="size-4" />
          New release
        </Button>
      }
    />
  );
}

function NewNotesButton({
  pipeline,
  variant = "default",
}: {
  pipeline: ReleasePipeline;
  variant?: "default" | "outline";
}) {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const { mutate: save, isPending } = useSaveReleaseNoteMutation(workspace.id);
  const create = () =>
    save(
      { pipeline: pipeline.id },
      {
        onSuccess: (note) =>
          navigate({
            to: "/$workspace/release-note/$slug",
            params: { workspace: workspace.urlKey, slug: note.slugId },
          }),
      },
    );
  return (
    <Button
      variant={variant}
      size={variant === "outline" ? "sm" : "default"}
      isPending={isPending}
      onClick={create}
    >
      <PlusIcon className="size-4" />
      Write release notes
    </Button>
  );
}

function PipelineSwitch({
  pipelines,
  pipeline,
  onChange,
}: {
  pipelines: ReleasePipeline[];
  pipeline: ReleasePipeline;
  onChange: (slug: string) => void;
}) {
  return (
    <PropertyPicker
      label="Pipeline"
      value={[pipeline.id]}
      onChange={([id]) => {
        const next = pipelines.find((item) => item.id === id);
        if (next) onChange(next.slugId);
      }}
      options={pipelines.map((item) => ({
        value: item.id,
        label: item.name,
        description: item.isProduction ? "Production" : undefined,
        icon: <WorkflowIcon className="text-muted-foreground size-4" />,
      }))}
      trigger={
        <Button variant="outline" size="sm" aria-label={`Pipeline: ${pipeline.name}`}>
          <WorkflowIcon className="size-4" />
          {pipeline.name}
          {pipeline.isProduction && <Badge variant="success">Production</Badge>}
          <ChevronDownIcon className="text-muted-foreground size-3.5" />
        </Button>
      }
    />
  );
}

function TabSwitch({
  tab,
  onChange,
}: {
  tab: TReleasesTab;
  onChange: (tab: TReleasesTab) => void;
}) {
  const tabs: { value: TReleasesTab; label: string }[] = [
    { value: "releases", label: "Releases" },
    { value: "notes", label: "Release notes" },
  ];
  return (
    <div role="radiogroup" aria-label="Show" className="flex items-center gap-1">
      {tabs.map((item) => (
        <Button
          key={item.value}
          role="radio"
          aria-checked={tab === item.value}
          variant={tab === item.value ? "secondary" : "ghost"}
          size="sm"
          onClick={() => onChange(item.value)}
        >
          {item.label}
        </Button>
      ))}
    </div>
  );
}

function PipelineReleases({ pipeline }: { pipeline: ReleasePipeline }) {
  const { workspace, canEdit } = useWorkspace();
  const {
    data: releases = [],
    isPending,
    error,
    refetch,
  } = useQuery(releasesQuery(workspace.id, pipeline.id));
  if (isPending) return <ListSkeleton />;
  if (error) return <ErrorCard message={error.message} onRetry={() => void refetch()} />;
  if (releases.length === 0) {
    return (
      <EmptyState
        icon={RocketIcon}
        title="No releases yet"
        description={
          pipeline.type === "continuous"
            ? "In a continuous pipeline every deploy is a release. Create one, or let your deploys create them through the API."
            : "Plan what ships next: create a release, then add the issues that go into it."
        }
        action={
          canEdit && (
            <ReleaseDialog
              pipeline={pipeline}
              trigger={
                <Button size="sm" variant="outline">
                  <PlusIcon className="size-4" />
                  New release
                </Button>
              }
            />
          )
        }
      />
    );
  }
  const groups = groupByStage(pipeline.stages ?? [], releases);
  return (
    <div className="flex w-full flex-col overflow-hidden rounded-lg border">
      {groups.map((group) => (
        <section key={group.stage?.id ?? "none"} aria-label={group.stage?.name ?? "No stage"}>
          <h2 className="bg-muted flex h-9 items-center gap-2 border-b px-3 text-sm font-medium">
            {group.stage && <ReleaseStageIcon type={group.stage.type} color={group.stage.color} />}
            <span>{group.stage?.name ?? "No stage"}</span>
            <span className="text-muted-foreground text-xs font-normal">
              {group.releases.length}
            </span>
          </h2>
          <div role="list">
            {group.releases.map((release) => (
              <ReleaseRow key={release.id} release={release} />
            ))}
          </div>
        </section>
      ))}
    </div>
  );
}

function ReleaseRow({ release }: { release: Release }) {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const open = () =>
    void navigate({
      to: "/$workspace/release/$slug",
      params: { workspace: workspace.urlKey, slug: release.slugId },
    });
  const date = releaseDateText(release);
  return (
    <div
      role="listitem"
      tabIndex={0}
      aria-label={release.name}
      onClick={open}
      onKeyDown={(event) => {
        if (event.key === "Enter") open();
      }}
      className="has-hover:hover:bg-accent focus-visible:ring-primary/8-10 flex h-9 cursor-default items-center gap-2 border-b px-3 text-sm outline-hidden focus-visible:ring-1 focus-visible:ring-inset"
    >
      {release.stage && (
        <ReleaseStageIcon
          type={release.stage.type}
          color={release.stage.color}
          name={release.stage.name}
        />
      )}
      <span className="min-w-0 truncate font-medium">{release.name}</span>
      {release.version && (
        <span className="text-muted-foreground shrink-0 font-mono text-xs">{release.version}</span>
      )}
      <span className="flex-1" />
      {release.commitSha && (
        <span className="text-muted-foreground hidden shrink-0 items-center gap-1.5 font-mono text-xs md:flex">
          <GitCommitHorizontalIcon className="size-3.5" />
          {shortSha(release.commitSha)}
        </span>
      )}
      <span className="text-muted-foreground flex w-20 shrink-0 items-center justify-end gap-1.5 text-xs">
        <CircleDotIcon className="size-3.5" />
        {release.issueCount}
      </span>
      <span className="text-muted-foreground hidden w-28 shrink-0 text-right text-xs sm:inline">
        {date}
      </span>
    </div>
  );
}

function PipelineNotes({ pipeline }: { pipeline: ReleasePipeline }) {
  const { workspace, canEdit } = useWorkspace();
  const {
    data: notes = [],
    isPending,
    error,
    refetch,
  } = useQuery(releaseNotesQuery(workspace.id, pipeline.id));
  if (isPending) return <ListSkeleton />;
  if (error) return <ErrorCard message={error.message} onRetry={() => void refetch()} />;
  if (notes.length === 0) {
    return (
      <EmptyState
        icon={FileTextIcon}
        title="No release notes yet"
        description="Tell people what shipped: write notes that cover one release or a range of them."
        action={canEdit && <NewNotesButton pipeline={pipeline} variant="outline" />}
      />
    );
  }
  return (
    <div role="list" className="flex w-full flex-col overflow-hidden rounded-lg border">
      {notes.map((note) => (
        <NoteRow key={note.id} note={note} />
      ))}
    </div>
  );
}

function NoteRow({ note }: { note: ReleaseNote }) {
  const { workspace } = useWorkspace();
  const navigate = useNavigate();
  const open = () =>
    void navigate({
      to: "/$workspace/release-note/$slug",
      params: { workspace: workspace.urlKey, slug: note.slugId },
    });
  const covered = note.releases.map((release) => release.version ?? release.name).join(", ");
  return (
    <div
      role="listitem"
      tabIndex={0}
      aria-label={note.title}
      onClick={open}
      onKeyDown={(event) => {
        if (event.key === "Enter") open();
      }}
      className="has-hover:hover:bg-accent focus-visible:ring-primary/8-10 flex h-9 cursor-default items-center gap-2 border-b px-3 text-sm outline-hidden last:border-b-0 focus-visible:ring-1 focus-visible:ring-inset"
    >
      <FileTextIcon className="text-muted-foreground size-4 shrink-0" />
      <span className="min-w-0 truncate font-medium">{note.title}</span>
      {covered && (
        <span className="text-muted-foreground hidden truncate text-xs sm:inline">{covered}</span>
      )}
      <span className="flex-1" />
      {note.creator && <Avatar id={note.creator.id} name={note.creator.name} size="xs" />}
      <span className="text-muted-foreground w-16 shrink-0 text-right text-xs">
        <RelativeTime value={note.updatedAt} />
      </span>
    </div>
  );
}

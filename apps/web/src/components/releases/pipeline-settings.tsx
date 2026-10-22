import {
  closestCenter,
  DndContext,
  type DragEndEvent,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
} from "@dnd-kit/core";
import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import type { ReleasePipeline, ReleaseStage } from "@/api/ops.gen";
import {
  ArchiveIcon,
  GripVerticalIcon,
  PencilIcon,
  PlusIcon,
  Trash2Icon,
  WorkflowIcon,
} from "@/components/icons/symbols.gen";
import { EmptyState } from "@/components/layout/empty-state";
import { ErrorCard } from "@/components/layout/error-card";
import { List } from "@/components/layout/list";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { SettingsSection } from "@/components/layout/settings-section";
import { PipelineDialog } from "@/components/releases/pipeline-dialog";
import { ReleaseStageIcon } from "@/components/releases/release-stage-icon";
import { DeleteStageDialog, StageDialog } from "@/components/releases/stage-dialogs";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { isStageType, stageTypeLabels } from "@/lib/helpers/releases";
import {
  pipelinesQuery,
  useDeletePipelineMutation,
  useSaveStageMutation,
} from "@/lib/queries/releases";

export function NewPipelineButton({ variant = "default" }: { variant?: "default" | "outline" }) {
  return (
    <PipelineDialog
      trigger={
        <Button size="sm" variant={variant}>
          <PlusIcon className="size-4" />
          New pipeline
        </Button>
      }
    />
  );
}

export function PipelineSettings() {
  const { workspace } = useWorkspace();
  const {
    data: pipelines = [],
    isPending,
    error,
    refetch,
  } = useQuery(pipelinesQuery(workspace.id));
  if (isPending) return <ListSkeleton rows={4} />;
  if (error) return <ErrorCard message={error.message} onRetry={() => void refetch()} />;
  if (pipelines.length === 0) {
    return (
      <EmptyState
        icon={WorkflowIcon}
        title="No release pipelines yet"
        description="A pipeline is how a product ships, like your web app or mobile app. Its releases move through stages from planned to released."
        action={<NewPipelineButton variant="outline" />}
      />
    );
  }
  return pipelines.map((pipeline) => <PipelineSection key={pipeline.id} pipeline={pipeline} />);
}

function pipelineSummary(pipeline: ReleasePipeline) {
  return [
    pipeline.type === "continuous" ? "Every deploy is a release" : "Releases are planned ahead",
    `${pipeline.releaseCount} ${pipeline.releaseCount === 1 ? "release" : "releases"}`,
  ].join(" · ");
}

function PipelineSection({ pipeline }: { pipeline: ReleasePipeline }) {
  const { workspace } = useWorkspace();
  const { mutateAsync: archive } = useDeletePipelineMutation(workspace.id);
  const { mutate: saveStage } = useSaveStageMutation(workspace.id);
  const stages = [...(pipeline.stages ?? [])].sort((a, b) => a.position - b.position);
  const [dragOrder, setDragOrder] = useState<string[] | null>(null);
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );
  const ordered = dragOrder
    ? dragOrder
        .map((id) => stages.find((stage) => stage.id === id))
        .filter((stage): stage is ReleaseStage => !!stage)
    : stages;

  const onDragEnd = (event: DragEndEvent) => {
    const { active, over } = event;
    if (!over || active.id === over.id) return;
    const ids = ordered.map((stage) => stage.id);
    const next = arrayMove(ids, ids.indexOf(String(active.id)), ids.indexOf(String(over.id)));
    const index = next.indexOf(String(active.id));
    const before = ordered.find((stage) => stage.id === next[index - 1]);
    const after = ordered.find((stage) => stage.id === next[index + 1]);
    setDragOrder(next);
    saveStage(
      { id: String(active.id), position: between(before?.position, after?.position) },
      { onSettled: () => setDragOrder(null) },
    );
  };

  return (
    <SettingsSection
      title={pipeline.name}
      description={pipelineSummary(pipeline)}
      actions={
        <>
          {pipeline.isProduction && <Badge variant="success">Production</Badge>}
          <StageDialog
            pipeline={pipeline}
            trigger={
              <Button variant="ghost" size="icon" aria-label={`Add a stage to ${pipeline.name}`}>
                <PlusIcon className="size-4" />
              </Button>
            }
          />
          <PipelineDialog
            pipeline={pipeline}
            trigger={
              <Button variant="ghost" size="icon" aria-label={`Edit ${pipeline.name}`}>
                <PencilIcon className="size-4" />
              </Button>
            }
          />
          <ConfirmDialog
            trigger={
              <Button variant="ghost" size="icon" aria-label={`Archive ${pipeline.name}`}>
                <ArchiveIcon className="size-4" />
              </Button>
            }
            title={`Archive ${pipeline.name}`}
            description="It leaves the releases page. Its releases and notes stay readable."
            confirmLabel="Archive pipeline"
            onConfirm={() => archive(pipeline.id)}
          />
        </>
      }
    >
      <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={onDragEnd}>
        <SortableContext
          items={ordered.map((stage) => stage.id)}
          strategy={verticalListSortingStrategy}
        >
          <List>
            {ordered.map((stage) => (
              <StageRow key={stage.id} pipeline={pipeline} stage={stage} />
            ))}
          </List>
        </SortableContext>
      </DndContext>
    </SettingsSection>
  );
}

function between(before: number | undefined, after: number | undefined) {
  if (before === undefined && after === undefined) return 0;
  if (before === undefined) return (after ?? 0) - 1;
  if (after === undefined) return before + 1;
  return (before + after) / 2;
}

function StageRow({ pipeline, stage }: { pipeline: ReleasePipeline; stage: ReleaseStage }) {
  const sortable = useSortable({ id: stage.id });
  const { transform, transition, isDragging } = sortable;
  const typeLabel = isStageType(stage.type) ? stageTypeLabels[stage.type] : stage.type;
  return (
    <div
      ref={sortable.setNodeRef}
      role="listitem"
      data-dragging={isDragging || undefined}
      style={{
        transform: transform ? `translate3d(${transform.x}px, ${transform.y}px, 0)` : undefined,
        transition,
      }}
      className="bg-card data-dragging:bg-accent relative flex h-9 w-full items-center gap-2 border-b px-2 text-sm last:border-b-0 data-dragging:z-10"
    >
      <Button
        ref={sortable.setActivatorNodeRef}
        variant="ghost"
        size="icon"
        aria-label={`Reorder ${stage.name}`}
        className="text-muted-foreground cursor-grab"
        {...sortable.attributes}
        {...sortable.listeners}
      >
        <GripVerticalIcon className="size-4" />
      </Button>
      <ReleaseStageIcon type={stage.type} color={stage.color} />
      <span className="truncate">{stage.name}</span>
      {typeLabel.toLowerCase() !== stage.name.toLowerCase() && (
        <span className="text-muted-foreground hidden text-xs sm:inline">{typeLabel}</span>
      )}
      <div className="ml-auto flex shrink-0 items-center gap-2">
        <StageDialog
          pipeline={pipeline}
          stage={stage}
          trigger={
            <Button variant="ghost" size="icon" aria-label={`Edit ${stage.name}`}>
              <PencilIcon className="size-4" />
            </Button>
          }
        />
        <DeleteStageDialog
          pipeline={pipeline}
          stage={stage}
          trigger={
            <Button variant="ghost" size="icon" aria-label={`Delete ${stage.name}`}>
              <Trash2Icon className="size-4" />
            </Button>
          }
        />
      </div>
    </div>
  );
}

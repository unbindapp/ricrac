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
import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { GripVerticalIcon, PencilIcon, PlusIcon, WorkflowIcon } from "lucide-react";

import type { IssueStatus, Team } from "@/api/ops.gen";
import { EmptyState } from "@/components/layout/empty-state";
import { ErrorCard } from "@/components/layout/error-card";
import { List } from "@/components/layout/list";
import { ListSkeleton } from "@/components/layout/list-skeleton";
import { SettingsPage } from "@/components/layout/settings-page";
import { SettingsSection } from "@/components/layout/settings-section";
import { DeleteWorkflowStateDialog } from "@/components/settings/delete-workflow-state-dialog";
import { WorkflowStateDialog } from "@/components/settings/workflow-state-dialog";
import {
  STATE_TYPES,
  StateIcon,
  stateTypeLabels,
  type TStateType,
} from "@/components/shared/state-icon";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useSettingsTeam } from "@/components/settings/use-settings-team";
import { useWorkspace } from "@/components/workspace/workspace-provider";
import { teamStatusesQuery, useSaveIssueStatusMutation } from "@/lib/queries/teams";

export const Route = createFileRoute(
  "/$workspace/_workspace/settings/_workspace-settings/workflow",
)({
  component: WorkflowSettingsPage,
});

const typeDescriptions: Record<TStateType, string> = {
  triage: "New issues waiting to be reviewed.",
  backlog: "Issues nobody has planned yet.",
  unstarted: "Planned issues that have not started.",
  started: "Work in progress.",
  completed: "Finished issues.",
  canceled: "Issues that will not be done.",
};

function byPosition(a: IssueStatus, b: IssueStatus) {
  return a.position - b.position;
}

function WorkflowSettingsPage() {
  const team = useSettingsTeam();
  const { workspace } = useWorkspace();
  const {
    data: states = [],
    isPending,
    error,
    refetch,
  } = useQuery(teamStatusesQuery(workspace.id, team.id));
  const types = STATE_TYPES.filter(
    (type) => type !== "triage" || team.triageEnabled || states.some((s) => s.type === "triage"),
  );

  return (
    <SettingsPage
      title="Workflow"
      description="The statuses an issue moves through. Drag to reorder within a group."
    >
      {isPending && <ListSkeleton />}
      {!isPending && error && <ErrorCard message={error.message} onRetry={() => void refetch()} />}
      {!isPending && !error && states.length === 0 && (
        <EmptyState icon={WorkflowIcon} title="No statuses yet" />
      )}
      {!isPending &&
        !error &&
        states.length > 0 &&
        types.map((type) => (
          <StateGroup
            key={type}
            team={team}
            type={type}
            states={states.filter((state) => state.type === type).sort(byPosition)}
            allStates={states}
          />
        ))}
    </SettingsPage>
  );
}

function StateGroup({
  team,
  type,
  states,
  allStates,
}: {
  team: Team;
  type: TStateType;
  states: IssueStatus[];
  allStates: IssueStatus[];
}) {
  const { workspace } = useWorkspace();
  const [dragOrder, setDragOrder] = useState<string[] | null>(null);
  const { mutate: saveStatus } = useSaveIssueStatusMutation(workspace.id);
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );
  const ordered = dragOrder
    ? dragOrder
        .map((id) => states.find((state) => state.id === id))
        .filter((state): state is IssueStatus => !!state)
    : states;

  const onDragEnd = (event: DragEndEvent) => {
    const { active, over } = event;
    if (!over || active.id === over.id) return;
    const ids = ordered.map((state) => state.id);
    const next = arrayMove(ids, ids.indexOf(String(active.id)), ids.indexOf(String(over.id)));
    const index = next.indexOf(String(active.id));
    const before = ordered.find((state) => state.id === next[index - 1]);
    const after = ordered.find((state) => state.id === next[index + 1]);
    setDragOrder(next);
    saveStatus(
      { id: String(active.id), position: midpoint(before?.position, after?.position) },
      { onSettled: () => setDragOrder(null) },
    );
  };

  return (
    <SettingsSection
      title={stateTypeLabels[type]}
      description={typeDescriptions[type]}
      actions={
        <WorkflowStateDialog
          mode="create"
          teamId={team.id}
          type={type}
          trigger={
            <Button
              variant="ghost"
              size="icon"
              aria-label={`Add ${stateTypeLabels[type].toLowerCase()} status`}
            >
              <PlusIcon className="size-4" />
            </Button>
          }
        />
      }
    >
      {ordered.length === 0 ? (
        <p className="text-muted-foreground text-sm">No statuses of this type.</p>
      ) : (
        <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={onDragEnd}>
          <SortableContext
            items={ordered.map((state) => state.id)}
            strategy={verticalListSortingStrategy}
          >
            <List>
              {ordered.map((state) => (
                <SortableStateRow
                  key={state.id}
                  state={state}
                  allStates={allStates}
                  isDefault={state.isDefault}
                />
              ))}
            </List>
          </SortableContext>
        </DndContext>
      )}
    </SettingsSection>
  );
}

function midpoint(before: number | undefined, after: number | undefined) {
  if (before === undefined && after === undefined) return 0;
  if (before === undefined) return (after ?? 0) - 1;
  if (after === undefined) return before + 1;
  return (before + after) / 2;
}

function SortableStateRow({
  state,
  allStates,
  isDefault,
}: {
  state: IssueStatus;
  allStates: IssueStatus[];
  isDefault: boolean;
}) {
  const {
    attributes,
    listeners,
    setNodeRef,
    setActivatorNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: state.id });
  return (
    <div
      ref={setNodeRef}
      role="listitem"
      data-dragging={isDragging || undefined}
      style={{
        transform: transform ? `translate3d(${transform.x}px, ${transform.y}px, 0)` : undefined,
        transition,
      }}
      className="bg-card data-dragging:bg-accent relative flex h-9 w-full items-center gap-2 border-b px-2 text-sm last:border-b-0 data-dragging:z-10"
    >
      <Button
        ref={setActivatorNodeRef}
        variant="ghost"
        size="icon"
        aria-label={`Reorder ${state.name}`}
        className="text-muted-foreground cursor-grab"
        {...attributes}
        {...listeners}
      >
        <GripVerticalIcon className="size-4" />
      </Button>
      <StateIcon type={state.type} color={state.color} />
      <span className="truncate">{state.name}</span>
      {state.description && (
        <span className="text-muted-foreground hidden truncate text-xs sm:inline">
          {state.description}
        </span>
      )}
      {isDefault && <Badge variant="muted">Default</Badge>}
      <div className="ml-auto flex shrink-0 items-center gap-2">
        <WorkflowStateDialog
          mode="edit"
          state={state}
          trigger={
            <Button variant="ghost" size="icon" aria-label={`Edit ${state.name}`}>
              <PencilIcon className="size-4" />
            </Button>
          }
        />
        <DeleteWorkflowStateDialog state={state} states={allStates} />
      </div>
    </div>
  );
}
